// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
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
			s, a = pairOf(t, f, builtin("float32"), "")
			assert.Equal(t, s, emit.Number(emit.LiteralFloat, "1.5", 32), "a float states its width")
			assert.Equal(t, a.Bits, 32, "and so does its alternate")
			s, _ = pairOf(t, f, builtin("int64"), "")
			assert.Equal(t, s, emit.Number(emit.LiteralInt, "42", 64), "an integer states its width")
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
			for _, spelling := range []string{"any", "error", "comparable", "complex128", "unsafe.Pointer"} {
				sample, _ := gorules.New().SamplesOf(builtin(spelling), "", f.view)
				assert.Equal(t, sample.Refusal, rules.RefusedNoLiteral, spelling+" has no literal")
			}
			for _, spelling := range []string{"uuid.UUID", "Row"} {
				sample, _ := gorules.New().SamplesOf(builtin(spelling), "", f.view)
				assert.Equal(t, sample.Refusal, rules.RefusedUnresolved,
					spelling+" names a type the view does not hold")
			}
		})

		t.Run("derives composites from their children", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			s, a := pairOf(t, f, composite("*Row", symbol.FormOptional, ref(fxPath, "Row", symbol.KindStruct)), "")
			assert.Equal(t, s.Kind, emit.ValueAddress, "a pointer takes the address")
			assert.Equal(t, s.Inner.Kind, emit.ValueComposite, "of the inner composite")
			assert.True(t, s.Inner.Fields[0].Value.Text != a.Inner.Fields[0].Value.Text, "differing inside")
			pointer, _ := gorules.New().SamplesOf(composite("*int", symbol.FormOptional, builtin("int")), "", f.view)
			assert.Equal(t, pointer.Refusal, rules.RefusedNoLiteral,
				"a pointer to a builtin refuses, because Go takes no address of a literal")
			s, a = pairOf(t, f, composite("[]string", symbol.FormList, builtin("string")), "tag")
			assert.Equal(t, s.Kind, emit.ValueComposite, "a slice is a composite")
			assert.Length(t, s.Fields, 1, "of one element")
			assert.Equal(t, s.Fields[0].Name, "", "positional, because a list names nothing")
			assert.True(t, s.Fields[0].Value.Text != a.Fields[0].Value.Text, "differing in the element")
			s, a = pairOf(
				t,
				f,
				composite("map[string]int", symbol.FormMap, builtin("string"), builtin("int")),
				"k",
			)
			assert.Length(t, s.Fields, 1, "a map holds one keyed entry")
			assert.NotNil(t, s.Fields[0].Key, "which states its key")
			assert.True(t, s.Fields[0].Key.Text != a.Fields[0].Key.Text, "differing in the key")
			assert.Equal(t, s.Fields[0].Value.Text, a.Fields[0].Value.Text, "with one value")
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
				emit.Raw(golang.Lang, "0"),
				"the exact value the frontend stamped, as Go text",
			)
			assert.Equal(t, a.Inner.Text, "1", "and the next")
			s, _ = pairOf(t, f, ref(fxPath, "Mode", symbol.KindEnum), "")
			assert.Equal(t, *s.Inner, emit.Raw(golang.Lang, `"read"`), "a string enumeration's too")
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
		assert.Equal(t, zero(builtin("float32")), emit.Number(emit.LiteralFloat, "0", 32),
			"a float's at its width")
		assert.Equal(t, zero(builtin("unsafe.Pointer")).Literal, emit.LiteralNil, "an unsafe pointer's")
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
			zero(ref(fxPath, "Color", symbol.KindEnum)),
			emit.Conversion(rules.EmitRef(ref(fxPath, "Color", symbol.KindEnum)), emit.Literal(emit.LiteralInt, "0")),
			"an enumeration's converts its underlying zero",
		)
		assert.Equal(
			t,
			*zero(ref(fxPath, "Mode", symbol.KindEnum)).Inner,
			emit.Literal(emit.LiteralString, ""),
			"a string enumeration's zero is the empty string",
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

		t.Run("writes every Go literal as canonical decimal text", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				text string
				want emit.Value
			}{
				{"nil", emit.Literal(emit.LiteralNil, "")},
				{" true ", emit.Literal(emit.LiteralBool, "true")},
				{"false", emit.Literal(emit.LiteralBool, "false")},
				{"42", emit.Literal(emit.LiteralInt, "42")},
				{"0x1F", emit.Literal(emit.LiteralInt, "31")},
				{"017", emit.Literal(emit.LiteralInt, "15")},
				{"0o17", emit.Literal(emit.LiteralInt, "15")},
				{"0b101", emit.Literal(emit.LiteralInt, "5")},
				{"1_000", emit.Literal(emit.LiteralInt, "1000")},
				{"-5", emit.Literal(emit.LiteralInt, "-5")},
				{"+3", emit.Literal(emit.LiteralInt, "3")},
				{"18446744073709551615", emit.Literal(emit.LiteralInt, "18446744073709551615")},
				{"2.5", emit.Literal(emit.LiteralFloat, "2.5")},
				{"- 2.5", emit.Literal(emit.LiteralFloat, "-2.5")},
				{"0x1p-2", emit.Literal(emit.LiteralFloat, "0.25")},
				{"1_000.5", emit.Literal(emit.LiteralFloat, "1000.5")},
				{"1e21", emit.Literal(emit.LiteralFloat, "1e+21")},
				{"1e-7", emit.Literal(emit.LiteralFloat, "1e-7")},
				{"0.000001", emit.Literal(emit.LiteralFloat, "0.000001")},
				{`"hi"`, emit.Literal(emit.LiteralString, "hi")},
				{"`raw`", emit.Literal(emit.LiteralString, "raw")},
				{`"tab\t"`, emit.Literal(emit.LiteralString, "tab\t")},
				{"'a'", emit.Literal(emit.LiteralInt, "97")},
			}
			for _, tt := range tests {
				got, ok := gorules.New().LiteralFor(nil, nil, tt.text, rules.View{})
				assert.True(t, ok, tt.text+" is a literal")
				assert.Equal(t, got, tt.want, tt.text)
			}
		})

		t.Run("refuses text that is no single literal", func(t *testing.T) {
			t.Parallel()

			for _, text := range []string{
				"", "Row{}", "Inf", "NaN", "1i", `"a" + "b"`, "1 // note", "-'a'", "--1", "x", "1e400",
			} {
				_, ok := gorules.New().LiteralFor(nil, nil, text, rules.View{})
				assert.False(t, ok, text+" is no literal")
			}
		})

		t.Run("types the value by the builtin its reference names", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			tests := []struct {
				ref  *node.TypeRef
				text string
				want emit.Value
			}{
				{builtin("int8"), "127", emit.Number(emit.LiteralInt, "127", 8)},
				{builtin("int8"), "-128", emit.Number(emit.LiteralInt, "-128", 8)},
				{builtin("uint8"), "255", emit.Number(emit.LiteralInt, "255", 8)},
				{builtin("uint64"), "18446744073709551615", emit.Number(emit.LiteralInt, "18446744073709551615", 64)},
				{builtin("int"), "2.0", emit.Number(emit.LiteralInt, "2", 0)},
				{builtin("rune"), "'a'", emit.Number(emit.LiteralInt, "97", 32)},
				{builtin("float32"), "0.1", emit.Number(emit.LiteralFloat, "0.1", 32)},
				{builtin("float32"), "3", emit.Number(emit.LiteralFloat, "3", 32)},
				{builtin("float64"), "0x1p-2", emit.Number(emit.LiteralFloat, "0.25", 64)},
				{builtin("bool"), "true", emit.Literal(emit.LiteralBool, "true")},
				{builtin("string"), `"x"`, emit.Literal(emit.LiteralString, "x")},
				{builtin("any"), "nil", emit.Literal(emit.LiteralNil, "")},
				{composite("*int", symbol.FormOptional, builtin("int")), "nil", emit.Literal(emit.LiteralNil, "")},
				{ref(fxPath, "Weight", symbol.KindAlias), "1.5", emit.Number(emit.LiteralFloat, "1.5", 64)},
				{ref(fxPath, "Plain", symbol.KindAlias), "7", emit.Number(emit.LiteralInt, "7", 0)},
			}
			for _, tt := range tests {
				got, ok := gorules.New().LiteralFor(f.file, tt.ref, tt.text, f.view)
				assert.True(t, ok, tt.text+" is a "+tt.ref.Spelling)
				assert.Equal(t, got, tt.want, tt.text+" as "+tt.ref.Spelling)
			}
		})

		t.Run("refuses a value outside the builtin", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			tests := []struct {
				ref  *node.TypeRef
				text string
			}{
				{builtin("int8"), "128"},
				{builtin("int8"), "-129"},
				{builtin("uint8"), "-1"},
				{builtin("uint64"), "18446744073709551616"},
				{builtin("int"), "2.5"},
				{builtin("int"), `"2"`},
				{builtin("int"), "nil"},
				{builtin("float32"), "1e39"},
				{builtin("float64"), "true"},
				{builtin("bool"), "1"},
				{builtin("string"), "'a'"},
				{ref(fxPath, "Weight", symbol.KindAlias), `"heavy"`},
			}
			for _, tt := range tests {
				_, ok := gorules.New().LiteralFor(f.file, tt.ref, tt.text, f.view)
				assert.False(t, ok, tt.text+" is no "+tt.ref.Spelling)
			}
		})
	})
}
