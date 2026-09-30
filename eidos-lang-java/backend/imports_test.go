// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/render"
)

// The import fixture: a class-qualified path, the package Java
// imports into every file, and a class of it.
const (
	storeClassPath = "svc/api/Store"
	listClassPath  = "java/util/List"
	langPkg        = "java/lang"
	stringClass    = "String"
	apiPkg         = "svc/api"
	storeClass     = "Store"
)

// The import block is pinned byte for byte.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("Imports", func(t *testing.T) {
		t.Parallel()

		t.Run("writes one sorted statement per path", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.Add(storeClassPath)
			set.Add(listClassPath)
			assert.Equal(t, backend.Imports(&set), "import java.util.List;\nimport svc.api.Store;\n\n",
				"dots for slashes and a blank line after the block")
		})

		t.Run("writes a named entry and its bare twin once", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.AddNamed(collections, listClass)
			set.AddNamed(apiPkg, storeClass)
			set.Add(storeClassPath)
			assert.Equal(t, backend.Imports(&set), "import java.util.List;\nimport svc.api.Store;\n\n",
				"one statement per spelling")
		})

		t.Run("writes no statement for a class of java.lang", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.Claim(langPkg, stringClass)
			set.Claim(collections, listClass)
			assert.Equal(t, backend.Imports(&set), "import java.util.List;\n\n",
				"Java imports java.lang into every file")
		})

		t.Run("returns nothing for an empty set", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			assert.Equal(t, backend.Imports(&set), "", "no block")
		})
	})
}
