// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

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

			src := t.TempDir()
			assert.NoError(t, os.MkdirAll(filepath.Join(src, "a", "b"), 0o750), "the source nests")
			assert.NoError(t, os.WriteFile(filepath.Join(src, "top.txt"), []byte("top"), 0o600),
				"the top file writes")
			assert.NoError(t, os.WriteFile(filepath.Join(src, "a", "b", "deep.txt"), []byte("deep"), 0o600),
				"the nested file writes")

			root := coretest.CopyTree(t, src)
			top, err := os.ReadFile(filepath.Join(root, "top.txt"))
			assert.NoError(t, err, "the top file is in the copy")
			assert.Equal(t, string(top), "top", "with its content")
			deep, err := os.ReadFile(filepath.Join(root, "a", "b", "deep.txt"))
			assert.NoError(t, err, "the nested file is in the copy")
			assert.Equal(t, string(deep), "deep", "with its content")
		})

		t.Run("a write into the copy leaves the source unchanged", func(t *testing.T) {
			t.Parallel()

			src := t.TempDir()
			assert.NoError(t, os.WriteFile(filepath.Join(src, "f.txt"), []byte("before"), 0o600),
				"the source file writes")

			root := coretest.CopyTree(t, src)
			assert.NoError(t, os.WriteFile(filepath.Join(root, "f.txt"), []byte("after"), 0o600),
				"the copy is writable")
			got, err := os.ReadFile(filepath.Join(src, "f.txt"))
			assert.NoError(t, err, "the source file reads")
			assert.Equal(t, string(got), "before", "the source keeps its content")
		})
	})
}
