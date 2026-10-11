// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build unix && !darwin

package stagefile_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/internal/stagefile"
)

// The barrier that syncs each file fails the name of a staging file that
// does not sync, and the group removes that staging file.
func TestBarrierFsync(t *testing.T) {
	t.Parallel()

	t.Run("System", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a barrier that fails the name of a staging file that does not sync", func(t *testing.T) {
			t.Parallel()

			// A FIFO at the staging path opens for writing while a reader
			// has it open. It accepts the written bytes, and the sync that
			// follows fails on it.
			if runtime.GOOS != "linux" {
				t.Skip("the sync of a FIFO fails on Linux alone")
			}
			dir, r := rooted(t, nil)
			stage := filepath.Join(dir, target+stagefile.Suffix)
			assert.NoError(t, syscall.Mkfifo(stage, 0o600), "the fixture makes a FIFO at the staging path")
			reader, err := os.OpenFile(stage, os.O_RDONLY|syscall.O_NONBLOCK, 0)
			assert.NoError(t, err, "a reader opens the FIFO")
			t.Cleanup(func() { _ = reader.Close() })
			g := stagefile.NewGroup(r, stagefile.Reserved, stagefile.System())
			g.Write(target, []byte(body), mode)
			failed, err := g.Commit()
			assert.NoError(t, err, "no barrier fails")
			assert.HasError(t, failed[target], "unsynced bytes are never renamed over the target")
			files.Absent(t, stage, "the failed staging file is gone")
			files.Absent(t, filepath.Join(dir, target), "the target was never written")
		})

		t.Run("returns a barrier whose Durable fails for a directory that does not open", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{"unread/kept": files.Text("")})
			unread := filepath.Join(dir, "unread")
			restricted(t, unread, 0o755)
			g := stagefile.NewGroup(r, stagefile.Reserved, unreadable{Barrier: stagefile.System(), dir: unread})
			g.Write("unread/"+target, []byte(body), mode)
			failed, err := g.Commit()
			assert.ErrorIs(t, err, fs.ErrPermission, "the directory refuses its read for the sync")
			assert.Empty(t, failed, "and the write is renamed")
		})
	})
}

// unreadable is a barrier that takes the read permission from the
// directory dir before it passes Durable to the barrier that it wraps, so
// the sync of that directory does not open it. os.Root opens each
// directory of a path, so a directory without read permission refuses
// the writes below it too, and the mode changes after the renames.
type unreadable struct {
	stagefile.Barrier

	dir string
}

// Durable takes the read permission from the directory, and passes the
// call to the wrapped barrier.
func (u unreadable) Durable(dirs []string) error {
	if err := os.Chmod(u.dir, 0o300); err != nil {
		return err
	}
	return u.Barrier.Durable(dirs)
}
