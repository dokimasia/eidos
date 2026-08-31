// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package typescript_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/lang-typescript"
)

// The import block is pinned byte for byte, and its limit is
// stated: the set carries paths and no names, so only the
// side-effect form can be written from it today.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("renders sorted side-effect imports", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		set.Add("./store")
		set.Add("node:path")
		assert.Equal(t, typescript.Imports(&set),
			"import \"./store\";\nimport \"node:path\";\n\n",
			"one import per path, sorted, a blank line after the block")
	})

	t.Run("a file importing nothing renders no block", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		assert.Equal(t, typescript.Imports(&set), "", "no paths, no block")
	})
}
