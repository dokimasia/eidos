// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
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
			"the source spelling rides through verbatim")
		assert.Equal(t, backend.Spell(nil), "()",
			"a declaration stating no type reaches the unit type, and the "+
				"compiler's refusal names the file")
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
