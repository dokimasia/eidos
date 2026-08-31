// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-go/backend"
)

// The lowering is Go's declared idiom for the constructs it states
// in other declarations, so each reshaping is pinned.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("an enum becomes a defined type and its constants", func(t *testing.T) {
		t.Parallel()

		origin := symbol.Identity{Lang: "fixture", Package: "svc", Name: "phase", Kind: symbol.KindEnum}
		e := &emit.Enum{
			Origin: origin,
			Doc:    []string{"phase names a lifecycle step."},
			Name:   "phase",
		}
		e.Variants.Append(
			&emit.EnumVariant{Doc: []string{"open admits writes."}, Name: "open"},
			&emit.EnumVariant{Name: "closed", Value: "9"},
		)
		out, err := backend.Lower(e)
		assert.NoError(t, err, "the enum lowers")
		assert.Length(t, out, 3, "one defined type and one constant per variant")

		alias, held := out[0].(*emit.Alias)
		assert.True(t, held, "the principal output is the defined type")
		assert.True(t, alias.Defined, "defined rather than transparent")
		assert.Equal(t, alias.Name, "phase", "keeping the enum's name")
		assert.Equal(t, alias.Target.Spelling, "int", "over the int underlying")
		assert.Equal(t, alias.Origin, origin, "carrying the enum's origin")
		assert.Equal(t, alias.Doc, e.Doc, "and its documentation")

		first, held := out[1].(*emit.Constant)
		assert.True(t, held, "each variant becomes a constant")
		assert.Equal(t, first.Name, "phaseOpen",
			"named type-then-variant in the neutral form")
		assert.Equal(t, first.Type.Spelling, "phase",
			"typed by the defined type, so the reference follows a respell")
		assert.Equal(t, first.Value, "0", "an unstated value counts by ordinal")
		assert.Equal(t, first.Origin, origin, "under the enum's origin")

		second := out[2].(*emit.Constant)
		assert.Equal(t, second.Name, "phaseClosed", "the second variant beside it")
		assert.Equal(t, second.Value, "9", "a stated value spells verbatim")
	})

	t.Run("an enum carrying members refuses", func(t *testing.T) {
		t.Parallel()

		e := &emit.Enum{Name: "phase"}
		e.Variants.Append(&emit.EnumVariant{Name: "open"})
		e.Methods.Append(&emit.Method{Name: "describe"})
		_, err := backend.Lower(e)
		assert.HasError(t, err, "a constant group holds no members")
	})

	t.Run("everything else passes through unchanged", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Name: "row"}
		out, err := backend.Lower(s)
		assert.NoError(t, err, "a struct passes")
		assert.Length(t, out, 1, "alone")
		assert.True(t, out[0] == symbol.Symbol(s), "and untouched")

		sum := &emit.Sum{Name: "shape"}
		out, err = backend.Lower(sum)
		assert.NoError(t, err, "a sum passes too")
		assert.True(t, out[0] == symbol.Symbol(sum),
			"for the render to report as the unspelt kind it is")
	})
}
