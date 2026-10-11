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

	tests := []struct {
		name    string
		code    diag.Code
		spelled string
		meaning string
	}{
		{
			name: "Missing", code: callee.Missing, spelled: "REFERENCE-0001",
			meaning: "the options of a plugin set no function for the plugin to call",
		},
		{
			name: "UnknownPackage", code: callee.UnknownPackage, spelled: "REFERENCE-0005",
			meaning: "the options of a plugin name a package that the module of the subject does not contain",
		},
		{
			name: "UnknownFunction", code: callee.UnknownFunction, spelled: "REFERENCE-0006",
			meaning: "the options of a plugin name a function that their package does not declare",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("has the prefix of the ensemble and its number", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.code.String(), tt.spelled, "the spelling is pinned")
			})

			t.Run("has a meaning in the registry of the codes", func(t *testing.T) {
				t.Parallel()

				meaning, registered := diag.Kernel().Meaning(tt.code)
				assert.True(t, registered, "the package registers the code")
				assert.Equal(t, meaning, tt.meaning, "the meaning anchors the code in the index")
			})
		})
	}
}
