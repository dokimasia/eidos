// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"testing"

	"go.dokimi.dev/assert"

	typescript "go.dokimi.dev/eidos/lang/typescript"
)

// The rest of the system names this satellite by its identity, its
// extension and its comment forms, so each is pinned: a drift here
// breaks compositions, not this module.
func TestLang(t *testing.T) {
	t.Parallel()

	t.Run("pins the boundary spellings", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, typescript.Target, "typescript", "the target a plan resolves")
		assert.Equal(t, typescript.Name, "typescript", "the identity findings report under")
		assert.Equal(t, typescript.Extension, ".ts", "the suffix of every file")
		assert.Equal(t, typescript.CodePrefix, "TYPESCRIPT", "the prefix of every diagnostic code")
	})

	t.Run("declares the comment forms whole", func(t *testing.T) {
		t.Parallel()

		s := typescript.Syntax()
		assert.Equal(t, s.Line[0], "//", "the canonical line opener comes first")
		assert.True(t, len(s.Blocks) > 0, "and a block form is declared")
		for _, b := range s.Blocks {
			assert.True(t, b.Open != "" && b.Close != "",
				"every block form opens and closes")
		}
	})
}
