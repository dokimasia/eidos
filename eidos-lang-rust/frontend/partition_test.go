// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The nested package the partition cases place files under, and its
// manifest.
const (
	nestedManifest = "sub/Cargo.toml"
	nestedRoot     = "sub/src/lib.rs"
)

// The partition groups a workspace's Rust files into crate targets, so
// which files share a unit, and in what order, is pinned.
func TestPartition(t *testing.T) {
	t.Parallel()

	t.Run("partition", func(t *testing.T) {
		t.Parallel()

		t.Run("puts a crate's root first in its unit", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, unitsOf(t, crateTree(map[string]string{libRoot: "", "src/a.rs": ""})),
				[][]string{{libRoot, "src/a.rs"}}, "the root, then the members in path order")
		})

		t.Run("declares the manifest as every member's shared input", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, sharedOf(t, crateTree(map[string]string{libRoot: "", "src/a.rs": ""})),
				[][]string{{manifestPath}, {manifestPath}}, "the manifest states the crate")
		})

		t.Run("groups a file under the nearest manifest above it", func(t *testing.T) {
			t.Parallel()

			tree := crateTree(map[string]string{libRoot: "", nestedRoot: ""})
			tree[nestedManifest] = &fstest.MapFile{Data: []byte(manifestSrc)}
			assert.Equal(t, sharedOf(t, tree), [][]string{{manifestPath}, {nestedManifest}},
				"each package's manifest states its own files")
		})

		t.Run("finds the manifest of a file several directories below it", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, sharedOf(t, crateTree(map[string]string{libRoot: "", "src/a/b/c.rs": ""})),
				[][]string{{manifestPath}, {manifestPath}}, "the probe climbs to the root")
		})

		t.Run("makes a file without a manifest above it a unit without a shared input", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{"a.rs": {}, "b.rs": {}}
			assert.Equal(t, unitsOf(t, tree), [][]string{{"a.rs"}, {"b.rs"}}, "each file is a crate of its own")
			assert.Equal(t, sharedOf(t, tree), [][]string{nil, nil}, "with no manifest to read")
		})

		t.Run("makes each file of a manifest that does not parse a unit of its own", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{manifestPath: {Data: []byte("[package\n")}, libRoot: {}, "src/a.rs": {}}
			assert.Equal(t, unitsOf(t, tree), [][]string{{"src/a.rs"}, {libRoot}},
				"the manifest states no target, so the units are in path order")
			assert.Equal(t, sharedOf(t, tree), [][]string{{manifestPath}, {manifestPath}},
				"and each unit reads it to report it")
		})

		t.Run("makes each file of a manifest that names no package a unit of its own", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{manifestPath: {Data: []byte("[workspace]\n")}, libRoot: {}, "src/a.rs": {}}
			assert.Equal(t, unitsOf(t, tree), [][]string{{"src/a.rs"}, {libRoot}},
				"a virtual manifest states no target")
		})

		t.Run("orders the units of a package by their roots", func(t *testing.T) {
			t.Parallel()

			tree := crateTree(map[string]string{libRoot: "", "src/main.rs": "", "build.rs": ""})
			assert.Equal(t, unitsOf(t, tree), [][]string{{"build.rs"}, {libRoot}, {"src/main.rs"}},
				"the build script, the library and the binary in root order")
		})
	})
}

// partitioned partitions a tree's Rust files through the frontend and
// returns its units.
func partitioned(tb assert.TB, tree fstest.MapFS) [][]plugin.SourceRef {
	tb.Helper()

	var claimed []plugin.SourceRef
	for p := range tree {
		if strings.HasSuffix(p, rust.Extension) {
			claimed = append(claimed, plugin.SourceRef{Path: p})
		}
	}
	slices.SortFunc(claimed, func(a, b plugin.SourceRef) int { return strings.Compare(a.Path, b.Path) })
	units, err := frontend.New(nil).Partition(context.Background(), claimed, treeReader{tree})
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
