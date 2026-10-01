// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// A file's package path and the packages a specifier names are the
// joins between modules, so their spellings are pinned.
func TestModule(t *testing.T) {
	t.Parallel()

	t.Run("packageOf", func(t *testing.T) {
		t.Parallel()

		files := []struct {
			name string
			give string
			want string
		}{
			{
				name: "names a declaration file's package without its whole extension",
				give: "src/a.d.ts", want: aPackage,
			},
			{
				name: "names an ES module file's package without its extension",
				give: "src/a.mts", want: aPackage,
			},
			{
				name: "names a CommonJS module file's package without its extension",
				give: "src/a.cts", want: aPackage,
			},
		}
		for _, tt := range files {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, _ := parsedTree(t, fstest.MapFS{tt.give: {Data: []byte(exportClass)}}, tt.give, plugin.DepthFull)
				assert.Length(t, fileIn(t, gb, tt.want).Decls, 1, "the class is in "+tt.want)
			})
		}
	})

	t.Run("packages", func(t *testing.T) {
		t.Parallel()

		specifiers := []struct {
			name string
			give string
			want []symbol.Identity
		}{
			{
				name: "resolves a ../ specifier from the file's directory",
				give: "../lib", want: []symbol.Identity{id("lib", probeName)},
			},
			{
				name: "resolves the . specifier to the file's directory",
				give: ".", want: []symbol.Identity{id("src", probeName)},
			},
			{
				name: "resolves a specifier with a trailing slash to its directory",
				give: "./models/", want: []symbol.Identity{id("src/models", probeName)},
			},
			{
				name: "resolves a .ts specifier to the file's package",
				give: "./models/user.ts", want: []symbol.Identity{id("src/models/user", probeName)},
			},
		}
		for _, tt := range specifiers {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				scope := scopeIn(t, "import { X } from '"+tt.give+"';\n", aPackage)
				assert.Equal(t, resolved(scope, probeName)[1], tt.want, "the specifier's file")
			})
		}
	})

	t.Run("fileAndIndex", func(t *testing.T) {
		t.Parallel()

		t.Run("names the directory's index after the file", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "import { X } from './models';\n", aPackage)
			assert.Equal(t, resolved(scope, probeName)[2], []symbol.Identity{id("src/models/index", probeName)},
				"the index comes second")
		})
	})

	t.Run("chain", func(t *testing.T) {
		t.Parallel()

		t.Run("probes a script's global package once", func(t *testing.T) {
			t.Parallel()

			scope := scopeIn(t, "class A {}\n", "")
			assert.Length(t, resolved(scope, probeName), 1, "the script's own package is the global one")
		})
	})
}
