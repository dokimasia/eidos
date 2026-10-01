// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// A TypeScript file is a module of its own, so a routed file's package
// is its own path, whatever its directory holds.
func TestPackage(t *testing.T) {
	t.Parallel()

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.Placement
			want string
		}{
			{
				name: "returns the file's module path",
				give: plugin.Placement{Path: "svc/store.stub.ts"},
				want: "svc/store.stub",
			},
			{
				name: "returns the file's module path beside a loaded module",
				give: plugin.Placement{
					Path: "svc/store.stub.ts",
					Residents: []plugin.Resident{{
						File: "svc/store.ts",
						Pkg:  symbol.Identity{Lang: typescript.Lang, Package: "svc/store", Kind: symbol.KindPackage},
					}},
				},
				want: "svc/store.stub",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Package(tt.give)
				assert.NoError(t, err, "every path derives a package")
				assert.Equal(t, got, symbol.Identity{
					Lang: typescript.Lang, Package: tt.want, Name: "store.stub", Kind: symbol.KindPackage,
				}, "the module of the file")
			})
		}
	})
}
