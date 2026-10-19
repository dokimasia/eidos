// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"strings"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/parser"
	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/graphql/internal/document"
	"go.dokimi.dev/eidos/sdk/position"
)

// The allocations of the parse of the small type and of the small query: the
// source of the copy, and the 10 and 8 allocations of the parse of
// gqlparser. The list of the definitions that a copy leaves out fits on the
// stack.
const (
	parseSchemaAllocs = 1 + 10
	parseQueryAllocs  = 1 + 8
)

// The sources of the parse cases: a query, a schema with a broken field
// between two whole types, the field without a type alone, and a string and
// a block string where the grammar admits none.
const (
	smallQuery    = "query Q {\n  a\n}\n"
	brokenSchema  = "type A {\n  a: Int\n}\n\ntype B {\n  b:\n}\n\ntype C {\n  c: Int\n}\n"
	brokenField   = "type B {\n  b:\n}\n"
	brokenQuery   = "query A {\n  a(\n}\n\nquery B {\n  a\n}\n"
	strayString   = "directive @d \"x\" on FIELD\n"
	strayBlock    = "directive @d \"\"\"\nx\n\"\"\" on FIELD\n"
	strayVariable = "query Q \"x\" {\n  a\n}\n"
)

// The column of the opening quote of each stray string, on the first line
// of its source.
var (
	strayStringColumn   = strings.IndexByte(strayString, '"') + 1
	strayBlockColumn    = strings.IndexByte(strayBlock, '"') + 1
	strayVariableColumn = strings.IndexByte(strayVariable, '"') + 1
)

// TestParse pins the parse of each language of a file from its copy: the
// documents, the syntax errors at their positions in the file, and the
// definitions outside a broken one.
func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("ParseSchema", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the document of the type system of a file", func(t *testing.T) {
			t.Parallel()

			src := &ast.Source{Name: fileName, Input: smallType}
			lx := document.Lex(src)
			doc, errs := document.ParseSchema(src, lx, lx.Definitions())
			assert.Empty(t, errs, "the type parses")
			assert.Length(t, doc.Definitions, 1, "the file declares one type")
			assert.Equal(t, doc.Definitions[0].Name, "A", "the type is A")
		})

		t.Run("returns nil for a file without a definition of the type system", func(t *testing.T) {
			t.Parallel()

			src := &ast.Source{Name: fileName, Input: smallQuery}
			lx := document.Lex(src)
			doc, errs := document.ParseSchema(src, lx, lx.Definitions())
			expect.Nil(t, doc, "the file has no type system")
			expect.Empty(t, errs, "the parse reports nothing")
		})

		t.Run("returns the type system of a mixed file without its executable definitions", func(t *testing.T) {
			t.Parallel()

			src := &ast.Source{Name: fileName, Input: mixedFile}
			lx := document.Lex(src)
			doc, errs := document.ParseSchema(src, lx, lx.Definitions())
			assert.Empty(t, errs, "the copy without the query parses")
			assert.Length(t, doc.Definitions, 1, "the copy declares the type alone")
		})

		t.Run("returns a syntax error at its position in the file", func(t *testing.T) {
			t.Parallel()

			_, err := parser.ParseSchema(&ast.Source{Input: brokenField})
			ge := assert.ErrorAs[*gqlerror.Error](t, err, "gqlparser refuses the field without a type")
			src := &ast.Source{Name: fileName, Input: brokenSchema}
			lx := document.Lex(src)
			_, errs := document.ParseSchema(src, lx, lx.Definitions())
			assert.Equal(t, errs, []document.SyntaxError{
				{At: position.Pos{File: fileName, Line: 7, Col: 1}, Message: ge.Message},
			}, "the error is at the token that the parser did not expect")
		})

		t.Run("returns the definitions outside a broken definition", func(t *testing.T) {
			t.Parallel()

			src := &ast.Source{Name: fileName, Input: brokenSchema}
			lx := document.Lex(src)
			doc, _ := document.ParseSchema(src, lx, lx.Definitions())
			names := make([]string, 0, len(doc.Definitions))
			for _, d := range doc.Definitions {
				names = append(names, d.Name)
			}
			assert.Equal(t, names, []string{"A", "C"}, "the types around the broken one parse")
		})

		t.Run("returns an error for each broken definition", func(t *testing.T) {
			t.Parallel()

			src := &ast.Source{Name: fileName, Input: brokenField + "\n" + brokenField}
			lx := document.Lex(src)
			doc, errs := document.ParseSchema(src, lx, lx.Definitions())
			assert.Length(t, errs, 2, "each broken type reports")
			expect.Equal(t, errs[0].At.Line, 3, "the first error is in the first type")
			expect.Equal(t, errs[1].At.Line, 7, "the second error is in the second type")
			expect.Empty(t, doc.Definitions, "no type parses")
		})

		t.Run("returns the definitions after a broken definition that the parser reads past", func(t *testing.T) {
			t.Parallel()

			src := &ast.Source{Name: fileName, Input: "foo bar\n" + smallType}
			lx := document.Lex(src)
			doc, errs := document.ParseSchema(src, lx, lx.Definitions())
			assert.Length(t, errs, 1, "the broken words report once")
			expect.Equal(t, errs[0].At, position.Pos{File: fileName, Line: 1, Col: 1}, "the error is at the words")
			expect.Length(t, doc.Definitions, 1, "the type after the words parses")
		})

		strays := []struct {
			name     string
			give     string
			wantLine int
			wantCol  int
		}{
			{
				name: "returns a syntax error at a string at the opening quote of the string",
				give: strayString, wantLine: 1, wantCol: strayStringColumn,
			},
			{
				name: "returns a syntax error at a block string at the opening quotes of the string",
				give: strayBlock, wantLine: 1, wantCol: strayBlockColumn,
			},
		}
		for _, tt := range strays {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := parser.ParseSchema(&ast.Source{Input: tt.give})
				ge := assert.ErrorAs[*gqlerror.Error](t, err, "gqlparser refuses the stray string")
				src := &ast.Source{Name: fileName, Input: tt.give}
				lx := document.Lex(src)
				_, errs := document.ParseSchema(src, lx, lx.Definitions())
				assert.Equal(t, errs, []document.SyntaxError{
					{At: position.Pos{File: fileName, Line: tt.wantLine, Col: tt.wantCol}, Message: ge.Message},
				}, "the error is at the first rune of the string that the parser did not expect")
			})
		}
	})

	t.Run("ParseQuery", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the executable definitions of a mixed file without its type system", func(t *testing.T) {
			t.Parallel()

			src := &ast.Source{Name: fileName, Input: mixedFile}
			lx := document.Lex(src)
			doc, errs := document.ParseQuery(src, lx, lx.Definitions())
			assert.Empty(t, errs, "the copy without the type parses")
			assert.Length(t, doc.Operations, 1, "the copy declares the anonymous query")
		})

		t.Run("returns nil for a file without an executable definition", func(t *testing.T) {
			t.Parallel()

			src := &ast.Source{Name: fileName, Input: smallType}
			lx := document.Lex(src)
			doc, errs := document.ParseQuery(src, lx, lx.Definitions())
			expect.Nil(t, doc, "the file has no executable definition")
			expect.Empty(t, errs, "the parse reports nothing")
		})

		t.Run("returns a syntax error and the operations outside the broken one", func(t *testing.T) {
			t.Parallel()

			_, err := parser.ParseQuery(&ast.Source{Input: "query A {\n  a(\n}\n"})
			ge := assert.ErrorAs[*gqlerror.Error](t, err, "gqlparser refuses the open arguments")
			src := &ast.Source{Name: fileName, Input: brokenQuery}
			lx := document.Lex(src)
			doc, errs := document.ParseQuery(src, lx, lx.Definitions())
			assert.Equal(t, errs, []document.SyntaxError{
				{At: position.Pos{File: fileName, Line: 3, Col: 1}, Message: ge.Message},
			}, "the error is at the token that the parser did not expect")
			assert.Length(t, doc.Operations, 1, "the other operation parses")
			assert.Equal(t, doc.Operations[0].Name, "B", "the other operation is B")
		})

		t.Run("returns a syntax error at a string at the opening quote of the string", func(t *testing.T) {
			t.Parallel()

			_, err := parser.ParseQuery(&ast.Source{Input: strayVariable})
			ge := assert.ErrorAs[*gqlerror.Error](t, err, "gqlparser refuses the string after the name")
			src := &ast.Source{Name: fileName, Input: strayVariable}
			lx := document.Lex(src)
			_, errs := document.ParseQuery(src, lx, lx.Definitions())
			assert.Equal(t, errs, []document.SyntaxError{
				{At: position.Pos{File: fileName, Line: 1, Col: strayVariableColumn}, Message: ge.Message},
			}, "the error is at the first rune of the string that the parser did not expect")
		})
	})
}

// A parse allocates the list of the definitions that its copy leaves out,
// the source of the copy and what gqlparser allocates. The ordinary run,
// which runs no benchmark, checks those ceilings here.
func TestParseAllocs(t *testing.T) {
	schemaSrc := &ast.Source{Name: fileName, Input: smallType}
	schemaLexed := document.Lex(schemaSrc)
	schemaDefs := schemaLexed.Definitions()
	var schema *ast.SchemaDocument
	assert.MaxAllocs(t, func() { schema, _ = document.ParseSchema(schemaSrc, schemaLexed, schemaDefs) },
		parseSchemaAllocs, "ParseSchema allocates its copy and the parse of gqlparser")
	assert.Length(t, schema.Definitions, 1, "ParseSchema parses the type")

	querySrc := &ast.Source{Name: fileName, Input: smallQuery}
	queryLexed := document.Lex(querySrc)
	queryDefs := queryLexed.Definitions()
	var query *ast.QueryDocument
	assert.MaxAllocs(t, func() { query, _ = document.ParseQuery(querySrc, queryLexed, queryDefs) },
		parseQueryAllocs, "ParseQuery allocates its copy and the parse of gqlparser")
	assert.Length(t, query.Operations, 1, "ParseQuery parses the query")
}

// BenchmarkParse measures the parse of each language of every file of a
// unit.
func BenchmarkParse(b *testing.B) {
	b.Run("ParseSchema", func(b *testing.B) {
		src := &ast.Source{Name: fileName, Input: smallType}
		lx := document.Lex(src)
		defs := lx.Definitions()
		c := bench.Start(b).MaxAllocs(parseSchemaAllocs)
		defer c.End()
		var got *ast.SchemaDocument
		for c.Loop() {
			got, _ = document.ParseSchema(src, lx, defs)
		}
		assert.Length(b, got.Definitions, 1, "ParseSchema parses the type")
	})

	b.Run("ParseQuery", func(b *testing.B) {
		src := &ast.Source{Name: fileName, Input: smallQuery}
		lx := document.Lex(src)
		defs := lx.Definitions()
		c := bench.Start(b).MaxAllocs(parseQueryAllocs)
		defer c.End()
		var got *ast.QueryDocument
		for c.Loop() {
			got, _ = document.ParseQuery(src, lx, defs)
		}
		assert.Length(b, got.Operations, 1, "ParseQuery parses the query")
	})
}
