// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// enumAllocs counts the projection of an enum of three members. The
// projection allocates the list of members, the list of variants, the
// reference to the enum, the conversion, and the decimal text of the
// out-of-range number.
const enumAllocs = 5

// TestEnum checks the form, the texts and the derived facts of the
// projection of each kind of TypeScript enum.
func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("EnumOf", func(t *testing.T) {
		t.Parallel()

		t.Run("projects a numeric enum by its members' names", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			color, _ := f.decl(t, aPkg, "Color", symbol.KindEnum).(*node.Enum)
			info := tsrules.Rules{}.EnumOf(color, f.view)
			expect.Equal(t, info.Form, rules.EnumIdentifier, "a member's text is its name")
			expect.Equal(t, info.Variants, []rules.VariantText{
				{Name: "Red", Text: emit.Literal(emit.LiteralString, "Red")},
				{Name: "Green", Text: emit.Literal(emit.LiteralString, "Green")},
				{Name: "Blue", Text: emit.Literal(emit.LiteralString, "Blue")},
			}, "the variants are the members in declaration order")
			expect.Equal(t, info.Zero, "Red", "the first member is 0")
			expect.Empty(t, info.Duplicate, "no two members share a value")
			assert.Equal(t, info.OutOfRange.Kind, emit.ValueConversion,
				"the out-of-range value is asserted to the enum")
			expect.Equal(t, info.OutOfRange.Type.Target, color.ID, "the assertion is to the enum's type")
			expect.Equal(t, *info.OutOfRange.Inner, emit.Number(emit.LiteralFloat, "3", 64),
				"the value is one more than the largest value")
			expect.Empty(t, info.Foreign, "every member is in the enum's package")
		})

		t.Run("projects a string enum by its members' values", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			mode, _ := f.decl(t, aPkg, "Mode", symbol.KindEnum).(*node.Enum)
			info := tsrules.Rules{}.EnumOf(mode, f.view)
			expect.Equal(t, info.Form, rules.EnumValue, "a member's text is its value")
			expect.Equal(t, info.Variants, []rules.VariantText{
				{Name: "Read", Text: emit.Literal(emit.LiteralString, "read")},
				{Name: "Write", Text: emit.Literal(emit.LiteralString, "write")},
			}, "the variants are the members in declaration order")
			expect.Empty(t, info.Zero, "no member has the empty text")
			assert.Equal(t, info.OutOfRange.Kind, emit.ValueConversion,
				"the out-of-range value is asserted to the enum")
			expect.Equal(t, *info.OutOfRange.Inner, emit.Literal(emit.LiteralString, ""),
				"the value is the empty text, which no member has")
		})

		t.Run("projects an enum of constant expressions by their values", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			flags, _ := f.decl(t, aPkg, "Flags", symbol.KindEnum).(*node.Enum)
			info := tsrules.Rules{}.EnumOf(flags, f.view)
			expect.Equal(t, info.Zero, "None", "None is 0")
			expect.Empty(t, info.Duplicate, "AB is A | B, and no other member has that value")
			expect.Equal(t, info.OutOfRange.Inner.Text, "4", "the value is one more than AB, which is 3")
		})

		t.Run("reports the first member whose value a following member repeats", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			same, _ := f.decl(t, aPkg, "Same", symbol.KindEnum).(*node.Enum)
			assert.Equal(t, tsrules.Rules{}.EnumOf(same, f.view).Duplicate, "One", "Uno repeats the value of One")
		})

		t.Run("reports the first member of a string enum whose text a following member repeats", func(t *testing.T) {
			t.Parallel()

			info := tsrules.Rules{}.EnumOf(enumOf("A", `"x"`, "B", `"y"`, "C", `"x"`), rules.View{})
			expect.Equal(t, info.Form, rules.EnumValue, "every member has a text")
			expect.Equal(t, info.Duplicate, "A", "C repeats the text of A")
		})

		t.Run("reports the member of a string enum whose text is empty as the zero", func(t *testing.T) {
			t.Parallel()

			info := tsrules.Rules{}.EnumOf(enumOf("Empty", `""`, "Full", `"x"`), rules.View{})
			expect.Equal(t, info.Zero, "Empty", "the empty text is a string's zero")
			expect.True(t, info.OutOfRange.IsZero(),
				"a member has the empty text, so EnumOf derives no out-of-range text")
		})

		t.Run("numbers a member without an initializer from the member before it", func(t *testing.T) {
			t.Parallel()

			info := tsrules.Rules{}.EnumOf(enumOf("A", "5", "B", ""), rules.View{})
			expect.Empty(t, info.Zero, "no member is 0")
			expect.Equal(t, info.OutOfRange.Inner.Text, "7", "the value is one more than B, which is 6")
		})

		t.Run("leaves the value of a member without an initializer after a text unknown", func(t *testing.T) {
			t.Parallel()

			info := tsrules.Rules{}.EnumOf(enumOf("A", `"x"`, "B", ""), rules.View{})
			expect.Equal(t, info.Form, rules.EnumIdentifier, "B has no text")
			expect.True(t, info.OutOfRange.IsZero(), "no member has a number")
		})

		t.Run("leaves a member of an unknown value out of the derived facts", func(t *testing.T) {
			t.Parallel()

			info := tsrules.Rules{}.EnumOf(enumOf("A", "1", "B", "f()", "C", "1"), rules.View{})
			expect.Equal(t, info.Duplicate, "A", "C repeats A, and B has no value to repeat")
			expect.Equal(t, info.OutOfRange.Inner.Text, "2", "the value is one more than the largest known value")
		})

		t.Run("skips a nil member", func(t *testing.T) {
			t.Parallel()

			e := enumOf("A", "")
			e.Variants = append([]*node.EnumVariant{nil}, e.Variants...)
			info := tsrules.Rules{}.EnumOf(e, rules.View{})
			expect.Length(t, info.Variants, 1, "the variants have the one member")
			expect.Equal(t, info.Zero, "A", "the zero is that member")
		})

		t.Run("lists the other packages that declare a member in sorted order", func(t *testing.T) {
			t.Parallel()

			e := enumOf("A", "", "B", "", "C", "")
			e.Variants[0].ID.Package = "fx/z"
			e.Variants[1].ID.Package = "fx/b"
			e.Variants[2].ID.Package = "fx/z"
			assert.Equal(t, tsrules.Rules{}.EnumOf(e, rules.View{}).Foreign, []string{"fx/b", "fx/z"},
				"the list has each package once, in sorted order")
		})

		t.Run("returns the identifier form alone for a nil enum", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tsrules.Rules{}.EnumOf(nil, rules.View{}), rules.EnumInfo{Form: rules.EnumIdentifier},
				"EnumOf returns an empty info")
		})
	})
}

// TestEnumAllocs checks the allocation ceiling of the projection of an
// enum. The ordinary test run runs no benchmark, so this test checks the
// ceiling.
func TestEnumAllocs(t *testing.T) {
	checkAllocs(t, enumCalls())
}

// BenchmarkEnum measures the projection of an enum under its ceiling.
func BenchmarkEnum(b *testing.B) {
	benchCalls(b, enumCalls())
}

// enumCalls returns the measured call of enum.go. The call projects an
// enum of three members whose largest value is above 99, so the decimal
// text of the out-of-range number is not a constant string of strconv.
func enumCalls() []allocCall {
	r := tsrules.Rules{}
	e := enumOf("A", "100", "B", "", "C", "")
	var info rules.EnumInfo
	return []allocCall{{
		name:   "EnumOf",
		allocs: enumAllocs,
		call:   func() { info = r.EnumOf(e, rules.View{}) },
		check: func(tb assert.TB) {
			assert.Equal(tb, info.OutOfRange.Inner.Text, "103", "EnumOf derives one more than the largest value")
		},
	}}
}
