// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package stagefile_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/stagefile"
)

// The fixture's target, its bytes, and a mode every umask leaves whole.
const (
	target = "out.txt"
	body   = "replaced\n"
	mode   = fs.FileMode(0o600)
)

// The ceilings of one replacement. os.Root spends all but the staging
// name's: it splits each path it resolves, copies each name into the
// bytes a system call takes, and allocates the file it opens.
const (
	// replaceAllocs is one Replace: the staging name, and 11 that
	// os.Root spends opening the staging file and renaming it over the
	// target.
	replaceAllocs = 12
	// replaceSharedAllocs is one ReplaceShared: Replace's, and the
	// random staging name's digits.
	replaceSharedAllocs = replaceAllocs + 1
)

// A replacement is atomic per file: the target contains the old bytes
// or the new ones, and no staging file outlives the call.
func TestStagefile(t *testing.T) {
	t.Parallel()

	t.Run("Replace", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the body to a missing target with the given mode", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t)
			assert.NoError(t, stagefile.Replace(r, target, []byte(body), mode), "the target is written")
			got, err := os.ReadFile(filepath.Join(dir, target))
			assert.NoError(t, err, "the target reads")
			assert.Equal(t, string(got), body, "the target contains the body")
			info, err := os.Stat(filepath.Join(dir, target))
			assert.NoError(t, err, "the target stats")
			assert.Equal(t, info.Mode().Perm(), mode, "the target has the mode")
		})

		t.Run("replaces the bytes of an existing target", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t)
			assert.NoError(t, os.WriteFile(filepath.Join(dir, target), []byte("old and longer\n"), mode),
				"the old target is written")
			assert.NoError(t, stagefile.Replace(r, target, []byte(body), mode), "the target is replaced")
			got, err := os.ReadFile(filepath.Join(dir, target))
			assert.NoError(t, err, "the target reads")
			assert.Equal(t, string(got), body, "the target contains the new body alone")
		})

		t.Run("leaves no staging file behind", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t)
			assert.NoError(t, stagefile.Replace(r, target, []byte(body), mode), "the target is written")
			assert.Equal(t, entries(t, dir), []string{target}, "the target alone remains")
		})

		t.Run("truncates a staging file a killed process left", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t)
			assert.NoError(t, os.WriteFile(filepath.Join(dir, target+stagefile.Suffix),
				[]byte("a stale staging file much longer than the body\n"), mode), "the stale staging file is written")
			assert.NoError(t, stagefile.Replace(r, target, []byte(body), mode), "the target is written")
			got, err := os.ReadFile(filepath.Join(dir, target))
			assert.NoError(t, err, "the target reads")
			assert.Equal(t, string(got), body, "the target contains the body alone")
		})

		t.Run("returns an error for a target whose directory is missing", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t)
			assert.HasError(t, stagefile.Replace(r, "missing/"+target, []byte(body), mode), "nothing is written")
			assert.Empty(t, entries(t, dir), "the root is untouched")
		})

		t.Run("returns an error for a target that is a directory", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t)
			assert.NoError(t, os.Mkdir(filepath.Join(dir, target), 0o700), "the directory is made")
			assert.NoError(t, os.WriteFile(filepath.Join(dir, target, "kept"), nil, mode), "the directory is not empty")
			assert.HasError(t, stagefile.Replace(r, target, []byte(body), mode), "the rename is refused")
			assert.Equal(t, entries(t, dir), []string{target}, "the staging file is removed")
		})

		t.Run("returns an error for a target outside the root", func(t *testing.T) {
			t.Parallel()

			_, r := rooted(t)
			assert.HasError(t, stagefile.Replace(r, "../"+target, []byte(body), mode), "the root jails the write")
		})
	})

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

				dir, r := rooted(t)
				assert.NoError(t, stagefile.ReplaceShared(r, target, []byte(body), mode, tt.give),
					"the target is written")
				got, err := os.ReadFile(filepath.Join(dir, target))
				assert.NoError(t, err, "the target reads")
				assert.Equal(t, string(got), body, "the target contains the body")
				info, err := os.Stat(filepath.Join(dir, target))
				assert.NoError(t, err, "the target stats")
				assert.Equal(t, info.Mode().Perm(), mode, "the target has the mode")
				assert.Equal(t, entries(t, dir), []string{target}, "no staging file remains")
			})
		}

		t.Run("replaces the bytes of an existing target", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t)
			assert.NoError(t, os.WriteFile(filepath.Join(dir, target), []byte("old and longer\n"), mode),
				"the old target is written")
			assert.NoError(t, stagefile.ReplaceShared(r, target, []byte(body), mode, stagefile.Synced),
				"the target is replaced")
			got, err := os.ReadFile(filepath.Join(dir, target))
			assert.NoError(t, err, "the target reads")
			assert.Equal(t, string(got), body, "the target contains the new body alone")
		})

		t.Run("leaves one writer's whole body when writers replace one target at once", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t)
			bodies := make([]string, 16)
			for i := range bodies {
				bodies[i] = strings.Repeat(strconv.Itoa(i%10), 4096+i)
			}
			var wg sync.WaitGroup
			for _, b := range bodies {
				wg.Go(func() {
					assert.NoError(t, stagefile.ReplaceShared(r, target, []byte(b), mode, stagefile.Unsynced),
						"each writer replaces the target")
				})
			}
			wg.Wait()
			got, err := os.ReadFile(filepath.Join(dir, target))
			assert.NoError(t, err, "the target reads")
			assert.True(t, slices.Contains(bodies, string(got)), "the target is one writer's whole body")
			assert.Equal(t, entries(t, dir), []string{target}, "no staging file remains")
		})

		t.Run("returns an error for a target whose directory is missing", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t)
			assert.HasError(t, stagefile.ReplaceShared(r, "missing/"+target, []byte(body), mode, stagefile.Synced),
				"nothing is written")
			assert.Empty(t, entries(t, dir), "the root is untouched")
		})

		t.Run("returns an error for a target that is a directory", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t)
			assert.NoError(t, os.Mkdir(filepath.Join(dir, target), 0o700), "the directory is made")
			assert.NoError(t, os.WriteFile(filepath.Join(dir, target, "kept"), nil, mode), "the directory is not empty")
			assert.HasError(t, stagefile.ReplaceShared(r, target, []byte(body), mode, stagefile.Synced),
				"the rename is refused")
			assert.Equal(t, entries(t, dir), []string{target}, "the staging file is removed")
		})

		t.Run("returns an error for a target outside the root", func(t *testing.T) {
			t.Parallel()

			_, r := rooted(t)
			assert.HasError(t, stagefile.ReplaceShared(r, "../"+target, []byte(body), mode, stagefile.Synced),
				"the root jails the write")
		})
	})
}

// A replacement allocates within its ceiling in the ordinary run,
// which runs no benchmark.
func TestStagefileAllocs(t *testing.T) {
	_, r := rooted(t)
	data := []byte(body)
	assert.MaxAllocs(t, func() {
		if err := stagefile.Replace(r, target, data, mode); err != nil {
			t.Fatalf("Replace: unexpected error: %v", err)
		}
	}, replaceAllocs, "Replace allocates the staging name and the file")
	assert.MaxAllocs(t, func() {
		if err := stagefile.ReplaceShared(r, target, data, mode, stagefile.Unsynced); err != nil {
			t.Fatalf("ReplaceShared: unexpected error: %v", err)
		}
	}, replaceSharedAllocs, "ReplaceShared allocates the random staging name and the file")
}

// BenchmarkStagefile measures one replacement of an existing target in
// a temporary directory.
func BenchmarkStagefile(b *testing.B) {
	data := []byte(body)

	b.Run("Replace", func(b *testing.B) {
		dir, r := rooted(b)
		c := bench.Start(b).MaxAllocs(replaceAllocs)
		defer c.End()
		var err error
		for c.Loop() {
			err = stagefile.Replace(r, target, data, mode)
		}
		assert.NoError(b, err, "every replacement succeeds")
		assert.Equal(b, entries(b, dir), []string{target}, "and leaves the target alone")
	})

	b.Run("ReplaceShared", func(b *testing.B) {
		dir, r := rooted(b)
		c := bench.Start(b).MaxAllocs(replaceSharedAllocs)
		defer c.End()
		var err error
		for c.Loop() {
			err = stagefile.ReplaceShared(r, target, data, mode, stagefile.Unsynced)
		}
		assert.NoError(b, err, "every replacement succeeds")
		assert.Equal(b, entries(b, dir), []string{target}, "and leaves the target alone")
	})
}

// rooted returns a fresh directory and the root opened over it, closed
// when the test ends.
func rooted(tb testing.TB) (string, *os.Root) {
	tb.Helper()

	dir := tb.TempDir()
	r, err := os.OpenRoot(dir)
	assert.NoError(tb, err, "the root opens")
	tb.Cleanup(func() { _ = r.Close() })
	return dir, r
}

// entries returns the names a directory contains.
func entries(tb testing.TB, dir string) []string {
	tb.Helper()

	es, err := os.ReadDir(dir)
	assert.NoError(tb, err, "the directory reads")
	names := make([]string, 0, len(es))
	for _, e := range es {
		names = append(names, e.Name())
	}
	return names
}
