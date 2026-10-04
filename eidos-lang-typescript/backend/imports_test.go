// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/render"
)

// The import cases record a package specifier, a module imported for
// its side effects alone, and the names the imports bind.
const (
	pathPackage  = "node:path"
	sideEffect   = "side/effect"
	codecModule  = "./codec"
	codecName    = "Codec"
	decodeName   = "decode"
	keyedName    = "Keyed"
	storeTypeKey = "Store"
	joinName     = "join"
)

// importsAllocs is a block: the sorted entries, and the block, sized
// once.
const importsAllocs = 1 + 1

// The import block is pinned byte for byte.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("Imports", func(t *testing.T) {
		t.Parallel()

		t.Run("writes package specifiers before relative ones", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.Add(storeModule)
			set.Add(pathPackage)
			assert.Equal(t, backend.Imports(&set), "import 'node:path';\nimport './store';\n\n",
				"single quotes and a blank line after the block")
		})

		t.Run("writes import type for a path bound for the type checker alone", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.AddType(storeModule, rowName)
			set.AddType(storeModule, keyedName)
			assert.Equal(t, backend.Imports(&set), "import type { Keyed, Row } from './store';\n\n",
				"the names sorted")
		})

		t.Run("writes a value import for a path with one value binding", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.AddType(codecModule, codecName)
			set.AddNamed(codecModule, decodeName)
			assert.Equal(t, backend.Imports(&set), "import { Codec, decode } from './codec';\n\n",
				"a type-only binding beside a value one imports as a value")
		})

		t.Run("writes the value import alone for a path bound bare beside a value", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.AddNamed(storeModule, storeTypeKey)
			set.AddNamed(storeModule, rowName)
			set.Add(storeModule)
			set.Add(sideEffect)
			assert.Equal(t, backend.Imports(&set),
				"import 'side/effect';\nimport { Row, Store } from './store';\n\n",
				"a value import runs the module")
		})

		t.Run("writes the side-effect import before import type for a bare type-only path", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.AddType(storeModule, rowName)
			set.Add(storeModule)
			assert.Equal(t, backend.Imports(&set),
				"import './store';\nimport type { Row } from './store';\n\n",
				"TypeScript erases import type, so the module runs through the bare import alone")
		})

		t.Run("writes a renamed declaration behind as", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.BindItem(storeModule, rowName, true)
			set.BindItem(legacyModule, rowName, true)
			assert.Equal(t, backend.Imports(&set),
				"import type { Row as Row2 } from './legacy';\nimport type { Row } from './store';\n\n",
				"the exported name, then the name it binds under")
		})

		t.Run("writes a declaration bound as a value beside a type once", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.BindItem(storeModule, rowName, true)
			set.BindItem(storeModule, rowName, false)
			assert.Equal(t, backend.Imports(&set), "import { Row } from './store';\n\n",
				"one value specifier")
		})

		t.Run("returns nothing for an empty set", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			assert.Equal(t, backend.Imports(&set), "", "no block")
		})
	})
}

// A block allocates the sorted entries and the block itself, and an
// empty set allocates nothing. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestImportsAllocs(t *testing.T) {
	checkAllocs(t, importsCalls())
}

// BenchmarkImports measures the block the render writes once per file.
func BenchmarkImports(b *testing.B) {
	benchCalls(b, importsCalls())
}

// importsCalls returns a call of Imports over two paths of three
// names, one of them renamed, and over an empty set.
func importsCalls() []allocCall {
	var set, empty render.ImportSet
	set.BindItem(storeModule, rowName, true)
	set.BindItem(legacyModule, rowName, true)
	set.BindItem(pathPackage, joinName, false)
	var out string
	return []allocCall{
		{
			name: "Imports", allocs: importsAllocs,
			call: func() { out = backend.Imports(&set) },
			check: func(tb assert.TB) {
				assert.Equal(tb, out,
					"import { join } from 'node:path';\n"+
						"import type { Row as Row2 } from './legacy';\n"+
						"import type { Row } from './store';\n\n",
					"Imports writes the block")
			},
		},
		{
			name:  "Imports/an empty set",
			call:  func() { out = backend.Imports(&empty) },
			check: func(tb assert.TB) { assert.Equal(tb, out, "", "Imports writes no block") },
		},
	}
}
