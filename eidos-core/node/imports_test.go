// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package node_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/node"
)

// The package-level import list is derived, never stored, so the
// derivation is the one place its order is fixed.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("Imports", func(t *testing.T) {
		t.Parallel()

		t.Run("unions the files' imports in file order", func(t *testing.T) {
			t.Parallel()

			pkg := &node.Package{Files: []*node.File{
				{Path: "a.go", Imports: []*node.Import{{Path: "fmt"}, {Path: "io"}}},
				{Path: "b.go"},
				{Path: "c.go", Imports: []*node.Import{{Path: "fmt"}}},
			}}
			var paths []string
			for imp := range node.Imports(pkg) {
				paths = append(paths, imp.Path)
			}
			assert.Equal(t, paths, []string{"fmt", "io", "fmt"},
				"every statement arrives, a path repeated across files repeated")
		})

		t.Run("stops where the caller stops", func(t *testing.T) {
			t.Parallel()

			pkg := &node.Package{Files: []*node.File{
				{Path: "a.go", Imports: []*node.Import{{Path: "fmt"}, {Path: "io"}, {Path: "os"}}},
			}}
			first := slices.Collect(func(yield func(*node.Import) bool) {
				for imp := range node.Imports(pkg) {
					if !yield(imp) {
						return
					}
					break
				}
			})
			assert.Length(t, first, 1, "the sequence honours an early return")
		})

		t.Run("yields nothing for a nil package", func(t *testing.T) {
			t.Parallel()

			for range node.Imports(nil) {
				t.Error("a nil package imports nothing")
			}
		})
	})
}
