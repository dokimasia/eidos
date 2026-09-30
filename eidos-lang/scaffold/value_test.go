// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package scaffold_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// scriptedLang is the language the fixture targets name in every
// refusal.
const scriptedLang = "scripted"

// leaves is the fixture leaf spelling: a target named scripted, whose
// absent value is none, whose strings take single quotes, and whose
// numbers take a trailing mark.
var leaves = scaffold.Leaves{
	Lang:   scriptedLang,
	Absent: "none",
	Quote:  func(text string) string { return "'" + text + "'" },
	Number: func(v emit.Value) (string, error) { return v.Text + "n", nil },
}

// integer returns an integer literal.
func integer(text string) emit.Value { return emit.Literal(emit.LiteralInt, text) }

// The walk over a value tree is shared by four targets, so its
// order, its recursion and its refusals are contract for all of
// them.
func TestValue(t *testing.T) {
	t.Parallel()

	t.Run("Value", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give emit.Value
			want string
		}{
			{name: "writes a number as its text", give: integer("42"), want: "42"},
			{name: "writes a string in quotes", give: emit.Literal(emit.LiteralString, "hi"), want: `"hi"`},
			{
				name: "writes the absent value as the target spells it",
				give: emit.Literal(emit.LiteralNil, ""), want: "null",
			},
			{
				name: "writes a conversion around its inner value",
				give: emit.Conversion(ref("svc", "Weight"), emit.Literal(emit.LiteralFloat, "1.5")),
				want: "Weight(1.5)",
			},
			{
				name: "writes a composite's named fields in order",
				give: emit.Composite(ref("svc", "Row"),
					emit.ValueField{Name: "ID", Value: integer("1")},
					emit.ValueField{Name: "Tag", Value: emit.Literal(emit.LiteralString, "a")}),
				want: `Row{ID: 1, Tag: "a"}`,
			},
			{
				name: "writes a composite's keyed entry",
				give: emit.Composite(ref("svc", "Index"),
					emit.KeyedEntry(emit.Literal(emit.LiteralString, "k"), integer("2"))),
				want: `Index{"k": 2}`,
			},
			{
				name: "writes a composite's positional element",
				give: emit.Composite(ref("svc", "List"), emit.Element(integer("2"))),
				want: "List{2}",
			},
			{name: "writes raw text in the target's language", give: emit.Raw(scriptedLang, "Row{}"), want: "Row{}"},
			{
				name: "writes a call with its arguments",
				give: emit.Call(fn("time", "Unix"), integer("1"), integer("0")),
				want: "Unix(1, 0)",
			},
			{name: "writes a call without arguments", give: emit.Call(fn("time", "Now")), want: "Now()"},
			{
				name: "writes an address around its inner value",
				give: emit.Address(emit.Composite(ref("svc", "Row"))), want: "&Row{}",
			},
			{
				name: "writes nested forms",
				give: emit.Address(emit.Conversion(ref("svc", "Weight"),
					emit.Call(fn("m", "Sum"), integer("3")))),
				want: "&Weight(Sum(3))",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := scaffold.Value(&scripted{}, tt.give)
				assert.NoError(t, err, "the fixture spells")
				assert.Equal(t, got, tt.want, "the spelling")
			})
		}

		t.Run("passes Address the value it takes the address of", func(t *testing.T) {
			t.Parallel()

			target := &scripted{}
			_, err := scaffold.Value(target, emit.Address(emit.Composite(ref("svc", "Row"))))
			assert.NoError(t, err, "the tree spells")
			assert.Equal(t, target.addressed, []emit.ValueKind{emit.ValueComposite},
				"the target reads the inner value's kind, not only its spelling")
		})

		t.Run("records the imports the tree needs in walk order", func(t *testing.T) {
			t.Parallel()

			target := &scripted{}
			_, err := scaffold.Value(target, emit.Composite(ref("svc", "Row"),
				emit.ValueField{Name: "When", Value: emit.Call(fn("time", "Unix"))},
				emit.ValueField{Name: "W", Value: emit.Conversion(ref("units", "Weight"), integer("1"))}))
			assert.NoError(t, err, "the tree spells")
			assert.Equal(t, target.imported, []string{"svc", "time", "units"},
				"every reference and callee records its import, the outermost first")
		})

		walkErrors := []struct {
			name string
			give emit.Value
			want string
		}{
			{name: "returns a value error for a kind nothing declares", give: emit.Value{}, want: "no spelling"},
			{
				name: "returns a value error for a conversion that names no type",
				give: emit.Value{Kind: emit.ValueConversion, Inner: &emit.Value{}}, want: "names no type",
			},
			{
				name: "returns a value error for a conversion that wraps nothing",
				give: emit.Value{Kind: emit.ValueConversion, Type: ref("svc", "W")}, want: "wraps nothing",
			},
			{
				name: "returns a value error for a composite that names no type",
				give: emit.Value{Kind: emit.ValueComposite}, want: "names no type",
			},
			{
				name: "returns a value error for a composite whose type spells nothing",
				give: emit.Composite(&emit.TypeRef{}), want: "spells nothing",
			},
			{
				name: "returns a value error for an address that wraps nothing",
				give: emit.Value{Kind: emit.ValueAddress}, want: "wraps nothing",
			},
			{
				name: "returns a value error for a call that names no callee",
				give: emit.Value{Kind: emit.ValueCall}, want: "names no callee",
			},
			{
				name: "returns a value error for a call of a function without a name",
				give: emit.Call(symbol.Identity{Lang: scriptedLang, Package: "time"}), want: "spells nothing",
			},
			{
				name: "returns a value error for a broken value inside a composite",
				give: emit.Composite(ref("svc", "Row"), emit.ValueField{Name: "f", Value: emit.Value{}}),
				want: "no spelling",
			},
			{
				name: "returns a value error for a broken value inside a call",
				give: emit.Call(fn("m", "F"), emit.Value{}), want: "no spelling",
			},
		}
		for _, tt := range walkErrors {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := scaffold.Value(&scripted{}, tt.give)
				refused, isValue := errors.AsType[*render.ValueError](err)
				assert.True(t, isValue, "the walk refuses a value as a value refusal")
				assert.Contains(t, err.Error(), tt.want, "the refusal names what is missing")
				assert.Equal(t, refused.Lang, scriptedLang, "naming the target it was spelling for")
			})
		}

		targetErrors := []struct {
			name   string
			give   emit.Value
			target scaffold.Target
			want   string
		}{
			{
				name:   "returns the target's error for a literal it has no form for",
				give:   emit.Literal(emit.LiteralRaw, "Row{}"),
				target: &scripted{refuseRaw: true}, want: "another language",
			},
			{
				name:   "returns the target's error for a conversion it has no form for",
				give:   emit.Conversion(ref("svc", "W"), integer("1")),
				target: &unspelling{}, want: "no conversion form",
			},
			{
				name:   "returns the target's error for a composite it has no form for",
				give:   emit.Composite(ref("svc", "Row")),
				target: &unspelling{}, want: "no composite form",
			},
			{
				name:   "returns the target's error for an address it has no form for",
				give:   emit.Address(integer("1")),
				target: &unspelling{}, want: "no address form",
			},
		}
		for _, tt := range targetErrors {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := scaffold.Value(tt.target, tt.give)
				assert.HasError(t, err, "the target refuses")
				assert.Contains(t, err.Error(), tt.want, "the target's refusal passes through")
			})
		}
	})

	t.Run("Literal", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give emit.Value
			want string
		}{
			{name: "writes an integer through the number spelling", give: integer("42"), want: "42n"},
			{
				name: "writes a float through the number spelling",
				give: emit.Literal(emit.LiteralFloat, "1.5"), want: "1.5n",
			},
			{name: "writes a string through the quoting", give: emit.Literal(emit.LiteralString, "hi"), want: "'hi'"},
			{name: "writes true as itself", give: emit.Literal(emit.LiteralBool, "true"), want: "true"},
			{name: "writes false as itself", give: emit.Literal(emit.LiteralBool, "false"), want: "false"},
			{name: "writes the absent value as the target's", give: emit.Literal(emit.LiteralNil, ""), want: "none"},
			{name: "writes raw text in the target's language", give: emit.Raw(scriptedLang, "Row{}"), want: "Row{}"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := leaves.Literal(tt.give)
				assert.NoError(t, err, "the leaf spells")
				assert.Equal(t, got, tt.want, "the spelling")
			})
		}

		t.Run("writes a number as its text without a number spelling", func(t *testing.T) {
			t.Parallel()

			plain := leaves
			plain.Number = nil
			got, err := plain.Literal(integer("0x2A"))
			assert.NoError(t, err, "the number spells")
			assert.Equal(t, got, "0x2A", "as the derivation wrote it")
		})

		refusals := []struct {
			name string
			give emit.Value
			want string
		}{
			{name: "returns a value error for a number without text", give: integer(""), want: "has no text"},
			{
				name: "returns a value error for a boolean outside its two spellings",
				give: emit.Literal(emit.LiteralBool, "yes"), want: "true or false",
			},
			{
				name: "returns a value error for raw text in another language",
				give: emit.Raw("other", "{}"), want: `"{}" is written in other, not scripted`,
			},
			{
				name: "returns a value error for a literal kind nothing declares",
				give: emit.Value{Kind: emit.ValueLiteral}, want: "no spelling",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := leaves.Literal(tt.give)
				refused, isValue := errors.AsType[*render.ValueError](err)
				assert.True(t, isValue, "a leaf refuses as a value refusal")
				assert.Contains(t, err.Error(), tt.want, "the refusal names what it cannot spell")
				assert.Equal(t, refused.Lang, scriptedLang, "naming the target")
			})
		}
	})
}
