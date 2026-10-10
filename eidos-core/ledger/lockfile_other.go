// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build !(darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || windows)

package ledger

import (
	"errors"
	"fmt"
	"os"
)

// errNoFileLock reports a platform where the standard library offers no
// file lock. It wraps [errors.ErrUnsupported].
var errNoFileLock = fmt.Errorf("ledger: this platform offers no file lock: %w", errors.ErrUnsupported)

// lockFile returns [errNoFileLock], because the standard library offers
// no file lock on this platform.
func lockFile(*os.Root, string, string) (*os.File, error) { return nil, errNoFileLock }

// unlockFile does nothing, because lockFile takes no lock on this
// platform.
func unlockFile(*os.File) error { return nil }
