// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package callee_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/reference/internal/callee"
	"go.dokimi.dev/eidos/sdk/diag"
)

// The package registers its codes under the prefix of the ensemble when it
// initializes.
func TestCodes(t *testing.T) {
	t.Parallel()

	t.Run("Missing", func(t *testing.T) {
		t.Parallel()

		t.Run("is the first code of the prefix", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, callee.Missing.String(), "REFERENCE-0001",
				"the code has the prefix of the ensemble and the number 1")
		})

		t.Run("has a meaning in the registry of the codes", func(t *testing.T) {
			t.Parallel()

			meaning, registered := diag.Kernel().Meaning(callee.Missing)
			assert.True(t, registered, "the package registers the code")
			assert.Equal(t, meaning, "the options of a plugin set no function for the plugin to call",
				"the meaning anchors the code in the index")
		})
	})
}
