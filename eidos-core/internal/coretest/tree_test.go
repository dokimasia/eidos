// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/internal/coretest"
)

// A writing case runs inside a copied fixture tree. The copy must
// contain every file of the source, and a write into it must leave
// the source unchanged.
func TestTree(t *testing.T) {
	t.Parallel()

	t.Run("CopyTree", func(t *testing.T) {
		t.Parallel()

		t.Run("copies every file at its relative path", func(t *testing.T) {
			t.Parallel()

			tree := files.Tree{"top.txt": files.Text("top"), "a/b/deep.txt": files.Text("deep")}
			src := files.Workspace(t, tree)
			files.Equal(t, os.DirFS(coretest.CopyTree(t, src)), tree, "the copy contains every file with its content")
		})

		t.Run("a write into the copy leaves the source unchanged", func(t *testing.T) {
			t.Parallel()

			src := files.Workspace(t, files.Tree{"f.txt": files.Text("before")})
			root := coretest.CopyTree(t, src)
			assert.NoError(t, os.WriteFile(filepath.Join(root, "f.txt"), []byte("after"), 0o600),
				"the copy is writable")
			files.HasContent(t, filepath.Join(src, "f.txt"), "before", "the source keeps its content")
		})
	})
}
