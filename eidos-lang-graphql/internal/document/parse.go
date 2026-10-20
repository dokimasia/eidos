// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"errors"
	"math"

	"github.com/dokimasia/gqlparser/v2/ast"
	"github.com/dokimasia/gqlparser/v2/gqlerror"
	"github.com/dokimasia/gqlparser/v2/parser"

	"go.dokimi.dev/eidos/sdk/position"
)

// SyntaxError is one syntax error of a parse: its position in the file and
// the message of the parser.
type SyntaxError struct {
	At      position.Pos
	Message string
}

// ParseSchema parses the type system of the file src, which lx lexed and
// defs splits, from a copy of the file without its executable definitions.
// It returns the document and the syntax errors. The parser stops at its
// first error, so ParseSchema leaves out the definition that the parser read
// when it failed and parses the copy again, until the copy parses. A copy
// without a definition parses, so the loop ends. The document is nil for a
// file without a definition of the type system.
//
// ParseSchema allocates the source and the text of the copy of each round,
// the parse of gqlparser for each round, the syntax errors, and the list of
// the definitions that the copies leave out where the list does not fit on
// the stack.
func ParseSchema(src *ast.Source, lx *Lexed, defs []Definition) (*ast.SchemaDocument, []SyntaxError) {
	out := make([]bool, len(defs))
	kept := false
	for i, d := range defs {
		out[i] = d.Executable
		kept = kept || !d.Executable
	}
	if !kept {
		return nil, nil
	}
	var errs []SyntaxError
	for {
		doc, err := parser.ParseSchema(&ast.Source{Name: src.Name, Input: lx.CopyWithout(defs, out)})
		if err == nil {
			return doc, errs
		}
		e, i := locate(src.Name, lx, defs, out, err)
		errs = append(errs, e)
		out[i] = true
	}
}

// ParseQuery parses the executable definitions of the file src, which lx
// lexed and defs splits, from a copy of the file without its type system,
// as [ParseSchema] parses the type system. The document is nil for a file
// without an executable definition. It allocates as ParseSchema does.
func ParseQuery(src *ast.Source, lx *Lexed, defs []Definition) (*ast.QueryDocument, []SyntaxError) {
	out := make([]bool, len(defs))
	kept := false
	for i, d := range defs {
		out[i] = !d.Executable
		kept = kept || d.Executable
	}
	if !kept {
		return nil, nil
	}
	var errs []SyntaxError
	for {
		doc, err := parser.ParseQuery(&ast.Source{Name: src.Name, Input: lx.CopyWithout(defs, out)})
		if err == nil {
			return doc, errs
		}
		e, i := locate(src.Name, lx, defs, out, err)
		errs = append(errs, e)
		out[i] = true
	}
}

// locate returns the syntax error of a failed parse of a copy of the file
// named name, and the index of the definition that the parser read when it
// failed. out lists the definitions that the copy leaves out. The
// definition is the first broken definition that the copy keeps and that
// starts at or before the error, because the parser reads past a broken
// definition into the next one before it fails. Without a broken one, it is
// the last definition that the copy keeps and that starts at or before the
// error, and the first definition that the copy keeps where none starts
// before the error. A copy that keeps no definition parses, so a failed
// parse keeps one.
//
// An error without a location, which the parser returns for a limit of
// tokens, is at the start of the file and after every definition.
func locate(name string, lx *Lexed, defs []Definition, out []bool, err error) (SyntaxError, int) {
	e := SyntaxError{At: position.Pos{File: name, Line: 1, Col: 1}, Message: err.Error()}
	limit := gqlerror.Location{Line: math.MaxInt, Column: math.MaxInt}
	var ge *gqlerror.Error
	if errors.As(err, &ge) && len(ge.Locations) > 0 {
		line, column := lx.filePosition(ge.Locations[0].Line, ge.Locations[0].Column)
		limit = gqlerror.Location{Line: line, Column: column}
		e = SyntaxError{At: position.Pos{File: name, Line: line, Col: column}, Message: ge.Message}
	}
	broken, last := -1, -1
	for j, d := range defs {
		if out[j] {
			continue
		}
		tok := lx.At(d.First)
		before := tok.Pos.Line < limit.Line || tok.Pos.Line == limit.Line && tok.Pos.Column <= limit.Column
		if last < 0 || before {
			last = j
		}
		if before && d.Broken && broken < 0 {
			broken = j
		}
	}
	if broken >= 0 {
		return e, broken
	}
	return e, last
}
