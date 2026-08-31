// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package model_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gen/model"
	"go.dokimi.dev/eidos/core/internal/genfile"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

// moduleRoot returns the kernel's module root, which every case
// here generates against.
func moduleRoot(tb assert.TB) string {
	tb.Helper()

	root, err := gosource.ModuleRoot(".")
	assert.NoError(tb, err, "the kernel's module root resolves")
	return root
}

func TestGenerate(t *testing.T) {
	t.Parallel()

	t.Run("Generate", func(t *testing.T) {
		t.Parallel()

		t.Run("renders every owned file", func(t *testing.T) {
			t.Parallel()

			set, err := model.Generate(moduleRoot(t))
			assert.NoError(t, err, "the schema generates")
			want := []string{
				"emit/kinds.gen.go",
				"emit/kinds.gen_test.go",
				"emit/names.gen.go",
				"emit/names.gen_test.go",
				"emit/slots.gen.go",
				"emit/symbols.gen.go",
				"emit/symbols.gen_test.go",
				"emit/walk.gen.go",
				"emit/walk.gen_test.go",
				"match.gen.go",
				"match.gen_test.go",
				"node/kinds.gen.go",
				"node/kinds.gen_test.go",
				"node/symbols.gen.go",
				"node/symbols.gen_test.go",
				"node/walk.gen.go",
				"node/walk.gen_test.go",
				"symbol/kind.gen.go",
				"symbol/kind.gen_test.go",
			}
			assert.Equal(t, slices.Sorted(maps(set)), want,
				"every owned file renders, and nothing else")
		})

		t.Run("carries the schema's documentation into the models", func(t *testing.T) {
			t.Parallel()

			set, err := model.Generate(moduleRoot(t))
			assert.NoError(t, err, "the schema generates")
			kinds := string(set["node/kinds.gen.go"])
			for _, want := range []string{
				"// Struct is a type values can be made of",
				"// This is the node spelling of the kind.",
			} {
				assert.Contains(t, kinds, want,
					"the schema's documentation is carried into the models")
			}
		})

		t.Run("produces the same bytes twice", func(t *testing.T) {
			t.Parallel()

			root := moduleRoot(t)
			first, err := model.Generate(root)
			assert.NoError(t, err, "the first run generates")
			second, err := model.Generate(root)
			assert.NoError(t, err, "and the second")
			for path, want := range first {
				assert.Equal(t, string(second[path]), string(want),
					"two runs produce the same bytes")
			}
		})

		t.Run("reports a module holding no schema", func(t *testing.T) {
			t.Parallel()

			_, err := model.Generate(t.TempDir())
			assert.HasError(t, err, "a module holding no schema is reported")
		})

		t.Run("matches the committed tree", func(t *testing.T) {
			t.Parallel()

			root := moduleRoot(t)
			set, err := model.Generate(root)
			assert.NoError(t, err, "the schema generates")
			assert.NoError(t, genfile.Verify(root, set, model.OwnedDirs),
				"the committed tree matches the schema; run `make generate` and commit")
		})
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
