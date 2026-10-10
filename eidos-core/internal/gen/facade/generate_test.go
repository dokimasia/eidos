// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade_test

import (
	"cmp"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/internal/gen/facade"
	"go.dokimi.dev/eidos/core/internal/genfile"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

// otherModule is a go.mod naming a module other than the kernel.
const otherModule = "module example.test/other\n\ngo 1.27.0\n"

// The ceilings of a generation, each measured after one generation. A
// ceiling of the mini kernel is the average of 100 calls, with eight
// allocations of headroom.
const (
	// generateMiniAllocs is one generation of the mini kernel's facade:
	// the lowering, each surface's re-exports and spec, and gofmt's run
	// over each file.
	generateMiniAllocs = 15_389 + 8*1
	// generateKernelAllocs is a budget for one generation of the facade
	// of the kernel's 22 curated packages, about 5% above the 507,400
	// allocations of one measured run.
	generateKernelAllocs = 533_000
	// regenerateMiniAllocs is one regeneration of the mini kernel's
	// facade: the module check, the generation, and the write of each
	// file.
	regenerateMiniAllocs = 15_675 + 8*1
)

// The facade is generated from the kernel, so the files it renders,
// the documentation it copies and its bytes across runs are
// contract.
func TestGenerate(t *testing.T) {
	t.Parallel()

	t.Run("Generate", func(t *testing.T) {
		t.Parallel()

		t.Run("renders two files for every curated package", func(t *testing.T) {
			t.Parallel()

			set, err := facade.Generate(repoRoot(t))
			assert.NoError(t, err, "the kernel generates")
			want := make([]string, 0, 2*len(facade.Surfaces))
			for _, s := range facade.Surfaces {
				for _, name := range []string{facade.FileName, facade.TestFileName} {
					want = append(want, filepath.ToSlash(filepath.Join(facade.FacadeDir, s.FacadeRel(), name)))
				}
			}
			assert.Permutation(t, slices.Collect(maps.Keys(set)), want,
				"two files per curated package, and nothing else")
		})

		t.Run("pins every non-generic re-export in the spec", func(t *testing.T) {
			t.Parallel()

			set, err := facade.Generate(repoRoot(t))
			assert.NoError(t, err, "the kernel generates")
			assert.That(t, string(set["eidos-sdk/diag/facade.gen_test.go"])).
				Contains("package diag_test", "the spec is black-box").
				Contains("reflect.TypeFor[diag.Prefix](), reflect.TypeFor[core.Prefix]()",
					"an alias is pinned to the kernel's type").
				Contains("assert.Equal(t, diag.KernelPrefix, core.KernelPrefix,",
					"a constant is pinned to the kernel's value").
				Contains("reflect.TypeOf(diag.NewRegistry), reflect.TypeOf(core.NewRegistry)",
					"a wrapper is pinned to the kernel function's signature")
		})

		t.Run("copies the kernel's documentation", func(t *testing.T) {
			t.Parallel()

			set, err := facade.Generate(repoRoot(t))
			assert.NoError(t, err, "the kernel generates")
			root := string(set["eidos-sdk/facade.gen.go"])
			assert.Contains(t, root, "// NewPlugin starts a plugin declaration.",
				"a wrapper keeps its kernel docblock")
			kit := string(set["eidos-sdk/backendtest/facade.gen.go"])
			assert.Contains(t, kit,
				"// BenchPackages is the number of packages in the scaled fixture.",
				"a re-declared constant keeps its kernel docblock")
		})

		t.Run("returns the same bytes on every run", func(t *testing.T) {
			t.Parallel()

			assert.Deterministic(
				t,
				facade.Generate,
				repoRoot(t),
				"every run produces the same files with the same bytes",
			)
		})

		t.Run("returns an error for a tree without a kernel", func(t *testing.T) {
			t.Parallel()

			_, err := facade.Generate(t.TempDir())
			assert.HasError(t, err, "a tree without a kernel generates nothing")
		})

		t.Run("returns the bytes of the committed tree", func(t *testing.T) {
			t.Parallel()

			root := repoRoot(t)
			set, err := facade.Generate(root)
			assert.NoError(t, err, "the kernel generates")
			assert.NoError(t, genfile.Verify(root, set, facade.OwnedDirs),
				"the committed facade matches the kernel; run `make generate` and commit")
		})
	})

	t.Run("Regenerate", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the facade beside the kernel it ran inside", func(t *testing.T) {
			t.Parallel()

			root := mini(t)
			assert.NoError(t, facade.Regenerate(filepath.Join(root, facade.KernelDir, "emit")),
				"a directory anywhere inside the kernel regenerates")

			set, err := facade.Generate(root)
			assert.NoError(t, err, "the same tree generates in memory")
			assert.NoError(t, genfile.Verify(root, set, facade.OwnedDirs),
				"and the bytes on disk are what the generator produces, "+
					"in the facade module beside the kernel")
		})

		t.Run("writes nothing for a surface it cannot re-export", func(t *testing.T) {
			t.Parallel()

			root := mini(t)
			kernel := filepath.Join(root, facade.KernelDir)
			files.Write(t, kernel, files.Tree{poisonRel: files.Text(
				"package emit\n\n// Fetch returns a row.\nfunc Fetch() row { return row{} }\n\ntype row struct{}\n")})
			var err error
			files.Unchanged(t, os.DirFS(root), func() { err = facade.Regenerate(kernel) },
				"no file is written: a refused surface leaves no half-generated facade")
			assert.HasError(t, err, "a surface that cannot re-export refuses the run")
			assert.Contains(t, err.Error(), "unexported row", "naming what defeated it")
		})

		t.Run("returns an error for a module that is not the kernel", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{"go.mod": files.Text(otherModule)})
			var err error
			files.Unchanged(t, os.DirFS(root), func() { err = facade.Regenerate(root) },
				"nothing is written: a wrong tree is refused before it is generated into")
			assert.HasError(t, err, "a module that is not the kernel is refused")
			assert.That(t, err.Error()).
				Contains("example.test/other", "the refusal names the module it found").
				Contains(facade.KernelModule, "and the kernel it wanted")
		})
	})
}

// A generation and a regeneration of the mini kernel allocate within
// their ceilings in the ordinary run, which runs no benchmark. The
// kernel's own generation takes too long to repeat 101 times, so only
// [BenchmarkGenerate] checks its ceiling. Each count keeps the first
// error of its calls, which cmp.Or returns without allocating. The check
// runs alone, because the count includes every goroutine's allocations.
func TestGenerateAllocs(t *testing.T) {
	root := mini(t)
	var (
		set genfile.Set
		err error
	)
	assert.MaxAllocs(t, func() {
		var gerr error
		set, gerr = facade.Generate(root)
		err = cmp.Or(err, gerr)
	}, generateMiniAllocs, "Generate allocates the lowering, the rendering and the formatting")
	assert.NoError(t, err, "the mini kernel generates")
	assert.NotEmpty(t, set, "every file")
	dir := filepath.Join(root, facade.KernelDir)
	assert.MaxAllocs(t, func() { err = cmp.Or(err, facade.Regenerate(dir)) }, regenerateMiniAllocs,
		"Regenerate allocates the generation and the writes")
	assert.NoError(t, err, "the mini kernel regenerates")
}

// BenchmarkGenerate measures a generation of the mini kernel's facade
// and of the kernel's own, and a regeneration of the mini kernel's, each
// after one run, which the go:generate wrapper and the mirror guard
// make.
func BenchmarkGenerate(b *testing.B) {
	generations := []struct {
		name   string
		root   string
		allocs uint64
	}{
		{name: "the mini kernel", root: mini(b), allocs: generateMiniAllocs},
		{name: "the kernel", root: repoRoot(b), allocs: generateKernelAllocs},
	}
	b.Run("Generate", func(b *testing.B) {
		for _, tt := range generations {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					set genfile.Set
					err error
				)
				for c.Loop() {
					set, err = facade.Generate(tt.root)
				}
				assert.NoError(b, err, "the facade generates")
				assert.NotEmpty(b, set, "every file")
			})
		}
	})

	b.Run("Regenerate", func(b *testing.B) {
		b.Run("the mini kernel", func(b *testing.B) {
			dir := filepath.Join(mini(b), facade.KernelDir)
			c := bench.Start(b).Warmup(1).MaxAllocs(regenerateMiniAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				err = facade.Regenerate(dir)
			}
			assert.NoError(b, err, "the facade regenerates")
		})
	})
}

// repoRoot returns the repository root, one directory above the
// kernel module every case here generates from.
func repoRoot(tb assert.TB) string {
	tb.Helper()

	root, err := gosource.ModuleRoot(".")
	assert.NoError(tb, err, "the kernel's module root resolves")
	return filepath.Dir(root)
}
