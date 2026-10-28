// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package descriptor_test

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/descriptor"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The files of the store of the dependency cases.
const (
	storeFirst  = "acme/a.proto"
	storeSecond = "acme/deep/b.proto"
	storeOther  = "acme/README.md"
)

// errListing is the failure of the listing of a store.
var errListing = errors.New("the store does not list")

// storeReader reads the store of the dependency cases. A listing returns
// err, of the directory failAt where it is set, and of every directory
// otherwise.
type storeReader struct {
	tree   fstest.MapFS
	err    error
	failAt string
}

// Read reads one qualified path of the store.
func (r storeReader) Read(path string) ([]byte, error) {
	_, inner, _ := plugin.CutStorePath(path)
	return r.tree.ReadFile(inner)
}

// ReadDir lists one qualified directory of the store, or returns the
// error of the reader.
func (r storeReader) ReadDir(path string) ([]fs.DirEntry, error) {
	_, inner, _ := plugin.CutStorePath(path)
	if r.err != nil && (r.failAt == "" || r.failAt == inner) {
		return nil, r.err
	}
	if inner == "" {
		inner = "."
	}
	return r.tree.ReadDir(inner)
}

func TestFrontend(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		f := descriptor.New()

		t.Run("returns a frontend with the name and the language of the satellite", func(t *testing.T) {
			t.Parallel()

			expect.Equal(t, f.Name(), protobuf.Name, "the name is the name of the source frontend")
			expect.Equal(t, f.Lang(), protobuf.Lang, "the language is protobuf")
		})

		t.Run("returns a frontend in the dependent role", func(t *testing.T) {
			t.Parallel()

			_, dependent := f.(plugin.Dependent)
			assert.True(t, dependent, "the frontend loads the files of the store")
		})

		t.Run("returns a frontend in the importer role", func(t *testing.T) {
			t.Parallel()

			_, importer := f.(plugin.Importer)
			assert.True(t, importer, "a proto import names a file")
		})

		t.Run("returns a partition of one unit per file with the registry as its shared input", func(t *testing.T) {
			t.Parallel()

			units, err := f.Partition(t.Context(),
				[]plugin.SourceRef{{Path: storeSecond}, {Path: storeFirst}}, nil)
			assert.NoError(t, err, "the partition succeeds")
			assert.Equal(t, units, [][]plugin.SourceRef{
				{{Path: storeFirst, Shared: []string{descriptor.Registry}}},
				{{Path: storeSecond, Shared: []string{descriptor.Registry}}},
			}, "one unit per file in path order")
		})
	})

	t.Run("Dependencies", func(t *testing.T) {
		t.Parallel()

		dependent, _ := descriptor.New().(plugin.Dependent)
		tree := fstest.MapFS{storeSecond: {}, storeFirst: {}, storeOther: {}}

		t.Run("returns one unit for each proto file of the store in the first round", func(t *testing.T) {
			t.Parallel()

			units, err := dependent.Dependencies(
				t.Context(),
				&plugin.DependencyRound{Number: 1},
				storeReader{tree: tree},
			)
			assert.NoError(t, err, "the round succeeds")
			assert.Equal(t, units, [][]plugin.SourceRef{
				{{Path: plugin.StorePath(descriptor.Store, storeFirst), Shared: []string{descriptor.Registry}}},
				{{Path: plugin.StorePath(descriptor.Store, storeSecond), Shared: []string{descriptor.Registry}}},
			}, "each proto file in path order")
		})

		t.Run("returns no unit in a later round", func(t *testing.T) {
			t.Parallel()

			units, err := dependent.Dependencies(
				t.Context(),
				&plugin.DependencyRound{Number: 2},
				storeReader{tree: tree},
			)
			assert.NoError(t, err, "the round succeeds")
			assert.Empty(t, units, "the first round loaded every file")
		})

		t.Run("returns no unit for a load without the store", func(t *testing.T) {
			t.Parallel()

			units, err := dependent.Dependencies(t.Context(), &plugin.DependencyRound{Number: 1},
				storeReader{err: plugin.ErrStoreAbsent})
			assert.NoError(t, err, "an absent store is no failure")
			assert.Empty(t, units, "nothing loads")
		})

		t.Run("returns the error of a listing", func(t *testing.T) {
			t.Parallel()

			_, err := dependent.Dependencies(t.Context(), &plugin.DependencyRound{Number: 1},
				storeReader{err: errListing})
			assert.ErrorIs(t, err, errListing, "the listing fails")
		})

		t.Run("returns the error of a listing of a directory below the root", func(t *testing.T) {
			t.Parallel()

			r := storeReader{tree: fstest.MapFS{"acme/v1/rows.proto": {}}, err: errListing, failAt: "acme"}
			_, err := dependent.Dependencies(t.Context(), &plugin.DependencyRound{Number: 1}, r)
			assert.ErrorIs(t, err, errListing, "the listing of the directory fails")
		})
	})

	t.Run("MissingSourceInfo", func(t *testing.T) {
		t.Parallel()

		t.Run("spells the code of the satellite", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, descriptor.MissingSourceInfo.String(), "PROTO-0007", "the spelling is pinned")
		})

		t.Run("registers its meaning", func(t *testing.T) {
			t.Parallel()

			meaning, held := diag.Kernel().Meaning(descriptor.MissingSourceInfo)
			assert.True(t, held, "the code registered at initialization")
			assert.Equal(t, meaning, "a file to generate has no source code info", "the meaning is pinned")
		})
	})
}
