// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package load_test

import (
	"os"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"
)

// On Linux the gate reads a file's change time and inode from the
// kernel's stat record, so a file replaced under the same size and
// modification time still reads as changed.
func TestSysstat(t *testing.T) {
	t.Parallel()

	t.Run("sysStat", func(t *testing.T) {
		t.Parallel()

		t.Run("records the change time of a file on disk", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{"one.zz": files.Text("package one\n")})
			_, report, _ := loadTree(t, os.DirFS(dir))
			rec := recordOf(t, report, "one.zz")
			assert.False(t, rec.Change.IsZero(), "the kernel's stat states a change time")
		})

		t.Run("records the inode of a file on disk", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{"one.zz": files.Text("package one\n")})
			_, report, _ := loadTree(t, os.DirFS(dir))
			rec := recordOf(t, report, "one.zz")
			assert.NotEqual(t, rec.Inode, uint64(0), "the kernel's stat states an inode")
		})

		t.Run("records no change time for a file of an in-memory tree", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, fstest.MapFS{"one.zz": {Data: []byte("package one\n")}})
			rec := recordOf(t, report, "one.zz")
			assert.True(t, rec.Change.IsZero(), "an in-memory tree has no kernel stat")
		})

		t.Run("records no inode for a file of an in-memory tree", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, fstest.MapFS{"one.zz": {Data: []byte("package one\n")}})
			rec := recordOf(t, report, "one.zz")
			assert.Equal(t, rec.Inode, uint64(0), "an in-memory tree has no inode")
		})
	})
}
