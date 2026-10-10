// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/ledger"
)

// The lock's two files in a ledger's directory, which a contender and a
// person read: the file the operating system locks, and the holder's
// record.
const (
	lockFile   = "lock"
	holderFile = "lock.json"
)

// The child process that the case of a killed holder starts: the variable
// that names the workspace root whose state directory it locks, the
// caller it records, the line it writes once it has the lock, and the
// deadline after which the case kills it.
const (
	lockChildEnv  = "EIDOS_LEDGER_LOCK_CHILD"
	childCaller   = "a child that waits to be killed"
	lockedLine    = "locked"
	childDeadline = 30 * time.Second
)

// heldSince is the time a fixture holder took the lock, in UTC and
// without a monotonic reading, so it compares equal after the record's
// round trip through JSON.
var heldSince = time.Date(2026, time.October, 8, 12, 30, 0, 0, time.UTC)

// fixtureHolder is the holder that the lock fixtures take the lock for.
var fixtureHolder = ledger.Holder{PID: 4242, Host: "build-7", Caller: "acme run ./svc/...", Since: heldSince}

// TestMain runs the test binary as the child that the case of a killed
// holder starts, where the environment names a workspace root, and runs
// the tests otherwise.
func TestMain(m *testing.M) {
	if root := os.Getenv(lockChildEnv); root != "" {
		os.Exit(holdLock(root))
	}
	os.Exit(m.Run())
}

// A lock records its holder, and a contender's error names that holder.
func TestLock(t *testing.T) {
	t.Parallel()

	t.Run("Holder", func(t *testing.T) {
		t.Parallel()

		t.Run("String", func(t *testing.T) {
			t.Parallel()

			t.Run("returns a description with the time in UTC", func(t *testing.T) {
				t.Parallel()

				h := fixtureHolder
				h.Since = heldSince.In(time.FixedZone("CEST", 2*60*60))
				assert.Equal(t, h.String(), "process 4242 on build-7 since 2026-10-08T12:30:00Z (acme run ./svc/...)",
					"the holder in one line")
			})

			t.Run("returns another process for the zero Holder", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, ledger.Holder{}.String(), "another process", "no record names no process")
			})
		})
	})

	t.Run("LockedError", func(t *testing.T) {
		t.Parallel()

		t.Run("Error", func(t *testing.T) {
			t.Parallel()

			t.Run("returns ErrLocked's text with the holder", func(t *testing.T) {
				t.Parallel()

				err := &ledger.LockedError{Holder: fixtureHolder}
				want := "ledger: the state directory is locked by " +
					"process 4242 on build-7 since 2026-10-08T12:30:00Z (acme run ./svc/...)"
				assert.Equal(t, err.Error(), want, "the error names the holder")
			})
		})

		t.Run("Unwrap", func(t *testing.T) {
			t.Parallel()

			t.Run("returns ErrLocked", func(t *testing.T) {
				t.Parallel()

				var err error = &ledger.LockedError{}
				assert.ErrorIs(t, err, ledger.ErrLocked, "a locked error is ErrLocked")
			})
		})
	})
}

// holdLock is the child that the case of a killed holder starts. It takes
// the lock of the state directory under root, writes [lockedLine] on its
// standard output, and reads its standard input until the parent kills it
// or closes the pipe. It returns the status the child exits with, 1 where
// it does not take the lock.
func holdLock(root string) int {
	d, err := ledger.OpenDir(root, brand)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	// The child is a process of its own, so Background is its root context.
	if _, err := d.Lock(context.Background(), ledger.Holder{PID: os.Getpid(), Caller: childCaller}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintln(os.Stdout, lockedLine)
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
