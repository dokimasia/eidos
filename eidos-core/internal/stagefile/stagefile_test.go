// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package stagefile_test

import (
	"cmp"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"
	"go.dokimi.dev/assert/history"

	"go.dokimi.dev/eidos/core/internal/stagefile"
)

// The fixture's target, its bytes, and a mode every umask leaves whole.
const (
	target = "out.txt"
	body   = "replaced\n"
	mode   = fs.FileMode(0o600)
)

// replaceSharedAllocs is one ReplaceShared: the random staging name's
// digits and the name, and 11 that os.Root spends opening the staging
// file and renaming it over the target. os.Root splits each path it
// resolves, copies each name into the bytes a system call takes, and
// allocates the file it opens.
const replaceSharedAllocs = 13

// A replacement is atomic per file: the target contains the old bytes
// or the new ones, and no staging file outlives the call.
func TestStagefile(t *testing.T) {
	t.Parallel()

	t.Run("ReplaceShared", func(t *testing.T) {
		t.Parallel()

		durabilities := []struct {
			name string
			give stagefile.Durability
		}{
			{name: "writes the synced body to a missing target with the given mode", give: stagefile.Synced},
			{name: "writes the unsynced body to a missing target with the given mode", give: stagefile.Unsynced},
		}
		for _, tt := range durabilities {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				dir, r := rooted(t, nil)
				assert.NoError(t, stagefile.ReplaceShared(r, target, []byte(body), mode, tt.give),
					"the target is written")
				files.Equal(t, os.DirFS(dir), files.Tree{target: files.Text(body).WithMode(mode)},
					"the target alone contains the body under the mode, and no staging file remains")
			})
		}

		t.Run("replaces the bytes of an existing target", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{target: files.Text("old and longer\n").WithMode(mode)})
			assert.NoError(t, stagefile.ReplaceShared(r, target, []byte(body), mode, stagefile.Synced),
				"the target is replaced")
			files.HasContent(t, filepath.Join(dir, target), body, "the target contains the new body alone")
		})

		t.Run("leaves one writer's whole body when writers replace one target at once", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, nil)
			bodies := make([]string, 16)
			for i := range bodies {
				bodies[i] = strings.Repeat(strconv.Itoa(i%10), 4096+i)
			}
			outcomes := history.Concurrently(len(bodies), time.Minute, func(writer int) (any, error) {
				return writer, stagefile.ReplaceShared(r, target, []byte(bodies[writer]), mode, stagefile.Unsynced)
			})
			for _, o := range outcomes {
				expect.True(t, o.Finished, "each writer finishes")
				expect.NoError(t, o.Error, "each writer replaces the target")
			}
			got := files.Read(t, filepath.Join(dir, target))
			assert.Contains(t, bodies, got, "the target is one writer's whole body")
			files.Equal(t, os.DirFS(dir), files.Tree{target: files.Text(got)}, "no staging file remains")
		})

		t.Run("returns an error for a target whose directory is missing", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, nil)
			var err error
			files.Unchanged(t, os.DirFS(dir), func() {
				err = stagefile.ReplaceShared(r, "missing/"+target, []byte(body), mode, stagefile.Synced)
			}, "the root is untouched")
			assert.HasError(t, err, "nothing is written")
		})

		t.Run("returns an error for a target that is a directory", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, files.Tree{target + "/kept": files.Text("")})
			var err error
			files.Unchanged(t, os.DirFS(dir), func() {
				err = stagefile.ReplaceShared(r, target, []byte(body), mode, stagefile.Synced)
			}, "the staging file is removed")
			assert.HasError(t, err, "the rename is refused")
		})

		t.Run("returns an error for a target outside the root", func(t *testing.T) {
			t.Parallel()

			_, r := rooted(t, nil)
			assert.HasError(t, stagefile.ReplaceShared(r, "../"+target, []byte(body), mode, stagefile.Synced),
				"the root jails the write")
		})
	})
}

// A replacement allocates within its ceiling in the ordinary run, which
// runs no benchmark. The count keeps the first error of its calls, which
// cmp.Or returns without allocating. Darwin and Windows allocate more in
// their file system calls, so the test runs on Linux alone.
func TestStagefileAllocs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the ceilings count the allocations of Linux's file system calls")
	}
	dir, r := rooted(t, nil)
	data := []byte(body)
	var err error
	assert.MaxAllocs(t, func() {
		err = cmp.Or(err, stagefile.ReplaceShared(r, target, data, mode, stagefile.Unsynced))
	}, replaceSharedAllocs, "ReplaceShared allocates the random staging name and the file")
	assert.NoError(t, err, "every shared replacement succeeds")
	files.Equal(t, os.DirFS(dir), files.Tree{target: files.Text(body)}, "and leaves the target alone")
}

// BenchmarkStagefile measures one replacement of an existing target in a
// temporary directory.
func BenchmarkStagefile(b *testing.B) {
	data := []byte(body)

	b.Run("ReplaceShared", func(b *testing.B) {
		dir, r := rooted(b, nil)
		c := bench.Start(b).MaxAllocs(replaceSharedAllocs)
		defer c.End()
		var err error
		for c.Loop() {
			err = stagefile.ReplaceShared(r, target, data, mode, stagefile.Unsynced)
		}
		assert.NoError(b, err, "every replacement succeeds")
		files.Equal(b, os.DirFS(dir), files.Tree{target: files.Text(body)}, "and leaves the target alone")
	})
}

// rooted returns a fresh directory that contains tree, and the root
// opened over it, closed when the test ends.
func rooted(tb testing.TB, tree files.Tree) (string, *os.Root) {
	tb.Helper()

	dir := files.Workspace(tb, tree)
	r, err := os.OpenRoot(dir)
	assert.NoError(tb, err, "the root opens")
	tb.Cleanup(func() { _ = r.Close() })
	return dir, r
}
