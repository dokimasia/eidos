// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The second module the partition cases claim beside src/a.ts.
const bFile = "src/b.ts"

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
	})
}

// partitioned partitions every file of a tree through the frontend.
func partitioned(tb assert.TB, tree fstest.MapFS, claimed ...string) [][]plugin.SourceRef {
	tb.Helper()

	refs := make([]plugin.SourceRef, 0, len(claimed))
	for _, p := range claimed {
		refs = append(refs, plugin.SourceRef{Path: p})
	}
	units, err := frontend.New().Partition(context.Background(), refs, treeReader{tree})
	assert.NoError(tb, err, "the tree partitions")
	return units
}
