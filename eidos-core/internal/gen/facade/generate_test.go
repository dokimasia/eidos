// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade_test

import (
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
		assert.Equal(t, slices.Sorted(maps(set)), want,
			"one facade file per curated package, and nothing else")
	})

	t.Run("carries the kernel's documentation", func(t *testing.T) {
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

	t.Run("reports a tree holding no kernel", func(t *testing.T) {
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
}

// maps yields a set's paths, so a case can sort them.
func maps(set genfile.Set) func(func(string) bool) {
	return func(yield func(string) bool) {
		for path := range set {
			if !yield(path) {
				return
			}
		}
	}
}
