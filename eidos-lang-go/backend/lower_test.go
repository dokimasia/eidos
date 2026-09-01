// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang-go/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
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

	t.Run("an announced failure gains the error return", func(t *testing.T) {
		t.Parallel()

		f := &emit.Function{
			Name:    "fetch",
			Returns: []*emit.Return{{Type: &emit.TypeRef{Spelling: "row"}}},
			Throws: []*emit.TypeRef{
				{Spelling: "notFound"}, {Spelling: "timeout"},
			},
		}
		out, err := backend.Lower(f)
		assert.NoError(t, err, "the throwing function lowers")
		assert.Length(t, out, 0, "in place: a nil list keeps the declaration")
		assert.Length(t, f.Throws, 0, "the fact is consumed")
		assert.Length(t, f.Returns, 2, "the error return appends")
		assert.Equal(t, f.Returns[1].Type.Spelling, "error",
			"one error whatever the announced count, because the "+
				"concrete types arrive through errors.As")

		host := &emit.Struct{Name: "row"}
		host.Methods.Append(&emit.Method{
			Name:   "save",
			Throws: []*emit.TypeRef{{Spelling: "conflict"}},
		})
		_, err = backend.Lower(host)
		assert.NoError(t, err, "a host's member methods lower with it")
		m := host.Methods.Items()[0]
		assert.Length(t, m.Throws, 0, "consumed")
		assert.Equal(t, m.Returns[0].Type.Spelling, "error",
			"a bare thrower returns the error alone")
	})

	t.Run("everything else passes through unchanged", func(t *testing.T) {
		t.Parallel()

		out, err := backend.Lower(&emit.Struct{Name: "row"})
		assert.NoError(t, err, "a struct passes")
		assert.Length(t, out, 0, "as a nil list, so the store keeps it")

		out, err = backend.Lower(&emit.Sum{Name: "shape"})
		assert.NoError(t, err, "a sum passes too")
		assert.Length(t, out, 0,
			"for the render to report as the unspelt kind it is")
	})
}
