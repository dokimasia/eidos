// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build !(darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || windows)

package ledger_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"
)

// The standard library offers no file lock on these platforms, so the
// disk ledger's lock reports the gap.
func TestLockFile(t *testing.T) {
	t.Parallel()

	t.Run("Dir", func(t *testing.T) {
		t.Parallel()

		t.Run("Lock", func(t *testing.T) {
			t.Parallel()

			t.Run("returns ErrUnsupported", func(t *testing.T) {
				t.Parallel()

				d, _ := opened(t)
				_, err := d.Lock(t.Context(), fixtureHolder)
				assert.ErrorIs(t, err, errors.ErrUnsupported, "the platform has no file lock")
			})
		})
	})
}
