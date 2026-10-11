// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package stagefile

import (
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"strconv"
)

// Suffix ends the name of every staging file. A caller that stages paths
// of its own reserves it, so a staged path never collides with a
// replacement in progress.
const Suffix = ".stage"

// Staging is the rule that sets the name of the staging file of a write.
type Staging uint8

const (
	// Reserved stages a write of name in name followed by [Suffix], and
	// truncates a staging file that a killed process left there. The
	// caller reserves the suffix, so no target ends in it.
	Reserved Staging = 1
	// Unique stages a write of name in name, a dot, 16 random hex digits
	// and [Suffix], created exclusively. Groups that write one name at once
	// never share a staging file. A staging name that another writer took
	// is an error that wraps [fs.ErrExist]. The 63 random bits of the
	// digits make it vanishingly rare.
	Unique Staging = 2
)

// open creates the staging file of a write of name with perm by the rule
// s, and returns the open file and its name.
//
// Error modes: the error of the open, wrapped.
func (s Staging) open(r *os.Root, name string, perm fs.FileMode) (*os.File, string, error) {
	var (
		stage string
		flag  int
	)
	if s == Unique {
		stage = name + "." + strconv.FormatUint(rand.Uint64()|1<<63, 16) + Suffix
		flag = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	} else {
		stage = name + Suffix
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := r.OpenFile(stage, flag, perm)
	if err != nil {
		return nil, "", fmt.Errorf("stagefile: %w", err)
	}
	return f, stage, nil
}

// Durability states whether a replacement's bytes are on stable storage
// before the rename makes them visible.
type Durability uint8

const (
	// Synced syncs the staging file before the rename. Once the caller
	// also syncs the target's directory, a crash of the machine keeps the
	// new bytes.
	Synced Durability = 1
	// Unsynced renames without a sync. A crash of the process loses
	// nothing, and a crash of the machine can leave the old bytes, the new
	// ones or an empty file.
	Unsynced Durability = 2
)

// ReplaceShared replaces name through a [Unique] staging file, which
// belongs to this call alone. It writes body to the staging file with perm
// and syncs it where d is [Synced]. It then closes the staging file and
// renames it over name, so a reader of name sees the old file or the new
// one. Writers that replace one name at once never share a staging file,
// so a reader of name sees the whole bytes of one writer. The directory of
// name must exist. A staging file that fails to write, sync, close or
// rename is removed, and name remains as it was. A staging file a killed
// process left behind remains until the owner of its directory removes it.
func ReplaceShared(r *os.Root, name string, body []byte, perm fs.FileMode, d Durability) error {
	f, stage, err := Unique.open(r, name, perm)
	if err != nil {
		return err
	}
	return finish(r, f, stage, name, body, d)
}

// finish writes body to the open staging file, syncs it where d is
// Synced, closes it and renames it over name, removing the staging file
// where any step fails.
func finish(r *os.Root, f *os.File, stage, name string, body []byte, d Durability) error {
	_, err := f.Write(body)
	if err == nil && d == Synced {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = r.Rename(stage, name)
	}
	if err != nil {
		_ = r.Remove(stage)
		return fmt.Errorf("stagefile: %w", err)
	}
	return nil
}
