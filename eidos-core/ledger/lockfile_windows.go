// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package ledger

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// errSharingViolation is ERROR_SHARING_VIOLATION, which CreateFile
// returns where another handle has the file open under a share mode that
// excludes the new open. The syscall package does not export it.
const errSharingViolation syscall.Errno = 32

// lockFile opens the lock file at the absolute path abs, creating it,
// through CreateFile with a share mode of zero, so no other handle opens
// the file while this one is open. The handle ends at its close and at the
// process's exit. The open does not go through the root, because os.Root
// opens every file under a share mode that admits other handles.
//
// Error modes: [errBusy] where another handle has the file open, and the
// operating system's error where the file does not open.
func lockFile(_ *os.Root, _, abs string) (*os.File, error) {
	p, err := syscall.UTF16PtrFromString(abs)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", abs, err)
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if errors.Is(err, errSharingViolation) {
		return nil, errBusy
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", abs, err)
	}
	return os.NewFile(uintptr(h), abs), nil
}

// unlockFile does nothing on Windows: the handle's close ends the lock,
// and a child process inherits no handle that CreateFile opened without
// security attributes.
func unlockFile(*os.File) error { return nil }
