// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/gen/facade"
)

// The fixture surfaces: the kernel's root package, and a nested one
// whose facade package lies flat under the facade.
var (
	rootSurface   = facade.Surface{Rel: "", Name: "sdk"}
	nestedSurface = facade.Surface{Rel: "backend/render", Name: "render"}
)

// The ceilings of a lowering, each measured over 24 fresh processes
// after one lowering, and allowing eight standard deviations above the
// mean.
const (
	// lowerMiniAllocs is one lowering of the mini kernel: the go.mod
	// check, and the parse of each curated package's files.
	lowerMiniAllocs = 1_621 + 8*1
	// lowerKernelAllocs is one lowering of the kernel's curated
	// packages, 307,381 on average with a standard deviation of 5.
	lowerKernelAllocs = 307_381 + 8*5
)

// Lowering owns what the renderer may assume: the curated
// packages parse in order, and what would defeat syntax-level
// re-export is refused before anything renders.
func TestIR(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the kernel's surfaces in curated order", func(t *testing.T) {
			t.Parallel()

			surfaces, err := facade.Lower(filepath.Join(repoRoot(t), facade.KernelDir))
			assert.NoError(t, err, "the kernel lowers")
			assert.Length(t, surfaces, len(facade.Surfaces), "one surface per curated entry")
			for i, ps := range surfaces {
				assert.Equal(t, ps.Rel, facade.Surfaces[i].Rel, "in curated order")
				assert.True(t, len(ps.Files) > 0, "each surface parsed its files")
			}
			assert.Equal(t, surfaces[0].KernelName, "eidos",
				"the root surface is the kernel's authoring package")
		})

		t.Run("returns an error for a dot import", func(t *testing.T) {
			t.Parallel()

			root := mini(t)
			poison(t, root, poisonRel, "package emit\n\nimport . \"strings\"\n")
			_, err := facade.Lower(filepath.Join(root, facade.KernelDir))
			assert.HasError(t, err, "a dot import erases the qualifier")
			assert.Contains(t, err.Error(), "dot import", "the refusal says why")
			assert.Contains(t, err.Error(), "poison.go:", "at the kernel position")
		})

		t.Run("returns an error for a package split", func(t *testing.T) {
			t.Parallel()

			root := mini(t)
			poison(t, root, poisonRel, "package emitx\n")
			_, err := facade.Lower(filepath.Join(root, facade.KernelDir))
			assert.HasError(t, err, "two package names in one directory")
			assert.Contains(t, err.Error(), "declared beside", "the refusal names the split")
			assert.Contains(t, err.Error(), "emitx", "and the name that arrived beside the package")
		})

		t.Run("returns an error for a curated package it cannot parse", func(t *testing.T) {
			t.Parallel()

			root := mini(t)
			poison(t, root, poisonRel, "package emit\n\nfunc Broken(\n")
			_, err := facade.Lower(filepath.Join(root, facade.KernelDir))
			assert.HasError(t, err, "a curated package that does not parse is reported")
			assert.Contains(t, err.Error(), "go.dokimi.dev/eidos/core/emit",
				"naming the curated package it was reading")
			assert.Contains(t, err.Error(), "poison.go", "and the file inside it")
		})

		t.Run("returns an error for a root of another module", func(t *testing.T) {
			t.Parallel()

			_, err := facade.Lower(filepath.Join(repoRoot(t), "eidos-lang"))
			assert.HasError(t, err, "a non-kernel module is not generated from")
			assert.Contains(t, err.Error(), "not the kernel", "the refusal names the mismatch")
		})

		t.Run("returns an error for a tree without a module", func(t *testing.T) {
			t.Parallel()

			_, err := facade.Lower(t.TempDir())
			assert.HasError(t, err, "a bare tree is reported")
		})
	})

	t.Run("KernelImportPath", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the kernel module for the root surface", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rootSurface.KernelImportPath(), facade.KernelModule,
				"the root surface is the kernel's authoring package")
		})

		t.Run("returns the kernel module joined to a nested surface's path", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, nestedSurface.KernelImportPath(), facade.KernelModule+"/backend/render",
				"a nested surface keeps the kernel's directory")
		})
	})

	t.Run("FacadeRel", func(t *testing.T) {
		t.Parallel()

		t.Run("returns empty for the root surface", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rootSurface.FacadeRel(), "", "the root surface is the facade module's root")
		})

		t.Run("returns the package name for a nested surface", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, nestedSurface.FacadeRel(), "render", "the facade lays its packages flat")
		})
	})

	t.Run("FacadeImportPath", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the facade module for the root surface", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rootSurface.FacadeImportPath(), facade.FacadeModule,
				"the root surface is the facade module itself, not a package under it")
		})

		t.Run("returns a nested kernel package's path flat under the facade", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, nestedSurface.FacadeImportPath(), facade.FacadeModule+"/render",
				"the facade path is the package name alone, so a kernel regroup "+
					"moves no facade import path")
		})
	})
}

// A lowering of the mini kernel and the surfaces' paths allocate within
// their ceilings in the ordinary run, which runs no benchmark. The
// kernel's own lowering takes too long to repeat 101 times, so only
// [BenchmarkIR] checks its ceiling. The check runs alone, because
// AllocsPerRun counts every goroutine's allocations and refuses to run
// beside parallel tests.
func TestIRAllocs(t *testing.T) {
	root := filepath.Join(mini(t), facade.KernelDir)
	assert.MaxAllocs(t, func() {
		if _, err := facade.Lower(root); err != nil {
			t.Fatalf("Lower: unexpected error: %v", err)
		}
	}, lowerMiniAllocs, "Lower allocates the parse of each curated package")
	var got string
	assert.MaxAllocs(t, func() { got = nestedSurface.KernelImportPath() }, 1,
		"KernelImportPath allocates a nested surface's path")
	assert.MaxAllocs(t, func() { got = nestedSurface.FacadeRel() }, 0, "FacadeRel allocates nothing")
	assert.MaxAllocs(t, func() { got = nestedSurface.FacadeImportPath() }, 1,
		"FacadeImportPath allocates a nested surface's path")
	assert.Equal(t, got, facade.FacadeModule+"/render", "FacadeImportPath returns the flat path")
}

// BenchmarkIR measures a lowering of the mini kernel and of the
// kernel's curated packages, each after one lowering, and a surface's
// paths.
func BenchmarkIR(b *testing.B) {
	lowerings := []struct {
		name   string
		root   string
		allocs uint64
	}{
		{name: "the mini kernel", root: filepath.Join(mini(b), facade.KernelDir), allocs: lowerMiniAllocs},
		{
			name: "the kernel's curated packages",
			root: filepath.Join(repoRoot(b), facade.KernelDir), allocs: lowerKernelAllocs,
		},
	}
	b.Run("Lower", func(b *testing.B) {
		for _, tt := range lowerings {
			b.Run(tt.name, func(b *testing.B) {
				_, err := facade.Lower(tt.root)
				assert.NoError(b, err, "the kernel lowers before the measurement")
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var surfaces []*facade.PackageSurface
				for c.Loop() {
					surfaces, err = facade.Lower(tt.root)
				}
				assert.NoError(b, err, "the kernel lowers")
				assert.NotEmpty(b, surfaces, "into its curated surfaces")
			})
		}
	})

	b.Run("Surface.KernelImportPath", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got string
		for c.Loop() {
			got = nestedSurface.KernelImportPath()
		}
		assert.Equal(b, got, facade.KernelModule+"/backend/render", "the kernel's path")
	})

	b.Run("Surface.FacadeRel", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = nestedSurface.FacadeRel()
		}
		assert.Equal(b, got, "render", "the package name")
	})

	b.Run("Surface.FacadeImportPath", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got string
		for c.Loop() {
			got = nestedSurface.FacadeImportPath()
		}
		assert.Equal(b, got, facade.FacadeModule+"/render", "the flat path")
	})
}
