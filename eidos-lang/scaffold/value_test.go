// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package scaffold_test

import (
	"bytes"
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// leaves is the fixture leaf spelling: a target named scripted, whose
// absent value is none, whose strings take single quotes, and whose
// numbers take a trailing mark.
var leaves = scaffold.Leaves{
	Lang:   "scripted",
	Absent: "none",
	Quote:  func(text string) string { return "'" + text + "'" },
	Number: func(v emit.Value) (string, error) { return v.Text + "n", nil },
}

// The walk over a value tree is shared by four targets, so its
// order, its recursion and its refusals are contract for all of
// them.
func TestValue(t *testing.T) {
	t.Parallel()

	t.Run("Value", func(t *testing.T) {
		t.Parallel()

		t.Run("walks every form the vocabulary declares", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name  string
				value emit.Value
				want  string
			}{
				{"a number spells its text", emit.Literal(emit.LiteralInt, "42"), "42"},
				{"a string quotes", emit.Literal(emit.LiteralString, "hi"), `"hi"`},
				{"the absent value spells the target's", emit.Literal(emit.LiteralNil, ""), "null"},
				{
					"a conversion wraps its inner value",
					emit.Conversion(ref("svc", "Weight"), emit.Literal(emit.LiteralFloat, "1.5")),
					"Weight(1.5)",
				},
				{
					"a composite spells its named fields in order",
					emit.Composite(ref("svc", "Row"),
						emit.ValueField{Name: "ID", Value: emit.Literal(emit.LiteralInt, "1")},
						emit.ValueField{Name: "Tag", Value: emit.Literal(emit.LiteralString, "a")}),
					`Row{ID: 1, Tag: "a"}`,
				},
				{
					"a composite spells a keyed entry",
					emit.Composite(ref("svc", "Index"), emit.KeyedEntry(
						emit.Literal(emit.LiteralString, "k"), emit.Literal(emit.LiteralInt, "2"),
					)),
					`Index{"k": 2}`,
				},
				{
					"a composite spells a positional element",
					emit.Composite(ref("svc", "List"), emit.Element(emit.Literal(emit.LiteralInt, "2"))),
					"List{2}",
				},
				{
					"a raw literal spells where the target admits its language",
					emit.Raw("scripted", "Row{}"), "Row{}",
				},
				{
					"a call spells its callee and its arguments",
					emit.Call(fn("time", "Unix"),
						emit.Literal(emit.LiteralInt, "1"), emit.Literal(emit.LiteralInt, "0")),
					"Unix(1, 0)",
				},
				{"a call may take none", emit.Call(fn("time", "Now")), "Now()"},
				{
					"an address wraps its inner value",
					emit.Address(emit.Composite(ref("svc", "Row"))), "&Row{}",
				},
				{
					"the forms nest",
					emit.Address(emit.Conversion(ref("svc", "Weight"),
						emit.Call(fn("m", "Sum"), emit.Literal(emit.LiteralInt, "3")))),
					"&Weight(Sum(3))",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					got, err := scaffold.Value(&scripted{}, tt.value)
					assert.NoError(t, err, "the fixture spells")
					assert.Equal(t, got, tt.want, tt.name)
				})
			}
		})

		t.Run("records the import every reference and callee needs", func(t *testing.T) {
			t.Parallel()

			target := &scripted{}
			_, err := scaffold.Value(target, emit.Composite(ref("svc", "Row"),
				emit.ValueField{Name: "When", Value: emit.Call(fn("time", "Unix"))},
				emit.ValueField{Name: "W", Value: emit.Conversion(ref("units", "Weight"),
					emit.Literal(emit.LiteralInt, "1"))}))
			assert.NoError(t, err, "the tree spells")
			assert.Equal(t, target.imported, []string{"svc", "time", "units"},
				"every reference and callee the tree names records its import, outermost first")
		})

		t.Run("refuses a tree it cannot walk", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name    string
				value   emit.Value
				target  scaffold.Target
				mention string
				// walk marks a refusal the walk makes itself, which is a
				// value refusal the render reports under its value code.
				walk bool
			}{
				{"a kind nothing declares", emit.Value{}, &scripted{}, "no spelling", true},
				{
					"a conversion naming no type",
					emit.Value{Kind: emit.ValueConversion, Inner: &emit.Value{}},
					&scripted{}, "names no type", true,
				},
				{
					"a conversion wrapping nothing",
					emit.Value{Kind: emit.ValueConversion, Type: ref("svc", "W")},
					&scripted{}, "wraps nothing", true,
				},
				{
					"a composite naming no type",
					emit.Value{Kind: emit.ValueComposite},
					&scripted{}, "names no type", true,
				},
				{
					"a composite naming a type that spells nothing",
					emit.Composite(&emit.TypeRef{}), &scripted{}, "spells nothing", true,
				},
				{
					"an address wrapping nothing",
					emit.Value{Kind: emit.ValueAddress},
					&scripted{}, "wraps nothing", true,
				},
				{
					"a call naming no callee",
					emit.Value{Kind: emit.ValueCall},
					&scripted{}, "names no callee", true,
				},
				{
					"a call naming a function without a name",
					emit.Call(symbol.Identity{Lang: "scripted", Package: "time"}),
					&scripted{}, "spells nothing", true,
				},
				{
					"a broken value inside a composite",
					emit.Composite(ref("svc", "Row"), emit.ValueField{Name: "f", Value: emit.Value{}}),
					&scripted{}, "no spelling", true,
				},
				{
					"a broken value inside a call",
					emit.Call(fn("m", "F"), emit.Value{}), &scripted{}, "no spelling", true,
				},
				{
					"a literal the target has no form for",
					emit.Literal(emit.LiteralRaw, "Row{}"), &scripted{refuseRaw: true}, "another language", false,
				},
				{
					"a target without a conversion form",
					emit.Conversion(ref("svc", "W"), emit.Literal(emit.LiteralInt, "1")),
					&unspelling{}, "no conversion form", false,
				},
				{
					"a target without a composite form",
					emit.Composite(ref("svc", "Row")), &unspelling{}, "no composite form", false,
				},
				{
					"a target without an address form",
					emit.Address(emit.Literal(emit.LiteralInt, "1")), &unspelling{}, "no address form", false,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					_, err := scaffold.Value(tt.target, tt.value)
					assert.HasError(t, err, tt.name)
					assert.Contains(t, err.Error(), tt.mention, "the refusal names what is missing")
					if tt.walk {
						refused, isValue := errors.AsType[*render.ValueError](err)
						assert.True(t, isValue, "the walk refuses a value as a value refusal")
						assert.Equal(t, refused.Lang, "scripted", "naming the target it was spelling for")
					}
				})
			}
		})

		t.Run("writes a value through the expression writer", func(t *testing.T) {
			t.Parallel()

			var b bytes.Buffer
			expr := emit.ValueExpr(emit.Literal(emit.LiteralInt, "42"))
			assert.NoError(t, scaffold.Expr(&b, expr, &scripted{}), "a value expression spells")
			assert.Equal(t, b.String(), "42", "as the value it carries")

			b.Reset()
			outer := emit.Expr{
				Kind: emit.ExprCall, Fn: &emit.Expr{Kind: emit.ExprName, Name: "want"},
				Args: []emit.Expr{expr},
			}
			assert.NoError(t, scaffold.Expr(&b, outer, &scripted{}), "a call takes one as an argument")
			assert.Equal(t, b.String(), "want(42)", "which is what a check's call needs")

			b.Reset()
			assert.HasError(t, scaffold.Expr(&b, emit.Expr{Kind: emit.ExprValue}, &scripted{}),
				"a value expression carrying none refuses")
			assert.HasError(t, scaffold.Expr(&b, expr, nil),
				"and one without a target to spell it refuses too")
		})

		t.Run("spells the value kind and the expression kind", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, emit.ExprValue.String(), "value", "the expression kind names itself")
			assert.Equal(t, symbol.Identity{}.IsZero(), true, "a zero callee is the one a call refuses")
		})
	})

	t.Run("Leaves", func(t *testing.T) {
		t.Parallel()

		t.Run("spells every literal kind through the target's parts", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name  string
				value emit.Value
				want  string
			}{
				{"an integer through the number spelling", emit.Literal(emit.LiteralInt, "42"), "42n"},
				{"a float through the number spelling", emit.Literal(emit.LiteralFloat, "1.5"), "1.5n"},
				{"a string through the quoting", emit.Literal(emit.LiteralString, "hi"), "'hi'"},
				{"true", emit.Literal(emit.LiteralBool, "true"), "true"},
				{"false", emit.Literal(emit.LiteralBool, "false"), "false"},
				{"the absent value", emit.Literal(emit.LiteralNil, ""), "none"},
				{"raw text in the target's language", emit.Raw("scripted", "Row{}"), "Row{}"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					got, err := leaves.Literal(tt.value)
					assert.NoError(t, err, "the leaf spells")
					assert.Equal(t, got, tt.want, tt.name)
				})
			}
		})

		t.Run("writes a number as its text without a number spelling", func(t *testing.T) {
			t.Parallel()

			plain := leaves
			plain.Number = nil
			got, err := plain.Literal(emit.Literal(emit.LiteralInt, "0x2A"))
			assert.NoError(t, err, "the number spells")
			assert.Equal(t, got, "0x2A", "as the derivation wrote it")
		})

		t.Run("refuses a leaf the target cannot spell", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name    string
				value   emit.Value
				mention string
			}{
				{"a number with no text", emit.Literal(emit.LiteralInt, ""), "carries no text"},
				{"a boolean outside its two spellings", emit.Literal(emit.LiteralBool, "yes"), "true or false"},
				{
					"raw text in another language",
					emit.Raw("other", "{}"), `"{}" is written in other, not scripted`,
				},
				{"a literal kind nothing declares", emit.Value{Kind: emit.ValueLiteral}, "no spelling"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					_, err := leaves.Literal(tt.value)
					assert.HasError(t, err, tt.name)
					assert.Contains(t, err.Error(), tt.mention, "the refusal names what it cannot spell")
					refused, isValue := errors.AsType[*render.ValueError](err)
					assert.True(t, isValue, "as a value refusal")
					assert.Equal(t, refused.Lang, "scripted", "naming the target")
				})
			}
		})
	})
}
