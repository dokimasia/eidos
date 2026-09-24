// Copyright ThesmOS B.V. 2026
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

// An enum projects from the numbers the schema declares, so the zero,
// the alias, the foreign variant and the out-of-range value are each
// pinned.
func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("EnumOf", func(t *testing.T) {
		t.Parallel()

		t.Run("projects an enum from the numbers the schema declared", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			colour, is := f.decl(t, id(svcPkg, "Colour", symbol.KindEnum)).(*node.Enum)
			assert.True(t, is, "Colour is an enum")
			r, held := protorules.New().(rules.EnumRules)
			assert.True(t, held, "protobuf's rules project enums")

			info := r.EnumOf(colour, f.view)
			assert.Equal(t, info.Form, rules.EnumIdentifier, "identifier-formed")
			assert.Length(t, info.Variants, 2, "one entry per value")
			assert.Equal(t, info.Variants[0].Name, "COLOUR_UNSPECIFIED", "in declaration order")
			assert.Equal(t, info.Zero, "COLOUR_UNSPECIFIED", "the zero is the first declared value")
			assert.Equal(t, info.Duplicate, "", "no two share a number")
			assert.Equal(t, outOfRange(t, info), "2", "one past the largest")
			assert.Equal(t, info.OutOfRange.Inner.Bits, 32, "at the wire's width")
			assert.Empty(t, info.Foreign, "every variant is the enum's own")
		})

		t.Run("reads numbers in every base and finds an alias by number", func(t *testing.T) {
			t.Parallel()

			r, _ := protorules.New().(rules.EnumRules)
			info := r.EnumOf(enumOf("Mode",
				[2]string{"MODE_HIGH", "0x10"},
				[2]string{"MODE_TOP", "16"},
				[2]string{"MODE_OCTAL", "017"},
				[2]string{"MODE_LOW", "- 1"},
			), rules.View{})
			assert.Equal(t, info.Zero, "MODE_HIGH",
				"the zero is the first declared value, which proto2 lets be any number")
			assert.Equal(t, info.Duplicate, "MODE_HIGH",
				"sixteen written in two bases is one number, and the alias is reported")
			assert.Equal(t, outOfRange(t, info), "17", "the largest number read as hexadecimal")
		})

		t.Run("takes one below the smallest number when the largest is the int32 maximum", func(t *testing.T) {
			t.Parallel()

			r, _ := protorules.New().(rules.EnumRules)
			info := r.EnumOf(enumOf("Edge",
				[2]string{"EDGE_ZERO", "0"},
				[2]string{"EDGE_MAX", "2147483647"},
			), rules.View{})
			assert.Equal(t, outOfRange(t, info), "-1", "no int32 lies past the maximum")

			full := r.EnumOf(enumOf("Full",
				[2]string{"FULL_MIN", "-2147483648"},
				[2]string{"FULL_MAX", "2147483647"},
			), rules.View{})
			assert.Equal(t, full.OutOfRange, emit.Value{}, "and an enum spanning the whole range has none")
		})

		t.Run("skips a number that is no int32", func(t *testing.T) {
			t.Parallel()

			r, _ := protorules.New().(rules.EnumRules)
			info := r.EnumOf(enumOf("Wide",
				[2]string{"WIDE_ONE", "1"},
				[2]string{"WIDE_HUGE", "9223372036854775807"},
			), rules.View{})
			assert.Length(t, info.Variants, 2, "every named value is a variant")
			assert.Equal(t, outOfRange(t, info), "2", "and the range counts the numbers the wire encodes")
		})

		t.Run("reports a variant from another namespace and skips a nil one", func(t *testing.T) {
			t.Parallel()

			r, _ := protorules.New().(rules.EnumRules)
			assert.Empty(t, r.EnumOf(nil, rules.View{}).Variants, "nothing projects to nothing")

			e := enumOf("Mode", [2]string{"MODE_OFF", "0"})
			far := &node.EnumVariant{
				ID: id("elsewhere", "MODE_FAR", symbol.KindEnumVariant), Name: "MODE_FAR", Value: "5",
			}
			e.Variants = append(e.Variants, far, nil)
			info := r.EnumOf(e, rules.View{})
			assert.Length(t, info.Variants, 2, "a nil variant is skipped")
			assert.Equal(t, info.Foreign, []string{"elsewhere"},
				"a variant declared outside the enum's namespace is foreign")
		})
	})
}
