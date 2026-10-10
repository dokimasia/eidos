// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// enumAllocs is an open enum of its own package's variants: the list of
// its variants, and the out-of-range value, which is the enum's
// reference and the conversion over the number. closedEnumAllocs is a
// closed one, which has no out-of-range value.
const (
	enumAllocs       = 1 + 2
	closedEnumAllocs = 1
)

// closedSource declares an enum of proto2, which is closed.
const closedSource = `syntax = "proto2";

package svc.c;

enum Closed {
  CLOSED_ZERO = 0;
}
`

// An enum projects from the numbers the schema declares, so the zero,
// the alias, the foreign variant and the out-of-range value are each
// pinned.
func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("EnumOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the identifier form", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, colourInfo(t).Form, rules.EnumIdentifier, "protobuf states no string form of its own")
		})

		t.Run("returns every variant in declaration order", func(t *testing.T) {
			t.Parallel()

			info := colourInfo(t)
			assert.Length(t, info.Variants, 2, "one entry per value")
			assert.Equal(t, info.Variants[0].Name, "COLOUR_UNSPECIFIED", "the first declared")
		})

		t.Run("returns each variant's name as its text", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, colourInfo(t).Variants[1].Text, emit.Literal(emit.LiteralString, "COLOUR_RED"),
				"the text is the declared name")
		})

		t.Run("returns the first declared variant as the zero", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, colourInfo(t).Zero, "COLOUR_UNSPECIFIED", "protobuf's default for an enum field")
		})

		t.Run("returns no duplicate for distinct numbers", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, colourInfo(t).Duplicate, "", "no two variants share a number")
		})

		t.Run("returns a conversion of one past the largest number as out of range", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, outOfRange(t, colourInfo(t)), "2", "one past COLOUR_RED")
		})

		t.Run("returns the out-of-range number at the wire's width", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, colourInfo(t).OutOfRange.Inner.Bits, 32, "an enum number is an int32 on the wire")
		})

		t.Run("returns no foreign package for the enum's own variants", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, colourInfo(t).Foreign, "every variant is the enum's own")
		})

		t.Run("returns the first declared variant as the zero whatever its number", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, enumRules(t).EnumOf(modeEnum(), rules.View{}).Zero, "MODE_HIGH",
				"proto2 lets the first value be any number")
		})

		t.Run("returns the first of two variants at one number as the duplicate", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, enumRules(t).EnumOf(modeEnum(), rules.View{}).Duplicate, "MODE_HIGH",
				"sixteen written in two bases is one number")
		})

		t.Run("returns one past the largest number read in any base as out of range", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, outOfRange(t, enumRules(t).EnumOf(modeEnum(), rules.View{})), "17",
				"the largest number is 0x10")
		})

		t.Run("reads a minus sign apart from its digits", func(t *testing.T) {
			t.Parallel()

			low := enumOf("Low", [2]string{"LOW_ONLY", "- 5"})
			assert.Equal(t, outOfRange(t, enumRules(t).EnumOf(low, rules.View{})), "-4", "one past minus five")
		})

		t.Run("returns one below the smallest number when the largest is the int32 maximum", func(t *testing.T) {
			t.Parallel()

			edge := enumOf("Edge", [2]string{"EDGE_ZERO", "0"}, [2]string{"EDGE_MAX", "2147483647"})
			assert.Equal(t, outOfRange(t, enumRules(t).EnumOf(edge, rules.View{})), "-1",
				"no int32 lies past the maximum")
		})

		t.Run("returns no out-of-range value for a closed enum", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, closedInfo(t).OutOfRange, emit.Value{},
				"a field of a closed enum keeps an undeclared number as an unknown field")
		})

		t.Run("returns no out-of-range value for an enum that spans every int32", func(t *testing.T) {
			t.Parallel()

			full := enumOf("Full", [2]string{"FULL_MIN", "-2147483648"}, [2]string{"FULL_MAX", "2147483647"})
			assert.Equal(t, enumRules(t).EnumOf(full, rules.View{}).OutOfRange, emit.Value{}, "no number is left")
		})

		t.Run("returns a variant whose number is no int32", func(t *testing.T) {
			t.Parallel()

			info := enumRules(t).EnumOf(wideEnum(), rules.View{})
			assert.Length(t, info.Variants, 2, "every named value is a variant")
		})

		t.Run("returns out of range from the numbers the wire encodes", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, outOfRange(t, enumRules(t).EnumOf(wideEnum(), rules.View{})), "2",
				"the number past int32 is no part of the range")
		})

		t.Run("returns no variant for a missing enum", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, enumRules(t).EnumOf(nil, rules.View{}).Variants, "nothing projects to nothing")
		})

		t.Run("skips a missing variant", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, enumRules(t).EnumOf(farEnum(), rules.View{}).Variants, 2, "MODE_OFF and MODE_FAR")
		})

		t.Run("returns the package of a variant declared elsewhere as foreign", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, enumRules(t).EnumOf(farEnum(), rules.View{}).Foreign, []string{"elsewhere"},
				"MODE_FAR is declared outside the enum's namespace")
		})
	})
}

// A projection allocates its variant list and its out-of-range value.
// The ordinary run, which runs no benchmark, checks that ceiling here.
func TestEnumAllocs(t *testing.T) {
	checkAllocs(t, enumCalls(t))
}

// BenchmarkEnum measures the projection a generated check makes once
// per enum.
func BenchmarkEnum(b *testing.B) {
	benchCalls(b, enumCalls(b))
}

// enumCalls returns a call of EnumOf over the fixture's Colour, which is
// open, and over a closed enum.
func enumCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := enumRules(tb)
	colour, _ := f.decl(tb, id(svcPkg, "Colour", symbol.KindEnum)).(*node.Enum)
	c := loadedFrom(tb, map[string]string{"svc/c.proto": closedSource})
	closed, _ := c.decl(tb, id("svc.c", "Closed", symbol.KindEnum)).(*node.Enum)
	var info rules.EnumInfo
	return []allocCall{
		{
			name: "EnumOf", caseName: "an open enum", allocs: enumAllocs,
			call:  func() { info = r.EnumOf(colour, f.view) },
			check: func(tb assert.TB) { assert.Length(tb, info.Variants, 2, "EnumOf returns the two variants") },
		},
		{
			name: "EnumOf", caseName: "a closed enum", allocs: closedEnumAllocs,
			call:  func() { info = r.EnumOf(closed, c.view) },
			check: func(tb assert.TB) { assert.Length(tb, info.Variants, 1, "EnumOf returns the one variant") },
		},
	}
}

// closedInfo returns the projection of a closed enum of proto2.
func closedInfo(tb assert.TB) rules.EnumInfo {
	tb.Helper()

	f := loadedFrom(tb, map[string]string{"svc/c.proto": closedSource})
	closed, is := f.decl(tb, id("svc.c", "Closed", symbol.KindEnum)).(*node.Enum)
	assert.True(tb, is, "Closed is an enum")
	return enumRules(tb).EnumOf(closed, f.view)
}

// enumRules returns protobuf's rules as the enum capability.
func enumRules(tb assert.TB) rules.EnumRules {
	tb.Helper()

	r, is := protorules.New().(rules.EnumRules)
	assert.True(tb, is, "protobuf's rules project enums")
	return r
}

// colourInfo returns the projection of the fixture's Colour.
func colourInfo(tb assert.TB) rules.EnumInfo {
	tb.Helper()

	f := loaded(tb)
	colour, is := f.decl(tb, id(svcPkg, "Colour", symbol.KindEnum)).(*node.Enum)
	assert.True(tb, is, "Colour is an enum")
	return enumRules(tb).EnumOf(colour, f.view)
}

// modeEnum returns an enum whose numbers are written in every base
// protobuf admits, sixteen twice, the first of them not zero.
func modeEnum() *node.Enum {
	return enumOf("Mode",
		[2]string{"MODE_HIGH", "0x10"},
		[2]string{"MODE_TOP", "16"},
		[2]string{"MODE_OCTAL", "017"},
		[2]string{"MODE_LOW", "- 1"},
	)
}

// wideEnum returns an enum with one number past int32.
func wideEnum() *node.Enum {
	return enumOf("Wide", [2]string{"WIDE_ONE", "1"}, [2]string{"WIDE_HUGE", "9223372036854775807"})
}

// farEnum returns an enum with a variant declared in another
// namespace, and a missing variant.
func farEnum() *node.Enum {
	e := enumOf("Mode", [2]string{"MODE_OFF", "0"})
	far := &node.EnumVariant{
		ID: id("elsewhere", "MODE_FAR", symbol.KindEnumVariant), Name: "MODE_FAR", Value: "5",
	}
	e.Variants = append(e.Variants, far, nil)
	return e
}

// enumOf builds an enum in the fixture's namespace from name and
// number pairs, in declaration order.
func enumOf(name string, variants ...[2]string) *node.Enum {
	e := &node.Enum{ID: id(svcPkg, name, symbol.KindEnum), Name: name}
	for _, v := range variants {
		e.Variants = append(e.Variants, &node.EnumVariant{
			ID: member(svcPkg, name, v[0], symbol.KindEnumVariant), Name: v[0], Value: v[1],
		})
	}
	return e
}

// outOfRange returns the number an enum's out-of-range value states.
func outOfRange(tb assert.TB, info rules.EnumInfo) string {
	tb.Helper()

	assert.Equal(tb, info.OutOfRange.Kind, emit.ValueConversion,
		"the out-of-range value is a number converted to the enum type")
	assert.NotNil(tb, info.OutOfRange.Inner, "over the number")
	return info.OutOfRange.Inner.Text
}
