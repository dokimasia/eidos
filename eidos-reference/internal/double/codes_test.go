// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package double_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/reference/internal/double"
	"go.dokimi.dev/eidos/sdk/diag"
)

// The package registers its codes under the prefix of the ensemble when it
// initializes, after the code of the package internal/callee.
func TestCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		code    diag.Code
		spelled string
		meaning string
	}{
		{
			name: "IncompleteMembers", code: double.IncompleteMembers, spelled: "REFERENCE-0002",
			meaning: "a double cannot implement an interface whose members are incomplete",
		},
		{
			name: "Property", code: double.Property, spelled: "REFERENCE-0003",
			meaning: "a double cannot implement a property of its interface",
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
