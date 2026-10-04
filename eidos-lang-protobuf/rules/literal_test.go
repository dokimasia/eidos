// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/rules"
)

// A literal reads in protoc's grammar, not Go's, so every form
// protobuf writes and every form only Go writes are pinned. Each case
// lifts its text against no type, so each literal lifts as its own
// kind.
func TestLiteral(t *testing.T) {
	t.Parallel()

	t.Run("LiteralFor", func(t *testing.T) {
		t.Parallel()

		reads := []struct {
			name string
			give string
			want emit.Value
		}{
			{name: "returns true for true", give: "true", want: emit.Literal(emit.LiteralBool, "true")},
			{name: "returns false for false", give: "false", want: emit.Literal(emit.LiteralBool, "false")},
			{
				name: "trims the white space around a literal",
				give: " true ", want: emit.Literal(emit.LiteralBool, "true"),
			},
			{
				name: "returns the content of a double-quoted string",
				give: `"hi"`, want: emit.Literal(emit.LiteralString, "hi"),
			},
			{
				name: "returns the content of a single-quoted string",
				give: `'hi'`, want: emit.Literal(emit.LiteralString, "hi"),
			},
			{
				name: "concatenates adjacent strings",
				give: `"a" 'b'`, want: emit.Literal(emit.LiteralString, "ab"),
			},
			{
				name: "returns the byte of every named escape",
				give: `"\a\b\f\n\r\t\v\\\'\"\?"`, want: emit.Literal(emit.LiteralString, "\a\b\f\n\r\t\v\\'\"?"),
			},
			{
				name: "returns the byte of a two-digit hexadecimal escape",
				give: `"\x41"`, want: emit.Literal(emit.LiteralString, "A"),
			},
			{
				name: "returns the byte of a hexadecimal escape behind an upper-case X",
				give: `"\X42"`, want: emit.Literal(emit.LiteralString, "B"),
			},
			{
				name: "returns the byte of a one-digit hexadecimal escape",
				give: `"\x7"`, want: emit.Literal(emit.LiteralString, "\x07"),
			},
			{
				name: "returns the byte of a three-digit octal escape",
				give: `"\101"`, want: emit.Literal(emit.LiteralString, "A"),
			},
			{
				name: "returns the byte of a one-digit octal escape",
				give: `"\0"`, want: emit.Literal(emit.LiteralString, "\x00"),
			},
			{
				name: "returns the code point of a four-digit escape",
				give: `"é"`, want: emit.Literal(emit.LiteralString, "é"),
			},
			{
				name: "returns the code point of an eight-digit escape",
				give: `"\U0001F600"`, want: emit.Literal(emit.LiteralString, "😀"),
			},
			{name: "returns a decimal integer", give: "42", want: emit.Literal(emit.LiteralInt, "42")},
			{name: "returns a negative integer", give: "-7", want: emit.Literal(emit.LiteralInt, "-7")},
			{
				name: "returns a negative integer with the sign apart from its digits",
				give: "- 7", want: emit.Literal(emit.LiteralInt, "-7"),
			},
			{
				name: "returns a hexadecimal integer in decimal",
				give: "0x10", want: emit.Literal(emit.LiteralInt, "16"),
			},
			{name: "returns an octal integer in decimal", give: "017", want: emit.Literal(emit.LiteralInt, "15")},
			{name: "returns a float", give: "2.5", want: emit.Number(emit.LiteralFloat, "2.5", 0)},
			{
				name: "returns a float that opens with its point",
				give: ".5", want: emit.Number(emit.LiteralFloat, "0.5", 0),
			},
			{
				name: "returns a float that closes with its point",
				give: "5.", want: emit.Number(emit.LiteralFloat, "5", 0),
			},
			{
				name: "returns an exponent in canonical text",
				give: "1e21", want: emit.Number(emit.LiteralFloat, "1e+21", 0),
			},
		}
		for _, tt := range reads {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := untyped(tt.give)
				assert.True(t, ok, "the text is a literal")
				assert.Equal(t, got, tt.want, "the literal as its own kind")
			})
		}

		outside := []struct {
			name string
			give string
		}{
			{name: "reports false for an escape protoc does not define", give: `"\q"`},
			{name: "reports false for a hexadecimal escape without a digit", give: `"\x"`},
			{name: "reports false for an octal escape above one byte", give: `"\400"`},
			{name: "reports false for a code point short of four digits", give: `"\u12"`},
			{name: "reports false for a code point past Unicode", give: `"\U00110000"`},
			{name: "reports false for an unterminated string", give: `"open`},
			{name: "reports false for a backslash that ends the text", give: `"a\`},
			{name: "reports false for text after the closing quote", give: `"a"b`},
			{name: "reports false for a string that spans two lines", give: "\"line\nbreak\""},
			{name: "reports false for a digit separator", give: "1_000"},
			{name: "reports false for a binary integer", give: "0b101"},
			{name: "reports false for an octal integer behind Go's prefix", give: "0o7"},
			{name: "reports false for an octal integer with a decimal digit", give: "08"},
			{name: "reports false for a hexadecimal prefix without digits", give: "0x"},
			{name: "reports false for a sign without digits", give: "-"},
			{name: "reports false for two signs", give: "--1"},
			{name: "reports false for an enum value's name", give: "COLOUR_RED"},
			{name: "reports false for an identifier that opens with an underscore", give: "_x1"},
			{name: "reports false for inf", give: "inf"},
			{name: "reports false for nan", give: "nan"},
			{name: "reports false for an empty text", give: ""},
			{name: "reports false for digits followed by letters", give: "1bad"},
			{name: "reports false for a message literal", give: "{ }"},
		}
		for _, tt := range outside {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, ok := untyped(tt.give)
				assert.False(t, ok, "the text is no literal of its own kind")
			})
		}
	})
}

// untyped lifts a literal against no type, so each lifts as its own
// kind.
func untyped(text string) (emit.Value, bool) {
	return protorules.New().LiteralFor(nil, nil, text, rules.View{})
}
