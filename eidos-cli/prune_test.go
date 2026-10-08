// Copyright ThesmOS B.V. 2026
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
