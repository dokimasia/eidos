// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// pairOf derives a reference's pair and fails unless both derived.
func pairOf(tb assert.TB, f *fixture, ref *node.TypeRef, hint string) (emit.Value, emit.Value) {
	tb.Helper()

	sample, alternate := gorules.New().SamplesOf(ref, hint, f.view)
	assert.True(tb, sample.OK(), "the sample derives: "+sample.Refusal.String())
	assert.True(tb, alternate.OK(), "the alternate derives: "+alternate.Refusal.String())
	return sample.Value, alternate.Value
}

func TestValues(t *testing.T) {
	t.Parallel()

	t.Run("SamplesOf", func(t *testing.T) {
		t.Parallel()

		t.Run("derives the builtins from the table", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s, a := pairOf(t, f, builtin("int"), "")
			assert.Equal(t, s, emit.Literal(emit.LiteralInt, "42"), "an integer")
			assert.Equal(t, a, emit.Literal(emit.LiteralInt, "7"), "and its alternate")
			s, _ = pairOf(t, f, builtin("float64"), "")
			assert.Equal(t, s.Literal, emit.LiteralFloat, "a float")
			s, a = pairOf(t, f, builtin("bool"), "")
			assert.Equal(t, s.Text, "true", "a boolean")
			assert.Equal(t, a.Text, "false", "and its opposite")
			s, a = pairOf(t, f, builtin("string"), "name")
			assert.Equal(
				t,
				s,
				emit.Literal(emit.LiteralString, "name-a"),
				"a string carries the hint",
			)
			assert.Equal(t, a.Text, "name-b", "and differs in its suffix")
			s, _ = pairOf(t, f, builtin("string"), "")
			assert.Equal(t, s.Text, "sample-a", "a string without a hint takes the default")
			s, _ = pairOf(t, f, builtin("byte"), "")
			assert.Equal(t, s.Text, "1", "a byte stays small")
			s, a = pairOf(t, f, builtin("time.Time"), "")
			assert.Equal(t, s.Kind, emit.ValueCall, "time.Time is a call")
			assert.Equal(t, s.Callee.Package, "time", "into the time package")
			assert.True(t, s.Args[0].Text != a.Args[0].Text, "differing in its first argument")
			s, _ = pairOf(t, f, builtin("time.Duration"), "")
			assert.Equal(t, s.Kind, emit.ValueConversion, "time.Duration is a conversion")
			for _, spelling := range []string{"any", "error", "comparable"} {
				sample, _ := gorules.New().SamplesOf(builtin(spelling), "", f.view)
				assert.Equal(t, sample.Refusal, rules.RefusedNoLiteral, spelling+" has no literal")
			}
		})

		t.Run("derives composites from their children", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s, a := pairOf(t, f, composite("*int", symbol.FormOptional, builtin("int")), "")
			assert.Equal(t, s.Kind, emit.ValueAddress, "a pointer takes the address")
			assert.Equal(t, s.Inner.Text, "42", "of the inner value")
			assert.Equal(t, a.Inner.Text, "7", "differing inside")
			s, a = pairOf(t, f, composite("[]string", symbol.FormList, builtin("string")), "tag")
			assert.Equal(t, s.Kind, emit.ValueComposite, "a slice is a composite")
			assert.Length(t, s.Args, 1, "of one element")
			assert.True(t, s.Args[0].Text != a.Args[0].Text, "differing in the element")
			s, a = pairOf(
				t,
				f,
				composite("map[string]int", symbol.FormMap, builtin("string"), builtin("int")),
				"k",
			)
			assert.Length(t, s.Args, 2, "a map holds one entry, key then value")
			assert.True(t, s.Args[0].Text != a.Args[0].Text, "differing in the key")
			assert.Equal(t, s.Args[1].Text, a.Args[1].Text, "with one value")
			anyMap := composite("map[string]any", symbol.FormMap, builtin("string"), builtin("any"))
			sample, _ := gorules.New().SamplesOf(anyMap, "", f.view)
			assert.Equal(t, sample.Refusal, rules.RefusedNoLiteral, "a map refuses with its value")
			for _, ref := range []*node.TypeRef{
				composite("func()", symbol.FormFunc),
				composite("chan int", symbol.FormStream, builtin("int")),
				{Spelling: "struct{}", Form: symbol.FormInline},
			} {
				refusedSample, _ := gorules.New().SamplesOf(ref, "", f.view)
				assert.Equal(
					t,
					refusedSample.Refusal,
					rules.RefusedNoLiteral,
					ref.Spelling+" has no literal",
				)
			}
			sample, _ = gorules.New().SamplesOf(nil, "", f.view)
			assert.Equal(t, sample.Refusal, rules.RefusedNoLiteral, "no reference, no literal")
		})

		t.Run("derives workspace types from their declarations", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s, a := pairOf(t, f, ref(fxPath, "Row", symbol.KindStruct), "")
			assert.Equal(t, s.Kind, emit.ValueComposite, "a struct is a composite")
			assert.Equal(t, s.Fields[0].Name, "ID", "setting its first exported field")
			assert.True(t, s.Fields[0].Value.Text != a.Fields[0].Value.Text, "differing there")
			assert.Equal(
				t,
				s.Type.Target,
				id(fxPath, "Row", symbol.KindStruct),
				"typed by the reference",
			)
			s, _ = pairOf(t, f, ref(fxPath, "Weight", symbol.KindAlias), "")
			assert.Equal(t, s.Kind, emit.ValueConversion, "a defined type converts")
			assert.Equal(t, s.Inner.Literal, emit.LiteralFloat, "its underlying pair")
			s, _ = pairOf(t, f, ref(fxPath, "Plain", symbol.KindAlias), "")
			assert.Equal(t, s.Kind, emit.ValueLiteral, "a transparent alias is its target")
			s, a = pairOf(t, f, ref(fxPath, "Color", symbol.KindEnum), "")
			assert.Equal(t, s.Kind, emit.ValueConversion, "an enumeration converts")
			assert.Equal(
				t,
				*s.Inner,
				emit.Literal(emit.LiteralRaw, "0"),
				"the exact value the frontend stamped",
			)
			assert.Equal(t, a.Inner.Text, "1", "and the next")
			sample, _ := gorules.New().SamplesOf(ref(fxPath, "Bare", symbol.KindStruct), "", f.view)
			assert.Equal(
				t,
				sample.Refusal,
				rules.RefusedNoLiteral,
				"a struct with nothing settable has one value",
			)
			sample, _ = gorules.New().
				SamplesOf(ref(fxPath, "Reader", symbol.KindInterface), "", f.view)
			assert.Equal(t, sample.Refusal, rules.RefusedNoLiteral, "an interface has no literal")
			sample, _ = gorules.New().SamplesOf(ref(fxPath, "Ghost", symbol.KindStruct), "", f.view)
			assert.Equal(
				t,
				sample.Refusal,
				rules.RefusedUnresolved,
				"a type the view does not hold is unresolved",
			)
		})

		t.Run("stops a self-referential derivation at the budget", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			next := f.field(t, "Row", "Next")
			sample, _ := gorules.New().SamplesOf(next.Type, "", f.view)
			assert.True(t, sample.OK(),
				"a pointer to the row derives, because the row's first field is not the pointer")
			loop := &node.TypeRef{
				Spelling: "*Loop",
				Form:     symbol.FormOptional,
				Elems: []*node.TypeRef{
					{Spelling: "Loop", Target: id(fxPath, "Loop", symbol.KindStruct)},
				},
			}
			sample, _ = gorules.New().SamplesOf(loop, "", f.view)
			assert.Equal(
				t,
				sample.Refusal,
				rules.RefusedUnresolved,
				"a type outside the fixture is unresolved before any depth",
			)
		})
	})

	t.Run("ZeroValue", func(t *testing.T) {
		t.Parallel()

		f := loaded(t)
		r := gorules.New()
		zero := func(ref *node.TypeRef) emit.Value {
			v, ok := r.ZeroValue(ref, f.view)
			assert.True(t, ok, "the zero derives for "+ref.Spelling)
			return v
		}
		assert.Equal(
			t,
			zero(builtin("int")),
			emit.Literal(emit.LiteralInt, "0"),
			"an integer's zero",
		)
		assert.Equal(t, zero(builtin("float64")).Literal, emit.LiteralFloat, "a float's")
		assert.Equal(t, zero(builtin("bool")).Text, "false", "a boolean's")
		assert.Equal(t, zero(builtin("string")), emit.Literal(emit.LiteralString, ""), "a string's")
		assert.Equal(t, zero(builtin("any")).Literal, emit.LiteralNil, "an interface's")
		assert.Equal(
			t,
			zero(builtin("time.Time")).Kind,
			emit.ValueComposite,
			"time.Time's is the empty struct",
		)
		assert.Equal(
			t,
			zero(builtin("time.Duration")).Kind,
			emit.ValueConversion,
			"time.Duration's converts",
		)
		assert.Equal(
			t,
			zero(composite("*int", symbol.FormOptional, builtin("int"))).Literal,
			emit.LiteralNil,
			"a pointer's",
		)
		assert.Equal(
			t,
			zero(composite("[3]int", symbol.FormArray, builtin("int"))).Kind,
			emit.ValueComposite,
			"an array's",
		)
		assert.Equal(
			t,
			zero(ref(fxPath, "Row", symbol.KindStruct)).Kind,
			emit.ValueComposite,
			"a struct's",
		)
		assert.Equal(
			t,
			zero(ref(fxPath, "Reader", symbol.KindInterface)).Literal,
			emit.LiteralNil,
			"an interface's",
		)
		assert.Equal(
			t,
			zero(ref(fxPath, "Color", symbol.KindEnum)).Kind,
			emit.ValueConversion,
			"an enumeration's",
		)
		assert.Equal(
			t,
			zero(ref(fxPath, "Weight", symbol.KindAlias)).Kind,
			emit.ValueConversion,
			"a defined type's",
		)
		assert.Equal(
			t,
			zero(ref(fxPath, "Plain", symbol.KindAlias)).Kind,
			emit.ValueLiteral,
			"a transparent alias's",
		)
		_, ok := r.ZeroValue(builtin("Unknown"), f.view)
		assert.False(t, ok, "a spelling the rules cannot place has no zero")
		_, ok = r.ZeroValue(ref(fxPath, "Ghost", symbol.KindStruct), f.view)
		assert.False(t, ok, "nor does a type the view does not hold")
		_, ok = r.ZeroValue(nil, f.view)
		assert.False(t, ok, "nor nothing")
	})

	t.Run("LiteralFor", func(t *testing.T) {
		t.Parallel()

		r := gorules.New()
		lit := func(text string) (emit.Value, bool) { return r.LiteralFor(nil, nil, text, rules.View{}) }
		v, ok := lit("nil")
		assert.True(t, ok && v.Literal == emit.LiteralNil, "nil")
		v, ok = lit(" true ")
		assert.True(t, ok && v.Literal == emit.LiteralBool, "a boolean, whitespace trimmed")
		v, ok = lit("0x1F")
		assert.True(
			t,
			ok && v.Literal == emit.LiteralInt && v.Text == "0x1F",
			"an integer in any base",
		)
		v, ok = lit("2.5")
		assert.True(t, ok && v.Literal == emit.LiteralFloat, "a float")
		v, ok = lit(`"hi"`)
		assert.True(
			t,
			ok && v.Literal == emit.LiteralString && v.Text == "hi",
			"a quoted string, unquoted",
		)
		v, ok = lit("`raw`")
		assert.True(t, ok && v.Text == "raw", "a raw string too")
		v, ok = lit("'a'")
		assert.True(t, ok && v.Literal == emit.LiteralInt && v.Text == "97", "a rune as its value")
		_, ok = lit("Row{}")
		assert.False(t, ok, "a composite is no literal")
		_, ok = lit("")
		assert.False(t, ok, "nor is nothing")
	})
}
