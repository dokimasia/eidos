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

func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("projects an enumeration from the frontend's exact values", func(t *testing.T) {
		t.Parallel()

		f := loaded(t)
		color, is := f.decl(t, id(fxPath, "Color", symbol.KindEnum)).(*node.Enum)
		assert.True(t, is, "Color is an enumeration")
		r, is := gorules.New().(rules.EnumRules)
		assert.True(t, is, "the Go rules project enumerations")
		info := r.EnumOf(color, f.view)
		assert.Equal(t, info.Form, rules.EnumIdentifier, "identifier-formed")
		assert.Length(t, info.Variants, 3, "three variants")
		assert.Equal(t, info.Variants[1].Name, "Green", "in declaration order")
		assert.Equal(
			t,
			info.Variants[1].Text,
			emit.Literal(emit.LiteralString, "Green"),
			"each text its name",
		)
		assert.Equal(t, info.Zero, "Red", "the zero variant is the one valued 0")
		assert.Equal(t, info.Duplicate, "", "no two share a value")
		assert.Equal(
			t,
			info.OutOfRange.Kind,
			emit.ValueConversion,
			"the out-of-range value converts",
		)
		assert.Equal(t, info.OutOfRange.Inner.Text, "3", "one past the largest")
		assert.Empty(t, info.Foreign, "every variant is the type's own")
	})

	t.Run("reads without facts and reports a duplicate and a foreign variant", func(t *testing.T) {
		t.Parallel()

		r, _ := gorules.New().(rules.EnumRules)
		info := r.EnumOf(nil, rules.View{})
		assert.Empty(t, info.Variants, "nothing projects to nothing")
		e := &node.Enum{
			ID:   id(fxPath, "Mode", symbol.KindEnum),
			Name: "Mode",
			Variants: []*node.EnumVariant{
				{ID: id(fxPath, "On", symbol.KindEnumVariant), Name: "On"},
				{ID: id("elsewhere", "Off", symbol.KindEnumVariant), Name: "Off"},
				nil,
			},
		}
		info = r.EnumOf(e, rules.View{})
		assert.Length(t, info.Variants, 2, "a nil variant is skipped")
		assert.Equal(
			t,
			info.Foreign,
			[]string{"elsewhere"},
			"a variant declared elsewhere is foreign",
		)
		assert.True(t, info.OutOfRange.IsZero(), "without values nothing is out of range")

		f := loaded(t)
		facts := f.Facts
		key, held := f.constKey()
		assert.True(t, held, "the fixture registered the constant key")
		for i := range 2 {
			stamp(t, facts, e.Variants[i].ID, key, "5")
		}
		info = r.EnumOf(e, f.view)
		assert.Equal(t, info.Duplicate, "On", "two variants at one value name the first")
		assert.Equal(t, info.OutOfRange.Inner.Text, "6", "and the range ends past the shared value")
	})
}
