// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package ledger_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/ledger"
)

// procDescriptors is the directory that lists the process's open
// descriptors on Linux, each a link to the file it refers to.
const procDescriptors = "/proc/self/fd"

// The disk ledger's lock is flock(2) on these platforms, so it excludes
// another program's flock of the lock file, and that program's flock
// excludes it.
func TestLockFile(t *testing.T) {
	t.Parallel()

	t.Run("Dir", func(t *testing.T) {
		t.Parallel()

		t.Run("Lock", func(t *testing.T) {
			t.Parallel()

			t.Run("excludes the flock of another open file until the release", func(t *testing.T) {
				t.Parallel()

				d, root := opened(t)
				release, err := d.Lock(t.Context(), fixtureHolder)
				assert.NoError(t, err, "the lock is taken")
				other := openLockFile(t, root)
				assert.ErrorIs(t, syscall.Flock(int(other.Fd()), syscall.LOCK_EX|syscall.LOCK_NB), syscall.EWOULDBLOCK,
					"another open file finds the lock taken")
				assert.NoError(t, release(), "the lock is released")
				assert.NoError(t, syscall.Flock(int(other.Fd()), syscall.LOCK_EX|syscall.LOCK_NB),
					"another open file takes the lock after the release")
			})

			t.Run("returns a LockedError while another open file has a flock", func(t *testing.T) {
				t.Parallel()

				d, root := opened(t)
				assert.NoError(t, os.MkdirAll(onDisk(root, "."), 0o755), "the state directory exists")
				other := openLockFile(t, root)
				assert.NoError(t, syscall.Flock(int(other.Fd()), syscall.LOCK_EX|syscall.LOCK_NB),
					"another open file takes the lock")
				_, err := d.Lock(t.Context(), fixtureHolder)
				locked := assert.ErrorAs[*ledger.LockedError](t, err, "the ledger finds the lock taken")
				assert.Equal(t, locked.Holder, ledger.Holder{}, "the other program recorded no holder")
			})

			t.Run("returns the lock to the next holder while a copy of the released descriptor remains open",
				func(t *testing.T) {
					t.Parallel()

					d, root := opened(t)
					release, err := d.Lock(t.Context(), fixtureHolder)
					assert.NoError(t, err, "the lock is taken")
					copied := duplicateLockFile(t, root)
					assert.NoError(t, release(), "the lock is released")
					again, err := reopened(t, root).Lock(t.Context(), fixtureHolder)
					assert.NoError(t, err, "the release ended the lock of every copy of the descriptor")
					assert.NoError(t, again(), "and the next holder releases it")
					assert.NoError(t, syscall.Close(copied), "the copy closes")
				})
		})
	})
}

// openLockFile opens the lock file of the state directory under root as
// another program does, creating it, and closes it in the cleanup.
func openLockFile(t *testing.T, root string) *os.File {
	t.Helper()

	f, err := os.OpenFile(onDisk(root, lockFile), os.O_RDWR|os.O_CREATE, 0o644)
	assert.NoError(t, err, "the lock file opens")
	t.Cleanup(func() { assert.NoError(t, f.Close(), "the lock file closes") })
	return f
}

// duplicateLockFile copies the descriptor that this process holds open on
// the lock file of the state directory under root, as a fork gives a child
// process a copy of it until the child's exec, and returns the copy. It
// finds the descriptor in /proc/self/fd, so the case runs on Linux alone.
func duplicateLockFile(t *testing.T, root string) int {
	t.Helper()

	if runtime.GOOS != "linux" {
		t.Skip("the process lists its descriptors in /proc on Linux alone")
	}
	want, err := filepath.EvalSymlinks(onDisk(root, lockFile))
	assert.NoError(t, err, "the lock file's path resolves")
	entries, err := os.ReadDir(procDescriptors)
	assert.NoError(t, err, "the process's descriptors list")
	var open []int
	for _, e := range entries {
		target, linkErr := os.Readlink(filepath.Join(procDescriptors, e.Name()))
		if linkErr != nil || target != want {
			continue
		}
		fd, numErr := strconv.Atoi(e.Name())
		assert.NoError(t, numErr, "a descriptor's name is its number")
		open = append(open, fd)
	}
	assert.Length(t, open, 1, "the ledger holds one descriptor of the lock file")
	copied, err := syscall.Dup(open[0])
	assert.NoError(t, err, "the descriptor is copied")
	return copied
}
