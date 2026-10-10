// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript"
	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/rules"
)

// TestLiteral checks each form of a TypeScript literal that the rules
// scan, and texts that are not literals. A literal without a type lifts
// as a value of its own kind.
func TestLiteral(t *testing.T) {
	t.Parallel()

	t.Run("scanLiteral", func(t *testing.T) {
		t.Parallel()

		reads := []struct {
			name string
			give string
			want emit.Value
		}{
			{name: "reads a decimal integer as a number", give: "42", want: emit.Number(emit.LiteralFloat, "42", 64)},
			{
				name: "reads a fraction with a minus sign", give: "-1.5",
				want: emit.Number(emit.LiteralFloat, "-1.5", 64),
			},
			{name: "reads a number with a plus sign", give: "+2", want: emit.Number(emit.LiteralFloat, "2", 64)},
			{
				name: "reads a minus sign that white space separates from its number", give: "- 5",
				want: emit.Number(emit.LiteralFloat, "-5", 64),
			},
			{
				name: "reads a hexadecimal number as decimal text", give: "0x10",
				want: emit.Number(emit.LiteralFloat, "16", 64),
			},
			{name: "reads a number with separators", give: "1_000", want: emit.Number(emit.LiteralFloat, "1000", 64)},
			{
				name: "reads a number with a signed exponent", give: "1e+3",
				want: emit.Number(emit.LiteralFloat, "1000", 64),
			},
			{name: "reads a number between white space", give: " 7 ", want: emit.Number(emit.LiteralFloat, "7", 64)},
			{name: "reads a bigint as TypeScript text", give: "42n", want: emit.Raw(typescript.Lang, "42n")},
			{name: "reads a bigint with a minus sign", give: "-42n", want: emit.Raw(typescript.Lang, "-42n")},
			{name: "reads the negative zero bigint as zero", give: "-0n", want: emit.Raw(typescript.Lang, "0n")},
			{
				name: "reads a hexadecimal bigint as decimal text", give: "0xFFn",
				want: emit.Raw(typescript.Lang, "255n"),
			},
			{name: "reads a string in double quotes", give: `"a"`, want: emit.Literal(emit.LiteralString, "a")},
			{name: "reads a string in single quotes", give: `'b'`, want: emit.Literal(emit.LiteralString, "b")},
			{name: "reads a template literal", give: "`c`", want: emit.Literal(emit.LiteralString, "c")},
			{name: "reads a string's escapes", give: `"a\tb"`, want: emit.Literal(emit.LiteralString, "a\tb")},
			{name: "reads true", give: "true", want: emit.Literal(emit.LiteralBool, "true")},
			{name: "reads false", give: "false", want: emit.Literal(emit.LiteralBool, "false")},
			{name: "reads null as the absent value", give: "null", want: emit.Literal(emit.LiteralNil, "")},
			{
				name: "reads undefined as TypeScript text", give: "undefined",
				want: emit.Raw(typescript.Lang, "undefined"),
			},
		}
		for _, tt := range reads {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := tsrules.New().LiteralFor(nil, nil, tt.give, rules.View{})
				assert.True(t, ok, "the text is a literal")
				assert.Equal(t, got, tt.want, "LiteralFor returns the literal as a value of its own kind")
			})
		}

		refusals := []struct {
			name string
			give string
		}{
			{name: "reports false for the empty text", give: ""},
			{name: "reports false for a bigint with a plus sign", give: "+1n"},
			{name: "reports false for two signs", give: "--1"},
			{name: "reports false for a sign before a string", give: `-"a"`},
			{name: "reports false for two literals", give: "1 2"},
			{name: "reports false for a keyword before another token", give: "true false"},
			{name: "reports false for a path without an enum type", give: "Color.Red"},
			{name: "reports false for a path with white space around its dot", give: "Color . Red"},
			{name: "reports false for a path that ends in a dot", give: "Color."},
			{name: "reports false for a path with two dots", give: "Color..Red"},
			{name: "reports false for a call", give: "f()"},
			{name: "reports false for a string without its closing quote", give: `"open`},
			{name: "reports false for a template literal with a substitution", give: "`a${b}`"},
			{name: "reports false for a number beyond the largest double", give: "1e400"},
			{name: "reports false for a radix mark without digits", give: "0x"},
			{name: "reports false for a hexadecimal bigint without digits", give: "0xn"},
			{name: "reports false for a character that is not a token", give: "@"},
			{name: "reports false for an operator alone", give: "+"},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, ok := tsrules.New().LiteralFor(nil, nil, tt.give, rules.View{})
				assert.False(t, ok, "the text is not a literal")
			})
		}
	})
}
