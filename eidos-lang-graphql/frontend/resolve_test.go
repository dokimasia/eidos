// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/frontend"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The files of a workspace whose schema shop shares the types of dep, and
// the configurations and the schema of dep.
const (
	shopSchema  = "shop/schema.graphqls"
	depGqlgen   = "dep/gqlgen.yml"
	depSchema   = "dep/money.graphqls"
	shopProject = "projects:\n  shop:\n    schema:\n      - shop/*.graphqls\n      - dep/*.graphqls\n"
	depMoney    = "type Money {\n  amount: Int!\n}\n"
)

// The allocations of a resolution.
const (
	// schemaAllocs is a name in a file of a schema without shared packages:
	// the list of tiers and the array of their candidates.
	schemaAllocs = 1 + 1
	// documentsAllocs is a name in a struct of a selection that a struct of
	// an operation nests: the joined chain of the owners, the list of tiers
	// and the array of their candidates.
	documentsAllocs = 1 + 1 + 1
)

// A name resolves in the scopes of GraphQL innermost first: the selections
// of a document, the documents, the schema and its shared packages.
func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves a name in the package of the schema", func(t *testing.T) {
			t.Parallel()

			g := rulestest.Loaded(t, frontend.New(), fstest.MapFS{
				appFile: {Data: []byte("type A {\n  b: B\n}\n\ntype B {\n  c: Int\n}\n")},
			}).Graph
			a := symbol.Identity{Lang: graphql.Lang, Package: appPkg, Name: "A", Kind: symbol.KindStruct}
			b := symbol.Identity{Lang: graphql.Lang, Package: appPkg, Name: "B", Kind: symbol.KindStruct}
			assert.Equal(t, fieldTypes(t, g, a)["b"].Elems[0].Target, b, "the type of the field")
		})

		t.Run("resolves a name in a shared package", func(t *testing.T) {
			t.Parallel()

			g := rulestest.Loaded(t, frontend.New(), fstest.MapFS{
				rootRC:     {Data: []byte(shopProject)},
				depGqlgen:  {Data: []byte(ownSchemas)},
				depSchema:  {Data: []byte(depMoney)},
				shopSchema: {Data: []byte("type Order {\n  total: Money!\n}\n")},
			}).Graph
			order := symbol.Identity{Lang: graphql.Lang, Package: "shop", Name: "Order", Kind: symbol.KindStruct}
			money := symbol.Identity{Lang: graphql.Lang, Package: "dep", Name: "Money", Kind: symbol.KindStruct}
			assert.Equal(t, fieldTypes(t, g, order)["total"].Target, money, "the type of the shared package")
		})

		t.Run("resolves a name that the schema extends alone in a shared package first", func(t *testing.T) {
			t.Parallel()

			g := rulestest.Loaded(t, frontend.New(), fstest.MapFS{
				rootRC:    {Data: []byte(shopProject)},
				depGqlgen: {Data: []byte(ownSchemas)},
				depSchema: {Data: []byte(depMoney)},
				shopSchema: {Data: []byte("extend type Money {\n  currency: String\n}\n\n" +
					"type Order {\n  total: Money!\n}\n")},
			}).Graph
			order := symbol.Identity{Lang: graphql.Lang, Package: "shop", Name: "Order", Kind: symbol.KindStruct}
			money := symbol.Identity{Lang: graphql.Lang, Package: "dep", Name: "Money", Kind: symbol.KindStruct}
			expect.Equal(t, fieldTypes(t, g, order)["total"].Target, money,
				"the reference binds the type that the extension extends")
			_, extended := g.Lookup(symbol.Identity{
				Lang: graphql.Lang, Package: "shop", Name: "Money", Kind: symbol.KindStruct,
			})
			expect.True(t, extended, "the extension loads as a declaration of the schema")
		})

		t.Run("resolves a name that the schema extends alone in the schema without a shared declaration",
			func(t *testing.T) {
				t.Parallel()

				g := rulestest.Loaded(t, frontend.New(), fstest.MapFS{
					appFile: {Data: []byte("extend type Review {\n  stars: Int\n}\n\n" +
						"type Query {\n  review: Review\n}\n")},
				}).Graph
				query := symbol.Identity{Lang: graphql.Lang, Package: appPkg, Name: "Query", Kind: symbol.KindStruct}
				review := symbol.Identity{Lang: graphql.Lang, Package: appPkg, Name: "Review", Kind: symbol.KindStruct}
				assert.Equal(t, fieldTypes(t, g, query)["review"].Elems[0].Target, review,
					"the reference binds the declaration of the extension")
			})

		t.Run("resolves a name of a selection in the chain of its selections innermost first", func(t *testing.T) {
			t.Parallel()

			g := rulestest.Loaded(t, frontend.New(), fstest.MapFS{
				operationsSchemaFile: {Data: []byte(selectionSchema)},
				appFile:              {Data: []byte("query Me {\n  me {\n    best {\n      id\n    }\n  }\n}\n")},
			}).Graph
			me := symbol.Identity{
				Lang: graphql.Lang, Package: appDocs, Owner: "MeQuery", Name: "me", Kind: symbol.KindStruct,
			}
			best := symbol.Identity{
				Lang: graphql.Lang, Package: appDocs, Owner: "MeQuery.me", Name: "best", Kind: symbol.KindStruct,
			}
			assert.Equal(t, fieldTypes(t, g, me)["best"].Elems[0].Target, best,
				"the struct of the selection of best nests in the struct of me")
		})

		t.Run("resolves a name of a selection in the package of its schema", func(t *testing.T) {
			t.Parallel()

			g := rulestest.Loaded(t, frontend.New(), fstest.MapFS{
				operationsSchemaFile: {Data: []byte("enum Role {\n  READER\n}\n\ntype Query {\n  role: Role!\n}\n")},
				appFile:              {Data: []byte("query R {\n  role\n}\n")},
			}).Graph
			result := symbol.Identity{Lang: graphql.Lang, Package: appDocs, Name: "RQuery", Kind: symbol.KindStruct}
			role := symbol.Identity{Lang: graphql.Lang, Package: appPkg, Name: "Role", Kind: symbol.KindEnum}
			assert.Equal(t, fieldTypes(t, g, result)["role"].Target, role, "the enum of the schema")
		})

		t.Run("returns no candidate for a built-in scalar", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, aSource)
			scope := plugin.ImportScope{Bindings: gb.Scopes()[0].Bindings}
			assert.Empty(t, frontend.New().Resolve(scope, graphql.IntScalar), "the rules project a built-in scalar")
		})

		t.Run("returns no candidate for a scope without the bindings of the frontend", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, frontend.New().Resolve(plugin.ImportScope{}, "A"), "the scope has no bindings")
		})
	})
}

// A resolution allocates its candidates. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestResolveAllocs(t *testing.T) {
	checkAllocs(t, resolveCalls(t))
}

// BenchmarkResolve measures the resolution that the load makes for every
// type spelling of a schema and of a document.
func BenchmarkResolve(b *testing.B) {
	benchCalls(b, resolveCalls(b))
}

// resolveCalls returns a call of Resolve in a file of a schema and in a
// struct of a selection.
func resolveCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := frontend.New()
	gb, _ := parsedSource(tb, "type Query {\n  a: Int\n}\n\nquery Q {\n  a\n}\n")
	scopes := gb.Scopes()
	assert.Length(tb, scopes, 2, "the file has a scope in the schema and one in the documents")
	inSchema := plugin.ImportScope{Bindings: scopes[0].Bindings}
	inSelection := plugin.ImportScope{Bindings: scopes[1].Bindings, Owner: symbol.Identity{
		Lang: graphql.Lang, Package: appDocs, Owner: "MeQuery", Name: "me", Kind: symbol.KindStruct,
	}}
	var candidates plugin.Candidates
	return []allocCall{
		{
			name: "Resolve", caseName: "a name in a file of the schema", allocs: schemaAllocs,
			call: func() { candidates = f.Resolve(inSchema, "User") },
			check: func(tb assert.TB) {
				assert.Length(tb, candidates, 1, "Resolve returns the tier of the schema")
			},
		},
		{
			name: "Resolve", caseName: "a name in a struct of a selection", allocs: documentsAllocs,
			call: func() { candidates = f.Resolve(inSelection, "best") },
			check: func(tb assert.TB) {
				assert.Length(tb, candidates, 4, "Resolve returns two tiers of the chain, the documents and the schema")
			},
		},
	}
}

// fieldTypes returns the type of each field of a struct of the graph by the
// name of the field, and fails where the graph declares no such struct.
func fieldTypes(tb assert.TB, g *store.Graph, id symbol.Identity) map[string]*node.TypeRef {
	tb.Helper()

	decl, held := g.Lookup(id)
	assert.True(tb, held, "the graph declares "+id.String())
	st, is := decl.(*node.Struct)
	assert.True(tb, is, id.String()+" is a struct")
	out := map[string]*node.TypeRef{}
	for _, f := range st.Fields {
		out[f.Name] = f.Type
	}
	return out
}
