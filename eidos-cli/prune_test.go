// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/cli"
)

// notesPath is a file without the frame of the brand, which no run
// generated.
const notesPath = "svc/notes.txt"

// Prune removes the stale files of each workspace, and withholds every
// write.
func TestPrune(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("removes a generated file that no plan produces", func(t *testing.T) {
			t.Parallel()

			root := pruned(t)
			status, stdout, _ := invoke(t, compose, cmdPrune, root)
			expect.Equal(t, status, cli.StatusOK, "the prune succeeds")
			expect.HasPrefix(t, stdout, "stale svc/mirror.txt\n", "the output lists the removal")
			files.Absent(t, filepath.Join(root, "svc", "mirror.txt"), "the prune removes the mirror")
		})

		t.Run("commits nothing under --dry-run", func(t *testing.T) {
			t.Parallel()

			root := pruned(t)
			status, stdout, _ := invoke(t, compose, cmdPrune, root, dryRunFlag)
			expect.Equal(t, status, cli.StatusOK, "the dry prune succeeds")
			expect.HasPrefix(t, stdout, "stale svc/mirror.txt\n",
				"the output lists the removal that the prune prepared")
			files.IsFile(t, filepath.Join(root, "svc", "mirror.txt"), "the dry prune leaves the mirror")
		})

		t.Run("writes no file that a plan produces", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			_, stdout, _ := invoke(t, compose, cmdPrune, root)
			expect.HasPrefix(t, stdout, "withheld svc/mirror.txt\n", "the prune withholds the write")
			files.Absent(t, filepath.Join(root, "svc", "mirror.txt"), "the prune creates no mirror")
		})

		t.Run("leaves a stale file that was edited since its stamp", func(t *testing.T) {
			t.Parallel()

			root := pruned(t)
			drift(t, root)
			status, stdout, _ := invoke(t, compose, cmdPrune, root)
			expect.Equal(t, status, cli.StatusOK, "the kept file is a Warning")
			expect.HasPrefix(t, stdout, "drifted svc/mirror.txt\n", "the output lists the kept file")
			files.IsFile(t, filepath.Join(root, "svc", "mirror.txt"), "the prune leaves the edited mirror")
		})

		t.Run("leaves a file that no run generated", func(t *testing.T) {
			t.Parallel()

			root := pruned(t)
			files.Write(t, root, files.Tree{notesPath: files.Text(handWritten)})
			status, _, stderr := invoke(t, compose, cmdPrune, root)
			assert.Equal(t, status, cli.StatusOK, "the prune succeeds: "+stderr)
			files.HasContent(t, filepath.Join(root, "svc", "notes.txt"), handWritten, "the prune leaves the notes")
		})
	})
}

// pruned returns a workspace with a stale mirror. A run mirrors the store,
// and then pruned removes the store.
func pruned(t *testing.T) string {
	t.Helper()

	root := stored(t, nil)
	ran(t, root)
	assert.NoError(t, os.Remove(filepath.Join(root, "svc", "store.zz")), "the test removes the store")
	return root
}
