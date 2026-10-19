// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// schemaAllocs is a schema statement: the text that the writer returns.
const schemaAllocs = 1

// The header markers of the roots of mutations and of subscriptions of a
// schema whose root types do not have the default names.
var (
	mutationRoot     = symbol.Annotation{Name: string(graphql.MutationMarker), Args: []string{"Mutations"}}
	subscriptionRoot = symbol.Annotation{Name: string(graphql.SubscriptionMarker), Args: []string{"Changes"}}
)

// TestHeader pins the schema statement of a file, byte for byte in the
// layout of graphql-js.
func TestHeader(t *testing.T) {
	t.Parallel()

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Schema", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give *emit.File
				want string
			}{
				{
					name: "returns nothing for a header without a statement",
					give: &emit.File{},
				},
				{
					name: "returns the root operation types of a schema in the order of the statement",
					give: &emit.File{Annotations: symbol.Annotations{subscriptionRoot, queryRoot, mutationRoot}},
					want: "schema {\n  query: Root\n  mutation: Mutations\n  subscription: Changes\n}\n\n",
				},
				{
					name: "returns the description and the directives of a schema",
					give: &emit.File{
						Doc: []string{"The schema of the shop."},
						Annotations: symbol.Annotations{
							queryRoot,
							{Name: "link", Args: []string{`url: "https://specs.apollo.dev/federation/v2.3"`}},
						},
					},
					want: "\"The schema of the shop.\"\n" +
						"schema @link(url: \"https://specs.apollo.dev/federation/v2.3\") {\n  query: Root\n}\n\n",
				},
				{
					name: "returns an extension of the schema under the marker graphql:extend",
					give: &emit.File{Annotations: symbol.Annotations{extendMarker, mutationRoot}},
					want: "extend schema {\n  mutation: Mutations\n}\n\n",
				},
				{
					name: "returns an extension of the schema that adds a directive alone",
					give: &emit.File{Annotations: symbol.Annotations{extendMarker, shareable}},
					want: "extend schema @shareable\n\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, (&backend.Vocabulary{}).Schema(tt.give), tt.want, "the schema statement")
				})
			}
		})
	})
}

// Schema allocates the text of a statement, once, and nothing for a header
// without one. The ordinary run, which runs no benchmark, checks those
// ceilings here.
func TestHeaderAllocs(t *testing.T) {
	v := &backend.Vocabulary{}
	header := &emit.File{Annotations: symbol.Annotations{queryRoot}}
	var got string
	assert.MaxAllocs(t, func() { got = v.Schema(header) }, schemaAllocs, "Schema allocates the text of the statement")
	assert.Equal(t, got, "schema {\n  query: Root\n}\n\n", "Schema writes the statement")
	empty := &emit.File{}
	assert.MaxAllocs(t, func() { got = v.Schema(empty) }, 0, "Schema allocates nothing for a header without one")
	assert.Empty(t, got, "Schema writes no statement")
}

// BenchmarkHeader measures the schema statement that every file writes, and
// the header of a file without one.
func BenchmarkHeader(b *testing.B) {
	headers := []struct {
		name   string
		give   *emit.File
		want   string
		allocs uint64
	}{
		{
			name: "a statement", give: &emit.File{Annotations: symbol.Annotations{queryRoot}},
			want: "schema {\n  query: Root\n}\n\n", allocs: schemaAllocs,
		},
		{name: "a header without a statement", give: &emit.File{}},
	}
	b.Run("Vocabulary", func(b *testing.B) {
		b.Run("Schema", func(b *testing.B) {
			for _, tt := range headers {
				b.Run(tt.name, func(b *testing.B) {
					v := &backend.Vocabulary{}
					c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
					defer c.End()
					var got string
					for c.Loop() {
						got = v.Schema(tt.give)
					}
					assert.Equal(b, got, tt.want, "Schema writes the statement of the header")
				})
			}
		})
	})
}
