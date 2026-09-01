// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
)

// The identity, the extension and the comment forms are what the
// rest of the system spells to reach this satellite, so each is
// pinned: a drift here breaks compositions, not this module.
func TestLang(t *testing.T) {
	t.Parallel()

	t.Run("pins the boundary spellings", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, golang.Target, "golang", "the target a plan resolves")
		assert.Equal(t, golang.Name, "golang", "the identity findings report under")
		assert.Equal(t, golang.Extension, ".go", "the suffix every file carries")
	})

	t.Run("declares the comment forms whole", func(t *testing.T) {
		t.Parallel()

		s := golang.Syntax()
		assert.Equal(t, s.Line[0], "//", "the canonical line opener comes first")
		assert.True(t, len(s.Blocks) > 0, "and a block form is declared")
		for _, b := range s.Blocks {
			assert.True(t, b.Open != "" && b.Close != "",
				"every block form opens and closes")
		}
	})
}
