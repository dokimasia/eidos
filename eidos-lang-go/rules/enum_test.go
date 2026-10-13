// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

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

// The exact values that the frontend stamps for the cases: two strings,
// the empty string, the truth values, two fractions and the bounds of
// float32.
const (
	readValue  = `"read"`
	writeValue = `"write"`
	emptyValue = `""`
	longValue  = `"ab"`
	trueValue  = "true"
	falseValue = "false"
	halfValue  = "1/2"
	threeHalf  = "3/2"
	maxFloat32 = "340282346638528859811704183484516925440"
	minFloat32 = "-340282346638528859811704183484516925440"
	pastInt64  = "9223372036854775808"
)

// hugeValue is the exact value 10^300. A float64 near it has 17
// significant digits, so the sum with one rounds back to it.
var hugeValue = "1" + strings.Repeat("0", 300)

// The probes that EnumOf returns for the cases: the empty text, a text of
// question marks one byte longer than "ab", and the float values past the
// cases.
const (
	longerProbe   = "???"
	pastThreeHalf = "2.5"
	belowZero     = "-1"
	pastHuge      = "1.0000000000000002e+300"
)

// The names of the enumerations and the variants of the identifier cases.
const (
	statusName   = "Status"
	activeName   = "StatusActive"
	phaseName    = "Phase"
	unsetName    = "Phase_UNSPECIFIED"
	colorName    = "Color"
	colorlessNm  = "Colorless"
	trailingName = "Status_"
	lowerName    = "level"
	lowerVariant = "levelLow"
)

// TestEnum pins the form, the texts, the zero, the duplicate and the
// out-of-range value that EnumOf returns, with the exact values of the
// frontend and without them. A generated check reads each of them.
func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("EnumOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the identifier form for an enumeration over an integer", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, colorInfo(t).Form, rules.EnumIdentifier, "a Go enumeration names its variants")
		})

		t.Run("returns the value form for an enumeration over a string", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fixtureInfo(t, "Mode").Form, rules.EnumValue, "the declared strings are the texts")
		})

		t.Run("returns every variant in declaration order", func(t *testing.T) {
			t.Parallel()

			info := colorInfo(t)
			assert.Length(t, info.Variants, 3, "Red, Green and Blue")
			assert.Equal(t, info.Variants[1].Name, "Green", "the second declared")
		})

		t.Run("returns the declared value of a variant as its text for a string enumeration", func(t *testing.T) {
			t.Parallel()

			info := fixtureInfo(t, "Mode")
			assert.Length(t, info.Variants, 2, "ModeRead and ModeWrite")
			assert.Equal(t, info.Variants[0].Text, emit.Literal(emit.LiteralString, "read"),
				"the text is the declared value")
		})

		t.Run("returns no text for a variant of an enumeration over a string without a value", func(t *testing.T) {
			t.Parallel()

			info := levelInfo(t, builtin(stringSpelling), readValue, "")
			assert.Length(t, info.Variants, 2, "both variants")
			assert.True(t, info.Variants[1].Text.IsZero(), "the second variant has no evaluated value")
		})

		texts := []struct {
			name     string
			giveType string
			giveName string
			want     string
		}{
			{
				name:     "returns the name without the name of the type as the text",
				giveType: statusName, giveName: activeName, want: "Active",
			},
			{
				name:     "returns the name without the name of the type and an underscore as the text",
				giveType: phaseName, giveName: unsetName, want: "UNSPECIFIED",
			},
			{
				name:     "returns the name without the name of an unexported type as the text",
				giveType: lowerName, giveName: lowerVariant, want: "Low",
			},
			{
				name:     "returns the whole name where a lower-case letter follows the name of the type",
				giveType: colorName, giveName: colorlessNm, want: colorlessNm,
			},
			{
				name:     "returns the whole name where only an underscore follows the name of the type",
				giveType: statusName, giveName: trailingName, want: trailingName,
			},
			{
				name:     "returns the whole name where it does not start with the name of the type",
				giveType: colorName, giveName: "Red", want: "Red",
			},
		}
		for _, tt := range texts {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				e := &node.Enum{
					ID: id(fxPath, tt.giveType, symbol.KindEnum), Name: tt.giveType,
					Underlying: builtin(intSpelling),
					Variants:   []*node.EnumVariant{{Name: tt.giveName}},
				}
				info := enumRules(t).EnumOf(e, rules.View{})
				assert.Length(t, info.Variants, 1, "the one variant")
				assert.Equal(t, info.Variants[0].Text, emit.Literal(emit.LiteralString, tt.want), "the text")
			})
		}

		zeros := []struct {
			name      string
			giveUnder *node.TypeRef
			giveVals  []string
			want      string
		}{
			{
				name:      "returns the variant valued 0 as the zero of an enumeration over an integer",
				giveUnder: builtin(intSpelling), giveVals: []string{"1", "0"}, want: levelName + "1",
			},
			{
				name:      "returns the variant valued 0 as the zero of an enumeration over a float",
				giveUnder: builtin(float64Spelling), giveVals: []string{halfValue, "0"}, want: levelName + "1",
			},
			{
				name:      "returns the variant with the empty string as the zero of an enumeration over a string",
				giveUnder: builtin(stringSpelling), giveVals: []string{readValue, emptyValue}, want: levelName + "1",
			},
			{
				name:      "returns the variant with false as the zero of an enumeration over a bool",
				giveUnder: builtin(boolSpelling), giveVals: []string{trueValue, falseValue}, want: levelName + "1",
			},
			{
				name:      "returns no zero for an enumeration over a string without the empty string",
				giveUnder: builtin(stringSpelling), giveVals: []string{readValue, writeValue}, want: "",
			},
		}
		for _, tt := range zeros {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, levelInfo(t, tt.giveUnder, tt.giveVals...).Zero, tt.want, "the zero variant")
			})
		}

		t.Run("returns the variant valued 0 of the fixture as the zero", func(t *testing.T) {
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

		t.Run("returns no out-of-range value where a variant has no value", func(t *testing.T) {
			t.Parallel()

			assert.True(t, levelInfo(t, builtin(intSpelling), "0", "").OutOfRange.IsZero(),
				"the variant without a value can have any value")
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
				underlying: float64Spelling, values: []string{"0", "1"}, want: "2",
			},
			{
				name:       "returns one past the largest fractional value of a float type",
				underlying: float64Spelling, values: []string{halfValue, threeHalf}, want: pastThreeHalf,
			},
			{
				name:       "returns the next float past a largest value that one more rounds back to",
				underlying: float64Spelling, values: []string{"0", hugeValue}, want: pastHuge,
			},
			{
				name:       "returns one below the smallest value where the largest value fills float32",
				underlying: float32Spelling, values: []string{"0", maxFloat32}, want: belowZero,
			},
			{
				name:       "returns one past the largest value within int64 of a uint64 set past it",
				underlying: "uint64", values: []string{"1", pastInt64}, want: "2",
			},
		}
		for _, tt := range bounded {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				out := levelInfo(t, builtin(tt.underlying), tt.values...).OutOfRange
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
			{
				name:       "returns no out-of-range value for a set at both ends of float32",
				underlying: float32Spelling, values: []string{minFloat32, maxFloat32},
			},
			{
				name:       "returns no out-of-range value for a set with both truth values",
				underlying: boolSpelling, values: []string{trueValue, falseValue},
			},
			{
				name:       "returns no out-of-range value for a uint64 set past int64",
				underlying: "uint64", values: []string{pastInt64},
			},
			{
				name:       "returns no out-of-range value for a float set whose value does not read",
				underlying: float64Spelling, values: []string{unknownSpelling},
			},
		}
		for _, tt := range filled {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.True(t, levelInfo(t, builtin(tt.underlying), tt.values...).OutOfRange.IsZero(),
					"the underlying type holds no value outside the range of the set")
			})
		}

		probes := []struct {
			name      string
			giveUnder *node.TypeRef
			giveVals  []string
			want      emit.Value
		}{
			{
				name:      "returns the empty string as out of range for an enumeration over a string without it",
				giveUnder: builtin(stringSpelling), giveVals: []string{readValue, writeValue},
				want: emit.Literal(emit.LiteralString, ""),
			},
			{
				name:      "returns a text longer than every text as out of range where a variant has the empty string",
				giveUnder: builtin(stringSpelling), giveVals: []string{emptyValue, longValue},
				want: emit.Literal(emit.LiteralString, longerProbe),
			},
			{
				name:      "returns false as out of range for an enumeration over a bool without it",
				giveUnder: builtin(boolSpelling), giveVals: []string{trueValue},
				want: emit.Literal(emit.LiteralBool, falseValue),
			},
			{
				name:      "returns true as out of range for an enumeration over a bool without it",
				giveUnder: builtin(boolSpelling), giveVals: []string{falseValue},
				want: emit.Literal(emit.LiteralBool, trueValue),
			},
			{
				name:      "returns a probe of the target of a defined type that the view contains",
				giveUnder: ref(fxPath, weightType, symbol.KindAlias), giveVals: []string{halfValue},
				want: emit.Number(emit.LiteralFloat, "1.5", bits64),
			},
			{
				name:      "returns a probe of the underlying type of an enumeration that the view contains",
				giveUnder: ref(fxPath, modeType, symbol.KindEnum), giveVals: []string{readValue},
				want: emit.Literal(emit.LiteralString, ""),
			},
			{
				name:      "returns a probe of a string for an unresolved underlying type with quoted values",
				giveUnder: builtin("geo.Code"), giveVals: []string{readValue},
				want: emit.Literal(emit.LiteralString, ""),
			},
			{
				name:      "returns a probe of a bool for an unresolved underlying type with truth values",
				giveUnder: builtin("geo.Flag"), giveVals: []string{trueValue},
				want: emit.Literal(emit.LiteralBool, falseValue),
			},
		}
		for _, tt := range probes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				out := levelInfo(t, tt.giveUnder, tt.giveVals...).OutOfRange
				assert.Equal(t, out.Kind, emit.ValueConversion, "the value converts to the type")
				want := tt.want
				assert.Equal(t, out.Inner, &want, "the probe that no variant has")
			})
		}

		t.Run("returns the value form for an unresolved underlying type with quoted values", func(t *testing.T) {
			t.Parallel()

			info := levelInfo(t, builtin("geo.Code"), "", readValue)
			expect.That(t, info.Form).Equal(rules.EnumValue, "a quoted value has a string type")
			expect.That(t, info.Variants[1].Text).Equal(emit.Literal(emit.LiteralString, "read"),
				"the text is the declared value")
		})

		t.Run("returns no out-of-range value for an unresolved underlying type with numbers", func(t *testing.T) {
			t.Parallel()

			assert.True(t, levelInfo(t, builtin("geo.Count"), "1").OutOfRange.IsZero(),
				"the width of the number is unknown")
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

	return fixtureInfo(tb, "Color")
}

// fixtureInfo returns the projection of an enumeration of the fixture,
// whose values the frontend stamped.
func fixtureInfo(tb assert.TB, name string) rules.EnumInfo {
	tb.Helper()

	f := loaded(tb)
	e, is := f.decl(tb, id(fxPath, name, symbol.KindEnum)).(*node.Enum)
	assert.True(tb, is, name+" is an enumeration")
	return enumRules(tb).EnumOf(e, f.view)
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

// levelInfo returns the projection of the enumeration Level over the
// underlying type under, in the view of the fixture. The fixture stamps
// the variant at each index with the value at that index. A variant whose
// value is empty has no stamp, as a variant whose value the frontend did
// not evaluate.
func levelInfo(tb assert.TB, under *node.TypeRef, values ...string) rules.EnumInfo {
	tb.Helper()

	f := loaded(tb)
	key, held := f.constKey()
	assert.True(tb, held, "the fixture registered the constant key")
	e := &node.Enum{
		ID: id(fxPath, levelName, symbol.KindEnum), Name: levelName,
		Underlying: under,
	}
	for i, value := range values {
		name := levelName + strconv.Itoa(i)
		variant := &node.EnumVariant{ID: id(fxPath, name, symbol.KindEnumVariant), Name: name}
		e.Variants = append(e.Variants, variant)
		if value != "" {
			stamp(tb, f.Facts, variant.ID, key, value)
		}
	}
	return enumRules(tb).EnumOf(e, f.view)
}

// sharedInfo returns the projection of the two-variant enumeration over
// int with both variants stamped at one value.
func sharedInfo(tb assert.TB) rules.EnumInfo {
	tb.Helper()

	f := loaded(tb)
	key, held := f.constKey()
	assert.True(tb, held, "the fixture registered the constant key")
	e := modeEnum()
	e.Underlying = builtin(intSpelling)
	for i := range 2 {
		stamp(tb, f.Facts, e.Variants[i].ID, key, "5")
	}
	return enumRules(tb).EnumOf(e, f.view)
}
