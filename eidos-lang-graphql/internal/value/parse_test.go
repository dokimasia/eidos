// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package value_test

import (
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql/internal/value"
)

// The values that the allocation test and the benchmark parse: a scalar,
// and an object with a list.
const (
	scalarText = "42"
	objectText = `{name: "Ada", tags: [ADMIN, OWNER]}`
)

// The ceilings of the parses. A scalar allocates the source of the lexer
// and its value. The object allocates the source, five values, four
// entries, and two growths of the list of the entries of the object and of
// the list.
const (
	scalarAllocs = 1 + 1
	objectAllocs = 1 + 5 + 4 + 2 + 2
)

// TestParse pins the grammar of the constant values of GraphQL that the
// rules and the backend read.
func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want *ast.Value
		}{
			{name: "returns an integer", give: "42", want: &ast.Value{Kind: ast.IntValue, Raw: "42"}},
			{name: "returns a negative integer", give: "-7", want: &ast.Value{Kind: ast.IntValue, Raw: "-7"}},
			{name: "returns a float", give: "1.5e3", want: &ast.Value{Kind: ast.FloatValue, Raw: "1.5e3"}},
			{
				name: "returns the content of a string without its escapes",
				give: `"a\"b\nc"`,
				want: &ast.Value{Kind: ast.StringValue, Raw: "a\"b\nc"},
			},
			{
				name: "returns the content of a block string",
				give: `"""block"""`,
				want: &ast.Value{Kind: ast.BlockValue, Raw: "block"},
			},
			{name: "returns true as a boolean", give: "true", want: &ast.Value{Kind: ast.BooleanValue, Raw: "true"}},
			{name: "returns false as a boolean", give: "false", want: &ast.Value{Kind: ast.BooleanValue, Raw: "false"}},
			{name: "returns null", give: "null", want: &ast.Value{Kind: ast.NullValue, Raw: "null"}},
			{name: "returns an enum value", give: "RED", want: &ast.Value{Kind: ast.EnumValue, Raw: "RED"}},
			{
				name: "returns a list of its items",
				give: `[1, "a"]`,
				want: &ast.Value{Kind: ast.ListValue, Children: ast.ChildValueList{
					{Value: &ast.Value{Kind: ast.IntValue, Raw: "1"}},
					{Value: &ast.Value{Kind: ast.StringValue, Raw: "a"}},
				}},
			},
			{name: "returns an empty list", give: "[]", want: &ast.Value{Kind: ast.ListValue}},
			{
				name: "returns an object of its fields",
				give: "{a: 1, b: [true]}",
				want: &ast.Value{Kind: ast.ObjectValue, Children: ast.ChildValueList{
					{Name: "a", Value: &ast.Value{Kind: ast.IntValue, Raw: "1"}},
					{Name: "b", Value: &ast.Value{Kind: ast.ListValue, Children: ast.ChildValueList{
						{Value: &ast.Value{Kind: ast.BooleanValue, Raw: "true"}},
					}}},
				}},
			},
			{name: "returns an empty object", give: "{}", want: &ast.Value{Kind: ast.ObjectValue}},
			{
				name: "returns a value between white space, commas and comments",
				give: " # a note\n 1 , ",
				want: &ast.Value{Kind: ast.IntValue, Raw: "1"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := value.Parse(tt.give)
				assert.NoError(t, err, "the text is a constant value")
				assert.Equal(t, got, tt.want, "the value in the abstract syntax of gqlparser")
			})
		}

		failures := []struct {
			name string
			give string
			want string
		}{
			{name: "returns an error for empty text", give: "", want: "unexpected <EOF>"},
			{name: "returns an error for a variable", give: "$id", want: "unexpected $"},
			{name: "returns an error for a token after the value", give: "1 2", want: `unexpected Int "2" at 1:3`},
			{name: "returns an error for a list that does not close", give: "[1", want: "unexpected <EOF>"},
			{name: "returns an error for a field without a colon", give: "{a 1}", want: `unexpected Int "1"`},
			{name: "returns an error for a field without a name", give: "{1: 2}", want: `unexpected Int "1"`},
			{name: "returns an error for a field without a value", give: "{a: }", want: "unexpected }"},
			{name: "returns the error of the lexer", give: `"abc`, want: "Unterminated string"},
			{name: "returns the error of the lexer inside a list", give: `["abc`, want: "Unterminated string"},
			{
				name: "returns the error of the lexer after the name of a field",
				give: `{a "b`,
				want: "Unterminated string",
			},
		}
		for _, tt := range failures {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := value.Parse(tt.give)
				assert.HasError(t, err, "the text is no constant value")
				assert.Contains(t, err.Error(), tt.want, "the error names the cause")
				assert.Nil(t, got, "a text that does not parse has no value")
			})
		}
	})
}

// TestParseAllocs checks the ceilings of Parse in the ordinary run, which
// does not run benchmarks.
func TestParseAllocs(t *testing.T) {
	t.Run("Parse", func(t *testing.T) {
		t.Run("allocates the value of a scalar", func(t *testing.T) {
			var got *ast.Value
			assert.MaxAllocs(t, func() { got, _ = value.Parse(scalarText) }, scalarAllocs,
				"Parse allocates the source and the value")
			assert.NotNil(t, got, "the scalar parses")
		})

		t.Run("allocates the values and the entries of an object", func(t *testing.T) {
			var got *ast.Value
			assert.MaxAllocs(t, func() { got, _ = value.Parse(objectText) }, objectAllocs,
				"Parse allocates the source, the values and the entries")
			assert.NotNil(t, got, "the object parses")
		})
	})
}

// BenchmarkParse measures the parse of a scalar, which a literal of a
// declaration usually is, and of an object with a list.
func BenchmarkParse(b *testing.B) {
	b.Run("Parse", func(b *testing.B) {
		b.Run("a scalar", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(scalarAllocs)
			defer c.End()
			var got *ast.Value
			for c.Loop() {
				got, _ = value.Parse(scalarText)
			}
			assert.NotNil(b, got, "the scalar parses")
		})

		b.Run("an object", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(objectAllocs)
			defer c.End()
			var got *ast.Value
			for c.Loop() {
				got, _ = value.Parse(objectText)
			}
			assert.NotNil(b, got, "the object parses")
		})
	})
}
