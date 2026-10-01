// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"testing"

	"go.dokimi.dev/assert"

	typescript "go.dokimi.dev/eidos/lang/typescript"
)

// A file's module path is its package, so the frontend that loads a
// module and the backend that writes one agree on it.
func TestModulePath(t *testing.T) {
	t.Parallel()

	t.Run("ModulePath", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{
				name: "returns the path of a TypeScript file without its extension",
				give: "svc/store.ts",
				want: "svc/store",
			},
			{name: "returns the path of a TSX file without its extension", give: "ui/view.tsx", want: "ui/view"},
			{
				name: "returns the path of an ES module file without its extension",
				give: "svc/store.mts",
				want: "svc/store",
			},
			{
				name: "returns the path of a CommonJS file without its extension",
				give: "svc/store.cts",
				want: "svc/store",
			},
			{name: "returns the path a JavaScript specifier names", give: "./store.js", want: "./store"},
			{
				name: "returns a declaration file's path without its whole extension",
				give: "types/api.d.ts",
				want: "types/api",
			},
			{
				name: "returns a generated file's path without its last extension",
				give: "svc/store.stub.ts",
				want: "svc/store.stub",
			},
			{name: "returns a path without an extension as written", give: "lodash", want: "lodash"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, typescript.ModulePath(tt.give), tt.want, "the module path")
			})
		}
	})

	t.Run("DeclarationFile", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "reports true for a .d.ts file", give: "types/api.d.ts", want: true},
			{name: "reports true for a .d.mts file", give: "types/api.d.mts", want: true},
			{name: "reports true for a .d.cts file", give: "types/api.d.cts", want: true},
			{name: "reports false for a source file", give: "svc/store.ts", want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, typescript.DeclarationFile(tt.give), tt.want, "whether the file declares only")
			})
		}
	})
}
