// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The enum that the evaluation cases declare, and its first member.
const (
	evalEnum  = "E"
	evalFirst = "A"
)

// TestEvaluate checks the grammar and the arithmetic of the evaluation
// of a constant enum expression. A case declares one member, so the
// enum's out-of-range value is one more than the member's value.
func TestEvaluate(t *testing.T) {
	t.Parallel()

	t.Run("evaluate", func(t *testing.T) {
		t.Parallel()

		values := []struct {
			name string
			give string
			want string
		}{
			{name: "multiplies before it adds", give: "1 + 2 * 3", want: "8"},
			{name: "evaluates a parenthesized expression first", give: "(1 + 2) * 3", want: "10"},
			{name: "subtracts from the left", give: "10 - 4 - 3", want: "4"},
			{name: "divides as doubles", give: "7 / 2", want: "4.5"},
			{name: "takes the remainder with the sign of the dividend", give: "-7 % 4", want: "-2"},
			{name: "negates a number", give: "-(2)", want: "-1"},
			{name: "evaluates a unary plus", give: "+3", want: "4"},
			{name: "inverts the bits of a 32-bit integer", give: "~1", want: "-1"},
			{name: "shifts left in 32 bits", give: "1 << 31", want: "-2147483647"},
			{name: "shifts right with the sign", give: "-16 >> 2", want: "-3"},
			{name: "shifts right without the sign", give: "-1 >>> 28", want: "16"},
			{name: "masks the shift count to 5 bits", give: "1 << 33", want: "3"},
			{name: "ands the bits", give: "5 & 3", want: "2"},
			{name: "ors the bits", give: "5 | 3", want: "8"},
			{name: "xors the bits", give: "5 ^ 3", want: "7"},
			{name: "ranks the bitwise operators by TypeScript's precedence", give: "1 | 2 ^ 3 & 1", want: "4"},
			{name: "converts an integer beyond 32 bits modulo 2^32", give: "4294967296 | 0", want: "1"},
			{name: "converts a negative integer beyond 32 bits modulo 2^32", give: "-4294967297 | 0", want: "0"},
			{name: "converts infinity to 0 in a bitwise operation", give: "(1 / 0) | 0", want: "1"},
			{name: "evaluates numbers in every radix", give: "0x10 + 0b1 + 0o1", want: "19"},
		}
		for _, tt := range values {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				info := tsrules.Rules{}.EnumOf(enumOf(evalFirst, tt.give), rules.View{})
				assert.Equal(t, info.OutOfRange.Inner.Text, tt.want,
					"the out-of-range value is one more than the member's value")
			})
		}

		t.Run("evaluates a reference to an earlier member", func(t *testing.T) {
			t.Parallel()

			e := enumOf(evalFirst, "5", "B", "A + 1", "C", evalEnum+".B * 2")
			assert.Equal(t, tsrules.Rules{}.EnumOf(e, rules.View{}).OutOfRange.Inner.Text, "13",
				"C is twice B, which is A plus one")
		})

		t.Run("joins two texts", func(t *testing.T) {
			t.Parallel()

			info := tsrules.Rules{}.EnumOf(enumOf(evalFirst, `"a" + 'b'`), rules.View{})
			assert.Equal(t, info.Variants[0].Text, emit.Literal(emit.LiteralString, "ab"),
				"the member's text is the joined text")
		})

		unknowns := []struct {
			name string
			give string
		}{
			{name: "leaves the value of an operator outside the grammar unknown", give: "2 ** 3"},
			{name: "leaves the value of a division by zero unknown", give: "1 / 0"},
			{name: "leaves the value of a reference to an unknown name unknown", give: "NaN"},
			{name: "leaves the value of a member of another enum unknown", give: "Other.A"},
			{name: "leaves the value of a path that ends in a dot unknown", give: evalEnum + "."},
			{name: "leaves the value of a unary operator on a text unknown", give: `-"a"`},
			{name: "leaves the value of a text joined to a number unknown", give: `"a" + 1`},
			{name: "leaves the value of a text subtracted from a text unknown", give: `"a" - "b"`},
			{name: "leaves the value of an unclosed parenthesis unknown", give: "(1"},
			{name: "leaves the value of a closing parenthesis alone unknown", give: "1)"},
			{name: "leaves the value of an operator without its right operand unknown", give: "1 +"},
			{name: "leaves the value of an operand followed by another unknown", give: "1 2"},
			{name: "leaves the value of a call unknown", give: `"x".length`},
		}
		for _, tt := range unknowns {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				info := tsrules.Rules{}.EnumOf(enumOf(evalFirst, tt.give), rules.View{})
				assert.True(t, info.OutOfRange.IsZero(), "no known value bounds the enum")
			})
		}
	})
}

// enumOf returns an enum E with the members of pairs. pairs lists a name
// and then its initializer for each member, and an empty initializer for
// a member without one.
func enumOf(pairs ...string) *node.Enum {
	e := &node.Enum{
		ID:   symbol.Identity{Lang: typescript.Lang, Package: aPkg, Name: evalEnum, Kind: symbol.KindEnum},
		Name: evalEnum,
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		e.Variants = append(e.Variants, &node.EnumVariant{
			ID: symbol.Identity{
				Lang: typescript.Lang, Package: aPkg, Owner: evalEnum, Name: pairs[i], Kind: symbol.KindEnumVariant,
			},
			Name:  pairs[i],
			Value: pairs[i+1],
		})
	}
	return e
}
