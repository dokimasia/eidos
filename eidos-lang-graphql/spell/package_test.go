// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// schemaPackage is the one package of every GraphQL file of a plan.
var schemaPackage = symbol.Identity{Lang: graphql.Lang, Kind: symbol.KindPackage}

// TestPackage pins the one package of every routed GraphQL file, because a
// schema has one namespace.
func TestPackage(t *testing.T) {
	t.Parallel()

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.Placement
		}{
			{
				name: "returns the one package for a file of a schema",
				give: plugin.Placement{Path: "graph/catalog_schema.graphql"},
			},
			{
				name: "returns the one package for a file of another directory",
				give: plugin.Placement{Path: "client/operations.graphql"},
			},
			{
				name: "returns the one package for a file whose units derive from a package",
				give: plugin.Placement{
					Path: "graph/catalog_schema.graphql",
					Origin: symbol.Identity{
						Lang: "golang", Package: "example.com/catalog", Name: "catalog", Kind: symbol.KindPackage,
					},
				},
			},
			{
				name: "returns the one package for a file with a header",
				give: plugin.Placement{
					Path:   "graph/catalog_schema.graphql",
					Header: symbol.Annotations{{Name: string(graphql.QueryMarker), Args: []string{"Query"}}},
				},
			},
			{
				name: "returns the one package for a file below an import base",
				give: plugin.Placement{Path: "out/graph/catalog_schema.graphql", ImportBase: ".", BaseDir: "out"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Package(tt.give)
				assert.NoError(t, err, "every file has a package")
				assert.Equal(t, got, schemaPackage, "the package of the schema")
			})
		}
	})
}

// TestPackageAllocs checks that Package does not allocate, in the ordinary
// run, which does not run benchmarks.
func TestPackageAllocs(t *testing.T) {
	at := plugin.Placement{Path: "graph/catalog_schema.graphql"}
	var got symbol.Identity
	assert.MaxAllocs(t, func() { got, _ = spell.Package(at) }, 0, "Package does not allocate")
	assert.Equal(t, got, schemaPackage, "Package returns the package of the schema")
}

// BenchmarkPackage measures the package that the layout names for every
// routed file.
func BenchmarkPackage(b *testing.B) {
	b.Run("Package", func(b *testing.B) {
		at := plugin.Placement{Path: "graph/catalog_schema.graphql"}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got symbol.Identity
		for c.Loop() {
			got, _ = spell.Package(at)
		}
		assert.Equal(b, got, schemaPackage, "Package returns the package of the schema")
	})
}
