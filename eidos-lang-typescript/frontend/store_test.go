// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"path"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

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

// The fragments that the store cases open: the statements of Error in ES5,
// the statements of a name that ES5 does not declare, a fragment of a file
// that the release does not have, and a fragment of the directory of the
// library. ES5 declares Error twice, as an interface and as a variable.
const (
	errorName       = "Error"
	errorFragment   = storeFile + "#" + errorName
	missingFragment = storeFile + "#Missing"
	unknownFragment = missingFile + "#" + errorName
	dirFragment     = storeKey + "/.#" + errorName
	errorVariable   = "declare var Error: ErrorConstructor;"
)

// openFragmentAllocs is the count of the open of a fragment: the buffer of
// its statements, the reader over them and the open file.
const openFragmentAllocs = 3

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

		t.Run("returns a store that serves the statements of a name of a file at its fragment", func(t *testing.T) {
			t.Parallel()

			src, err := fs.ReadFile(library.Files, path.Join(library.Dir, es5Root))
			assert.NoError(t, err, "the embed reads the file")
			var want []byte
			for _, s := range library.Index[es5Root].Statements {
				if s.Name == errorName {
					want = append(append(want, src[s.Start:s.End]...), '\n')
				}
			}
			got, err := fs.ReadFile(store, errorFragment)
			assert.NoError(t, err, "the store reads the fragment")
			assert.Equal(t, got, want, "the fragment has each statement of the name, followed by a line break")
			expect.That(t, string(got)).Contains(es5Interface, "the fragment has the interface").
				Contains(errorVariable, "the fragment has the variable")
		})

		t.Run("returns a store that states the stat of a fragment", func(t *testing.T) {
			t.Parallel()

			info, err := fs.Stat(store, errorFragment)
			assert.NoError(t, err, "the store states the fragment")
			got, err := fs.ReadFile(store, errorFragment)
			assert.NoError(t, err, "the store reads the fragment")
			want, err := fs.Stat(library.Files, path.Join(library.Dir, es5Root))
			assert.NoError(t, err, "the embed states the file")
			expect.Equal(t, info.Name(), path.Base(errorFragment), "the stat has the base name of the fragment")
			expect.Equal(t, info.Size(), int64(len(got)), "the stat has the size of the statements")
			expect.Equal(t, info.Mode(), want.Mode(), "the stat has the mode of an embedded file")
			expect.Equal(t, info.ModTime(), want.ModTime(), "the stat has the time of an embedded file")
			expect.False(t, info.IsDir(), "a fragment is a file")
			expect.Nil(t, info.Sys(), "a fragment has no data of a file system")
		})

		faults := []struct {
			name string
			give string
			want error
		}{
			{name: "returns a store that refuses a path below a file", give: belowFile, want: fs.ErrNotExist},
			{name: "returns a store that refuses a file outside the release", give: missingFile, want: fs.ErrNotExist},
			{name: "returns a store that refuses an invalid path", give: invalidPath, want: fs.ErrInvalid},
			{
				name: "returns a store that refuses a fragment of a name that the file does not declare",
				give: missingFragment, want: fs.ErrNotExist,
			},
			{
				name: "returns a store that refuses a fragment of a file outside the release",
				give: unknownFragment, want: fs.ErrNotExist,
			},
		}
		for _, tt := range faults {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := store.Open(tt.give)
				refused := assert.ErrorAs[*fs.PathError](t, err, "the store refuses with a path error")
				expect.Equal(t, refused.Path, tt.give, "the path error has the path of the open")
				expect.Equal(t, refused.Err, tt.want, "the path error wraps the cause alone")
			})
		}

		t.Run("returns a store that refuses a fragment of the directory of the library", func(t *testing.T) {
			t.Parallel()

			_, err := store.Open(dirFragment)
			refused := assert.ErrorAs[*fs.PathError](t, err, "the store refuses with a path error")
			expect.Equal(t, refused.Path, dirFragment, "the path error has the path of the open")
			expect.ErrorIsNot(t, err, fs.ErrNotExist, "the path error has the cause of the embed")
		})
	})
}

// The open of a fragment allocates the statements of its name and its open
// file. The ordinary run, which runs no benchmark, checks that ceiling
// here.
func TestStoreAllocs(t *testing.T) {
	checkAllocs(t, storeCalls(t))
}

// BenchmarkStore measures the open of a fragment, which the gate and the
// parse of each member of a library unit make.
func BenchmarkStore(b *testing.B) {
	benchCalls(b, storeCalls(b))
}

// storeCalls returns an open of the fragment of Error in ES5.
func storeCalls(tb testing.TB) []allocCall {
	tb.Helper()

	stores, err := frontend.Stores(nil)
	assert.NoError(tb, err, "the frontend locates its stores")
	store := stores[typescript.LibraryStore]
	var (
		f    fs.File
		open error
	)
	return []allocCall{
		{
			name: "Open", caseName: "a fragment", allocs: openFragmentAllocs,
			call: func() { f, open = store.Open(errorFragment) },
			check: func(tb testing.TB) {
				tb.Helper()

				assert.NoError(tb, open, "the store opens the fragment")
				info, err := f.Stat()
				assert.NoError(tb, err, "the fragment states itself")
				assert.Equal(tb, info.Name(), path.Base(errorFragment), "Open opens the fragment of Error")
			},
		},
	}
}
