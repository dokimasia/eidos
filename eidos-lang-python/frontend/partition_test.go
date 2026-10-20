// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The modules of the partition cases, and the module of their package
// directory.
const (
	moduleA    = "shop/a.py"
	moduleB    = "shop/b.py"
	stubA      = "shop/a.pyi"
	shopInit   = "shop/__init__.py"
	modelsFile = "lib/shop/models.py"
	libInit    = "lib/shop/__init__.py"
)

// runProbes are the modules of the run of package directories above
// lib/shop: the modules of lib/shop, which is a package directory, and of
// lib, which ends the run.
var runProbes = []string{"lib/shop/__init__.py", "lib/shop/__init__.pyi", "lib/__init__.py", "lib/__init__.pyi"}

// shopProbes are the modules of the run of package directories above shop:
// the modules of shop, which is a package directory, and of the root of
// the workspace, which ends the run.
var shopProbes = []string{"shop/__init__.py", "shop/__init__.pyi", "__init__.py", "__init__.pyi"}

// TestPartition pins the units of a workspace: one module for each unit,
// with the shared inputs that place it.
func TestPartition(t *testing.T) {
	t.Parallel()

	t.Run("Partition", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give fstest.MapFS
			want [][]plugin.SourceRef
		}{
			{
				name: "makes each module a unit of its own",
				give: fstest.MapFS{projectFile: {Data: []byte(rootProject)}, moduleA: {}, moduleB: {}},
				want: [][]plugin.SourceRef{
					{{Path: moduleA, Shared: []string{projectFile}}},
					{{Path: moduleB, Shared: []string{projectFile}}},
				},
			},
			{
				name: "pairs a source and its stub in one unit with the source first",
				give: fstest.MapFS{projectFile: {Data: []byte(rootProject)}, moduleA: {}, stubA: {}},
				want: [][]plugin.SourceRef{
					{{Path: moduleA, Shared: []string{projectFile}}, {Path: stubA, Shared: []string{projectFile}}},
				},
			},
			{
				name: "declares the modules of the run of package directories of a file outside every project",
				give: fstest.MapFS{libInit: {}, modelsFile: {}},
				want: [][]plugin.SourceRef{
					{{Path: libInit, Shared: runProbes}},
					{{Path: modelsFile, Shared: runProbes}},
				},
			},
			{
				name: "declares the run of a file whose project file does not decode",
				give: fstest.MapFS{projectFile: {Data: []byte(badProject)}, shopInit: {}, moduleA: {}},
				want: [][]plugin.SourceRef{
					{{Path: shopInit, Shared: append([]string{projectFile}, shopProbes...)}},
					{{Path: moduleA, Shared: append([]string{projectFile}, shopProbes...)}},
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				units, err := frontend.New(nil).Partition(t.Context(), claimedIn(tt.give), treeReader{tt.give})
				assert.NoError(t, err, "the workspace partitions")
				assert.Equal(t, units, tt.want, "the units in path order with their shared inputs")
			})
		}
	})
}
