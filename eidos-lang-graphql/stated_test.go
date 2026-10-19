// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql"
	graphqlfrontend "go.dokimi.dev/eidos/lang/graphql/frontend"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/plugintest"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package of the schema of the restated tree, the package of its
// documents and its files.
const (
	schemaPkg     = "app"
	documentsPkg  = "app/documents"
	schemaFile    = "app/schema.graphqls"
	liveFile      = "app/live.graphqls"
	ordersFile    = "app/orders.graphql"
	anonymousFile = "app/anonymous.graphql"
	renameFile    = "app/rename.graphql"
)

// The selection sets of the operations and of the fragment of the restated
// tree, as written.
const (
	getBookSelection    = "{\n  book(id: $id) {\n    ...BookFields\n  }\n}"
	bookFieldsSelection = "{\n  id\n  title\n}"
	anonymousSelection  = "{\n  book(id: \"1\") {\n    id\n  }\n}"
)

// The ceilings of the restatements. Stated of an operation allocates its
// list of markers and the arguments of its operation marker. Stated of a
// directive definition allocates its list of markers, and the read of the
// locations allocates the copy of the stamp and its box. Header of a file
// with a schema definition allocates the header, its documentation, its
// list of annotations and the arguments of its two root markers, and the
// read of the roots allocates the copy of the stamp and its box.
const (
	operationAllocs = 1 + 1
	directiveAllocs = 1 + 2
	headerAllocs    = 1 + 1 + 1 + 2 + 2
)

// restatedTree is one unit: a schema file with a schema definition, a
// repeatable directive, the root types, an input object and two extensions
// of types outside the schema, a schema file with an extension of the schema
// that adds the root of subscriptions, a document with an operation and a
// fragment, a document with an anonymous operation, and a document with an
// extension of the schema and an operation.
var restatedTree = fstest.MapFS{
	schemaFile: {Data: []byte(`"The schema of the shop."
schema @tag(name: "shop") {
  query: Root
  mutation: Change
}

directive @tag(name: String!) repeatable on SCHEMA | OBJECT | FIELD_DEFINITION

type Root {
  book(id: ID!): Book
}

type Change {
  rename(id: ID!, title: String!): Book
}

type Book {
  id: ID!
  title: String!
}

input BookInput {
  title: String!
}

extend type Review {
  stars: Int!
}

extend input ReviewInput {
  stars: Int!
}
`)},
	liveFile: {Data: []byte(`extend schema @tag(name: "live") {
  subscription: Live
}

type Live {
  changed: Book
}
`)},
	ordersFile: {Data: []byte(`query GetBook($id: ID!) {
  book(id: $id) {
    ...BookFields
  }
}

fragment BookFields on Book {
  id
  title
}
`)},
	anonymousFile: {Data: []byte(`{
  book(id: "1") {
    id
  }
}
`)},
	renameFile: {Data: []byte(`extend schema @tag(name: "mixed")

mutation Rename($id: ID!) {
  rename(id: $id, title: "New") {
    id
  }
}
`)},
}

// TestStated pins the channels through which Stated and Header restate the
// stamps of a GraphQL source.
func TestStated(t *testing.T) {
	t.Parallel()

	t.Run("Stated", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Identity
			want graphql.Statement
		}{
			{
				name: "returns the operation marker of the root type of queries",
				give: symbol.Identity{Lang: graphql.Lang, Package: schemaPkg, Name: "Root", Kind: symbol.KindStruct},
				want: graphql.Statement{Annotations: symbol.Annotations{
					{Name: string(graphql.OperationMarker), Args: []string{string(graphql.OperationQuery)}},
				}},
			},
			{
				name: "returns the operation marker of the root type of mutations",
				give: symbol.Identity{Lang: graphql.Lang, Package: schemaPkg, Name: "Change", Kind: symbol.KindStruct},
				want: graphql.Statement{Annotations: symbol.Annotations{
					{Name: string(graphql.OperationMarker), Args: []string{string(graphql.OperationMutation)}},
				}},
			},
			{
				name: "returns the operation marker of a root type that an extension of the schema names",
				give: symbol.Identity{Lang: graphql.Lang, Package: schemaPkg, Name: "Live", Kind: symbol.KindStruct},
				want: graphql.Statement{Annotations: symbol.Annotations{
					{Name: string(graphql.OperationMarker), Args: []string{string(graphql.OperationSubscription)}},
				}},
			},
			{
				name: "returns the input marker of an input object",
				give: symbol.Identity{
					Lang: graphql.Lang, Package: schemaPkg, Name: "BookInput", Kind: symbol.KindStruct,
				},
				want: graphql.Statement{Annotations: symbol.Annotations{{Name: string(graphql.InputMarker)}}},
			},
			{
				name: "returns the extend marker of an extension of a type outside the schema",
				give: symbol.Identity{Lang: graphql.Lang, Package: schemaPkg, Name: "Review", Kind: symbol.KindStruct},
				want: graphql.Statement{Annotations: symbol.Annotations{{Name: string(graphql.ExtendMarker)}}},
			},
			{
				name: "returns the input marker before the extend marker of an extension of an input object",
				give: symbol.Identity{
					Lang: graphql.Lang, Package: schemaPkg, Name: "ReviewInput", Kind: symbol.KindStruct,
				},
				want: graphql.Statement{Annotations: symbol.Annotations{
					{Name: string(graphql.InputMarker)}, {Name: string(graphql.ExtendMarker)},
				}},
			},
			{
				name: "returns the directive marker with the locations of a repeatable directive",
				give: symbol.Identity{Lang: graphql.Lang, Package: schemaPkg, Name: "tag", Kind: symbol.KindInterface},
				want: graphql.Statement{Annotations: symbol.Annotations{
					{Name: string(graphql.DirectiveMarker), Args: []string{"SCHEMA", "OBJECT", "FIELD_DEFINITION"}},
					{Name: string(graphql.RepeatableMarker)},
				}},
			},
			{
				name: "returns the operation marker with the selection set of an operation",
				give: symbol.Identity{
					Lang: graphql.Lang, Package: documentsPkg, Name: "GetBook", Kind: symbol.KindFunction,
				},
				want: graphql.Statement{
					Annotations: symbol.Annotations{
						{Name: string(graphql.OperationMarker), Args: []string{string(graphql.OperationQuery)}},
					},
					Body: emit.Body{Verbatim: getBookSelection},
				},
			},
			{
				name: "returns the anonymous marker of an operation without a name",
				give: symbol.Identity{
					Lang: graphql.Lang, Package: documentsPkg, Name: "anonymous", Kind: symbol.KindFunction,
				},
				want: graphql.Statement{
					Annotations: symbol.Annotations{
						{Name: string(graphql.OperationMarker), Args: []string{string(graphql.OperationQuery)}},
						{Name: string(graphql.AnonymousMarker)},
					},
					Body: emit.Body{Verbatim: anonymousSelection},
				},
			},
			{
				name: "returns the fragment marker with the type condition of a fragment",
				give: symbol.Identity{
					Lang: graphql.Lang, Package: documentsPkg, Name: "BookFields", Kind: symbol.KindFunction,
				},
				want: graphql.Statement{
					Annotations: symbol.Annotations{{Name: string(graphql.FragmentMarker), Args: []string{"Book"}}},
					Body:        emit.Body{Verbatim: bookFieldsSelection},
				},
			},
			{
				name: "returns the zero statement for a declaration without stamps",
				give: symbol.Identity{Lang: graphql.Lang, Package: schemaPkg, Name: "Book", Kind: symbol.KindStruct},
			},
		}
		got := make([]graphql.Statement, 0, len(tests))
		restate(t, func(m *eidos.GraphMatch) {
			for _, tt := range tests {
				got = append(got, graphql.Stated(m, tt.give))
			}
		})
		assert.Length(t, got, len(tests), "Stated returns one statement per identity")
		for i, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, got[i], tt.want, "the statement of the stamps")
			})
		}
	})

	t.Run("Header", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Identity
			want *emit.File
		}{
			{
				name: "returns the description, the directives and the roots of a schema definition",
				give: symbol.Identity{Lang: graphql.Lang, Package: schemaPkg, Name: "Book", Kind: symbol.KindStruct},
				want: &emit.File{
					Doc: []string{"The schema of the shop."},
					Annotations: symbol.Annotations{
						{Name: "tag", Args: []string{`name: "shop"`}},
						{Name: string(graphql.QueryMarker), Args: []string{"Root"}},
						{Name: string(graphql.MutationMarker), Args: []string{"Change"}},
					},
				},
			},
			{
				name: "returns the header of a file node",
				give: symbol.Identity{Lang: graphql.Lang, Package: schemaPkg, Name: schemaFile, Kind: symbol.KindFile},
				want: &emit.File{
					Doc: []string{"The schema of the shop."},
					Annotations: symbol.Annotations{
						{Name: "tag", Args: []string{`name: "shop"`}},
						{Name: string(graphql.QueryMarker), Args: []string{"Root"}},
						{Name: string(graphql.MutationMarker), Args: []string{"Change"}},
					},
				},
			},
			{
				name: "returns the extend marker after the roots of an extension of the schema",
				give: symbol.Identity{Lang: graphql.Lang, Package: schemaPkg, Name: "Live", Kind: symbol.KindStruct},
				want: &emit.File{Annotations: symbol.Annotations{
					{Name: "tag", Args: []string{`name: "live"`}},
					{Name: string(graphql.SubscriptionMarker), Args: []string{"Live"}},
					{Name: string(graphql.ExtendMarker)},
				}},
			},
			{
				name: "returns the header of the file of an operation from the package of the schema",
				give: symbol.Identity{
					Lang: graphql.Lang, Package: documentsPkg, Name: "Rename", Kind: symbol.KindFunction,
				},
				want: &emit.File{Annotations: symbol.Annotations{
					{Name: "tag", Args: []string{`name: "mixed"`}},
					{Name: string(graphql.ExtendMarker)},
				}},
			},
			{
				name: "returns nil for a file without a schema statement",
				give: symbol.Identity{
					Lang: graphql.Lang, Package: documentsPkg, Name: "GetBook", Kind: symbol.KindFunction,
				},
			},
		}
		got := make([]*emit.File, 0, len(tests))
		restate(t, func(m *eidos.GraphMatch) {
			for _, tt := range tests {
				found, held := m.Reader().Lookup(tt.give)
				assert.True(t, held, "the graph declares "+tt.give.String())
				d, is := found.(node.Declaration)
				assert.True(t, is, tt.give.String()+" is a declaration")
				got = append(got, graphql.Header(m, d))
			}
		})
		assert.Length(t, got, len(tests), "Header returns one header per declaration")
		for i, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, got[i], tt.want, "the header of the file of the declaration")
			})
		}
	})
}

// TestStatedAllocs checks the ceilings of Stated and Header in the ordinary
// run, which does not run benchmarks. Each call runs in the match of a
// generate phase over restatedTree.
func TestStatedAllocs(t *testing.T) {
	t.Run("Stated", func(t *testing.T) {
		t.Run("allocates the markers of an operation", func(t *testing.T) {
			id := symbol.Identity{Lang: graphql.Lang, Package: documentsPkg, Name: "GetBook", Kind: symbol.KindFunction}
			restate(t, func(m *eidos.GraphMatch) {
				var got graphql.Statement
				assert.MaxAllocs(t, func() { got = graphql.Stated(m, id) }, operationAllocs,
					"Stated allocates the markers and the arguments of the operation marker")
				assert.Length(t, got.Annotations, 1, "the operation has its operation marker")
			})
		})

		t.Run("allocates the markers of a directive definition", func(t *testing.T) {
			id := symbol.Identity{Lang: graphql.Lang, Package: schemaPkg, Name: "tag", Kind: symbol.KindInterface}
			restate(t, func(m *eidos.GraphMatch) {
				var got graphql.Statement
				assert.MaxAllocs(t, func() { got = graphql.Stated(m, id) }, directiveAllocs,
					"Stated allocates the markers and the read of the locations")
				assert.Length(t, got.Annotations, 2, "the directive has its directive and repeatable markers")
			})
		})

		t.Run("does not allocate for a declaration without stamps", func(t *testing.T) {
			id := symbol.Identity{Lang: graphql.Lang, Package: schemaPkg, Name: "Book", Kind: symbol.KindStruct}
			restate(t, func(m *eidos.GraphMatch) {
				var got graphql.Statement
				assert.MaxAllocs(t, func() { got = graphql.Stated(m, id) }, 0,
					"Stated does not allocate without stamps")
				assert.Empty(t, got.Annotations, "a declaration without stamps has no markers")
			})
		})
	})

	t.Run("Header", func(t *testing.T) {
		t.Run("allocates the header of a file with a schema definition", func(t *testing.T) {
			d := &node.Struct{
				ID:  symbol.Identity{Lang: graphql.Lang, Package: schemaPkg, Name: "Book", Kind: symbol.KindStruct},
				Pos: position.Pos{File: schemaFile},
			}
			restate(t, func(m *eidos.GraphMatch) {
				var got *emit.File
				assert.MaxAllocs(t, func() { got = graphql.Header(m, d) }, headerAllocs,
					"Header allocates the header, its parts and the read of the roots")
				assert.NotNil(t, got, "the file has a schema definition")
			})
		})

		t.Run("does not allocate for a file without a schema statement", func(t *testing.T) {
			d := &node.Function{
				ID: symbol.Identity{
					Lang: graphql.Lang, Package: documentsPkg, Name: "GetBook", Kind: symbol.KindFunction,
				},
				Pos: position.Pos{File: ordersFile},
			}
			restate(t, func(m *eidos.GraphMatch) {
				got := &emit.File{}
				assert.MaxAllocs(t, func() { got = graphql.Header(m, d) }, 0,
					"Header does not allocate without a schema statement")
				assert.Nil(t, got, "the file has no schema statement")
			})
		})
	})
}

// BenchmarkStated measures the restatements that a generator over GraphQL
// sources makes for each declaration, at the ceilings of TestStatedAllocs.
// Each call runs in the match of a generate phase over restatedTree.
func BenchmarkStated(b *testing.B) {
	b.Run("Stated", func(b *testing.B) {
		id := symbol.Identity{Lang: graphql.Lang, Package: documentsPkg, Name: "GetBook", Kind: symbol.KindFunction}
		restate(b, func(m *eidos.GraphMatch) {
			c := bench.Start(b).Warmup(1).MaxAllocs(operationAllocs)
			defer c.End()
			var got graphql.Statement
			for c.Loop() {
				got = graphql.Stated(m, id)
			}
			assert.Length(b, got.Annotations, 1, "the operation has its operation marker")
		})
	})

	b.Run("Header", func(b *testing.B) {
		d := &node.Struct{
			ID:  symbol.Identity{Lang: graphql.Lang, Package: schemaPkg, Name: "Book", Kind: symbol.KindStruct},
			Pos: position.Pos{File: schemaFile},
		}
		restate(b, func(m *eidos.GraphMatch) {
			c := bench.Start(b).Warmup(1).MaxAllocs(headerAllocs)
			defer c.End()
			var got *emit.File
			for c.Loop() {
				got = graphql.Header(m, d)
			}
			assert.NotNil(b, got, "the file has a schema definition")
		})
	})
}

// restate loads restatedTree through the GraphQL frontend, and runs a
// generate phase whose graph handler calls run with its match. The match is
// valid during the call alone.
func restate(tb testing.TB, run func(m *eidos.GraphMatch)) {
	tb.Helper()

	f := plugintest.Loaded(tb, plugintest.Source{
		Tree: restatedTree, Brand: frontendtest.Brand, Frontends: []plugin.Frontend{graphqlfrontend.New()},
	})
	restater := eidos.NewPlugin("restater").
		Output(plugin.Output{Per: plugin.PerPlan, Word: "restated"}).
		Handle(eidos.OnGraph(func(m *eidos.GraphMatch, _ *eidos.Emitter) error {
			run(m)
			return nil
		})).Generator()
	r := f.Generate(tb, restater)
	assert.NoError(tb, r.Err, "the generate phase completes")
}
