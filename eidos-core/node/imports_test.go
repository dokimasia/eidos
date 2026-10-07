// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package node_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/node"
)

// importing is the package the allocation check and the benchmark
// range: three files with three import statements among them.
var importing = &node.Package{Files: []*node.File{
	{Path: "a.go", Imports: []*node.Import{{Path: "fmt"}, {Path: "io"}}},
	{Path: "b.go"},
	{Path: "c.go", Imports: []*node.Import{{Path: "fmt"}}},
}}

// The package-level import list is derived, never stored, so the
// derivation is the one place its order is fixed.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("Imports", func(t *testing.T) {
		t.Parallel()

		t.Run("unions the files' imports in file order", func(t *testing.T) {
			t.Parallel()

			var paths []string
			for imp := range node.Imports(importing) {
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

			assert.Empty(t, slices.Collect(node.Imports(nil)), "a nil package imports nothing")
		})
	})
}

// A range over the imports allocates nothing in the ordinary run, which
// runs no benchmark.
func TestImportsZeroAlloc(t *testing.T) {
	var n int
	assert.MaxAllocs(t, func() {
		n = 0
		for range node.Imports(importing) {
			n++
		}
	}, 0, "a range over Imports allocates nothing")
	assert.Equal(t, n, 3, "Imports yields every statement")
}

// BenchmarkImports measures a range over a package's import statements.
func BenchmarkImports(b *testing.B) {
	b.Run("Imports", func(b *testing.B) {
		c := bench.Start(b).Warmup(1).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got = 0
			for range node.Imports(importing) {
				got++
			}
		}
		assert.Equal(b, got, 3, "the range yields every statement")
	})
}
