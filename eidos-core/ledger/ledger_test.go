// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/output"
)

// brand is the brand every fixture ledger records under.
const brand output.Brand = "acme"

// The state directory belongs to the brand, so two tools built on the
// kernel never read each other's records.
func TestLedger(t *testing.T) {
	t.Parallel()

	t.Run("StateDir", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the brand's dot directory", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, ledger.StateDir(brand), ".acme", "the state directory")
		})
	})

	t.Run("ManifestPath", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the manifest inside the brand's state directory", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, ledger.ManifestPath(brand), ".acme/manifest.json", "the manifest's path")
		})
	})
}
