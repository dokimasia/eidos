// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/render"
)

// The import block is pinned byte for byte: it is the one part of
// the file the formatter reorders rather than reformats, so the
// renderer has to hand it over already in gofmt's shape.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("groups the standard library apart and spells bindings", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		set.Add("context")
		set.AddNamed("database/sql", "_")
		set.AddNamed("go.dokimi.dev/eidos/sdk", "eidos")
		set.Add("go.dokimi.dev/assert")
		assert.Equal(t, backend.Imports(&set),
			"import (\n"+
				"\t\"context\"\n"+
				"\t_ \"database/sql\"\n"+
				"\n"+
				"\t\"go.dokimi.dev/assert\"\n"+
				"\teidos \"go.dokimi.dev/eidos/sdk\"\n"+
				")\n",
			"standard library first, a blank line, then module paths, "+
				"aliases and blank imports spelt before their path")
	})

	t.Run("renders one sorted parenthesised block", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		set.Add("svc/store")
		set.Add("context")
		set.Add("svc/store")
		assert.Equal(t, backend.Imports(&set),
			"import (\n\t\"context\"\n\t\"svc/store\"\n)\n",
			"sorted, deduplicated, quoted, tab-indented")
	})

	t.Run("a file importing nothing renders no block", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		assert.Equal(t, backend.Imports(&set), "",
			"an empty import block is not what gofmt leaves either")
	})
}
