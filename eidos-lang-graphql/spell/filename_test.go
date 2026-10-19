// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// filenameAllocs is a filename of a per-source unit whose parts are snake
// case already: the list of its parts, their join and the name with its
// extension.
const filenameAllocs = 3

// The unit of the family schema of graph/catalog.go, and its filename.
const (
	catalogKey      = "graph/catalog.go"
	schemaWord      = "schema"
	catalogFilename = "catalog_schema.graphql"
)

// TestFilename pins the filename of every kind of unit that a plan
// admits, because a configuration of graphql-config and of gqlgen names
// the files of a schema by their paths.
func TestFilename(t *testing.T) {
	t.Parallel()

	t.Run("Filename", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.Unit
			want string
		}{
			{
				name: "joins the stem to the word of a per-source unit",
				give: plugin.Unit{Per: plugin.PerSource, Key: catalogKey, Word: schemaWord},
				want: catalogFilename,
			},
			{
				name: "returns a Pascal-case stem in snake case",
				give: plugin.Unit{Per: plugin.PerSource, Key: "graph/CatalogItem.java", Word: schemaWord},
				want: "catalog_item_schema.graphql",
			},
			{
				name: "returns the stem of a schema file of gqlgen without its extension",
				give: plugin.Unit{Per: plugin.PerSource, Key: "graph/catalog.graphqls", Word: "mirror"},
				want: "catalog_mirror.graphql",
			},
			{
				name: "returns the tag after the word",
				give: plugin.Unit{Per: plugin.PerSource, Key: catalogKey, Word: schemaWord, Tag: "v1"},
				want: "catalog_schema_v1.graphql",
			},
			{
				name: "returns the word alone for a per-package unit",
				give: plugin.Unit{Per: plugin.PerPackage, Key: "graph/api", Word: schemaWord},
				want: "schema.graphql",
			},
			{
				name: "returns the word alone for a plan unit",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "operations"},
				want: "operations.graphql",
			},
			{
				name: "returns an acronym in the word as one name",
				give: plugin.Unit{Per: plugin.PerPlan, Word: "HTTPSchema"},
				want: "http_schema.graphql",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, spell.Filename(tt.give), tt.want, "the filename")
			})
		}
	})
}

// TestFilenameAllocs checks the ceiling of Filename in the ordinary run,
// which does not run benchmarks.
func TestFilenameAllocs(t *testing.T) {
	unit := plugin.Unit{Per: plugin.PerSource, Key: catalogKey, Word: schemaWord}
	var got string
	assert.MaxAllocs(t, func() { got = spell.Filename(unit) }, filenameAllocs,
		"Filename allocates the parts, their join and the name")
	assert.Equal(t, got, catalogFilename, "Filename spells the unit")
}

// BenchmarkFilename measures the filename that a backend spells once for
// each unit.
func BenchmarkFilename(b *testing.B) {
	b.Run("Filename", func(b *testing.B) {
		unit := plugin.Unit{Per: plugin.PerSource, Key: catalogKey, Word: schemaWord}
		c := bench.Start(b).MaxAllocs(filenameAllocs)
		defer c.End()
		var got string
		for c.Loop() {
			got = spell.Filename(unit)
		}
		assert.Equal(b, got, catalogFilename, "Filename spells the unit")
	})
}
