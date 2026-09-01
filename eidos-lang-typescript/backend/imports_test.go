// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/render"
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
			"import 'node:path';\nimport './store';\n\n",
			"one import per path, packages before relative specifiers, "+
				"single quotes, a blank line after the block")
	})

	t.Run("renders type-only bindings as import type", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		set.AddType("./store", "Row")
		set.AddType("./store", "Keyed")
		set.AddType("./codec", "Codec")
		set.AddNamed("./codec", "decode")
		assert.Equal(t, backend.Imports(&set),
			"import { Codec, decode } from './codec';\n"+
				"import type { Keyed, Row } from './store';\n\n",
			"a path bound only for the type checker imports type; one "+
				"value binding beside a type-only one makes the whole "+
				"import a value import")
	})

	t.Run("renders the names bound under a path", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		set.AddNamed("./store", "Store")
		set.AddNamed("./store", "Row")
		set.Add("./store")
		set.Add("side/effect")
		assert.Equal(t, backend.Imports(&set),
			"import 'side/effect';\n"+
				"import { Row, Store } from './store';\n\n",
			"named form covers its path, side-effect form the rest")
	})

	t.Run("a file importing nothing renders no block", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		assert.Equal(t, backend.Imports(&set), "", "no paths, no block")
	})
}
