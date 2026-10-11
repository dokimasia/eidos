// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package stagefile

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// darwinBarrier is the barrier of macOS. fsync(2) moves the data of a file
// from the host to the drive, and F_FULLFSYNC drains the queue of the
// device, so data that fsync(2) moved to the device before is on stable
// storage when it returns. [os.File.Sync] issues F_FULLFSYNC on macOS, and
// fsync(2) where the filesystem refuses F_FULLFSYNC with ENOTSUP.
type darwinBarrier struct {
	root *os.Root
	// dir is the root from Start to Close, on which each barrier issues
	// F_FULLFSYNC.
	dir *os.File
	// dev is the device of the root.
	dev int32
}

// System returns a new barrier of macOS. File syncs each staging file with
// fsync(2), and the order barrier issues one F_FULLFSYNC on the root. The
// durability barrier syncs each changed directory with fsync(2) and issues
// one F_FULLFSYNC on the root. A staging file or a directory on another
// device than the root gets an F_FULLFSYNC of its own. It allocates the
// barrier.
func System() Barrier { return &darwinBarrier{} }

// Start opens the root, on which each barrier issues F_FULLFSYNC, and
// reads its device from fstat(2).
//
// Error modes: a root that does not open, and an fstat(2) that fails,
// each wrapped.
func (b *darwinBarrier) Start(r *os.Root, _ int) error {
	dir, err := r.Open(".")
	if err != nil {
		return fmt.Errorf("stagefile: open the root: %w", err)
	}
	var st unix.Stat_t
	if err := unix.Fstat(int(dir.Fd()), &st); err != nil {
		return errors.Join(fmt.Errorf("stagefile: fstat the root: %w", err), dir.Close())
	}
	b.root, b.dir, b.dev = r, dir, st.Dev
	return nil
}

// File syncs f with fsync(2) where f is on the root's device, and with
// F_FULLFSYNC where it is on another device.
//
// Error modes: an fstat(2) or a sync that fails, wrapped.
func (b *darwinBarrier) File(f *os.File) error { return b.sync(f) }

// Order issues F_FULLFSYNC on the root.
//
// Error modes: the error of the sync, wrapped.
func (b *darwinBarrier) Order() error {
	if err := b.dir.Sync(); err != nil {
		return fmt.Errorf("stagefile: %w", err)
	}
	return nil
}

// Durable syncs each directory of dirs on the pool, as File syncs a file,
// then issues F_FULLFSYNC on the root.
//
// Error modes: the error of each directory that does not open, sync or
// close, and the error of the last sync, joined.
func (b *darwinBarrier) Durable(dirs []string) error {
	errs := make([]error, len(dirs), len(dirs)+1)
	Each(len(dirs), func(i int) {
		f, err := b.root.Open(dirs[i])
		if err != nil {
			errs[i] = fmt.Errorf("stagefile: open %s to sync it: %w", dirs[i], err)
			return
		}
		errs[i] = errors.Join(b.sync(f), f.Close())
	})
	return errors.Join(append(errs, b.Order())...)
}

// Close closes the root.
//
// Error modes: the error of the close, wrapped.
func (b *darwinBarrier) Close() error {
	if err := b.dir.Close(); err != nil {
		return fmt.Errorf("stagefile: close the root: %w", err)
	}
	return nil
}

// sync syncs one file or directory with fsync(2) where it is on the root's
// device, and with F_FULLFSYNC where it is on another device.
//
// Error modes: an fstat(2) or a sync that fails, wrapped.
func (b *darwinBarrier) sync(f *os.File) error {
	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil {
		return fmt.Errorf("stagefile: fstat %s: %w", f.Name(), err)
	}
	if st.Dev != b.dev {
		if err := f.Sync(); err != nil {
			return fmt.Errorf("stagefile: %w", err)
		}
		return nil
	}
	if err := unix.Fsync(int(f.Fd())); err != nil {
		return fmt.Errorf("stagefile: fsync %s: %w", f.Name(), err)
	}
	return nil
}
