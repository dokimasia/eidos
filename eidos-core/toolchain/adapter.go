// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

import (
	"context"
	"io/fs"
	"maps"

	"go.dokimi.dev/eidos/core/symbol"
)

// Generated is the output of a run, as an assertion receives it. It
// contains the rendered files and what a compiler needs beside them to read
// them.
//
// A fixture is a value. An adapter lays it out on disk, through [Scratch]
// with the project files that its language needs.
type Generated struct {
	// Files are the rendered outputs, keyed by their slash-separated paths
	// in the output tree.
	Files map[string][]byte
	// Sources is the tree that the run read, for a language whose
	// generated output refers to it. [Prepare] adds each file of Sources
	// that Files does not contain. A fixture whose output does not refer
	// to its sources leaves it nil.
	Sources fs.FS
	// Module is the module or the namespace that the laid-out project
	// declares, for a language that needs one to resolve its own imports.
	Module string
}

// IsEmpty reports whether the fixture has no output. Every assertion
// refuses an empty fixture, because a toolchain run over nothing passes and
// proves nothing.
func (g Generated) IsEmpty() bool { return len(g.Files) == 0 }

// laidOut returns the files of a project of g: every file of the sources
// of g at its path in the sources, and every generated file, which
// replaces a source file at the same path. It returns g.Files itself
// where g has no sources, and the error of a source that does not read.
func (g Generated) laidOut() (map[string][]byte, error) {
	if g.Sources == nil {
		return g.Files, nil
	}
	laid := maps.Clone(g.Files)
	err := fs.WalkDir(g.Sources, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if _, generated := laid[path]; generated {
			return nil
		}
		laid[path], err = fs.ReadFile(g.Sources, path)
		return err
	})
	return laid, err
}

// TestReport is the result of a run of the tests of a language.
type TestReport struct {
	// Passed, Failed and Skipped count the cases that the run reported.
	Passed, Failed, Skipped int
	// Output is the text of the run, so a failure message can quote the
	// toolchain.
	Output string
}

// OK reports whether at least one case ran and every case that ran passed.
// A report of no case is not a pass, for the same reason as an empty
// fixture.
func (r TestReport) OK() bool { return r.Failed == 0 && r.Passed > 0 }

// Adapter is the interface that one language implements over its own
// toolchain.
//
// Every method takes or returns a plain value, so the kernel drives a
// compiler that it knows nothing about. An operation that the tooling of
// the language cannot perform returns an error, and the assertion reports
// the error as a failure. The kernel does not guess at a missing
// capability.
//
// Parse, TypeCheck, RunTests and Satisfies take the context of the caller
// first. When the context ends, the adapter stops the process of the
// toolchain and returns an error that wraps the error of the context.
type Adapter interface {
	// Lang returns the language. The messages of the assertions start
	// with it.
	Lang() symbol.Lang
	// Available reports whether the toolchain is on this machine, and the
	// reason where it is absent. A local skip records the reason, and a
	// failure in CI contains it.
	Available() (bool, string)
	// Layout writes a scratch project that the toolchain accepts and
	// returns its directory. The caller removes the directory.
	Layout(g Generated) (dir string, err error)
	// Parse reads the syntax of the project and nothing else, so a syntax
	// error is told apart from a type error.
	Parse(ctx context.Context, dir string) error
	// TypeCheck checks the project against the type rules of its language.
	TypeCheck(ctx context.Context, dir string) error
	// RunTests runs the tests of the project.
	RunTests(ctx context.Context, dir string) (TestReport, error)
	// Satisfies reports whether one type meets one contract, such as an
	// interface, a trait or a protocol.
	Satisfies(ctx context.Context, dir, typeName, contract string) (bool, error)
}
