// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/node"
)

// TestDocstring pins the documentation that the docstrings of Python
// state, trimmed as PEP 257 trims them.
func TestDocstring(t *testing.T) {
	t.Parallel()

	t.Run("trimDoc", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want []string
		}{
			{
				name: "trims the indentation that the lines share",
				give: "\"\"\"Summary.\n\n        Detail.\n          More.\n        \"\"\"",
				want: []string{"Summary.", "", "Detail.", "  More."},
			},
			{
				name: "drops the blank lines at both ends",
				give: "\"\"\"\n\n    Summary.\n\n    \"\"\"",
				want: []string{"Summary."},
			},
			{
				name: "expands a tab to the next column of eight",
				give: "\"\"\"Summary.\n\t  Tabbed.\n\t\"\"\"",
				want: []string{"Summary.", "Tabbed."},
			},
			{
				name: "reads a carriage return before a line feed as the line break",
				give: "\"\"\"A.\r\n    B.\r\n\"\"\"",
				want: []string{"A.", "B."},
			},
			{name: "reads a raw docstring", give: "r\"\"\"A \\d.\"\"\"", want: []string{"A \\d."}},
			{name: "reads a concatenation of literals", give: "\"A\" \"B.\"", want: []string{"AB."}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				fn := named[*node.Function](t, declsOf(t, "def f():\n    "+tt.give+"\n"), "f")
				assert.Equal(t, fn.Doc, tt.want, "the lines of the docstring")
			})
		}
	})

	t.Run("docstring", func(t *testing.T) {
		t.Parallel()

		t.Run("reads no f-string as a docstring", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "def f():\n    f\"{x}\"\n"), "f")
			assert.Empty(t, fn.Doc, "an f-string is no docstring")
		})

		t.Run("reads no statement after the first as a docstring", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "def f():\n    pass\n    \"\"\"Late.\"\"\"\n"), "f")
			assert.Empty(t, fn.Doc, "the docstring is the first statement")
		})
	})

	t.Run("attributeDoc", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the docstring after the comments that follow an attribute", func(t *testing.T) {
			t.Parallel()

			v := named[*node.Variable](t, declsOf(t, "count = 1\n# a comment\n\"\"\"The count.\"\"\"\n"), "count")
			assert.Equal(t, v.Doc, []string{"The count."}, "the comment between does not count")
		})
	})
}
