// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade_test

import (
	"cmp"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

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
	// packages, 311,002 on average with a standard deviation of 4.8,
	// rounded up to 5.
	lowerKernelAllocs = 311_002 + 8*5
)

// A lowering establishes what the renderer assumes. The curated
// packages parse in curated order, and Lower returns an error for a
// construct that defeats a re-export at the syntax level before
// anything renders.
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
				expect.Equal(t, ps.Rel, facade.Surfaces[i].Rel, "in curated order")
				expect.NotEmpty(t, ps.Files, "each surface parsed its files")
			}
			assert.Equal(t, surfaces[0].KernelName, "eidos",
				"the root surface is the kernel's authoring package")
		})

		t.Run("returns an error for a dot import", func(t *testing.T) {
			t.Parallel()

			kernel := filepath.Join(mini(t), facade.KernelDir)
			files.Write(t, kernel, files.Tree{poisonRel: files.Text("package emit\n\nimport . \"strings\"\n")})
			_, err := facade.Lower(kernel)
			assert.HasError(t, err, "a dot import erases the qualifier")
			assert.That(t, err.Error()).
				Contains("dot import", "the error names the cause").
				Contains("poison.go:", "at the kernel position")
		})

		t.Run("returns an error for a package split", func(t *testing.T) {
			t.Parallel()

			kernel := filepath.Join(mini(t), facade.KernelDir)
			files.Write(t, kernel, files.Tree{poisonRel: files.Text("package emitx\n")})
			_, err := facade.Lower(kernel)
			assert.HasError(t, err, "two package names in one directory")
			assert.That(t, err.Error()).
				Contains("declared beside", "the refusal names the split").
				Contains("emitx", "and the name that arrived beside the package")
		})

		t.Run("returns an error for a curated package it cannot parse", func(t *testing.T) {
			t.Parallel()

			kernel := filepath.Join(mini(t), facade.KernelDir)
			files.Write(t, kernel, files.Tree{poisonRel: files.Text("package emit\n\nfunc Broken(\n")})
			_, err := facade.Lower(kernel)
			assert.HasError(t, err, "a curated package that does not parse is reported")
			assert.That(t, err.Error()).
				Contains("go.dokimi.dev/eidos/core/emit", "naming the curated package it was reading").
				Contains("poison.go", "and the file inside it")
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
// [BenchmarkIR] checks its ceiling. The count keeps the first error of
// its calls, which cmp.Or returns without allocating. The check runs
// alone, because the count includes every goroutine's allocations.
func TestIRAllocs(t *testing.T) {
	root := filepath.Join(mini(t), facade.KernelDir)
	var err error
	assert.MaxAllocs(t, func() {
		_, lerr := facade.Lower(root)
		err = cmp.Or(err, lerr)
	}, lowerMiniAllocs, "Lower allocates the parse of each curated package")
	assert.NoError(t, err, "the mini kernel lowers")
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
				c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					surfaces []*facade.PackageSurface
					err      error
				)
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
