// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

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

// Qualified paths address a file inside a named store, and the read
// and stat helpers resolve them against the load's tree.
func TestStore(t *testing.T) {
	t.Parallel()

	t.Run("StorePath", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the store, the separator and the path inside the store", func(t *testing.T) {
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
				name: "returns the store and the path of a qualified path",
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
