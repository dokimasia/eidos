// Copyright Dokimasia B.V. 2026
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

// enumAllocs is an enumeration with stamped values: the list of its
// variants, and the out-of-range value, a conversion of the reference
// to the type.
const enumAllocs = 1 + 2

// A generated check reads an enumeration's variants, its zero, its
// duplicate and its out-of-range value through EnumOf. Each is pinned,
// with the frontend's exact values and without them.
func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("EnumOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the identifier form", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, colorInfo(t).Form, rules.EnumIdentifier, "a Go enumeration names its variants")
		})

		t.Run("returns every variant in declaration order", func(t *testing.T) {
			t.Parallel()

			info := colorInfo(t)
			assert.Length(t, info.Variants, 3, "Red, Green and Blue")
			assert.Equal(t, info.Variants[1].Name, "Green", "the second declared")
		})

		t.Run("returns each variant's name as its text", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, colorInfo(t).Variants[1].Text, emit.Literal(emit.LiteralString, "Green"),
				"the text is the name")
		})

		t.Run("returns the variant valued 0 as the zero", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, colorInfo(t).Zero, "Red", "iota starts at 0")
		})

		t.Run("returns no duplicate for distinct values", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, colorInfo(t).Duplicate, "", "no two variants share a value")
		})

		t.Run("returns a conversion of one past the largest value as out of range", func(t *testing.T) {
			t.Parallel()

			out := colorInfo(t).OutOfRange
			assert.Equal(t, out.Kind, emit.ValueConversion, "the value converts to the type")
			assert.Equal(t, out.Inner.Text, "3", "one past Blue")
		})

		t.Run("returns no foreign package for the type's own variants", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, colorInfo(t).Foreign, "every variant is the type's own")
		})

		t.Run("returns no variant for a missing enumeration", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, enumRules(t).EnumOf(nil, rules.View{}).Variants, "nothing projects to nothing")
		})

		t.Run("skips a missing variant", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, enumRules(t).EnumOf(modeEnum(), rules.View{}).Variants, 2, "On and Off")
		})

		t.Run("returns the package of a variant declared elsewhere as foreign", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, enumRules(t).EnumOf(modeEnum(), rules.View{}).Foreign, []string{"elsewhere"},
				"Off is declared in another package")
		})

		t.Run("returns no out-of-range value without values", func(t *testing.T) {
			t.Parallel()

			assert.True(t, enumRules(t).EnumOf(modeEnum(), rules.View{}).OutOfRange.IsZero(),
				"nothing is out of range without a stamped value")
		})

		t.Run("returns the first of two variants at one value as the duplicate", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, sharedInfo(t).Duplicate, "On", "On and Off share 5")
		})

		t.Run("returns one past a shared value as out of range", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, sharedInfo(t).OutOfRange.Inner.Text, "6", "the range ends past 5")
		})
	})
}

// A projection allocates its variant list and its out-of-range value.
// The ordinary run, which runs no benchmark, checks that ceiling here.
func TestEnumAllocs(t *testing.T) {
	checkAllocs(t, enumCalls(t))
}

// BenchmarkEnum measures the projection a generated check makes once
// per enumeration.
func BenchmarkEnum(b *testing.B) {
	benchCalls(b, enumCalls(b))
}

// enumCalls returns a call of EnumOf over the fixture's Color.
func enumCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := enumRules(tb)
	color, _ := f.decl(tb, id(fxPath, "Color", symbol.KindEnum)).(*node.Enum)
	var info rules.EnumInfo
	return []allocCall{
		{
			name: "EnumOf", allocs: enumAllocs,
			call:  func() { info = r.EnumOf(color, f.view) },
			check: func(tb assert.TB) { assert.Length(tb, info.Variants, 3, "EnumOf returns the three variants") },
		},
	}
}

// enumRules returns the Go rules as the enumeration capability.
func enumRules(tb assert.TB) rules.EnumRules {
	tb.Helper()

	r, is := gorules.New().(rules.EnumRules)
	assert.True(tb, is, "the Go rules project enumerations")
	return r
}

// colorInfo returns the projection of the fixture's Color, whose values
// the frontend stamped.
func colorInfo(tb assert.TB) rules.EnumInfo {
	tb.Helper()

	f := loaded(tb)
	color, is := f.decl(tb, id(fxPath, "Color", symbol.KindEnum)).(*node.Enum)
	assert.True(tb, is, "Color is an enumeration")
	return enumRules(tb).EnumOf(color, f.view)
}

// modeEnum returns an enumeration of two variants, one declared in
// another package, and a missing one.
func modeEnum() *node.Enum {
	return &node.Enum{
		ID:   id(fxPath, "Mode", symbol.KindEnum),
		Name: "Mode",
		Variants: []*node.EnumVariant{
			{ID: id(fxPath, "On", symbol.KindEnumVariant), Name: "On"},
			{ID: id("elsewhere", "Off", symbol.KindEnumVariant), Name: "Off"},
			nil,
		},
	}
}

// sharedInfo returns the projection of the two-variant enumeration with
// both variants stamped at one value.
func sharedInfo(tb assert.TB) rules.EnumInfo {
	tb.Helper()

	f := loaded(tb)
	key, held := f.constKey()
	assert.True(tb, held, "the fixture registered the constant key")
	e := modeEnum()
	for i := range 2 {
		stamp(tb, f.Facts, e.Variants[i].ID, key, "5")
	}
	return enumRules(tb).EnumOf(e, f.view)
}
