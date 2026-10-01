// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package java_test

import (
	"testing"

	"go.dokimi.dev/assert"

	java "go.dokimi.dev/eidos/lang/java"
)

// The rest of the system names this satellite by its identity, its
// extension and its comment forms, so each is pinned: a drift here
// breaks compositions, not this module.
func TestLang(t *testing.T) {
	t.Parallel()

	t.Run("pins the boundary spellings", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, java.Target, "java", "the target a plan resolves")
		assert.Equal(t, java.Name, "java", "the identity findings report under")
		assert.Equal(t, java.Extension, ".java", "the suffix of every file")
		assert.Equal(t, java.CodePrefix, "JAVA", "the prefix of every diagnostic code")
	})

	t.Run("declares the comment forms whole", func(t *testing.T) {
		t.Parallel()

		s := java.Syntax()
		assert.Equal(t, s.Line[0], "//", "the canonical line opener comes first")
		assert.True(t, len(s.Blocks) > 0, "and a block form is declared")
		for _, b := range s.Blocks {
			assert.True(t, b.Open != "" && b.Close != "",
				"every block form opens and closes")
		}
	})
}
