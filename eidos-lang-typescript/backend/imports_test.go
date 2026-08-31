// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/lang-typescript/backend"
)

// The import block is pinned byte for byte.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("renders sorted side-effect imports", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		set.Add("./store")
		set.Add("node:path")
		assert.Equal(t, backend.Imports(&set),
			"import \"./store\";\nimport \"node:path\";\n\n",
			"one import per path, sorted, a blank line after the block")
	})

	t.Run("renders the names bound under a path", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		set.AddNamed("./store", "Store")
		set.AddNamed("./store", "Row")
		set.Add("./store")
		set.Add("side/effect")
		assert.Equal(t, backend.Imports(&set),
			"import { Row, Store } from \"./store\";\n"+
				"import \"side/effect\";\n\n",
			"named form covers its path, side-effect form the rest")
	})

	t.Run("a file importing nothing renders no block", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		assert.Equal(t, backend.Imports(&set), "", "no paths, no block")
	})
}
