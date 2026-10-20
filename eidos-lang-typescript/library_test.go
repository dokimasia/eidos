// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The packages that the library cases name: the package of a library, a
// namespace inside it, a path of another store, and a module of the
// workspace.
const (
	libraryPackage   = "typescript-lib://lib.es2024.d.ts"
	namespacePackage = libraryPackage + "/Intl"
	foreignPackage   = "typeshed://stdlib"
	modulePackage    = "svc/store"
	errorName        = "Error"
)

// A reference into a library names a global of TypeScript, as a name
// without a target does, and the rules and the backend agree on it.
func TestLibrary(t *testing.T) {
	t.Parallel()

	t.Run("Library", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "reports true for the package of a library", give: libraryPackage, want: true},
			{name: "reports true for a namespace inside a library", give: namespacePackage, want: true},
			{name: "reports false for a path of another store", give: foreignPackage, want: false},
			{name: "reports false for a module of the workspace", give: modulePackage, want: false},
			{name: "reports false for the global package of the workspace", give: "", want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, typescript.Library(tt.give), tt.want, "whether the package is a library")
			})
		}
	})

	t.Run("Global", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			giveTarget symbol.Identity
			givePkg    string
			want       bool
		}{
			{name: "reports true for a reference without a target or a package", want: true},
			{
				name:       "reports true for a reference to a declaration of a library",
				giveTarget: symbol.Identity{Lang: typescript.Lang, Package: libraryPackage, Name: errorName},
				want:       true,
			},
			{
				name:       "reports false for a reference to a declaration of the global package of the workspace",
				giveTarget: symbol.Identity{Lang: typescript.Lang, Name: errorName},
				want:       false,
			},
			{
				name:       "reports false for a reference to a declaration of a module",
				giveTarget: symbol.Identity{Lang: typescript.Lang, Package: modulePackage, Name: errorName},
				want:       false,
			},
			{
				name:    "reports false for a reference without a target that an import binds",
				givePkg: modulePackage,
				want:    false,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, typescript.Global(tt.giveTarget, tt.givePkg), tt.want,
					"whether the reference names a global")
			})
		}
	})
}

// The rules and the backend ask once per reference whether it names a
// global, so neither question allocates. The ordinary run, which runs no
// benchmark, checks that here.
func TestLibraryAllocs(t *testing.T) {
	target := symbol.Identity{Lang: typescript.Lang, Package: libraryPackage, Name: errorName}
	var library, global bool
	assert.MaxAllocs(t, func() { library = typescript.Library(namespacePackage) }, 0, "Library allocates nothing")
	assert.True(t, library, "Library reports a namespace inside a library")
	assert.MaxAllocs(t, func() { global = typescript.Global(target, "") }, 0, "Global allocates nothing")
	assert.True(t, global, "Global reports a reference to a declaration of a library")
}

// BenchmarkLibrary measures the questions that the rules and the backend
// ask of each reference.
func BenchmarkLibrary(b *testing.B) {
	b.Run("Library", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var library bool
		for c.Loop() {
			library = typescript.Library(namespacePackage)
		}
		assert.True(b, library, "Library reports a namespace inside a library")
	})

	b.Run("Global", func(b *testing.B) {
		target := symbol.Identity{Lang: typescript.Lang, Package: libraryPackage, Name: errorName}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var global bool
		for c.Loop() {
			global = typescript.Global(target, "")
		}
		assert.True(b, global, "Global reports a reference to a declaration of a library")
	})
}
