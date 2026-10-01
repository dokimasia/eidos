// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rust_test

import (
	"testing"

	"go.dokimi.dev/assert"

	rust "go.dokimi.dev/eidos/lang/rust"
)

// The rest of the system names this satellite by its identity, its
// extension and its comment forms, so each is pinned: a drift here
// breaks compositions, not this module.
func TestLang(t *testing.T) {
	t.Parallel()

	t.Run("pins the boundary spellings", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, rust.Target, "rust", "the target a plan resolves")
		assert.Equal(t, rust.Name, "rust", "the identity findings report under")
		assert.Equal(t, rust.Extension, ".rs", "the suffix of every file")
		assert.Equal(t, rust.CodePrefix, "RUST", "the prefix of every diagnostic code")
	})

	t.Run("declares the comment forms whole", func(t *testing.T) {
		t.Parallel()

		s := rust.Syntax()
		assert.Equal(t, s.Line, []string{"//", "///", "//!"},
			"all three line forms, the plain one canonical")
		assert.True(t, len(s.Blocks) > 0, "and a block form is declared")
		for _, b := range s.Blocks {
			assert.True(t, b.Open != "" && b.Close != "",
				"every block form opens and closes")
		}
	})

	t.Run("declares the doc block forms with their star gutter", func(t *testing.T) {
		t.Parallel()

		var opens []string
		for _, b := range rust.Syntax().Blocks {
			if b.Gutter == "*" {
				opens = append(opens, b.Open)
			}
		}
		assert.Equal(t, opens, []string{"/**", "/*!"}, "the outer and the inner doc block")
	})
}
