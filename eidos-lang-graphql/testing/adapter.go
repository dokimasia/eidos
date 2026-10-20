// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"

	"github.com/dokimasia/gqlparser/v2/ast"
	"github.com/dokimasia/gqlparser/v2/gqlerror"
	"github.com/dokimasia/gqlparser/v2/validator"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/internal/document"
	"go.dokimi.dev/eidos/sdk/symbol"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// scratchPattern names the scratch directory of a laid-out project.
const scratchPattern = "eidos-graphql-*"

// extensions are the extensions of the GraphQL files that the frontend
// claims.
var extensions = []string{graphql.Extension, graphql.SchemaExtension, graphql.ShortExtension}

// The refusals of the adapter: a project without a GraphQL file, the run of
// tests that GraphQL does not declare, and a question about a type that the
// schema does not declare.
var (
	errNoGraphQL = errors.New("testing: the laid-out project has no GraphQL file")
	errNoTests   = errors.New("testing: a GraphQL document declares no test, so GraphQL runs none")
	errNoType    = errors.New("testing: the schema does not declare the type")
)

// adapter parses and validates generated GraphQL files in process. It has
// no state, so one value serves every check.
type adapter struct{}

// project is the parse of the GraphQL files of a laid-out project: the
// documents of the type system, the executable documents, and one error for
// each syntax error of the lexer and of the parser.
type project struct {
	schemas []*ast.SchemaDocument
	queries []*ast.QueryDocument
	errs    []error
}

// New returns the GraphQL toolchain adapter.
func New() toolchain.Adapter { return adapter{} }

// Lang returns [graphql.Lang], with which the messages of the assertions of
// the kernel start.
func (adapter) Lang() symbol.Lang { return graphql.Lang }

// Available reports true with no reason, because the parse and the
// validation run in process on every machine.
func (adapter) Available() (bool, string) { return true, "" }

// Layout writes the generated output as a source tree in a scratch
// directory, with every file at its own path. A write that fails, such as
// the write of a path that leaves the directory, returns an error and
// removes the directory.
func (adapter) Layout(g toolchain.Generated) (string, error) {
	return toolchain.Scratch(scratchPattern, g, nil)
}

// Parse reads the syntax of every GraphQL file of the project. A file can
// mix the type system and executable definitions, so Parse parses each file
// as two documents, as the frontend does. It returns an error that joins one
// error for each syntax error of the lexer and of the parser, with its file,
// its line and its column, and the error of the context once the context
// ends. A project without a GraphQL file returns an error, because a parse
// of nothing proves nothing.
func (adapter) Parse(ctx context.Context, dir string) error {
	p, err := parseProject(ctx, dir)
	if err != nil {
		return err
	}
	return errors.Join(p.errs...)
}

// TypeCheck loads the schema of the project from the type system of every
// GraphQL file over the prelude of gqlparser, which validates the schema
// with the built-in scalars and directives of the specification. It then
// validates the executable definitions of every file against the schema as
// a client validates them. The validator reads the files together, so a
// spread can name a fragment of another file, and an anonymous operation is
// the only operation of its file.
//
// TypeCheck returns an error that joins the syntax errors of a project that
// does not parse. It returns the error of a schema that does not load, and
// otherwise an error that joins one error for each error of the validation.
// Each error contains its file, its line and its column. TypeCheck returns
// the error of the context where the context ends before the parse does. A
// project without a GraphQL file returns an error.
func (adapter) TypeCheck(ctx context.Context, dir string) error {
	p, err := parseProject(ctx, dir)
	if err != nil {
		return err
	}
	if len(p.errs) > 0 {
		return errors.Join(p.errs...)
	}
	schema, err := loadSchema(p)
	if err != nil {
		return err
	}
	merged := &ast.QueryDocument{}
	var errs []error
	for _, q := range p.queries {
		merged.Operations = append(merged.Operations, q.Operations...)
		merged.Fragments = append(merged.Fragments, q.Fragments...)
		for _, op := range q.Operations {
			if op.Name == "" && len(q.Operations) > 1 {
				errs = append(errs, fmt.Errorf("testing: %s:%d:%d: %s", op.Position.Src.Name, op.Position.Line,
					op.Position.Column, document.LoneAnonymous))
			}
		}
	}
	for _, e := range validator.ValidateWithRulesWithSources(schema, merged, document.Rules()) {
		errs = append(errs, fmt.Errorf("testing: %w", e))
	}
	return errors.Join(errs...)
}

// RunTests returns an error, because a GraphQL document declares no test.
func (adapter) RunTests(context.Context, string) (toolchain.TestReport, error) {
	return toolchain.TestReport{}, errNoTests
}

// Satisfies reports whether the schema of the project lists the contract
// for the type typeName in [ast.Schema.Implements]. The list contains the
// interfaces that an object type or an interface implements, and the unions
// that an object type is a member of. Satisfies returns an error for a
// project that does not parse, for a schema that does not load, and for a
// type or a contract that the schema does not declare.
func (adapter) Satisfies(ctx context.Context, dir, typeName, contract string) (bool, error) {
	p, err := parseProject(ctx, dir)
	if err != nil {
		return false, err
	}
	if len(p.errs) > 0 {
		return false, errors.Join(p.errs...)
	}
	schema, err := loadSchema(p)
	if err != nil {
		return false, err
	}
	for _, name := range [...]string{typeName, contract} {
		if schema.Types[name] == nil {
			return false, fmt.Errorf("%w %s", errNoType, name)
		}
	}
	return slices.ContainsFunc(schema.Implements[typeName], func(d *ast.Definition) bool {
		return d.Name == contract
	}), nil
}

// parseProject parses every GraphQL file below dir in lexical order,
// through the split of the type system and the executable definitions that
// the frontend uses. It returns an error for a directory or a file that does
// not read, for a directory without a GraphQL file, and the error of the
// context once the context ends.
func parseProject(ctx context.Context, dir string) (project, error) {
	tree := os.DirFS(dir)
	var p project
	found := false
	err := fs.WalkDir(tree, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !slices.Contains(extensions, path.Ext(name)) {
			return err
		}
		if cause := ctx.Err(); cause != nil {
			return cause
		}
		text, err := fs.ReadFile(tree, name)
		if err != nil {
			return err
		}
		found = true
		src := &ast.Source{Name: name, Input: string(text)}
		lx := document.Lex(src)
		defs := lx.Definitions()
		for _, e := range lx.Errors {
			p.errs = append(p.errs, fmt.Errorf("testing: %s:%d:%d: %s", name, e.At.Line, e.At.Column, e.Message))
		}
		schema, schemaErrs := document.ParseSchema(src, lx, defs)
		query, queryErrs := document.ParseQuery(src, lx, defs)
		for _, e := range append(schemaErrs, queryErrs...) {
			p.errs = append(p.errs, fmt.Errorf("testing: %s:%d:%d: %s", e.At.File, e.At.Line, e.At.Col, e.Message))
		}
		if schema != nil {
			p.schemas = append(p.schemas, schema)
		}
		if query != nil {
			p.queries = append(p.queries, query)
		}
		return nil
	})
	switch {
	case err != nil:
		return project{}, fmt.Errorf("testing: %w", err)
	case !found:
		return project{}, errNoGraphQL
	}
	return p, nil
}

// loadSchema loads the schema of the project p over the prelude of
// gqlparser. For a schema that does not load, it returns an error that
// wraps the error of gqlparser, which contains the file, the line and the
// column of the fault.
func loadSchema(p project) (*ast.Schema, error) {
	schema, err := document.Load(p.schemas)
	if err != nil {
		cause := err
		if ge, ok := errors.AsType[*gqlerror.Error](err); ok {
			cause = ge
		}
		return nil, fmt.Errorf("testing: %w", cause)
	}
	return schema, nil
}
