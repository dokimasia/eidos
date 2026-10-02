// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package stagefile

import (
	"io/fs"
	"os"
)

// Suffix names the staging file a replacement writes beside its target.
// A caller that stages paths of its own reserves it, so a staged path
// never collides with a replacement in progress.
const Suffix = ".stage"

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
	_, err = f.Write(body)
	if err == nil {
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
