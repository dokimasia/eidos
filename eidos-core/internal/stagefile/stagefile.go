// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package stagefile

import (
	"io/fs"
	"math/rand/v2"
	"os"
	"strconv"
)

// Suffix names the staging file a replacement writes beside its target.
// A caller that stages paths of its own reserves it, so a staged path
// never collides with a replacement in progress.
const Suffix = ".stage"

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

// Replace writes body to name and the staging suffix with perm, syncs
// it, closes it and renames it over name, so a reader of name sees the
// old file or the new one. The directory of name must exist. A staging
// file that fails to write, sync, close or rename is removed, and name
// remains as it was.
func Replace(r *os.Root, name string, body []byte, perm fs.FileMode) error {
	stage := name + Suffix
	f, err := r.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	return finish(r, f, stage, name, body, Synced)
}

// ReplaceShared replaces name the way [Replace] does, through a staging
// file that belongs to this call alone: name, a dot, 16 random hex
// digits and [Suffix], created exclusively. Writers that replace one
// name at once never share a staging file, so a reader of name sees the
// whole bytes of one writer. It syncs the staging file before the rename
// where d is [Synced]. The directory of name must exist. A staging name
// another writer took is an error wrapping [fs.ErrExist], which 63
// random bits make vanishingly rare. A staging file that fails to write,
// sync, close or rename is removed, and name remains as it was. A
// staging file a killed process left behind remains until the owner of
// its directory removes it.
func ReplaceShared(r *os.Root, name string, body []byte, perm fs.FileMode, d Durability) error {
	stage := name + "." + strconv.FormatUint(rand.Uint64()|1<<63, 16) + Suffix
	f, err := r.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
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
	}
	return err
}
