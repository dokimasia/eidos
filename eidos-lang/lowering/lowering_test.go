// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package lowering_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/lowering"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The copies restate shapes without sharing, and the member check
// names its language, so both contracts are pinned here.
func TestLowering(t *testing.T) {
	t.Parallel()

	t.Run("CopyTypeParams/shares no node with its input", func(t *testing.T) {
		t.Parallel()

		bound := &emit.TypeRef{Spelling: "Ord", Args: []*emit.TypeRef{{Spelling: "K"}}}
		in := []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{bound}}}
		out := lowering.CopyTypeParams(in)
		assert.Length(t, out, 1, "one parameter copies")
		assert.True(t, out[0] != in[0], "as a fresh parameter")
		assert.True(t, out[0].Bounds[0] != bound, "with a fresh bound")
		assert.True(t, out[0].Bounds[0].Args[0] != bound.Args[0],
			"down to the argument tree")
		assert.Equal(t, out[0].Bounds[0].Args[0].Spelling, "K", "spelled the same")
		assert.Nil(t, lowering.CopyTypeParams(nil), "emptiness copies to nil")
	})

	t.Run("CopyTypeRef/copies the form's children as well as the arguments", func(t *testing.T) {
		t.Parallel()

		in := &emit.TypeRef{
			Spelling: "map[string]*User", Form: symbol.FormMap,
			Elems: []*emit.TypeRef{
				{Spelling: "string"},
				{Spelling: "*User", Form: symbol.FormOptional, Elems: []*emit.TypeRef{{Spelling: "User"}}},
			},
		}
		out := lowering.CopyTypeRef(in)
		assert.Equal(t, out, in, "the copy restates the whole tree")
		assert.False(t, out.Elems[1] == in.Elems[1], "and shares no child with its input")
		assert.False(t, out.Elems[1].Elems[0] == in.Elems[1].Elems[0], "at any depth")
	})

	t.Run("CopyTypeRef/copies nothing from nil", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, lowering.CopyTypeRef(nil), "a nil reference stays nil")
		assert.Nil(t, lowering.CopyTypeRefs(nil), "and so does a nil list")
	})

	t.Run("UniqueMethods/names the duplicate under the language", func(t *testing.T) {
		t.Parallel()

		err := lowering.UniqueMethods("rust", "Shape", []*emit.Method{
			{Name: "area"}, {Name: "area"},
		})
		assert.HasError(t, err, "a duplicate refuses")
		assert.Contains(t, err.Error(), "rust: method area", "under the language's prefix")
		assert.NoError(t,
			lowering.UniqueMethods("go", "Shape", []*emit.Method{{Name: "area"}}),
			"a unique set passes")
	})
}
