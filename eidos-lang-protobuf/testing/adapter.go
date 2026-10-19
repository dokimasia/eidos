// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/bufbuild/protocompile/experimental/incremental"
	"github.com/bufbuild/protocompile/experimental/incremental/queries"
	"github.com/bufbuild/protocompile/experimental/ir"
	"github.com/bufbuild/protocompile/experimental/report"
	"github.com/bufbuild/protocompile/experimental/source"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/internal/parse"
	"go.dokimi.dev/eidos/sdk/symbol"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// scratchPattern names the scratch directory of a laid-out project.
const scratchPattern = "eidos-protobuf-*"

// The refusals of the adapter: a project without a proto file, and the two
// operations that a schema language does not have.
var (
	errNoProto     = errors.New("testing: the laid-out project has no proto file")
	errNoTests     = errors.New("testing: a protobuf file declares no test, so protobuf runs none")
	errNoContracts = errors.New("testing: a protobuf message implements no contract, so protobuf checks none")
)

// adapter parses and compiles generated proto files in process. It has no
// state, so one value serves every check.
type adapter struct{}

// protoFile is one proto file of a laid-out project: its path below the
// directory of the project with slashes, which is its import path, and its
// text.
type protoFile struct {
	path string
	text string
}

// New returns the protobuf toolchain adapter.
func New() toolchain.Adapter { return adapter{} }

// Lang returns [protobuf.Lang], with which the messages of the assertions
// of the kernel start.
func (adapter) Lang() symbol.Lang { return protobuf.Lang }

// Available reports true with no reason, because the parse and the compile
// run in process on every machine.
func (adapter) Available() (bool, string) { return true, "" }

// Layout writes the generated output as a source tree in a scratch
// directory, with every file at its own path. The path of a file below the
// directory is its import path. A write that fails, such as the write of a
// path that leaves the directory, returns an error and removes the
// directory.
func (adapter) Layout(g toolchain.Generated) (string, error) {
	return toolchain.Scratch(scratchPattern, g, nil)
}

// Parse reads the syntax of every proto file of the project through the
// parser of the protobuf frontend. It returns an error that joins one error
// for each syntax error, with its file and its line, and the error of the
// context once the context ends. A project without a proto file returns an
// error, because a parse of nothing proves nothing.
func (adapter) Parse(ctx context.Context, dir string) error {
	files, err := protoFiles(dir)
	if err != nil {
		return err
	}
	var errs []error
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("testing: %w", err)
		}
		_, r, _ := parse.File(f.path, f.text)
		errs = append(errs, refusals(r)...)
	}
	return errors.Join(errs...)
}

// TypeCheck compiles every proto file of the project with the experimental
// compiler of protocompile, which links the references of the files and
// interprets their options. An import of a well-known type reads the
// sources that protocompile includes. It returns an error that joins one
// error for each error of the compiler, with its file and its line, and
// the error of the context where the context ends before the compile does.
// A project without a proto file returns an error.
func (adapter) TypeCheck(ctx context.Context, dir string) error {
	files, err := protoFiles(dir)
	if err != nil {
		return err
	}
	texts := make(map[string]*source.File, len(files))
	paths := make([]string, 0, len(files))
	for _, f := range files {
		text, _ := parse.Text(f.path, f.text)
		texts[f.path] = source.NewFile(f.path, text)
		paths = append(paths, f.path)
	}
	exec := incremental.New(incremental.WithReportOptions(report.Options{SuppressWarnings: true}))
	_, r, err := incremental.Run(ctx, exec, queries.Link{
		Opener:    &source.Openers{source.NewMap(texts), source.WKTs()},
		Session:   new(ir.Session),
		Workspace: source.NewWorkspace(paths...),
	})
	if err != nil {
		return fmt.Errorf("testing: %w", err)
	}
	return errors.Join(refusals(r)...)
}

// RunTests returns an error, because a protobuf file declares no test.
func (adapter) RunTests(context.Context, string) (toolchain.TestReport, error) {
	return toolchain.TestReport{}, errNoTests
}

// Satisfies returns an error, because a protobuf message implements no
// contract.
func (adapter) Satisfies(context.Context, string, string, string) (bool, error) {
	return false, errNoContracts
}

// protoFiles returns every proto file below dir in lexical order, with its
// path below dir with slashes. It returns an error for a directory that
// does not read, for a file that does not read and for a directory without
// a proto file.
func protoFiles(dir string) ([]protoFile, error) {
	tree := os.DirFS(dir)
	var files []protoFile
	err := fs.WalkDir(tree, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, protobuf.Extension) {
			return err
		}
		text, err := fs.ReadFile(tree, path)
		files = append(files, protoFile{path: path, text: string(text)})
		return err
	})
	switch {
	case err != nil:
		return nil, fmt.Errorf("testing: %w", err)
	case len(files) == 0:
		return nil, errNoProto
	}
	return files, nil
}

// refusals returns one error for each diagnostic of r, with the file, the
// line and the message of the diagnostic. The parse and the compile record
// no warning, so each diagnostic is an error.
func refusals(r *report.Report) []error {
	errs := make([]error, 0, len(r.Diagnostics))
	for i := range r.Diagnostics {
		d := &r.Diagnostics[i]
		errs = append(errs, fmt.Errorf("testing: %s:%d: %s", d.File(), d.Primary().StartLoc().Line, d.Message()))
	}
	return errs
}
