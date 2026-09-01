// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/render"
)

// The import block is pinned byte for byte.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("renders sorted import statements", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		set.Add("svc/api/Store")
		set.Add("java/util/List")
		assert.Equal(t, backend.Imports(&set),
			"import java.util.List;\nimport svc.api.Store;\n\n",
			"dots for slashes, sorted, a blank line after the block")
	})

	t.Run("binds a named entry and dedupes its bare twin", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		set.AddNamed("java/util", "List")
		set.AddNamed("svc/api", "Store")
		set.Add("svc/api/Store")
		assert.Equal(t, backend.Imports(&set),
			"import java.util.List;\nimport svc.api.Store;\n\n",
			"path.Name per statement, one statement per spelling")
	})

	t.Run("a file importing nothing renders no block", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		assert.Equal(t, backend.Imports(&set), "", "no paths, no block")
	})
}
