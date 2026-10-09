// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
	"go.dokimi.dev/eidos/sdk/diag"
)

// A code is API: consumers script against it and the published index
// anchors to it, so its number and its meaning are contract.
func TestCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		code    diag.Code
		spelled string
		meaning string
	}{
		{
			name: "SpecInvalid", code: specfront.SpecInvalid, spelled: "SHAPESPEC-0001",
			meaning: "a spec does not decode, or breaks a rule of its form",
		},
		{
			name: "SpecDuplicate", code: specfront.SpecDuplicate, spelled: "SHAPESPEC-0002",
			meaning: "two specs have one name, or the specs give one Go identifier twice",
		},
		{
			name: "PrecedenceCycle", code: specfront.PrecedenceCycle, spelled: "SHAPESPEC-0003",
			meaning: "the yields_to lists of detected shapes form a cycle",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("spells its prefix and padded number", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.code.String(), tt.spelled, "the spelling is pinned")
			})

			t.Run("registers its meaning in the kernel registry", func(t *testing.T) {
				t.Parallel()

				meaning, held := diag.Kernel().Meaning(tt.code)
				assert.True(t, held, "the code registered at initialization")
				assert.Equal(t, meaning, tt.meaning, "the meaning is pinned")
			})
		})
	}
}
