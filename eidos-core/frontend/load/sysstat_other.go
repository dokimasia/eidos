// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package load

import (
	"io/fs"
	"time"
)

// sysStat returns zero: the gate compares a change time and an inode on
// Linux alone, and the size and modification time everywhere.
func sysStat(fs.FileInfo) (time.Time, uint64) { return time.Time{}, 0 }
