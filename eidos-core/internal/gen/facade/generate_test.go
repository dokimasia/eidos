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

	"go.dokimi.dev/eidos/core/internal/gen/facade"
	"go.dokimi.dev/eidos/core/internal/genfile"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

// repoRoot returns the repository root, one directory above the
// kernel module every case here generates from.
func repoRoot(tb assert.TB) string {
	tb.Helper()

	root, err := gosource.ModuleRoot(".")
	assert.NoError(tb, err, "the kernel's module root resolves")
	return filepath.Dir(root)
}

func TestGenerate(t *testing.T) {
	t.Parallel()

	t.Run("Generate", func(t *testing.T) {
		t.Parallel()

		t.Run("renders every owned file", func(t *testing.T) {
			t.Parallel()

			set, err := facade.Generate(repoRoot(t))
			assert.NoError(t, err, "the kernel generates")
			want := make([]string, 0, len(facade.Surfaces))
			for _, s := range facade.Surfaces {
				want = append(want, filepath.ToSlash(filepath.Join(
					facade.FacadeDir, s.FacadeRel(), facade.FileName,
				)))
			}
			slices.Sort(want)
			assert.Equal(t, slices.Sorted(maps.Keys(set)), want,
				"one facade file per curated package, and nothing else")
		})

		t.Run("copies the kernel's documentation", func(t *testing.T) {
			t.Parallel()

			set, err := facade.Generate(repoRoot(t))
			assert.NoError(t, err, "the kernel generates")
			root := string(set["eidos-sdk/facade.gen.go"])
			assert.Contains(t, root, "// NewPlugin starts a plugin declaration.",
				"a wrapper carries its kernel docblock")
			kit := string(set["eidos-sdk/backendtest/facade.gen.go"])
			assert.Contains(t, kit,
				"// BenchPackages is how many packages the scaled fixture holds.",
				"a re-declared constant carries its kernel docblock")
		})

		t.Run("produces the same bytes twice", func(t *testing.T) {
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

		t.Run("reports a tree without a kernel", func(t *testing.T) {
			t.Parallel()

			_, err := facade.Generate(t.TempDir())
			assert.HasError(t, err, "a tree holding no kernel is reported")
		})

		t.Run("matches the committed tree", func(t *testing.T) {
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
				"and what landed on disk is what the generator produces, "+
					"in the facade module beside the kernel")
		})

		t.Run("writes nothing when one surface refuses", func(t *testing.T) {
			t.Parallel()

			root := mini(t)
			poison(t, root, poisonRel,
				"package emit\n\n// Fetch returns a row.\nfunc Fetch() row { return row{} }\n\ntype row struct{}\n")
			err := facade.Regenerate(filepath.Join(root, facade.KernelDir))
			assert.HasError(t, err, "a surface that cannot re-export refuses the run")
			assert.Contains(t, err.Error(), "unexported row", "naming what defeated it")
			assert.Empty(t, dirEntries(t, root, facade.FacadeDir),
				"and no file landed: a refused surface leaves no half-generated facade")
		})

		t.Run("refuses a module that is not the kernel", func(t *testing.T) {
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

// otherModule is a go.mod naming a module other than the kernel.
const otherModule = "module example.test/other\n\ngo 1.27.0\n"

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
