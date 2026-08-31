// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-rust/backend"
)

// ref is the fixture type reference.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// The vocabulary is what every kind template spells through, so
// each helper's output is pinned byte for byte.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Docs", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Docs(nil), "", "no lines, no comment")
		assert.Equal(t, backend.Docs([]string{"One.", "Two."}), "/// One.\n/// Two.\n",
			"one outer doc comment per line")
		assert.Equal(t, backend.Docs([]string{"Inner."}, "    "), "    /// Inner.\n",
			"a member's doc indents to the member's depth")
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Spell(ref("Vec<Row>")), "Vec<Row>",
			"the source spelling passes through verbatim")
		assert.Equal(t, backend.Spell(nil), "()",
			"a declaration stating no type reaches the unit type, and the "+
				"compiler's refusal names the file")
		assert.Equal(t, backend.Spell(&emit.TypeRef{
			Spelling: "Map",
			Args: []*emit.TypeRef{
				ref("String"),
				{Spelling: "Vec", Args: []*emit.TypeRef{ref("Row")}},
			},
		}), "Map<String, Vec<Row>>",
			"an argument list spells in angle brackets, arguments recursing")
	})

	t.Run("TypeParams", func(t *testing.T) {
		t.Parallel()

		got, err := backend.TypeParams(nil)
		assert.NoError(t, err, "no parameters spell")
		assert.Equal(t, got, "", "as nothing")

		got, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "T", Bounds: []*emit.TypeRef{ref("Codec"), ref("Send")}},
			{Name: "U", Default: ref("String")},
		})
		assert.NoError(t, err, "bounds and defaults spell")
		assert.Equal(t, got, "<T: Codec + Send, U = String>",
			"bounds joined by plus signs, the default behind equals")

		got, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "N", Const: true, Type: ref("usize"), DefaultValue: "4"},
		})
		assert.NoError(t, err, "a value parameter spells")
		assert.Equal(t, got, "<const N: usize = 4>",
			"the const form with its value's type and default")

		_, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "T", Variance: symbol.VarianceOut},
		})
		assert.HasError(t, err,
			"variance refuses, because Rust infers it from use")
	})

	t.Run("Binder", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Binder(ref("Row")), "",
			"a receiver taking no arguments binds nothing")
		assert.Equal(t, backend.Binder(&emit.TypeRef{
			Spelling: "Box", Args: []*emit.TypeRef{ref("T")},
		}), "<T>", "a generic receiver's arguments restate as the binder")
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Params([]*emit.Param{
			{Name: "key", Type: ref("String")},
			{Type: ref("u32")},
		}), "key: String, _: u32",
			"colon-typed names, the discard pattern where none was stated")
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Results(nil), "", "no results, no annotation")
		assert.Equal(t, backend.Results([]*emit.Return{{Type: ref("Row")}}),
			" -> Row", "one result is the arrow annotation")
		assert.Equal(t, backend.Results([]*emit.Return{
			{Type: ref("Row")}, {Type: ref("Error")},
		}), " -> (Row, Error)", "several return as one tuple")
	})
}
