// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// enumAllocs is the count of an enumeration with stamped values: the list
// of its variants, and the out-of-range value, which is a conversion of
// the reference to the type.
const enumAllocs = 1 + 2

// levelName is the name of the enumeration of the cases that state an
// underlying type.
const levelName = "Level"

// TestEnum pins the variants, the zero, the duplicate and the
// out-of-range value that EnumOf returns, with the exact values of the
// frontend and without them. A generated check reads each of them.
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

		bounded := []struct {
			name       string
			underlying string
			values     []string
			want       string
		}{
			{
				name:       "returns one past the largest value that the underlying type holds",
				underlying: "uint8", values: []string{"0", "254"}, want: "255",
			},
			{
				name:       "returns one below the smallest value where the largest value fills the underlying type",
				underlying: "uint8", values: []string{"1", "255"}, want: "0",
			},
			{
				name:       "returns one below the smallest value of a signed set that fills the top of its type",
				underlying: "int8", values: []string{"-5", "127"}, want: "-6",
			},
			{
				name:       "returns one past the largest value of an int within 32 bits",
				underlying: "int", values: []string{"0", "2147483646"}, want: "2147483647",
			},
			{
				name:       "returns one below the smallest value of an int that fills 32 bits at its top",
				underlying: "int", values: []string{"0", "2147483647"}, want: "-1",
			},
			{
				name:       "returns one past the largest value of a uint64 within int64",
				underlying: "uint64", values: []string{"0", "9223372036854775806"}, want: "9223372036854775807",
			},
			{
				name:       "returns one past the largest value of a float type",
				underlying: "float64", values: []string{"0", "1"}, want: "2",
			},
		}
		for _, tt := range bounded {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				out := boundedInfo(t, tt.underlying, tt.values...).OutOfRange
				assert.Equal(t, out.Kind, emit.ValueConversion, "the value converts to the type")
				assert.Equal(t, out.Inner.Text, tt.want, "the underlying type holds the out-of-range value")
			})
		}

		filled := []struct {
			name       string
			underlying string
			values     []string
		}{
			{
				name:       "returns no out-of-range value for a set at both ends of uint8",
				underlying: "uint8", values: []string{"0", "255"},
			},
			{
				name:       "returns no out-of-range value for a set at both ends of int64",
				underlying: "int64", values: []string{"-9223372036854775808", "9223372036854775807"},
			},
			{
				name:       "returns no out-of-range value for a set at both ends of uint64 within int64",
				underlying: "uint64", values: []string{"0", "9223372036854775807"},
			},
		}
		for _, tt := range filled {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.True(t, boundedInfo(t, tt.underlying, tt.values...).OutOfRange.IsZero(),
					"the underlying type holds no value outside the range of the set")
			})
		}
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

// boundedInfo returns the projection of an enumeration over the
// underlying type of the spelling underlying. The fixture stamps one
// variant with each of values.
func boundedInfo(tb assert.TB, underlying string, values ...string) rules.EnumInfo {
	tb.Helper()

	f := loaded(tb)
	key, held := f.constKey()
	assert.True(tb, held, "the fixture registered the constant key")
	e := &node.Enum{
		ID: id(fxPath, levelName, symbol.KindEnum), Name: levelName,
		Underlying: &node.TypeRef{Spelling: underlying},
	}
	for i, value := range values {
		name := levelName + strconv.Itoa(i)
		variant := &node.EnumVariant{ID: id(fxPath, name, symbol.KindEnumVariant), Name: name}
		e.Variants = append(e.Variants, variant)
		stamp(tb, f.Facts, variant.ID, key, value)
	}
	return enumRules(tb).EnumOf(e, f.view)
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
