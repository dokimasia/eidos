// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
)

// The memory ledger is a record without state on disk: each run begins
// with what the last one committed, and an equal record changes
// nothing.
func TestMem(t *testing.T) {
	t.Parallel()

	t.Run("BeginRun", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the empty manifest before the first commit", func(t *testing.T) {
			t.Parallel()

			got, err := ledger.NewMem().BeginRun(t.Context())
			assert.NoError(t, err, "the memory ledger never fails")
			assert.True(t, got.Equal(manifest.Manifest{Version: manifest.Version}), "the empty manifest")
		})

		t.Run("returns the manifest the last commit recorded", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			assert.NoError(t, l.CommitRun(t.Context(), recorded("platform")), "the record commits")
			got, err := l.BeginRun(t.Context())
			assert.NoError(t, err, "the memory ledger never fails")
			assert.True(t, got.Equal(recorded("platform")), "the recorded manifest")
		})
	})

	t.Run("CommitRun", func(t *testing.T) {
		t.Parallel()

		t.Run("records a manifest that changed", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			assert.NoError(t, l.CommitRun(t.Context(), recorded("platform")), "the record commits")
			assert.NoError(t, l.CommitRun(t.Context(), recorded("other")), "the changed record commits")
			assert.Equal(t, l.Writes(), 2, "each change writes")
		})

		t.Run("changes nothing for a manifest equal to the record", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			assert.NoError(t, l.CommitRun(t.Context(), recorded("platform")), "the record commits")
			assert.NoError(t, l.CommitRun(t.Context(), recorded("platform")), "the equal record commits")
			assert.Equal(t, l.Writes(), 1, "the equal record writes nothing")
		})
	})

	t.Run("Writes", func(t *testing.T) {
		t.Parallel()

		t.Run("returns zero before the first commit", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, ledger.NewMem().Writes(), 0, "nothing was written")
		})
	})
}
