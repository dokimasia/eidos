// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package stubgen_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/reference/stubgen"
	"go.dokimi.dev/eidos/sdk/diag"
)

// The package registers its codes under its own prefix when it
// initializes.
func TestCodes(t *testing.T) {
	t.Parallel()

	t.Run("GenericMethod", func(t *testing.T) {
		t.Parallel()

		t.Run("is the first code of the prefix", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, stubgen.GenericMethod.String(), "STUBGEN-0001",
				"the code has the prefix of the package and the number 1")
		})

		t.Run("has a meaning in the registry of the codes", func(t *testing.T) {
			t.Parallel()

			meaning, registered := diag.Kernel().Meaning(stubgen.GenericMethod)
			assert.True(t, registered, "the package registers the code")
			assert.Equal(t, meaning, "a call record cannot state the arguments of a method with type parameters",
				"the meaning anchors the code in the index")
		})
	})
}
