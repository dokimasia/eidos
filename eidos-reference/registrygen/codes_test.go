// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package registrygen_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/reference/registrygen"
	"go.dokimi.dev/eidos/sdk/diag"
)

// The package registers its codes under its own prefix when it
// initializes.
func TestCodes(t *testing.T) {
	t.Parallel()

	t.Run("Generic", func(t *testing.T) {
		t.Parallel()

		t.Run("is the first code of the prefix", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, registrygen.Generic.String(), "REGISTRYGEN-0001",
				"the code has the prefix of the package and the number 1")
		})

		t.Run("has a meaning in the registry of the codes", func(t *testing.T) {
			t.Parallel()

			meaning, registered := diag.Kernel().Meaning(registrygen.Generic)
			assert.True(t, registered, "the package registers the code")
			assert.Equal(t, meaning, "a registry cannot make a value of a struct with type parameters",
				"the meaning anchors the code in the index")
		})
	})
}
