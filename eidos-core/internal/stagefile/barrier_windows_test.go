// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package stagefile_test

import (
	"os"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/internal/stagefile"
)

// The barrier of Windows flushes without a flush of the device's cache
// only on the file systems that the documentation lists for the flag, and
// opens each directory for writing to flush it.
func TestBarrierWindows(t *testing.T) {
	t.Parallel()

	t.Run("NoSyncFlush", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "reports true for NTFS", give: "NTFS", want: true},
			{name: "reports true for FAT", give: "FAT", want: true},
			{name: "reports true for FAT32", give: "FAT32", want: true},
			{name: "reports true for exFAT", give: "exFAT", want: true},
			{name: "reports false for ReFS", give: "ReFS"},
			{name: "reports false for an empty name", give: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, stagefile.NoSyncFlush(tt.give), tt.want, "the documented support of the flag")
			})
		}
	})

	t.Run("System", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a barrier that flushes the directories that a write created", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, nil)
			g := stagefile.NewGroup(r, stagefile.Unique, stagefile.System())
			g.Write(nested, []byte(body), mode)
			committed(t, g)
			files.Equal(t, os.DirFS(dir), files.Tree{nested: files.Text(body)}, "the nested target is written")
		})
	})
}
