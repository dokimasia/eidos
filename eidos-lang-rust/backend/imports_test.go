// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/rust/backend"
	"go.dokimi.dev/eidos/sdk/render"
)

// The use cases name a standard library module, the name that a use of
// it binds, and an item of it.
const (
	collectionsModule = "std/collections"
	collectionsName   = "collections"
	hashMapName       = "HashMap"
	svcModule         = "svc/store"
)

// importsAllocs is a block: the sorted entries, and the statements
// sized once.
const importsAllocs = 2

// The use block is pinned byte for byte.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("Imports", func(t *testing.T) {
		t.Parallel()

		t.Run("writes one sorted use per module", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.Add(svcModule)
			set.Add(collectionsModule)
			assert.Equal(t, backend.Imports(&set), "use std::collections;\nuse svc::store;\n\n",
				"double colons for slashes and a blank line after")
		})

		t.Run("writes a named entry behind its module", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.AddNamed(collectionsModule, hashMapName)
			assert.Equal(t, backend.Imports(&set), "use std::collections::HashMap;\n\n", "path::Name")
		})

		t.Run("writes a renamed item behind as", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.BindItem(storeModule, rowName, false)
			set.BindItem(legacyModule, rowName, false)
			assert.Equal(t, backend.Imports(&set),
				"use crate::legacy::Row as Row2;\nuse crate::store::Row;\n\n",
				"the item, then the name it binds under")
		})

		t.Run("returns nothing for an empty set", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			assert.Equal(t, backend.Imports(&set), "", "no block")
		})
	})

	t.Run("TemplateImport", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			giveName  string
			wantLocal string
			wantItem  string
		}{
			{
				name:      "binds a module without a name under the last segment of its path",
				wantLocal: collectionsName,
			},
			{
				name:     "binds the item that the template names",
				giveName: hashMapName, wantLocal: hashMapName, wantItem: hashMapName,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				local, item, ok := backend.TemplateImport(collectionsModule, tt.giveName)
				expect.True(t, ok, "a use statement binds a name")
				expect.Equal(t, local, tt.wantLocal, "TemplateImport returns the name that the use binds")
				expect.Equal(t, item, tt.wantItem, "TemplateImport returns the item that the name refers to")
			})
		}
	})
}

// A block allocates its sorted entries and its statements, and an
// empty set nothing. The binding of a template's import allocates
// nothing. The ordinary run, which runs no benchmark, checks those
// ceilings here.
func TestImportsAllocs(t *testing.T) {
	checkAllocs(t, importsCalls())
}

// BenchmarkImports measures the block every rendered file that uses an
// item writes.
func BenchmarkImports(b *testing.B) {
	benchCalls(b, importsCalls())
}

// importsCalls returns a call of Imports over two modules, and over an
// empty set.
func importsCalls() []allocCall {
	var full, empty render.ImportSet
	full.Add(svcModule)
	full.Add(collectionsModule)
	var out string
	return []allocCall{
		{
			name: "Imports", caseName: "a set of two uses", allocs: importsAllocs,
			call: func() { out = backend.Imports(&full) },
			check: func(tb assert.TB) {
				assert.Equal(tb, out, "use std::collections;\nuse svc::store;\n\n", "Imports writes both uses")
			},
		},
		{
			name: "Imports", caseName: "an empty set",
			call:  func() { out = backend.Imports(&empty) },
			check: func(tb assert.TB) { assert.Equal(tb, out, "", "Imports writes no block") },
		},
		{
			name:     "TemplateImport",
			caseName: "an import without a name",
			call:     func() { out, _, _ = backend.TemplateImport(collectionsModule, "") },
			check:    func(tb assert.TB) { assert.Equal(tb, out, collectionsName, "TemplateImport binds the last segment") },
		},
	}
}
