// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/output"
)

// brand is the brand every fixture ledger records under.
const brand output.Brand = "acme"

// The fixture blobs: a name nested two directories deep, a sibling under
// another directory, and the bytes each stores.
const (
	docName   = "manifest/ab.json"
	segName   = "state/seg/ab/0123"
	docBody   = "the first document\n"
	otherBody = "a longer second document\n"
)

// invalidNames are names no ledger stores under, each with why.
var invalidNames = []struct {
	name string
	give string
}{
	{name: "returns ErrInvalid for an empty name", give: ""},
	{name: "returns ErrInvalid for the root's own name", give: "."},
	{name: "returns ErrInvalid for a name that climbs out", give: "../x"},
	{name: "returns ErrInvalid for an absolute name", give: "/x"},
	{name: "returns ErrInvalid for a name with an empty element", give: "a//b"},
	{name: "returns ErrInvalid for a name with a backslash", give: `a\b`},
}

// cancelled returns a context that is already cancelled.
func cancelled(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return ctx
}

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

		t.Run("returns the manifest's directory inside the brand's state directory", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, ledger.ManifestPath(brand), ".acme/manifest", "the manifest's directory")
		})
	})
}
