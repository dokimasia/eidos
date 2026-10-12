// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/render"
)

// The import cases name a class-qualified path, the package Java
// imports into every file, and a class of it.
const (
	storeClassPath = "svc/api/Store"
	listClassPath  = "java/util/List"
	langPkg        = "java/lang"
	stringClass    = "String"
	apiPkg         = "svc/api"
	storeClass     = "Store"
)

// importsAllocs is a block: the sorted entries, and the statements
// sized once.
const importsAllocs = 2

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

		t.Run("writes one statement for a named entry with a bare twin", func(t *testing.T) {
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

	t.Run("TemplateImport", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			givePath  string
			giveName  string
			wantLocal string
			wantItem  string
		}{
			{
				name:     "binds a class without a name under the last segment of its path",
				givePath: storeClassPath, wantLocal: storeClass,
			},
			{
				name:     "binds the class that the template names",
				givePath: apiPkg, giveName: storeClass, wantLocal: storeClass, wantItem: storeClass,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				local, item, ok := backend.TemplateImport(tt.givePath, tt.giveName)
				expect.True(t, ok, "an import statement binds a name")
				expect.Equal(t, local, tt.wantLocal, "TemplateImport returns the name that the import binds")
				expect.Equal(t, item, tt.wantItem, "TemplateImport returns the class that the name refers to")
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

// BenchmarkImports measures the block every rendered file that imports
// a class writes.
func BenchmarkImports(b *testing.B) {
	benchCalls(b, importsCalls())
}

// importsCalls returns a call of Imports over two classes, and over an
// empty set.
func importsCalls() []allocCall {
	var full, empty render.ImportSet
	full.Add(storeClassPath)
	full.Add(listClassPath)
	var out string
	return []allocCall{
		{
			name: "Imports", caseName: "a set of two imports", allocs: importsAllocs,
			call: func() { out = backend.Imports(&full) },
			check: func(tb assert.TB) {
				assert.Equal(tb, out, "import java.util.List;\nimport svc.api.Store;\n\n",
					"Imports writes both statements")
			},
		},
		{
			name: "Imports", caseName: "an empty set",
			call:  func() { out = backend.Imports(&empty) },
			check: func(tb assert.TB) { assert.Equal(tb, out, "", "Imports writes no block") },
		},
		{
			name: "TemplateImport", caseName: "an import without a name",
			call:  func() { out, _, _ = backend.TemplateImport(storeClassPath, "") },
			check: func(tb assert.TB) { assert.Equal(tb, out, storeClass, "TemplateImport binds the simple name") },
		},
	}
}
