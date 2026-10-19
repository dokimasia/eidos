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

// aliasAllocs is the definition of a scalar or of a union: the text that
// the writer returns.
const aliasAllocs = 1

// searchResult is the target of a union of two object types.
var searchResult = &emit.TypeRef{
	Spelling: "Book | Author", Form: symbol.FormUnion,
	Elems: []*emit.TypeRef{{Spelling: "Book"}, {Spelling: "Author"}},
}

// TestAlias pins the definition of a scalar and of a union, byte for byte in
// the layout of graphql-js.
func TestAlias(t *testing.T) {
	t.Parallel()

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Alias", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give *emit.Alias
				want string
			}{
				{
					name: "returns a scalar for an alias without a target",
					give: &emit.Alias{Name: "DateTime"},
					want: "scalar DateTime\n",
				},
				{
					name: "returns the description, the directives and the comment of a scalar",
					give: &emit.Alias{
						Name: "DateTime", Doc: []string{"DateTime is an instant."}, Comment: "RFC 3339",
						Annotations: symbol.Annotations{{
							Name: graphql.SpecifiedByDirective,
							Args: []string{`url: "https://scalars.graphql.org/andimarek/date-time"`},
						}},
					},
					want: "\"DateTime is an instant.\"\n" +
						"scalar DateTime @specifiedBy(url: \"https://scalars.graphql.org/andimarek/date-time\") # RFC 3339\n",
				},
				{
					name: "returns an extension of a scalar under the marker graphql:extend",
					give: &emit.Alias{Name: "DateTime", Annotations: symbol.Annotations{extendMarker, shareable}},
					want: "extend scalar DateTime @shareable\n",
				},
				{
					name: "returns a union of the members of its target",
					give: &emit.Alias{Name: "SearchResult", Target: searchResult},
					want: "union SearchResult = Book | Author\n",
				},
				{
					name: "returns the directives of a union before its members",
					give: &emit.Alias{
						Name: "SearchResult", Target: searchResult, Comment: "found",
						Annotations: symbol.Annotations{shareable},
					},
					want: "union SearchResult @shareable = Book | Author # found\n",
				},
				{
					name: "returns an extension of a union under the marker graphql:extend",
					give: &emit.Alias{
						Name: "SearchResult", Annotations: symbol.Annotations{extendMarker},
						Target: &emit.TypeRef{
							Spelling: "Magazine",
							Form:     symbol.FormUnion,
							Elems:    []*emit.TypeRef{{Spelling: "Magazine"}},
						},
					},
					want: "extend union SearchResult = Magazine\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, (&backend.Vocabulary{}).Alias(tt.give), tt.want, "the definition of the alias")
				})
			}
		})
	})
}

// Alias allocates the text of the definition, once. The ordinary run, which
// runs no benchmark, checks that ceiling here.
func TestAliasAllocs(t *testing.T) {
	v := &backend.Vocabulary{}
	union := &emit.Alias{Name: "SearchResult", Target: searchResult}
	var got string
	assert.MaxAllocs(t, func() { got = v.Alias(union) }, aliasAllocs, "Alias allocates the text of the definition")
	assert.Equal(t, got, "union SearchResult = Book | Author\n", "Alias writes the union")
}

// BenchmarkAlias measures the definition that every scalar and every union
// of a file write.
func BenchmarkAlias(b *testing.B) {
	b.Run("Vocabulary", func(b *testing.B) {
		b.Run("Alias", func(b *testing.B) {
			v := &backend.Vocabulary{}
			union := &emit.Alias{Name: "SearchResult", Target: searchResult}
			c := bench.Start(b).Warmup(1).MaxAllocs(aliasAllocs)
			defer c.End()
			var got string
			for c.Loop() {
				got = v.Alias(union)
			}
			assert.Equal(b, got, "union SearchResult = Book | Author\n", "Alias writes the union")
		})
	})
}
