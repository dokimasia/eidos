// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The files the partition cases place: a test file beside the fixture
// file's package, and two files of directories below one directory
// without a pom.xml.
const (
	testFile  = "src/test/java/com/acme/ATest.java"
	deepLeft  = "src/main/java/com/acme/left/L.java"
	deepRight = "src/main/java/com/acme/right/R.java"
)

// countingReader is a tree's door that counts the reads of each path.
type countingReader struct {
	tree  fstest.MapFS
	reads map[string]int
}

// Read returns one file's bytes and counts the read.
func (r *countingReader) Read(p string) ([]byte, error) {
	r.reads[p]++
	return r.tree.ReadFile(p)
}

// The partition groups a workspace's Java files into directories, so
// which files share a unit, and which pom.xml each declares, is pinned.
func TestPartition(t *testing.T) {
	t.Parallel()

	t.Run("partition", func(t *testing.T) {
		t.Parallel()

		t.Run("makes one unit of the files of one directory", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{srcFile: {}, siblingFile: {}}
			assert.Equal(t, unitsOf(t, tree), [][]string{{srcFile, siblingFile}}, "one package directory")
		})

		t.Run("makes a unit of each directory in path order", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{testFile: {}, srcFile: {}}
			assert.Equal(t, unitsOf(t, tree), [][]string{{srcFile}, {testFile}},
				"the main and the test directory of one package")
		})

		t.Run("declares the nearest pom.xml above a file as its shared input", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{pomFile: {}, srcFile: {}}
			assert.Equal(t, sharedOf(t, tree), [][]string{{pomFile}}, "the project file at the root")
		})

		t.Run("declares no shared input for a file no pom.xml governs", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, sharedOf(t, fstest.MapFS{srcFile: {}}), [][]string{nil}, "no project file")
		})
	})

	t.Run("governing", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a nested pom.xml before the one above it", func(t *testing.T) {
			t.Parallel()

			nestedPOM, nestedSrc := path.Join(nestedRoot, pomFile), path.Join(nestedRoot, srcFile)
			tree := fstest.MapFS{pomFile: {}, nestedPOM: {}, srcFile: {}, nestedSrc: {}}
			assert.Equal(t, sharedOf(t, tree), [][]string{{pomFile}, {nestedPOM}}, "each module's own")
		})

		t.Run("reads each directory's pom.xml once across the files below it", func(t *testing.T) {
			t.Parallel()

			r := &countingReader{tree: fstest.MapFS{deepLeft: {}, deepRight: {}}, reads: map[string]int{}}
			_, err := frontend.New(nil).Partition(context.Background(), claimedIn(r.tree), r)
			assert.NoError(t, err, "the tree partitions")
			assert.Equal(t, r.reads[pomFile], 1, "the root's probe is cached")
		})
	})
}

// claimedIn returns a tree's Java files as the claim the load hands the
// partition, in path order.
func claimedIn(tree fstest.MapFS) []plugin.SourceRef {
	var claimed []plugin.SourceRef
	for p := range tree {
		if strings.HasSuffix(p, java.Extension) {
			claimed = append(claimed, plugin.SourceRef{Path: p})
		}
	}
	slices.SortFunc(claimed, func(a, b plugin.SourceRef) int { return strings.Compare(a.Path, b.Path) })
	return claimed
}

// partitioned partitions a tree's Java files through the frontend and
// returns its units.
func partitioned(tb assert.TB, tree fstest.MapFS) [][]plugin.SourceRef {
	tb.Helper()

	units, err := frontend.New(nil).Partition(context.Background(), claimedIn(tree), treeReader{tree})
	assert.NoError(tb, err, "the tree partitions")
	return units
}

// unitsOf returns each unit's members of a tree's partition, in
// partition order.
func unitsOf(tb assert.TB, tree fstest.MapFS) [][]string {
	tb.Helper()

	var out [][]string
	for _, unit := range partitioned(tb, tree) {
		var members []string
		for _, ref := range unit {
			members = append(members, ref.Path)
		}
		out = append(out, members)
	}
	return out
}

// sharedOf returns every member's shared inputs of a tree's partition,
// member by member in partition order.
func sharedOf(tb assert.TB, tree fstest.MapFS) [][]string {
	tb.Helper()

	var out [][]string
	for _, unit := range partitioned(tb, tree) {
		for _, ref := range unit {
			out = append(out, ref.Shared)
		}
	}
	return out
}
