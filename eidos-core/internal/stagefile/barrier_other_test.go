// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin && !linux && !windows

package stagefile_test

import (
	"os"
	"testing"

	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/internal/stagefile"
)

// The barrier of another system syncs each staging file and each changed
// directory with fsync(2), the parents of created directories included.
func TestBarrierOther(t *testing.T) {
	t.Parallel()

	t.Run("System", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a barrier that syncs the directories that a write created", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, nil)
			g := stagefile.NewGroup(r, stagefile.Unique, stagefile.System())
			g.Write(nested, []byte(body), mode)
			committed(t, g)
			files.Equal(t, os.DirFS(dir), files.Tree{nested: files.Text(body)}, "the nested target is written")
		})
	})
}
