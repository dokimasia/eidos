// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/frontend/classfile"
)

// Has checks several flags at once, so a set that lacks one of them
// reports false.
func TestAccess(t *testing.T) {
	t.Parallel()

	t.Run("Has", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a set that has every flag", func(t *testing.T) {
			t.Parallel()

			set := classfile.AccPublic | classfile.AccStatic | classfile.AccFinal
			assert.True(t, set.Has(classfile.AccPublic|classfile.AccFinal), "both are set")
		})

		t.Run("reports false for a set that lacks one of the flags", func(t *testing.T) {
			t.Parallel()

			assert.False(t, classfile.AccPublic.Has(classfile.AccPublic|classfile.AccStatic), "static is not set")
		})
	})
}
