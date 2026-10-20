// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The second module the partition cases claim beside src/a.ts.
const bFile = "src/b.ts"

// Two modules in sibling directories below src, whose configuration the
// probe case counts the reads of.
const (
	xFile = "src/x/a.ts"
	yFile = "src/y/b.ts"
)

// TypeScript scopes a module's names to its file, so a unit is one
// file, and the chain it declares is what re-keys it.
func TestPartition(t *testing.T) {
	t.Parallel()

	t.Run("partition", func(t *testing.T) {
		t.Parallel()

		t.Run("makes each claimed file a unit of its own", func(t *testing.T) {
			t.Parallel()

			units := partitioned(t, fstest.MapFS{}, aFile, bFile)
			assert.Length(t, units, 2, "a unit per file")
			assert.Length(t, units[1], 1, "of one member")
		})

		t.Run("declares one chain for every file one configuration governs", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{rootConfig: {Data: []byte("{}")}}
			units := partitioned(t, tree, aFile, bFile)
			assert.Equal(t, units[0][0].Shared, units[1][0].Shared, "both files share the root configuration")
		})

		t.Run("declares the configurations a chain read before a fault", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				rootConfig: {Data: []byte(`{"extends": "./base.json"}`)},
				baseConfig: {Data: []byte("{")},
			}
			assert.Equal(t, partitioned(t, tree, aFile)[0][0].Shared, []string{rootConfig, baseConfig},
				"a fixed configuration re-keys the file")
		})

		t.Run("declares the nearest package.json after the chain", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{rootConfig: {Data: []byte("{}")}, packageFile: {Data: []byte("{}")}}
			assert.Equal(t, partitioned(t, tree, aFile)[0][0].Shared, []string{rootConfig, packageFile},
				"the manifest of the module follows the configuration")
		})

		t.Run("declares the nearest package.json alone without a configuration", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{packageFile: {Data: []byte("{}")}}
			assert.Equal(t, partitioned(t, tree, aFile)[0][0].Shared, []string{packageFile},
				"the manifest of the module is the one shared input")
		})

		t.Run("probes a directory above two siblings once", func(t *testing.T) {
			t.Parallel()

			r := countingReader{tree: fstest.MapFS{}, reads: map[string]int{}}
			refs := []plugin.SourceRef{{Path: xFile}, {Path: yFile}}
			_, err := frontend.New().Partition(t.Context(), refs, r)
			assert.NoError(t, err, "the tree partitions")
			assert.Equal(t, r.reads[srcConfig], 1, "the second probe stops at the directory the first visited")
		})

		libraries := []struct {
			name     string
			giveTree fstest.MapFS
			want     []string
		}{
			{
				name:     "declares the default library for a file without a configuration",
				giveTree: fstest.MapFS{},
				want:     []string{defaultInput},
			},
			{
				name:     "declares the library of the governing configuration after its chain",
				giveTree: fstest.MapFS{rootConfig: {Data: []byte(`{"compilerOptions": {"lib": ["ES2024"]}}`)}},
				want:     []string{rootConfig, es2024Input},
			},
			{
				name: "declares the library between the chain and the nearest package.json",
				giveTree: fstest.MapFS{
					rootConfig:  {Data: []byte(`{"compilerOptions": {"lib": ["ES2024"]}}`)},
					packageFile: {Data: []byte("{}")},
				},
				want: []string{rootConfig, es2024Input, packageFile},
			},
			{
				name:     "does not declare a library for a chain under noLib",
				giveTree: fstest.MapFS{rootConfig: {Data: []byte(`{"compilerOptions": {"noLib": true}}`)}},
				want:     []string{rootConfig},
			},
		}
		for _, tt := range libraries {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				units := partitionedThrough(t, storeTree{tt.giveTree, realStores(t)}, aFile)
				assert.Equal(t, units[0][0].Shared, tt.want, "the shared inputs of the file")
			})
		}

		t.Run("does not declare a library whose root the store lacks", func(t *testing.T) {
			t.Parallel()

			units := partitionedThrough(t, storeTree{fstest.MapFS{}, map[string]fs.FS{
				typescript.LibraryStore: fstest.MapFS{},
			}}, aFile)
			assert.Empty(t, units[0][0].Shared, "the partition does not declare a library that the store lacks")
		})

		t.Run("probes the store once for each library", func(t *testing.T) {
			t.Parallel()

			r := countingStoreReader{tree: storeTree{fstest.MapFS{}, realStores(t)}, reads: map[string]int{}}
			refs := []plugin.SourceRef{{Path: xFile}, {Path: yFile}}
			_, err := frontend.New().Partition(t.Context(), refs, r)
			assert.NoError(t, err, "the tree partitions")
			assert.Equal(t, r.reads[defaultInput], 1, "the second file takes the probe of the first")
		})

		t.Run("returns the error of a probe of the store that fails", func(t *testing.T) {
			t.Parallel()

			tree := storeTree{fstest.MapFS{}, map[string]fs.FS{
				typescript.LibraryStore: failingFS{fakeLibrary(), defaultRoot + "/" + defaultRoot},
			}}
			_, err := frontend.New().Partition(t.Context(), []plugin.SourceRef{{Path: aFile}}, roundReader{tree})
			assert.ErrorIs(t, err, errBroken, "the partition returns the failure of the probe")
		})
	})
}

// es2024Input is the shared input of the library of ES2024: its root inside
// its key.
var es2024Input = plugin.StorePath(typescript.LibraryStore, es2024Root+"/"+es2024Root)

// countingReader reads a tree and counts the reads of each path.
type countingReader struct {
	tree  fstest.MapFS
	reads map[string]int
}

// Read counts the read of path and returns the file's bytes.
func (r countingReader) Read(path string) ([]byte, error) {
	r.reads[path]++
	return r.tree.ReadFile(path)
}

// countingStoreReader reads a tree with stores and counts the reads of each
// path.
type countingStoreReader struct {
	tree  fs.FS
	reads map[string]int
}

// Read counts the read of path and returns the bytes of the file of the
// workspace or of a store.
func (r countingStoreReader) Read(path string) ([]byte, error) {
	r.reads[path]++
	return plugin.ReadFile(r.tree, path)
}

// partitioned partitions every file of a tree through the frontend.
func partitioned(tb testing.TB, tree fstest.MapFS, claimed ...string) [][]plugin.SourceRef {
	tb.Helper()

	refs := make([]plugin.SourceRef, 0, len(claimed))
	for _, p := range claimed {
		refs = append(refs, plugin.SourceRef{Path: p})
	}
	units, err := frontend.New().Partition(tb.Context(), refs, treeReader{tree})
	assert.NoError(tb, err, "the tree partitions")
	return units
}
