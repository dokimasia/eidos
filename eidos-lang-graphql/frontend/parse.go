// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/validator"

	"go.dokimi.dev/eidos/lang/graphql/internal/document"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// fileExtension is the extension of an error of gqlparser that names the
// file of the error.
const fileExtension = "file"

// parsed is one own file of a unit after the parse: its lowering and the
// documents that its two copies parse into.
type parsed struct {
	src    *source
	schema *ast.SchemaDocument
	query  *ast.QueryDocument
}

// parse loads one unit. It reads the configurations among the shared inputs
// of the unit, which decide the package of the schema and the shared
// packages, and reports a configuration that the frontend does not read
// under [UnreadConfig]. It parses each own file from two copies, one for the
// type system and one for the executable definitions, lowers the type
// system into the package of the schema, loads the schema of the unit
// through gqlparser, validates the documents against it, and lowers the
// executable definitions into the package of the documents.
//
// A syntax error reports under [UnparsedFile], and the file contributes
// every definition outside the definitions with an error. A schema that
// does not load reports its error under [InvalidDocument], and the
// documents of the unit load without types. An error of the validator in a
// document reports under [InvalidDocument].
//
// The depth does not change the result. A schema has no bodies and no
// unexported names, so [plugin.DepthSignatures] and [plugin.DepthFull]
// load the same declarations.
//
// Error modes: parse returns the error of a context that is done, and the
// error of a member or a shared input that does not read.
func parse(ctx context.Context, u *plugin.SourceUnit) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	files := u.Files()
	w, sharedFiles, err := readShared(u, files[0].Shared)
	if err != nil {
		return err
	}
	l := newLowering(u, w, files[0].Path, sharedFiles)

	units := make([]parsed, 0, len(files))
	for _, ref := range files {
		b, err := u.Read(ref.Path)
		if err != nil {
			return err
		}
		src := &ast.Source{Name: ref.Path, Input: string(b)}
		lx := document.Lex(src)
		defs := lx.Definitions()
		p := parsed{src: l.newSource(ref.Path, lx, defs)}
		var errs []document.SyntaxError
		p.schema, errs = document.ParseSchema(src, lx, defs)
		var queryErrs []document.SyntaxError
		p.query, queryErrs = document.ParseQuery(src, lx, defs)
		reportSyntax(u, lx, append(errs, queryErrs...))
		units = append(units, p)
	}
	docs := make([]*ast.SchemaDocument, 0, len(units)+len(sharedFiles))
	for _, p := range units {
		if p.schema != nil {
			docs = append(docs, p.schema)
		}
	}
	for _, f := range sharedFiles {
		b, err := u.Read(f)
		if err != nil {
			return err
		}
		src := &ast.Source{Name: f, Input: string(b)}
		lx := document.Lex(src)
		if doc, _ := document.ParseSchema(src, lx, lx.Definitions()); doc != nil {
			docs = append(docs, doc)
		}
	}

	var exts []extension
	var statements []statement
	for _, p := range units {
		if p.schema != nil || p.query == nil {
			p.src.schemaFile()
		}
		if p.schema == nil {
			continue
		}
		p.src.typeSystem(p.schema)
		for _, d := range p.schema.Extensions {
			exts = append(exts, extension{src: p.src, def: d})
		}
		for _, d := range p.schema.Schema {
			statements = append(statements, statement{src: p.src, def: d})
		}
		for _, d := range p.schema.SchemaExtension {
			statements = append(statements, statement{src: p.src, def: d, extension: true})
		}
	}
	l.extensions(exts)
	l.schemaStatements(statements)
	loaded := l.load(docs, units)
	l.operations(loaded, statements)
	l.documents(units, loaded)

	for _, p := range units {
		p.src.notes.sweep()
	}
	sortDecls(l.pkg.Files)
	if l.docs != nil {
		sortDecls(l.docs.Files)
	}
	return nil
}

// reportSyntax reports the errors of the lexer and of the parser of one
// file under [UnparsedFile], in the order of the file.
func reportSyntax(u *plugin.SourceUnit, lx *document.Lexed, errs []document.SyntaxError) {
	for _, e := range lx.Errors {
		errs = append(errs, document.SyntaxError{
			At:      position.Pos{File: lx.Src.Name, Line: e.At.Line, Col: e.At.Column},
			Message: e.Message,
		})
	}
	if len(errs) == 0 {
		return
	}
	slices.SortStableFunc(errs, func(a, b document.SyntaxError) int {
		return cmp.Or(cmp.Compare(a.At.Line, b.At.Line), cmp.Compare(a.At.Col, b.At.Col))
	})
	u.ReportSyntax(UnparsedFile, func(yield func(position.Pos, string) bool) {
		// The first false result of yield ends the calls, as the protocol of
		// an iterator requires.
		more := true
		for _, e := range errs {
			more = more && yield(e.At, e.Message)
		}
	})
}

// errorAt returns the position of the first location of an error of the
// validator, in the file of its source. An error without a location is at
// the start of the file first.
func errorAt(e *gqlerror.ErrorWithSources, first string) position.Pos {
	at := position.Pos{File: first, Line: 1, Col: 1}
	if len(e.Locations) > 0 {
		loc := e.Locations[0]
		at.Line, at.Col = loc.Line, loc.Column
		if loc.Source != nil {
			at.File = loc.Source.Name
		}
	}
	return at
}

// load loads the schema of the unit through [document.Load] from the
// documents of the type system of the own files and of the shared files,
// after the lowering read them. load reports an error of the schema in an
// own file under [InvalidDocument], with the message of gqlparser, and
// returns nil for a schema that does not load. An error without a file, or
// in a shared file, reports at the first own file.
func (l *lowering) load(docs []*ast.SchemaDocument, units []parsed) *ast.Schema {
	schema, err := document.Load(docs)
	if err == nil {
		return schema
	}
	at := position.Pos{File: units[0].src.path, Line: 1, Col: 1}
	if ge, ok := errors.AsType[*gqlerror.Error](err); ok {
		file, _ := ge.Extensions[fileExtension].(string)
		if len(ge.Locations) > 0 && slices.ContainsFunc(units, func(p parsed) bool { return p.src.path == file }) {
			at = position.Pos{File: file, Line: ge.Locations[0].Line, Col: ge.Locations[0].Column}
		}
		err = errors.New(ge.Message)
	}
	l.unit.Errorf(InvalidDocument, at, "the schema does not load, so the documents load without types: %v", err)
	return nil
}

// documents validates the executable definitions of the unit against its
// schema under [document.Rules] and lowers them into the package of the
// documents. The validator reads the operations and the fragments of every
// own file together, so a spread reaches a fragment of another file. Each
// file has at most one anonymous operation beside no other operation, which
// the frontend checks for each file itself.
func (l *lowering) documents(units []parsed, loaded *ast.Schema) {
	merged := &ast.QueryDocument{}
	fragments := map[string]*ast.FragmentDefinition{}
	for _, p := range units {
		if p.query == nil {
			continue
		}
		merged.Operations = append(merged.Operations, p.query.Operations...)
		merged.Fragments = append(merged.Fragments, p.query.Fragments...)
		for _, fr := range p.query.Fragments {
			if _, held := fragments[fr.Name]; !held {
				fragments[fr.Name] = fr
			}
		}
		for _, op := range p.query.Operations {
			if op.Name == "" && len(p.query.Operations) > 1 {
				l.unit.Errorf(InvalidDocument, p.src.at(op.Position), "%s", document.LoneAnonymous)
			}
		}
	}
	if len(merged.Operations)+len(merged.Fragments) == 0 {
		return
	}
	if loaded != nil {
		for _, e := range validator.ValidateWithRulesWithSources(loaded, merged, document.Rules()) {
			l.unit.Errorf(InvalidDocument, errorAt(e, units[0].src.path), "%s", e.Message)
		}
	}
	for _, p := range units {
		if p.query != nil {
			p.src.executable(p.query, &selector{src: p.src, schema: loaded, fragments: fragments})
		}
	}
}
