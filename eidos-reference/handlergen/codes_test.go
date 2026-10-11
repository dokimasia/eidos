// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package handlergen_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/sdk/diag"
)

// The package registers its codes under its own prefix when it
// initializes.
func TestCodes(t *testing.T) {
	t.Parallel()

	t.Run("MissingServe", func(t *testing.T) {
		t.Parallel()

		t.Run("is the first code of the prefix", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, handlergen.MissingServe.String(), "HANDLERGEN-0001",
				"the code has the prefix of the package and the number 1")
		})

		t.Run("has a meaning in the registry of the codes", func(t *testing.T) {
			t.Parallel()

			meaning, registered := diag.Kernel().Meaning(handlergen.MissingServe)
			assert.True(t, registered, "the package registers the code")
			assert.Equal(
				t,
				meaning,
				"a struct under the directive handler declares no method Serve for its handler to call",
				"the meaning anchors the code in the index",
			)
		})
	})
}
