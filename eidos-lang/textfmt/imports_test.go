// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/render"
)

// The import statements are spliced into every Java and Rust file, so
// the block is pinned byte for byte.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("ImportLines", func(t *testing.T) {
		t.Parallel()

		t.Run("writes nothing for an empty set", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.ImportLines(&render.ImportSet{}, "use", "::"), "",
				"no entries, no block")
		})

		t.Run("writes one statement per entry in the set's order", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.AddNamed("java/util", "List")
			set.Add("example/rows")
			set.AddNamed("example/rows", "Row")
			assert.Equal(t, textfmt.ImportLines(&set, "import", "."),
				"import example.rows;\nimport example.rows.Row;\nimport java.util.List;\n\n",
				"the path's slashes as the separator, the bound name behind it, a blank line after")
		})

		t.Run("writes a statement two entries spell alike once", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.AddNamed("crate/rows", "Row")
			set.Add("crate/rows/Row")
			assert.Equal(t, textfmt.ImportLines(&set, "use", "::"),
				"use crate::rows::Row;\n\n",
				"a bare path and a named entry spelling one statement render it once")
		})
	})
}
