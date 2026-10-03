// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package load

import (
	"io/fs"
	"syscall"
	"time"
)

// sysStat returns a file's change time and inode where its stat
// contains the kernel's record, and zero for a file from a tree that
// reports none, such as an in-memory one.
func sysStat(info fs.FileInfo) (time.Time, uint64) {
	st, kernel := info.Sys().(*syscall.Stat_t)
	if !kernel {
		return time.Time{}, 0
	}
	return time.Unix(st.Ctim.Sec, st.Ctim.Nsec), st.Ino
}
