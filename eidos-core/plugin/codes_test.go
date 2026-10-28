// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/plugin"
)

// A code is API: consumers script against it and the published
// index anchors to it, so its number and meaning are contract.
func TestCodes(t *testing.T) {
	t.Parallel()

	t.Run("ContinuedCarrier", func(t *testing.T) {
		t.Parallel()

		t.Run("spells the kernel prefix with its padded number", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, plugin.ContinuedCarrier.String(), "EID-0044", "the spelling is pinned")
		})

		t.Run("registers its meaning in the kernel registry", func(t *testing.T) {
			t.Parallel()

			meaning, held := diag.Kernel().Meaning(plugin.ContinuedCarrier)
			assert.True(t, held, "the code registered at initialization")
			assert.Equal(t, meaning, "a carrier in the tool-directive shape continues onto the next line",
				"the meaning is pinned")
		})
	})

	t.Run("MisspelledBrand", func(t *testing.T) {
		t.Parallel()

		t.Run("spells the kernel prefix with its padded number", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, plugin.MisspelledBrand.String(), "EID-0083", "the spelling is pinned")
		})

		t.Run("registers its meaning in the kernel registry", func(t *testing.T) {
			t.Parallel()

			meaning, held := diag.Kernel().Meaning(plugin.MisspelledBrand)
			assert.True(t, held, "the code registered at initialization")
			assert.Equal(t, meaning, "a mark or a marker is near the brand of the run and is not a directive",
				"the meaning is pinned")
		})
	})
}
