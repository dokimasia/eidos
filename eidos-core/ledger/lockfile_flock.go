// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package ledger

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// lockFile opens the lock file at name inside the root, creating it, and
// takes flock(2) on it with LOCK_EX|LOCK_NB. flock binds the lock to the
// open file description, so another open of the file conflicts with this
// one in this process as in another, and the lock ends when the last
// descriptor of the description closes, at the process's exit included.
// The absolute path is unused on these platforms.
//
// Error modes: [errBusy] where another open file description has the
// lock, and the operating system's error where the file does not open or
// lock.
func lockFile(r *os.Root, name, _ string) (*os.File, error) {
	f, err := r.OpenFile(name, os.O_RDWR|os.O_CREATE, filePerm)
	if err != nil {
		return nil, err
	}
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return nil, errors.Join(errBusy, f.Close())
	}
	if err != nil {
		return nil, errors.Join(fmt.Errorf("flock %s: %w", name, err), f.Close())
	}
	return f, nil
}

// unlockFile ends the flock(2) of a lock file with LOCK_UN before its
// close. A child process that a fork created holds a copy of the file's
// descriptor until its exec, and a close alone leaves the lock to that
// copy, while LOCK_UN ends the lock of the open file description through
// any of its descriptors.
//
// Error modes: the operating system's error, such as for a closed file.
func unlockFile(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		return fmt.Errorf("flock %s: %w", f.Name(), err)
	}
	return nil
}
