// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package stagefile_test

import (
	"os"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/internal/stagefile"
)

// removedFile is the file that the cases of the system's barrier remove.
const removedFile = "removed.txt"

// The barrier of the running system makes a group durable, whether the
// group fits one round of the pool or not, and serves one group.
func TestBarrier(t *testing.T) {
	t.Parallel()

	t.Run("System", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a new barrier for each call", func(t *testing.T) {
			t.Parallel()

			first, second := stagefile.System(), stagefile.System()
			assert.NotEqual(t, first, second, "two groups get two barriers", assert.ByIdentity())
		})

		sizes := []struct {
			name string
			give int
		}{
			{name: "returns a barrier that commits a group of one file", give: 1},
			{
				name: "returns a barrier that commits a group of more files than the pool has workers",
				give: stagefile.Workers + 8,
			},
		}
		for _, tt := range sizes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				dir, r := rooted(t, files.Tree{removedFile: files.Text(body)})
				g := stagefile.NewGroup(r, stagefile.Reserved, stagefile.System())
				want := files.Tree{}
				for i := range tt.give {
					name := "d" + strconv.Itoa(i%4) + "/f" + strconv.Itoa(i) + ".txt"
					g.Write(name, []byte(body), mode)
					want[name] = files.Text(body)
				}
				g.Remove(removedFile)
				committed(t, g)
				files.Equal(t, os.DirFS(dir), want, "every target is written, and the removed file is gone")
			})
		}

		t.Run("returns a barrier that commits a group of removals alone", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{removedFile: files.Text(body), "kept/" + removedFile: files.Text(body)})
			g := stagefile.NewGroup(r, stagefile.Reserved, stagefile.System())
			g.Remove(removedFile)
			g.Remove("kept/" + removedFile)
			committed(t, g)
			files.Equal(t, os.DirFS(dir), files.Tree{"kept": files.Dir()}, "both files are gone")
		})
	})
}
