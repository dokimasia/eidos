// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"strings"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/lexer"
	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/graphql/internal/document"
)

// The allocations of the split of the small type of the allocation cases:
// the lexed file, the source of its lexer, the growth of its list of tokens,
// and its lists of significant tokens, of pairs and of ends; the list of its
// definitions; the copy of the file without a definition; and the copy
// without three definitions, whose list of ranges does not fit on the stack.
const (
	lexAllocs         = 1 + 1 + 4 + 3
	definitionsAllocs = 1
	copyAllocs        = 1
	threeCopyAllocs   = 1 + 1
)

// threeScalars is a file of three definitions, which the copy of the
// allocation cases leaves out.
const threeScalars = "scalar A\nscalar B\nscalar C\n"

// commentedType is a type with a comment after its opening brace.
const commentedType = "type A { # c\n  a: Int\n}\n"

// recovered is the definition after each broken definition of the recovery
// cases, which opens with the keyword scalar.
const recovered = "\nscalar B"

// TestSplit pins the split of a file: its tokens, their positions and the
// pairs of their brackets, the top-level definitions by the grammar of
// GraphQL, the recovery after a broken definition, and the copies of the
// file without some of its definitions.
func TestSplit(t *testing.T) {
	t.Parallel()

	t.Run("Lex", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the tokens of a file in source order with its comments", func(t *testing.T) {
			t.Parallel()

			lx := document.Lex(&ast.Source{Name: fileName, Input: commentedType})
			kinds := make([]lexer.Type, 0, len(lx.Tokens))
			for _, tok := range lx.Tokens {
				kinds = append(kinds, tok.Kind)
			}
			assert.Equal(t, kinds, []lexer.Type{
				lexer.Name, lexer.Name, lexer.BraceL, lexer.Comment, lexer.Name, lexer.Colon, lexer.Name, lexer.BraceR,
			}, "the kinds of the tokens")
		})

		t.Run("returns the indices of the tokens that are not comments", func(t *testing.T) {
			t.Parallel()

			lx := document.Lex(&ast.Source{Name: fileName, Input: commentedType})
			assert.Equal(t, lx.Sig, []int{0, 1, 2, 4, 5, 6, 7}, "the significant tokens")
		})

		t.Run("returns the line and the column of the rune where a token starts", func(t *testing.T) {
			t.Parallel()

			a := document.Lex(&ast.Source{Name: fileName, Input: smallType}).At(3)
			expect.Equal(t, a.Value, "a", "the token of the field")
			expect.Equal(t, a.Pos.Line, 2, "the field is on the second line")
			expect.Equal(t, a.Pos.Column, 3, "the field is after its indentation")
		})

		t.Run("returns the line where a block string over several lines starts", func(t *testing.T) {
			t.Parallel()

			lx := document.Lex(&ast.Source{Name: fileName, Input: "\"\"\"\nRow is one record.\n\"\"\"\n" + smallType})
			expect.Equal(t, lx.At(0).Pos.Line, 1, "the block string starts on the first line")
			expect.Equal(t, lx.At(0).Pos.Column, 1, "the block string starts at its quotes")
		})

		t.Run("returns the line where a block string over several lines ends", func(t *testing.T) {
			t.Parallel()

			lx := document.Lex(&ast.Source{Name: fileName, Input: "\"\"\"\nRow is one record.\n\"\"\"\n" + smallType})
			assert.Equal(t, lx.Ends[0], 3, "the block string ends on the third line")
		})

		t.Run("returns the column of the quote of a string", func(t *testing.T) {
			t.Parallel()

			lx := document.Lex(&ast.Source{Name: fileName, Input: "\"Row.\" " + smallType})
			assert.Equal(t, lx.At(0).Pos.Column, 1, "the string starts at its quote")
		})

		endings := []struct {
			name string
			give string
		}{
			{name: "counts a carriage return and a line feed as one line", give: "type A {\r\n  a: Int\r\n}\r\n"},
			{name: "counts a carriage return as a line", give: "type A {\r  a: Int\r}\r"},
		}
		for _, tt := range endings {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				lx := document.Lex(&ast.Source{Name: fileName, Input: tt.give})
				expect.Equal(t, lx.At(3).Pos.Line, 2, "the field is on the second line")
				expect.Equal(t, lx.At(3).Pos.Column, 3, "the field is after its indentation")
				expect.Equal(t, lx.At(6).Pos.Line, 3, "the closing brace is on the third line")
			})
		}

		t.Run("returns an error of the lexer and the tokens of the lines after it", func(t *testing.T) {
			t.Parallel()

			lx := document.Lex(&ast.Source{Name: fileName, Input: smallType + "\n\"open\n\ntype B {\n  b: Int\n}\n"})
			assert.Length(t, lx.Errors, 1, "the lexer reports once")
			expect.Equal(t, lx.Errors[0].At.Line, 5, "the error is on the line of the open string")
			expect.Equal(t, lx.At(8).Value, "B", "the lexer resumes after the line of the error")
			expect.Equal(t, lx.At(8).Pos.Line, 7, "the token after the error has its line in the file")
		})

		t.Run("returns the message of an error of the lexer without its location", func(t *testing.T) {
			t.Parallel()

			open := lexer.New(&ast.Source{Input: "\"open"})
			_, err := open.ReadToken()
			ge := assert.ErrorAs[*gqlerror.Error](t, err, "the lexer of gqlparser refuses the open string")
			lx := document.Lex(&ast.Source{
				Name: fileName, Input: smallType + "\n\"open\n\ntype B {\n  b: Int\n}\n\"open\n",
			})
			assert.Length(t, lx.Errors, 2, "each open string reports")
			expect.Equal(t, lx.Errors[0].Message, ge.Message, "the message of the first error")
			expect.Equal(t, lx.Errors[1].Message, ge.Message, "the message of the error after the resume")
			expect.Equal(t, lx.Errors[1].At.Line, 10, "the error after the resume has its line in the file")
		})

		t.Run("returns an error of the lexer on the last line without a token after it", func(t *testing.T) {
			t.Parallel()

			lx := document.Lex(&ast.Source{Name: fileName, Input: smallType + "\"open"})
			expect.Length(t, lx.Errors, 1, "the lexer reports once")
			expect.Length(t, lx.Sig, 7, "the tokens before the error remain")
		})

		resumes := []struct {
			name string
			give string
		}{
			{
				name: "resumes after an error on a line that ends with a carriage return and a line feed",
				give: "\"open\r\ntype B {\r\n  b: Int\r\n}\r\n",
			},
			{
				name: "resumes after an error on a line that ends with a carriage return",
				give: "\"open\rtype B {\r  b: Int\r}\r",
			},
		}
		for _, tt := range resumes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				lx := document.Lex(&ast.Source{Name: fileName, Input: tt.give})
				expect.Length(t, lx.Errors, 1, "the lexer reports once")
				expect.Equal(t, lx.At(1).Value, "B", "the lexer resumes on the next line")
				expect.Equal(t, lx.At(1).Pos.Line, 2, "the token after the error is on the second line")
			})
		}

		t.Run("pairs each bracket with the bracket that closes it", func(t *testing.T) {
			t.Parallel()

			lx := document.Lex(&ast.Source{Name: fileName, Input: "type A { a(x: [Int]): Int }"})
			expect.Equal(t, lx.Pair[2], 13, "the opening brace pairs with the closing brace")
			expect.Equal(t, lx.Pair[13], 2, "the closing brace pairs with the opening brace")
			expect.Equal(t, lx.Pair[4], 10, "the opening parenthesis pairs with the closing parenthesis")
			expect.Equal(t, lx.Pair[7], 9, "the opening bracket pairs with the closing bracket")
		})

		t.Run("pairs an opening bracket that does not close with the end of the file", func(t *testing.T) {
			t.Parallel()

			lx := document.Lex(&ast.Source{Name: fileName, Input: "type A {\n  a: Int\n"})
			assert.Equal(t, lx.Pair[2], len(lx.Sig), "the open brace ends at the end of the file")
		})

		t.Run("pairs a closing bracket that closes nothing with -1", func(t *testing.T) {
			t.Parallel()

			lx := document.Lex(&ast.Source{Name: fileName, Input: "}"})
			assert.Equal(t, lx.Pair[0], -1, "the stray brace pairs with nothing")
		})

		t.Run("pairs an open bracket inside a pair with the closing bracket of the pair", func(t *testing.T) {
			t.Parallel()

			lx := document.Lex(&ast.Source{Name: fileName, Input: "{ ( }"})
			expect.Equal(t, lx.Pair[1], 2, "the open parenthesis ends at the closing brace")
			expect.Equal(t, lx.Pair[0], 2, "the opening brace pairs with the closing brace")
		})
	})

	t.Run("At", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the end of the file for an index past the last token", func(t *testing.T) {
			t.Parallel()

			lx := document.Lex(&ast.Source{Name: fileName, Input: "type A"})
			assert.Equal(t, lx.At(2).Kind, lexer.EOF, "an index past the tokens is the end of the file")
		})
	})

	t.Run("Find", func(t *testing.T) {
		t.Parallel()

		lx := document.Lex(&ast.Source{Name: fileName, Input: commentedType})

		t.Run("returns the index of the token that starts at an offset", func(t *testing.T) {
			t.Parallel()

			k, found := lx.Find(strings.Index(commentedType, "a:"))
			assert.True(t, found, "a token starts at the offset")
			assert.Equal(t, k, 3, "the field is the fourth significant token")
		})

		t.Run("reports false for an offset where no token starts", func(t *testing.T) {
			t.Parallel()

			_, found := lx.Find(1)
			assert.False(t, found, "the offset is inside the keyword type")
		})
	})

	t.Run("Is", func(t *testing.T) {
		t.Parallel()

		lx := document.Lex(&ast.Source{Name: fileName, Input: commentedType})
		tests := []struct {
			name      string
			giveIndex int
			giveWord  string
			want      bool
		}{
			{name: "reports true for the name word", giveWord: "type", want: true},
			{name: "reports false for another name", giveWord: "input"},
			{name: "reports false for a token that is not a name", giveIndex: 2, giveWord: "{"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, lx.Is(tt.giveIndex, tt.giveWord), tt.want, "whether the token is the word")
			})
		}
	})

	t.Run("Definitions", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want []document.Definition
		}{
			{name: "returns no definition for a file without tokens", give: "# notes\n"},
			{
				name: "returns a schema definition",
				give: "schema { query: Q }",
				want: []document.Definition{{First: 0, Last: 5}},
			},
			{
				name: "returns a scalar with its directives",
				give: "scalar S @d(a: 1)",
				want: []document.Definition{{First: 0, Last: 8}},
			},
			{
				name: "returns an object type with its interfaces and its directives",
				give: "type T implements & I & J @d { a: Int }",
				want: []document.Definition{{First: 0, Last: 13}},
			},
			{
				name: "returns an object type whose directive has no name as one definition",
				give: "type A @ {\n  a: Int\n}",
				want: []document.Definition{{First: 0, Last: 7}},
			},
			{
				name: "returns an interface with its interfaces",
				give: "interface I implements J { a: Int }",
				want: []document.Definition{{First: 0, Last: 8}},
			},
			{
				name: "returns a union with its directives and its members",
				give: "union U @d = | A | B",
				want: []document.Definition{{First: 0, Last: 8}},
			},
			{
				name: "returns a union without members",
				give: "union U",
				want: []document.Definition{{First: 0, Last: 1}},
			},
			{
				name: "returns a member of a union whose name is a keyword",
				give: "union U = query | type\nscalar S",
				want: []document.Definition{{First: 0, Last: 5}, {First: 6, Last: 7}},
			},
			{
				name: "returns an enum",
				give: "enum E { V }",
				want: []document.Definition{{First: 0, Last: 4}},
			},
			{
				name: "returns an input object",
				give: "input In { v: Int }",
				want: []document.Definition{{First: 0, Last: 6}},
			},
			{
				name: "returns a directive definition with its arguments and its locations",
				give: "directive @d(a: Int) repeatable on | OBJECT | UNION",
				want: []document.Definition{{First: 0, Last: 13}},
			},
			{
				name: "returns an extension",
				give: "extend type T { a: Int }",
				want: []document.Definition{{First: 0, Last: 7}},
			},
			{
				name: "returns a definition with its description",
				give: "\"A thing.\" scalar S",
				want: []document.Definition{{First: 0, Last: 2}},
			},
			{
				name: "returns an operation with its name, its variables and its directives",
				give: "query Q($v: Int) @d { a }",
				want: []document.Definition{{First: 0, Last: 12, Executable: true}},
			},
			{
				name: "returns an operation without a name",
				give: "mutation { a }",
				want: []document.Definition{{First: 0, Last: 3, Executable: true}},
			},
			{
				name: "returns a subscription",
				give: "subscription S { a }",
				want: []document.Definition{{First: 0, Last: 4, Executable: true}},
			},
			{
				name: "returns an anonymous query",
				give: "{ a }",
				want: []document.Definition{{First: 0, Last: 2, Executable: true}},
			},
			{
				name: "returns a fragment with its variables and its directives",
				give: "fragment F($v: Int) on T @d { a }",
				want: []document.Definition{{First: 0, Last: 14, Executable: true}},
			},
			{
				name: "returns the definitions of a mixed file in their order",
				give: "scalar S { a }",
				want: []document.Definition{{First: 0, Last: 1}, {First: 2, Last: 4, Executable: true}},
			},
			{
				name: "returns a broken definition that ends at the end of the file",
				give: "foo bar",
				want: []document.Definition{{First: 0, Last: 1, Broken: true}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := document.Lex(&ast.Source{Name: fileName, Input: tt.give}).Definitions()
				assert.Equal(t, got, tt.want, "the definitions of the file")
			})
		}

		broken := []struct {
			name           string
			give           string
			wantExecutable bool
		}{
			{name: "returns a broken type without a name", give: "type {\n  a: Int\n}"},
			{name: "returns a broken word that opens no definition", give: "foo bar"},
			{name: "returns a broken stray closing brace", give: "}"},
			{name: "returns a broken directive without its sigil", give: "directive d on OBJECT"},
			{name: "returns a broken directive without its name", give: "directive @ on OBJECT"},
			{name: "returns a broken directive without on", give: "directive @d(a: Int)"},
			{name: "returns a broken fragment without a name", give: "fragment {\n  a\n}", wantExecutable: true},
			{
				name:           "returns a broken fragment without on",
				give:           "fragment F User {\n  a\n}",
				wantExecutable: true,
			},
			{
				name:           "returns a broken fragment without its type condition",
				give:           "fragment F on {\n  a\n}",
				wantExecutable: true,
			},
			{name: "returns a broken operation without a body", give: "query Q", wantExecutable: true},
			{name: "returns a broken description of no definition", give: `"A thing." 42`},
			{name: "returns a broken extension of no kind", give: "extend foo"},
			{name: "returns a broken extension without a keyword", give: "extend {\n  a\n}"},
			{name: "returns a broken union without a name", give: "union = A"},
			{name: "returns a broken definition with a keyword inside a bracket", give: "foo { type }"},
		}
		for _, tt := range broken {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				src := tt.give + recovered
				lx := document.Lex(&ast.Source{Name: fileName, Input: src})
				k, found := lx.Find(strings.LastIndex(src, "scalar"))
				assert.True(t, found, "the definition after the broken one opens with scalar")
				assert.Equal(t, lx.Definitions(), []document.Definition{
					{First: 0, Last: k - 1, Executable: tt.wantExecutable, Broken: true},
					{First: k, Last: k + 1},
				}, "the broken definition ends before the next keyword at the top level")
			})
		}
	})

	t.Run("Text", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name             string
			give             string
			giveFrom, giveTo int
			want             string
		}{
			{
				name: "returns the text from the start of a token up to the end of another",
				give: "type A { a: Int }", giveFrom: 2, giveTo: 6, want: "{ a: Int }",
			},
			{
				name: "returns the text of a file with runes outside ASCII as written",
				give: "type A {\n  a(x: \"é ü\"): Int\n}\n", giveFrom: 4, giveTo: 8, want: "(x: \"é ü\")",
			},
			{
				name: "returns the text up to the end of a file that ends with the token",
				give: "type A { a(x: \"é\"): Int }", giveFrom: 2, giveTo: 11, want: "{ a(x: \"é\"): Int }",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				lx := document.Lex(&ast.Source{Name: fileName, Input: tt.give})
				assert.Equal(t, lx.Text(tt.giveFrom, tt.giveTo), tt.want, "the text of the tokens")
			})
		}
	})

	t.Run("CopyWithout", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    string
			giveOut []bool
			want    string
		}{
			{
				name: "returns the text of the file where it leaves nothing out",
				give: mixedFile, giveOut: []bool{false, false}, want: mixedFile,
			},
			{
				name: "returns a space for each rune of a definition that it leaves out",
				give: mixedFile, giveOut: []bool{false, true}, want: "type A {\n  a: Int\n}\n     \n",
			},
			{
				name: "returns the line terminators of a definition that it leaves out",
				give: mixedFile, giveOut: []bool{true, false}, want: "        \n        \n \n{ a }\n",
			},
			{
				name: "returns a space for each rune of the range that the lexer skipped",
				give: "\"open\n" + smallType, giveOut: []bool{false}, want: "     \n" + smallType,
			},
			{
				name: "returns one space for a rune outside ASCII that it leaves out",
				give: "\"é open\n" + smallType, giveOut: []bool{false}, want: "       \n" + smallType,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				lx := document.Lex(&ast.Source{Name: fileName, Input: tt.give})
				assert.Equal(t, lx.CopyWithout(lx.Definitions(), tt.giveOut), tt.want, "the copy of the file")
			})
		}
	})
}

// The split allocates its lists once each, the copy of a file without a
// definition allocates the copy and its ranges, and a lookup allocates
// nothing. The ordinary run, which runs no benchmark, checks those ceilings
// here.
func TestSplitAllocs(t *testing.T) {
	src := &ast.Source{Name: fileName, Input: smallType}
	var lx *document.Lexed
	assert.MaxAllocs(t, func() { lx = document.Lex(src) }, lexAllocs,
		"Lex allocates the lexed file and its lists")
	assert.Length(t, lx.Sig, 7, "Lex lexes the type")
	var defs []document.Definition
	assert.MaxAllocs(t, func() { defs = lx.Definitions() }, definitionsAllocs,
		"Definitions allocates the list of the definitions")
	assert.Length(t, defs, 1, "Definitions splits the type")
	var (
		tok   lexer.Token
		k     int
		found bool
		is    bool
		text  string
	)
	assert.MaxAllocs(t, func() { tok = lx.At(3) }, 0, "At allocates nothing")
	assert.Equal(t, tok.Value, "a", "At returns the field")
	assert.MaxAllocs(t, func() { k, found = lx.Find(11) }, 0, "Find allocates nothing")
	assert.True(t, found, "Find finds the field")
	assert.Equal(t, k, 3, "Find returns the index of the field")
	assert.MaxAllocs(t, func() { is = lx.Is(0, "type") }, 0, "Is allocates nothing")
	assert.True(t, is, "Is reports the keyword")
	assert.MaxAllocs(t, func() { text = lx.Text(2, 6) }, 0, "Text allocates nothing")
	assert.Equal(t, text, "{\n  a: Int\n}", "Text returns the body of the type")
	keep, drop := []bool{false}, []bool{true}
	assert.MaxAllocs(t, func() { text = lx.CopyWithout(defs, keep) }, 0,
		"CopyWithout allocates nothing where it leaves nothing out")
	assert.Equal(t, text, smallType, "CopyWithout returns the file")
	assert.MaxAllocs(t, func() { text = lx.CopyWithout(defs, drop) }, copyAllocs,
		"CopyWithout allocates the copy")
	assert.Equal(t, text, "        \n        \n \n", "CopyWithout leaves the type out")
	scalars := document.Lex(&ast.Source{Name: fileName, Input: threeScalars})
	scalarDefs, dropAll := scalars.Definitions(), []bool{true, true, true}
	assert.MaxAllocs(t, func() { text = scalars.CopyWithout(scalarDefs, dropAll) }, threeCopyAllocs,
		"CopyWithout allocates the copy and the list of three ranges")
	assert.Equal(t, text, "        \n        \n        \n", "CopyWithout leaves the three scalars out")
}

// BenchmarkSplit measures the split that every file of a unit passes
// through, and the lookups of the tokens that the lowering of the file
// makes.
func BenchmarkSplit(b *testing.B) {
	src := &ast.Source{Name: fileName, Input: smallType}
	lx := document.Lex(src)

	b.Run("Lex", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(lexAllocs)
		defer c.End()
		var got *document.Lexed
		for c.Loop() {
			got = document.Lex(src)
		}
		assert.Length(b, got.Sig, 7, "Lex lexes the type")
	})

	b.Run("At", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got lexer.Token
		for c.Loop() {
			got = lx.At(3)
		}
		assert.Equal(b, got.Value, "a", "At returns the field")
	})

	b.Run("Find", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got, _ = lx.Find(11)
		}
		assert.Equal(b, got, 3, "Find returns the index of the field")
	})

	b.Run("Is", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = lx.Is(0, "type")
		}
		assert.True(b, got, "Is reports the keyword")
	})

	b.Run("Definitions", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(definitionsAllocs)
		defer c.End()
		var got []document.Definition
		for c.Loop() {
			got = lx.Definitions()
		}
		assert.Length(b, got, 1, "Definitions splits the type")
	})

	b.Run("Text", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = lx.Text(2, 6)
		}
		assert.Equal(b, got, "{\n  a: Int\n}", "Text returns the body of the type")
	})

	b.Run("CopyWithout", func(b *testing.B) {
		scalars := document.Lex(&ast.Source{Name: fileName, Input: threeScalars})
		copies := []struct {
			name   string
			lexed  *document.Lexed
			out    []bool
			want   string
			allocs uint64
		}{
			{name: "a copy that leaves nothing out", lexed: lx, out: []bool{false}, want: smallType},
			{
				name: "a copy that leaves a definition out", lexed: lx, out: []bool{true},
				want: "        \n        \n \n", allocs: copyAllocs,
			},
			{
				name: "a copy that leaves three definitions out", lexed: scalars, out: []bool{true, true, true},
				want: "        \n        \n        \n", allocs: threeCopyAllocs,
			},
		}
		for _, tt := range copies {
			b.Run(tt.name, func(b *testing.B) {
				split := tt.lexed.Definitions()
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var got string
				for c.Loop() {
					got = tt.lexed.CopyWithout(split, tt.out)
				}
				assert.Equal(b, got, tt.want, "CopyWithout returns the copy")
			})
		}
	})
}
