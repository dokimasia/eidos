// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/treesitter/python"
)

// The sources the grammar parses, and the name it loads under. The
// module source has a lazy import, which Python 3.15 adds, and a
// class with a type parameter that has a default.
const (
	moduleFile   = "src/shop/models.py"
	moduleSource = "\"\"\"The models of the shop.\"\"\"\n\nlazy import json\n\n\n" +
		"class Page[T = int]:\n    items: list[T]\n\n    def first(self, /, *, strict: bool = False) -> T: ...\n"
	stubFile   = "src/shop/models.pyi"
	stubSource = "def total(*values: int) -> int: ...\n"
	pythonName = "python"
)

// The package loads one grammar, so the grammar is pinned as the one
// that parses Python.
func TestPython(t *testing.T) {
	t.Parallel()

	t.Run("Grammar", func(t *testing.T) {
		t.Parallel()

		sources := []struct {
			name string
			file string
			give string
		}{
			{name: "parses a module of Python 3.15 without an error", file: moduleFile, give: moduleSource},
			{name: "parses a stub file without an error", file: stubFile, give: stubSource},
		}
		for _, tt := range sources {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tree, err := python.Grammar.Parse(t.Context(), tt.file, []byte(tt.give))
				assert.NoError(t, err, "the source parses")
				defer tree.Close()
				assert.Empty(t, slices.Collect(tree.Errors()), "the source fits the grammar")
			})
		}

		t.Run("loads under the language's name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, python.Grammar.Name(), pythonName, "the name a frontend reports")
		})
	})
}
