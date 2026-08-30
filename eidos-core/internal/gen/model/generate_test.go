// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package model_test

import (
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/eidos/core/internal/gen/model"
	"go.dokimi.dev/eidos/core/internal/genfile"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

// moduleRoot answers the kernel's module root, which every case
// here generates against.
func moduleRoot(t *testing.T) string {
	t.Helper()

	root, err := gosource.ModuleRoot(".")
	if err != nil {
		t.Fatalf("ModuleRoot: %v", err)
	}
	return root
}

func TestGenerate(t *testing.T) {
	t.Parallel()

	t.Run("Generate", func(t *testing.T) {
		t.Parallel()

		t.Run("renders every owned file", func(t *testing.T) {
			t.Parallel()

			set, err := model.Generate(moduleRoot(t))
			if err != nil {
				t.Fatalf("Generate: unexpected error: %v", err)
			}
			want := []string{
				"emit/kinds.gen.go",
				"emit/kinds.gen_test.go",
				"emit/slots.gen.go",
				"emit/symbols.gen.go",
				"emit/symbols.gen_test.go",
				"emit/walk.gen.go",
				"emit/walk.gen_test.go",
				"node/kinds.gen.go",
				"node/kinds.gen_test.go",
				"node/symbols.gen.go",
				"node/symbols.gen_test.go",
				"node/walk.gen.go",
				"node/walk.gen_test.go",
				"symbol/kind.gen.go",
				"symbol/kind.gen_test.go",
			}
			got := slices.Sorted(maps(set))
			if !slices.Equal(got, want) {
				t.Fatalf("generated %v, want %v", got, want)
			}
		})

		t.Run("carries the schema's documentation into the models", func(t *testing.T) {
			t.Parallel()

			set, err := model.Generate(moduleRoot(t))
			if err != nil {
				t.Fatalf("Generate: unexpected error: %v", err)
			}
			kinds := string(set["node/kinds.gen.go"])
			for _, want := range []string{
				"// Struct is a type values can be made of",
				"// This is the node spelling of the kind.",
			} {
				if !strings.Contains(kinds, want) {
					t.Fatalf("node/kinds.gen.go is missing %q", want)
				}
			}
		})

		t.Run("produces the same bytes twice", func(t *testing.T) {
			t.Parallel()

			root := moduleRoot(t)
			first, err := model.Generate(root)
			if err != nil {
				t.Fatalf("Generate: unexpected error: %v", err)
			}
			second, err := model.Generate(root)
			if err != nil {
				t.Fatalf("Generate: unexpected error: %v", err)
			}
			for path, want := range first {
				if string(second[path]) != string(want) {
					t.Fatalf("%s differs between two runs", path)
				}
			}
		})

		t.Run("reports a module holding no schema", func(t *testing.T) {
			t.Parallel()

			if _, err := model.Generate(t.TempDir()); err == nil {
				t.Fatal("Generate: error = nil, want non-nil")
			}
		})

		t.Run("matches the committed tree", func(t *testing.T) {
			t.Parallel()

			root := moduleRoot(t)
			set, err := model.Generate(root)
			if err != nil {
				t.Fatalf("Generate: unexpected error: %v", err)
			}
			if err := genfile.Verify(root, set, model.OwnedDirs); err != nil {
				t.Fatalf("the models are out of date with the schema.\n"+
					"Run `make generate` and commit the result.\n%v", err)
			}
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
