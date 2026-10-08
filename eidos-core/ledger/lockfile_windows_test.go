// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package ledger_test

import (
	"os"
	"syscall"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/ledger"
)

// The disk ledger's lock is an open without sharing on Windows, so a
// handle that another program opened without sharing excludes it.
func TestLockFile(t *testing.T) {
	t.Parallel()

	t.Run("Dir", func(t *testing.T) {
		t.Parallel()

		t.Run("Lock", func(t *testing.T) {
			t.Parallel()

			t.Run("returns a LockedError while another handle has the lock file open", func(t *testing.T) {
				t.Parallel()

				d, root := opened(t)
				assert.NoError(t, os.MkdirAll(onDisk(root, "."), 0o755), "the state directory exists")
				p, err := syscall.UTF16PtrFromString(onDisk(root, lockFile))
				assert.NoError(t, err, "the path converts")
				h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
					syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
				assert.NoError(t, err, "another handle opens the lock file without sharing")
				t.Cleanup(func() { assert.NoError(t, syscall.CloseHandle(h), "the handle closes") })
				_, err = d.Lock(t.Context(), fixtureHolder)
				locked := assert.ErrorAs[*ledger.LockedError](t, err, "the ledger finds the lock taken")
				assert.Equal(t, locked.Holder, ledger.Holder{}, "the other program recorded no holder")
			})
		})
	})
}
