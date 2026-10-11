// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin && !windows

package stagefile

import (
	"errors"
	"fmt"
	"os"
)

// fileSync is the barrier that syncs each staging file and each
// directory alone with fsync(2). File syncs the file, and Durable syncs
// each directory on the pool. Order has nothing left to do. Every Unix
// system provides fsync(2), and the barrier costs one sync for each file
// and each directory.
type fileSync struct {
	root *os.Root
}

// Start keeps the root that Durable opens the directories in. It cannot
// fail.
func (b *fileSync) Start(r *os.Root, _ int) error {
	b.root = r
	return nil
}

// File syncs f with fsync(2).
//
// Error modes: the error of the sync, wrapped.
func (*fileSync) File(f *os.File) error {
	if err := f.Sync(); err != nil {
		return fmt.Errorf("stagefile: %w", err)
	}
	return nil
}

// Order returns nil, because File synced every staging file.
func (*fileSync) Order() error { return nil }

// Durable syncs each directory of dirs with fsync(2), on the pool.
//
// Error modes: the error of each directory that does not open, sync or
// close, joined.
func (b *fileSync) Durable(dirs []string) error { return syncDirs(b.root, dirs) }

// Close returns nil, because Start opened nothing.
func (*fileSync) Close() error { return nil }

// syncDirs syncs each directory of dirs inside the root with fsync(2), on
// the pool, and joins the error of each directory that fails.
func syncDirs(r *os.Root, dirs []string) error {
	errs := make([]error, len(dirs))
	Each(len(dirs), func(i int) {
		f, err := r.Open(dirs[i])
		if err != nil {
			errs[i] = fmt.Errorf("stagefile: open %s to sync it: %w", dirs[i], err)
			return
		}
		if err := errors.Join(f.Sync(), f.Close()); err != nil {
			errs[i] = fmt.Errorf("stagefile: sync %s: %w", dirs[i], err)
		}
	})
	return errors.Join(errs...)
}
