// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package rust_test

import (
	"testing"

	"go.dokimi.dev/assert"

	rust "go.dokimi.dev/eidos/lang-rust"
)

// The identity, the extension and the comment forms are what the
// rest of the system spells to reach this satellite, so each is
// pinned: a drift here breaks compositions, not this module.
func TestLang(t *testing.T) {
	t.Parallel()

	t.Run("pins the boundary spellings", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, rust.Target, "rust", "the target a plan resolves")
		assert.Equal(t, rust.Name, "rust", "the identity findings report under")
		assert.Equal(t, rust.Extension, ".rs", "the suffix every file carries")
	})

	t.Run("declares the comment forms whole", func(t *testing.T) {
		t.Parallel()

		s := rust.Syntax()
		assert.Equal(t, s.Line, []string{"//", "///", "//!"},
			"all three line forms, the plain one canonical, so the "+
				"frontend strips doc comments too")
		assert.True(t, len(s.Blocks) > 0, "and a block form is declared")
		for _, b := range s.Blocks {
			assert.True(t, b.Open != "" && b.Close != "",
				"every block form opens and closes")
		}
	})
}
