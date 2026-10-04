// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade_test

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/gen/facade"
	"go.dokimi.dev/eidos/core/internal/genfile"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

// otherModule is a go.mod naming a module other than the kernel.
const otherModule = "module example.test/other\n\ngo 1.27.0\n"

// The ceilings of a generation, each measured over 24 fresh processes
// after one generation, and allowing eight standard deviations above
// the mean.
const (
	// generateMiniAllocs is one generation of the mini kernel's facade:
	// the lowering, each surface's re-exports and spec, and gofmt's run
	// over each file.
	generateMiniAllocs = 14_730 + 8*1
	// generateKernelAllocs is one generation of the kernel's facade,
	// 484,978 on average with a standard deviation of 13.
	generateKernelAllocs = 484_978 + 8*13
	// regenerateMiniAllocs is one regeneration of the mini kernel's
	// facade: the module check, the generation, and the write of each
	// file.
	regenerateMiniAllocs = 15_004 + 8*1
)

// repoRoot returns the repository root, one directory above the
// kernel module every case here generates from.
func repoRoot(tb assert.TB) string {
	tb.Helper()

	root, err := gosource.ModuleRoot(".")
	assert.NoError(tb, err, "the kernel's module root resolves")
	return filepath.Dir(root)
}

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
			slices.Sort(want)
			assert.Equal(t, slices.Sorted(maps.Keys(set)), want,
				"two files per curated package, and nothing else")
		})

		t.Run("pins every non-generic re-export in the spec", func(t *testing.T) {
			t.Parallel()

			set, err := facade.Generate(repoRoot(t))
			assert.NoError(t, err, "the kernel generates")
			spec := string(set["eidos-sdk/diag/facade.gen_test.go"])
			assert.Contains(t, spec, "package diag_test", "the spec is black-box")
			assert.Contains(t, spec,
				"reflect.TypeFor[diag.Prefix](), reflect.TypeFor[core.Prefix]()",
				"an alias is pinned to the kernel's type")
			assert.Contains(t, spec, "assert.Equal(t, diag.KernelPrefix, core.KernelPrefix,",
				"a constant is pinned to the kernel's value")
			assert.Contains(t, spec,
				"reflect.TypeOf(diag.NewRegistry), reflect.TypeOf(core.NewRegistry)",
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

		t.Run("returns the same bytes on a second run", func(t *testing.T) {
			t.Parallel()

			root := repoRoot(t)
			first, err := facade.Generate(root)
			assert.NoError(t, err, "the first run generates")
			second, err := facade.Generate(root)
			assert.NoError(t, err, "and the second")
			for path, want := range first {
				assert.Equal(t, string(second[path]), string(want),
					"two runs produce the same bytes")
			}
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
			poison(t, root, poisonRel,
				"package emit\n\n// Fetch returns a row.\nfunc Fetch() row { return row{} }\n\ntype row struct{}\n")
			err := facade.Regenerate(filepath.Join(root, facade.KernelDir))
			assert.HasError(t, err, "a surface that cannot re-export refuses the run")
			assert.Contains(t, err.Error(), "unexported row", "naming what defeated it")
			assert.Empty(t, dirEntries(t, root, facade.FacadeDir),
				"and no file was written: a refused surface leaves no half-generated facade")
		})

		t.Run("returns an error for a module that is not the kernel", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			assert.NoError(t,
				os.WriteFile(filepath.Join(root, "go.mod"), []byte(otherModule), 0o600),
				"the other module's go.mod writes")
			err := facade.Regenerate(root)
			assert.HasError(t, err, "a module that is not the kernel is refused")
			assert.Contains(t, err.Error(), "example.test/other",
				"the refusal names the module it found")
			assert.Contains(t, err.Error(), facade.KernelModule,
				"and the kernel it wanted")
			assert.Empty(t, dirEntries(t, root, facade.FacadeDir),
				"and nothing was written: a wrong tree is refused before it is generated into")
		})
	})
}

// A generation and a regeneration of the mini kernel allocate within
// their ceilings in the ordinary run, which runs no benchmark. The
// kernel's own generation takes too long to repeat 101 times, so only
// [BenchmarkGenerate] checks its ceiling. The check runs alone, because
// AllocsPerRun counts every goroutine's allocations and refuses to run
// beside parallel tests.
func TestGenerateAllocs(t *testing.T) {
	root := mini(t)
	assert.MaxAllocs(t, func() {
		if _, err := facade.Generate(root); err != nil {
			t.Fatalf("Generate: unexpected error: %v", err)
		}
	}, generateMiniAllocs, "Generate allocates the lowering, the rendering and the formatting")
	dir := filepath.Join(root, facade.KernelDir)
	assert.MaxAllocs(t, func() {
		if err := facade.Regenerate(dir); err != nil {
			t.Fatalf("Regenerate: unexpected error: %v", err)
		}
	}, regenerateMiniAllocs, "Regenerate allocates the generation and the writes")
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
		{name: "Generate/the mini kernel", root: mini(b), allocs: generateMiniAllocs},
		{name: "Generate/the kernel", root: repoRoot(b), allocs: generateKernelAllocs},
	}
	for _, tt := range generations {
		b.Run(tt.name, func(b *testing.B) {
			_, err := facade.Generate(tt.root)
			assert.NoError(b, err, "the facade generates before the measurement")
			c := bench.Start(b).MaxAllocs(tt.allocs)
			defer c.End()
			var set genfile.Set
			for c.Loop() {
				set, err = facade.Generate(tt.root)
			}
			assert.NoError(b, err, "the facade generates")
			assert.NotEmpty(b, set, "every file")
		})
	}

	b.Run("Regenerate/the mini kernel", func(b *testing.B) {
		dir := filepath.Join(mini(b), facade.KernelDir)
		err := facade.Regenerate(dir)
		assert.NoError(b, err, "the facade regenerates before the measurement")
		c := bench.Start(b).MaxAllocs(regenerateMiniAllocs)
		defer c.End()
		for c.Loop() {
			err = facade.Regenerate(dir)
		}
		assert.NoError(b, err, "the facade regenerates")
	})
}

// dirEntries lists the entries of a directory under root, and none
// for a directory that does not exist.
func dirEntries(t *testing.T, root, rel string) []os.DirEntry {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(root, rel))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	assert.NoError(t, err, "the directory reads")
	return entries
}
