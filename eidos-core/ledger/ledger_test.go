// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

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

	t.Run("LockPath", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the lock file inside the brand's state directory", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, ledger.LockPath(brand), ".acme/lock", "the lock file")
		})
	})
}

// The brand's paths allocate their spelling in the ordinary run, which
// runs no benchmark.
func TestLedgerAllocs(t *testing.T) {
	var got string
	assert.MaxAllocs(t, func() { got = ledger.StateDir(brand) }, 1, "StateDir allocates the directory's name")
	assert.Equal(t, got, ".acme", "StateDir returns the dot directory")
	assert.MaxAllocs(t, func() { got = ledger.ManifestPath(brand) }, 1, "ManifestPath allocates the joined path")
	assert.Equal(t, got, ".acme/manifest", "ManifestPath returns the manifest's directory")
}

// BenchmarkLedger measures the spelling of the brand's paths.
func BenchmarkLedger(b *testing.B) {
	b.Run("StateDir", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got string
		for c.Loop() {
			got = ledger.StateDir(brand)
		}
		assert.Equal(b, got, ".acme", "the state directory")
	})

	b.Run("ManifestPath", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got string
		for c.Loop() {
			got = ledger.ManifestPath(brand)
		}
		assert.Equal(b, got, ".acme/manifest", "the manifest's directory")
	})
}
