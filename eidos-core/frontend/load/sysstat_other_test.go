// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package load_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
)

// Off Linux the gate reads no change time and no inode, and proves a
// file unchanged by its size and modification time alone.
func TestSysstat(t *testing.T) {
	t.Parallel()

	t.Run("sysStat", func(t *testing.T) {
		t.Parallel()

		t.Run("records no change time for a file on disk", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			assert.NoError(t, os.WriteFile(filepath.Join(dir, "one.zz"), []byte("package one\n"), 0o600),
				"the file writes")
			_, report, _ := loadTree(t, os.DirFS(dir))
			rec := recordOf(t, report, "one.zz")
			assert.True(t, rec.Change.IsZero(), "the gate reads no change time here")
		})

		t.Run("records no inode for a file on disk", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			assert.NoError(t, os.WriteFile(filepath.Join(dir, "one.zz"), []byte("package one\n"), 0o600),
				"the file writes")
			_, report, _ := loadTree(t, os.DirFS(dir))
			rec := recordOf(t, report, "one.zz")
			assert.Equal(t, rec.Inode, uint64(0), "the gate reads no inode here")
		})
	})
}
