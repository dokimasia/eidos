// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"cmp"
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/plugin"
)

// The store the qualified-path cases read, one file inside it, the
// qualified path of that file, and a workspace file beside it.
const (
	cacheStore    = "gomod"
	cacheFile     = "example.test/lib@v1.0.0/lib.go"
	qualifiedFile = cacheStore + "://" + cacheFile
	cacheRoot     = cacheStore + "://"
	workspaceFile = "svc/a.go"
)

// storeTree is a workspace tree with named stores beside it, the shape
// the load hands its units.
type storeTree struct {
	fstest.MapFS
	stores map[string]fs.FS
}

// Store returns one named store.
func (t storeTree) Store(name string) (fs.FS, bool) {
	s, held := t.stores[name]
	return s, held
}

// storeRead is one read through the store helpers beside the read of
// the tree or the store it resolves to, which the helper adds nothing
// to: the helper, which names its benchmark, and the case it reads. The
// reads of one helper are adjacent in a list of reads.
type storeRead struct {
	name     string
	caseName string
	own      func() error
	plain    func() error
}

// Qualified paths address a file inside a named store, and the read
// and stat helpers resolve them against the load's tree.
func TestStore(t *testing.T) {
	t.Parallel()

	t.Run("StorePath", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the store-qualified spelling of a path", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, plugin.StorePath(cacheStore, cacheFile), qualifiedFile, "the qualified spelling")
		})
	})

	t.Run("CutStorePath", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			give      string
			wantStore string
			wantPath  string
			wantOK    bool
		}{
			{
				name: "returns the parts of a qualified path",
				give: qualifiedFile, wantStore: cacheStore, wantPath: cacheFile, wantOK: true,
			},
			{
				name: "returns the empty path for a store's root",
				give: cacheRoot, wantStore: cacheStore, wantOK: true,
			},
			{name: "reports false for a workspace path", give: workspaceFile},
			{name: "reports false for an empty store name", give: "://" + cacheFile},
			{name: "reports false for a store name with a slash", give: "a/b://" + cacheFile},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				store, path, ok := plugin.CutStorePath(tt.give)
				assert.Equal(t, ok, tt.wantOK, "whether the path is qualified")
				assert.Equal(t, store, tt.wantStore, "the store's name")
				assert.Equal(t, path, tt.wantPath, "the path inside the store")
			})
		}
	})

	t.Run("ValidStoreName", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "reports true for a plain name", give: cacheStore, want: true},
			{name: "reports false for the empty name", give: ""},
			{name: "reports false for a name with a colon", give: "go:mod"},
			{name: "reports false for a name with a slash", give: "go/mod"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, plugin.ValidStoreName(tt.give), tt.want, "the name's validity")
			})
		}
	})

	t.Run("ReadFile", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a workspace file from the tree", func(t *testing.T) {
			t.Parallel()

			b, err := plugin.ReadFile(withCache(), workspaceFile)
			assert.NoError(t, err, "the workspace file reads")
			assert.Equal(t, string(b), "package svc\n", "the tree's bytes")
		})

		t.Run("returns a qualified file from its store", func(t *testing.T) {
			t.Parallel()

			b, err := plugin.ReadFile(withCache(), qualifiedFile)
			assert.NoError(t, err, "the qualified file reads")
			assert.Equal(t, string(b), "package lib\n", "the store's bytes")
		})

		t.Run("returns ErrStoreAbsent for a store the tree does not provide", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.ReadFile(withCache(), plugin.StorePath("goroot", "fmt/print.go"))
			assert.ErrorIs(t, err, plugin.ErrStoreAbsent, "the source is off")
			assert.Contains(t, err.Error(), "goroot", "the error names the store")
		})

		t.Run("returns ErrStoreAbsent for a tree without stores", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.ReadFile(fstest.MapFS{}, qualifiedFile)
			assert.ErrorIs(t, err, plugin.ErrStoreAbsent, "a plain tree provides no store")
		})

		t.Run("returns fs.ErrNotExist for a file its store lacks", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.ReadFile(withCache(), plugin.StorePath(cacheStore, "example.test/gone@v1.0.0/g.go"))
			assert.ErrorIs(t, err, fs.ErrNotExist, "a configured store that lacks the file")
		})
	})

	t.Run("ReadDir", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a store root's entries", func(t *testing.T) {
			t.Parallel()

			entries, err := plugin.ReadDir(withCache(), cacheRoot)
			assert.NoError(t, err, "the root lists")
			assert.Length(t, entries, 1, "one top-level directory")
			assert.Equal(t, entries[0].Name(), "example.test", "the store's own entry")
		})

		t.Run("returns a workspace directory's entries", func(t *testing.T) {
			t.Parallel()

			entries, err := plugin.ReadDir(withCache(), "svc")
			assert.NoError(t, err, "the directory lists")
			assert.Length(t, entries, 1, "one file")
		})

		t.Run("returns ErrStoreAbsent for a store the tree does not provide", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.ReadDir(withCache(), "goroot://")
			assert.ErrorIs(t, err, plugin.ErrStoreAbsent, "the source is off")
		})
	})

	t.Run("Stat", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a store file's stat for a qualified path", func(t *testing.T) {
			t.Parallel()

			info, err := plugin.Stat(withCache(), qualifiedFile)
			assert.NoError(t, err, "the store provides the file")
			assert.Equal(t, info.Size(), int64(len("package lib\n")), "the stat is the store file's")
		})

		t.Run("returns a workspace file's stat", func(t *testing.T) {
			t.Parallel()

			info, err := plugin.Stat(withCache(), workspaceFile)
			assert.NoError(t, err, "the workspace provides the file")
			assert.Equal(t, info.Size(), int64(len("package svc\n")), "the stat is the workspace file's")
		})

		t.Run("returns ErrStoreAbsent for a store the tree does not provide", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.Stat(withCache(), "goroot://fmt/print.go")
			assert.ErrorIs(t, err, plugin.ErrStoreAbsent, "the source is off")
		})

		t.Run("returns ErrNotExist for a path no file is at", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.Stat(withCache(), "svc/absent.go")
			assert.ErrorIs(t, err, fs.ErrNotExist, "nothing is at the path")
		})
	})
}

// The qualified spelling allocates its joined path, the split and the
// check allocate nothing, and a read through a helper allocates what the
// read of the tree or the store allocates, in the ordinary run, which
// runs no benchmark. The check runs alone, because the count includes
// every goroutine's allocations.
func TestStoreAllocs(t *testing.T) {
	var spelled string
	assert.MaxAllocs(t, func() { spelled = plugin.StorePath(cacheStore, cacheFile) }, 1,
		"StorePath allocates the joined path")
	assert.Equal(t, spelled, qualifiedFile, "StorePath returns the qualified path")

	var store string
	assert.MaxAllocs(t, func() { store, _, _ = plugin.CutStorePath(qualifiedFile) }, 0,
		"CutStorePath allocates nothing")
	assert.Equal(t, store, cacheStore, "CutStorePath returns the store")

	valid := false
	assert.MaxAllocs(t, func() { valid = plugin.ValidStoreName(cacheStore) }, 0, "ValidStoreName allocates nothing")
	assert.True(t, valid, "ValidStoreName reports true for a plain name")

	for _, tt := range storeReads(t) {
		label := tt.name + " for " + tt.caseName
		var err error
		assert.MaxAllocs(t, func() { err = cmp.Or(err, tt.own()) }, plainAllocs(t, tt),
			label+" allocates what the read of its tree allocates")
		assert.NoError(t, err, label+" reads")
	}
}

// BenchmarkStore measures the qualified spelling, its split and its
// check, and each read helper over a workspace path and a qualified
// one, against what the read of the tree or the store allocates.
func BenchmarkStore(b *testing.B) {
	b.Run("StorePath", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got string
		for c.Loop() {
			got = plugin.StorePath(cacheStore, cacheFile)
		}
		assert.Equal(b, got, qualifiedFile, "StorePath returns the qualified path")
	})

	b.Run("CutStorePath", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var store string
		for c.Loop() {
			store, _, _ = plugin.CutStorePath(qualifiedFile)
		}
		assert.Equal(b, store, cacheStore, "CutStorePath returns the store")
	})

	b.Run("ValidStoreName", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		valid := false
		for c.Loop() {
			valid = plugin.ValidStoreName(cacheStore)
		}
		assert.True(b, valid, "ValidStoreName reports true for a plain name")
	})

	reads := storeReads(b)
	for len(reads) > 0 {
		n := 1
		for n < len(reads) && reads[n].name == reads[0].name {
			n++
		}
		helper := reads[:n]
		reads = reads[n:]
		b.Run(helper[0].name, func(b *testing.B) {
			for _, tt := range helper {
				b.Run(tt.caseName, func(b *testing.B) {
					c := bench.Start(b).MaxAllocs(plainAllocs(b, tt))
					defer c.End()
					var err error
					for c.Loop() {
						err = tt.own()
					}
					assert.NoError(b, err, "the read succeeds")
				})
			}
		})
	}
}

// withCache returns a workspace tree with one file and a module-cache
// store with one file.
func withCache() storeTree {
	return storeTree{
		MapFS: fstest.MapFS{workspaceFile: {Data: []byte("package svc\n")}},
		stores: map[string]fs.FS{
			cacheStore: fstest.MapFS{cacheFile: {Data: []byte("package lib\n")}},
		},
	}
}

// storeReads returns every read helper over a workspace path and a
// qualified path, each beside the read it resolves to. The tree is an
// interface value made once, so no read counts its conversion.
func storeReads(tb assert.TB) []storeRead {
	tb.Helper()

	tree := withCache()
	cache := tree.stores[cacheStore]
	var fsys fs.FS = tree
	return []storeRead{
		{
			name: "ReadFile", caseName: "a workspace file",
			own:   func() error { _, err := plugin.ReadFile(fsys, workspaceFile); return err },
			plain: func() error { _, err := fs.ReadFile(fsys, workspaceFile); return err },
		},
		{
			name: "ReadFile", caseName: "a store file",
			own:   func() error { _, err := plugin.ReadFile(fsys, qualifiedFile); return err },
			plain: func() error { _, err := fs.ReadFile(cache, cacheFile); return err },
		},
		{
			name: "ReadDir", caseName: "a store root",
			own:   func() error { _, err := plugin.ReadDir(fsys, cacheRoot); return err },
			plain: func() error { _, err := fs.ReadDir(cache, "."); return err },
		},
		{
			name: "Stat", caseName: "a store file",
			own:   func() error { _, err := plugin.Stat(fsys, qualifiedFile); return err },
			plain: func() error { _, err := fs.Stat(cache, cacheFile); return err },
		},
	}
}

// plainAllocs returns what the read a helper resolves to allocates,
// measured with testing.AllocsPerRun: the ceiling of the helper's own
// read, which adds nothing to it. It stops the check where the read of
// the tree fails.
func plainAllocs(tb assert.TB, r storeRead) uint64 {
	tb.Helper()

	var err error
	n := testing.AllocsPerRun(100, func() { err = cmp.Or(err, r.plain()) })
	assert.NoError(tb, err, "the read of the tree under "+r.name+" succeeds")
	return uint64(n)
}
