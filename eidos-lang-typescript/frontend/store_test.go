// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"path"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/lang/typescript/frontend/library"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The paths inside the library store that the store cases open: a file of
// the library of a key, the directory of a key, a path below a file, a
// file that the release does not have, and a path that fs.ValidPath
// refuses.
const (
	storeFile    = es2024Root + "/" + es5Root
	storeKey     = es2024Root
	belowFile    = storeFile + "/x"
	missingFile  = es2024Root + "/" + unknownMember
	invalidPath  = "../" + es5Root
	es5Interface = "interface Error {"
	releaseFiles = 108
)

// The library store serves every file of the embedded library below every
// key, so the files of two libraries are the members of two units.
func TestStore(t *testing.T) {
	t.Parallel()

	stores, err := frontend.Stores(nil)
	assert.NoError(t, err, "the frontend locates its stores")
	assert.Contains(t, stores, typescript.LibraryStore, "the stores have the library store")
	store := stores[typescript.LibraryStore]

	t.Run("Stores", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the library store alone", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, stores, 1, "the frontend reads no other store")
		})

		t.Run("returns a store that serves a file of the library of a key", func(t *testing.T) {
			t.Parallel()

			got, err := fs.ReadFile(store, storeFile)
			assert.NoError(t, err, "the store reads the file of a key")
			want, err := fs.ReadFile(library.Files, path.Join(library.Dir, es5Root))
			assert.NoError(t, err, "the embed reads the file")
			assert.Equal(t, got, want, "the store serves the embedded file")
			assert.Contains(t, string(got), es5Interface, "the file is the library of ES5")
		})

		t.Run("returns a store that lists a key as the directory of the library", func(t *testing.T) {
			t.Parallel()

			entries, err := fs.ReadDir(store, storeKey)
			assert.NoError(t, err, "the store lists a key")
			assert.Length(t, entries, releaseFiles, "the directory of a key has every file of the release")
		})

		t.Run("returns a store that lists its root as the directory of the library", func(t *testing.T) {
			t.Parallel()

			entries, err := fs.ReadDir(store, ".")
			assert.NoError(t, err, "the store lists its root")
			assert.NotEmpty(t, entries, "the root has the files of the release")
		})

		t.Run("returns a store that reads through the qualified path of a load", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.ReadFile(storeTree{nil, stores}, plugin.StorePath(typescript.LibraryStore, storeFile))
			assert.NoError(t, err, "a load reads the file through its qualified path")
		})

		faults := []struct {
			name string
			give string
			want error
		}{
			{name: "returns a store that refuses a path below a file", give: belowFile, want: fs.ErrNotExist},
			{name: "returns a store that refuses a file outside the release", give: missingFile, want: fs.ErrNotExist},
			{name: "returns a store that refuses an invalid path", give: invalidPath, want: fs.ErrInvalid},
		}
		for _, tt := range faults {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := store.Open(tt.give)
				assert.ErrorIs(t, err, tt.want, "the store refuses the path")
				refused := assert.ErrorAs[*fs.PathError](t, err, "the store refuses with a path error")
				assert.Equal(t, refused.Path, tt.give, "the path error has the path of the open")
			})
		}
	})
}
