// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/declarative"
)

// The package registers its codes under its own prefix when it
// initializes.
func TestCodes(t *testing.T) {
	t.Parallel()

	t.Run("IncompleteMembers", func(t *testing.T) {
		t.Parallel()

		t.Run("is the first code of the prefix", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, declarative.IncompleteMembers.String(), "DECLARATIVE-0001",
				"the code has the package's prefix and the number 1")
		})

		t.Run("has a meaning in the registry of the codes", func(t *testing.T) {
			t.Parallel()

			meaning, registered := diag.Kernel().Meaning(declarative.IncompleteMembers)
			assert.True(t, registered, "the package registers the code")
			assert.Equal(t, meaning, "a declarative plugin cannot mirror a declaration whose members are incomplete",
				"the meaning anchors the code in the index")
		})
	})

	t.Run("UndelegatedProperty", func(t *testing.T) {
		t.Parallel()

		t.Run("is the second code of the prefix", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, declarative.UndelegatedProperty.String(), "DECLARATIVE-0002",
				"the code has the package's prefix and the number 2")
		})

		t.Run("has a meaning in the registry of the codes", func(t *testing.T) {
			t.Parallel()

			meaning, registered := diag.Kernel().Meaning(declarative.UndelegatedProperty)
			assert.True(t, registered, "the package registers the code")
			assert.Equal(t, meaning, "an implementation cannot delegate a property of its interface",
				"the meaning anchors the code in the index")
		})
	})
}
