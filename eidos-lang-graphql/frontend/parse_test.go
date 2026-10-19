// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/rulestest"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// brokenField is a definition with a syntax error, which the cap case writes
// on each line.
const brokenField = "type A { a: }\n"

// typeB is the definition that each recovery case loads after a broken
// definition.
const typeB = "\n\ntype B {\n  b: Int\n}\n"

// parseAllocs is the budget of one parse of the canonical corpus of 200,000
// declarations in 1000 units. One run counted 2,720,596 allocations: about
// 1.37 million for the parse of gqlparser, about 950,000 for the nodes, the
// stamps and the references of the lowering, about 200,000 for the tokens
// and the indices of the split, and the validation of each schema, the
// copies of the prelude and the reads of the files the rest. The budget is
// about 5% above the count.
const parseAllocs = 2_860_000

// The second file of a document of the validation cases.
const secondDocument = "app/second.graphql"

// The documents of the cases of operations in two files: an anonymous query
// of the users and a query of the users named Users.
const (
	anonymousUsers = "{\n  users {\n    id\n  }\n}\n"
	namedUsers     = "query Users {\n  users {\n    id\n  }\n}\n"
)

// A parse loads one unit: the type system into the package of its schema
// and the executable definitions into the package of its documents, with a
// finding for each problem of a file and no error for it. A file splits
// into its definitions before it parses, so a syntax error costs the
// definition that contains it and no other.
func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("parse", func(t *testing.T) {
		t.Parallel()

		t.Run("loads the schema tree without an Error", func(t *testing.T) {
			t.Parallel()

			fixture := rulestest.Loaded(t, frontend.New(), os.DirFS("testdata/schema"))
			order := symbol.Identity{Lang: graphql.Lang, Package: "shop", Name: "Order", Kind: symbol.KindStruct}
			decl, held := fixture.Graph.Lookup(order)
			assert.True(t, held, "the project shop declares Order")
			st, _ := decl.(*node.Struct)
			var total *node.Field
			for _, f := range st.Fields {
				if f.Name == "total" {
					total = f
				}
			}
			assert.NotNil(t, total, "Order has its total")
			assert.Equal(t, total.Type.Target,
				symbol.Identity{Lang: graphql.Lang, Package: "dep", Name: "Money", Kind: symbol.KindStruct},
				"the total is the Money of dep, which the project shares")
		})

		t.Run("returns the error of a context that is done", func(t *testing.T) {
			t.Parallel()

			f := frontend.New()
			tree := fstest.MapFS{appFile: {Data: []byte(aSource)}}
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: appFile}}, tree, plugin.DepthFull,
				f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.HonoursCancellation(t, func(ctx context.Context) error { return f.Parse(ctx, u) },
				"a cancelled load stops before it parses")
		})

		t.Run("returns an error for a file that the tree does not contain", func(t *testing.T) {
			t.Parallel()

			f := frontend.New()
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: appFile}}, fstest.MapFS{}, plugin.DepthFull,
				f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.HasError(t, f.Parse(t.Context(), u),
				"a unit whose file is missing from the tree is a fault of the load, not of the schema")
		})

		t.Run("returns an error for a shared file that the tree does not contain", func(t *testing.T) {
			t.Parallel()

			f := frontend.New()
			tree := fstest.MapFS{appFile: {Data: []byte(aSource)}}
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: appFile, Shared: []string{depSchema}}}, tree,
				plugin.DepthFull, f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.HasError(t, f.Parse(t.Context(), u), "a shared file that does not read is a fault of the load")
		})

		t.Run("loads the same declarations at signature depth", func(t *testing.T) {
			t.Parallel()

			f := frontend.New()
			tree := fstest.MapFS{appFile: {Data: []byte(operationsSchema + "\nquery Q {\n  users {\n    id\n  }\n}\n")}}
			var names [2][]string
			for i, depth := range [...]plugin.Depth{plugin.DepthFull, plugin.DepthSignatures} {
				u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: appFile}}, tree, depth,
					f.Syntax(), brand, diag.NewSink(), f.Name())
				assert.NoError(t, f.Parse(t.Context(), u), "the unit parses")
				for _, pkg := range u.Graph().Packages() {
					for _, file := range pkg.Files {
						names[i] = append(names[i], declNames(file)...)
					}
				}
			}
			assert.Equal(t, names[1], names[0], "a schema has no bodies, so the depth changes nothing")
		})

		t.Run("reports UnparsedFile at a syntax error and loads the other definitions", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "type A {\n  a: Int\n}\n\ntype B {\n  b:\n}\n\ntype C {\n  c: Int\n}\n")
			found := findingsOf(sink, frontend.UnparsedFile)
			assert.Length(t, found, 1, "the broken definition reports once")
			expect.Equal(t, found[0].Pos, position.Pos{File: appFile, Line: 7, Col: 1},
				"the finding is at the token that the parser did not expect")
			expect.Equal(t, declNames(fileIn(t, gb, appPkg, appFile)), []string{"A", "C"},
				"every definition outside the broken one loads")
		})

		t.Run("caps the syntax errors of a file", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, strings.Repeat(brokenField, plugin.MaxSyntaxFindings+2))
			found := findingsOf(sink, frontend.UnparsedFile)
			assert.Length(t, found, plugin.MaxSyntaxFindings+1, "the cap of findings and one more")
			assert.Contains(t, found[plugin.MaxSyntaxFindings].Msg, "and 2 more syntax errors",
				"the last finding counts the errors past the cap")
		})

		t.Run("reports UnparsedFile for an executable definition and loads the other operations", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "type Query {\n  a: Int\n}\n\nquery A {\n  a(\n}\n\nquery B {\n  a\n}\n")
			expect.Length(t, findingsOf(sink, frontend.UnparsedFile), 1, "the broken operation reports once")
			expect.Equal(t, declNames(fileIn(t, gb, appDocs, appFile)), []string{"B", "BQuery"},
				"the other operation loads")
		})

		t.Run("reports UnparsedFile at an error of the lexer and loads the lines after it", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "type A {\n  a: Int\n}\n\n\"open\n\ntype B {\n  b: Int\n}\n")
			found := findingsOf(sink, frontend.UnparsedFile)
			assert.Length(t, found, 1, "the lexer reports once")
			expect.Equal(t, found[0].Pos.Line, 5, "the finding is on the line of the error")
			expect.Equal(t, declNames(fileIn(t, gb, appPkg, appFile)), []string{"A", "B"},
				"the definitions before and after the error load")
		})

		t.Run("stops at an error of the lexer on the last line", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "type A {\n  a: Int\n}\n\"open")
			expect.Length(t, findingsOf(sink, frontend.UnparsedFile), 1, "the lexer reports once")
			expect.Equal(t, declNames(fileIn(t, gb, appPkg, appFile)), []string{"A"}, "the definition before it loads")
		})

		resumes := []struct {
			name string
			give string
		}{
			{
				name: "resumes after an error on a line that ends with a carriage return and a line feed",
				give: "\"open\r\ntype B {\r\n  b: Int\r\n}\r\n",
			},
			{
				name: "resumes after an error on a line that ends with a carriage return",
				give: "\"open\rtype B {\r  b: Int\r}\r",
			},
		}
		for _, tt := range resumes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, sink := parsedSource(t, tt.give)
				expect.Length(t, findingsOf(sink, frontend.UnparsedFile), 1, "the lexer reports once")
				b, is := declOf(t, fileIn(t, gb, appPkg, appFile), "B").(*node.Struct)
				assert.True(t, is, "the definition after the error loads")
				expect.Equal(t, b.Pos.Line, 2, "the definition is on the line after the error")
			})
		}

		endings := []struct {
			name string
			give string
		}{
			{
				name: "counts a carriage return and a line feed as one line",
				give: "type A {\r\n  a: Int # the a\r\n}\r\n",
			},
			{
				name: "counts a carriage return as a line",
				give: "type A {\r  a: Int # the a\r}\r",
			},
		}
		for _, tt := range endings {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, sink := parsedSource(t, tt.give)
				assert.Empty(t, codesOf(sink), "the schema loads clean")
				a, _ := declOf(t, fileIn(t, gb, appPkg, appFile), "A").(*node.Struct)
				expect.Equal(t, a.Fields[0].Pos.Line, 2, "the field is on the second line")
				expect.Equal(t, a.Fields[0].Comment, "the a", "the comment trails the field on its line")
			})
		}

		t.Run("reports UnparsedFile for a definition whose bracket does not close at the end of the file",
			func(t *testing.T) {
				t.Parallel()

				gb, sink := parsedSource(t, "type A {\n  a: Int\n")
				expect.Equal(t, codesOf(sink), []diag.Code{frontend.UnparsedFile},
					"the parser reports the end of the file")
				expect.Empty(t, fileIn(t, gb, appPkg, appFile).Decls, "the open definition does not load")
			})

		broken := []struct {
			name string
			give string
		}{
			{name: "loads the definitions after a type without a name", give: "type {\n  a: Int\n}"},
			{name: "loads the definitions after a word that opens no definition", give: "foo bar"},
			{name: "loads the definitions after a stray closing brace", give: "}"},
			{name: "loads the definitions after a directive without its sigil", give: "directive d on OBJECT"},
			{name: "loads the definitions after a directive without its name", give: "directive @ on OBJECT"},
			{name: "loads the definitions after a directive without on", give: "directive @d(a: Int)"},
			{name: "loads the definitions after a fragment without a name", give: "fragment {\n  a\n}"},
			{name: "loads the definitions after a fragment without on", give: "fragment F User {\n  a\n}"},
			{
				name: "loads the definitions after a fragment without its type condition",
				give: "fragment F on {\n  a\n}",
			},
			{name: "loads the definitions after an operation without a body", give: "query Q"},
			{name: "loads the definitions after a description of no definition", give: `"A thing." 42`},
			{name: "loads the definitions after an extension of no kind", give: "extend foo"},
			{name: "loads the definitions after an extension without a keyword", give: "extend {\n  a\n}"},
			{name: "loads the definitions after a union without a name", give: "union = A"},
			{
				name: "loads the definitions after a directive of a type without its name",
				give: "type A @ {\n  a: Int\n}",
			},
		}
		for _, tt := range broken {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, sink := parsedSource(t, tt.give+typeB)
				expect.Equal(t, codesOf(sink), []diag.Code{frontend.UnparsedFile}, "the broken definition reports once")
				expect.Equal(t, declNames(fileIn(t, gb, appPkg, appFile)), []string{"B"}, "the next definition loads")
			})
		}

		t.Run("ends a broken definition at the end of the file", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "type B {\n  b: Int\n}\n\nfoo bar\n")
			expect.Equal(t, codesOf(sink), []diag.Code{frontend.UnparsedFile}, "the broken definition reports once")
			expect.Equal(t, declNames(fileIn(t, gb, appPkg, appFile)), []string{"B"}, "the definition before it loads")
		})

		t.Run("splits an anonymous query from the type system of its file", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "type Query {\n  a: Int\n}\n\n{\n  a\n}\n")
			assert.Empty(t, codesOf(sink), "the mixed file loads clean")
			expect.Equal(t, declNames(fileIn(t, gb, appPkg, appFile)), []string{"Query"}, "the type system")
			expect.Equal(t, declNames(fileIn(t, gb, appDocs, appFile)), []string{"schema", "schemaQuery"},
				"the anonymous operation")
		})

		t.Run("splits a definition of each keyword", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, `schema {
  query: Query
}

scalar S

type Query implements & I @d {
  i: Int
}

interface I {
  i: Int
}

union U @d = | Query

enum E {
  V
}

input In {
  v: Int
}

directive @d repeatable on | OBJECT | UNION

extend type Query {
  j: Int
}

query Q @skip(if: true) {
  i
}

mutation M {
  i
}

subscription Sub {
  i
}

fragment F on Query {
  i
}
`)
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.InvalidDocument, frontend.InvalidDocument},
				"the validator refuses the mutation and the subscription, which the schema does not root")
			expect.Equal(t, declNames(fileIn(t, gb, appPkg, appFile)),
				[]string{"S", "Query", "I", "U", "E", "In", "d"}, "every definition of the type system loads")
			expect.Length(t, fileIn(t, gb, appDocs, appFile).Decls, 8, "every executable definition loads")
		})

		t.Run("stamps a selection set with runes outside ASCII as written", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "type Query {\n  a(x: String): Int\n}\n\nquery Q {\n  a(x: \"é ü\")\n}\n")
			assert.Empty(t, codesOf(sink), "the document loads clean")
			fn := declOf(t, fileIn(t, gb, appDocs, appFile), "Q")
			assert.Equal(t, stampsOn(gb, fn, graphql.SelectionKey), []string{"{\n  a(x: \"é ü\")\n}"},
				"the text of the selection set")
		})

		t.Run("positions an executable definition after the type system at its place in the file", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "type Query {\n  a: Int\n}\n\nquery Q {\n  a\n}\n")
			fn := declOf(t, fileIn(t, gb, appDocs, appFile), "Q")
			assert.Equal(t, fn.Position(), position.Pos{File: appFile, Line: 5, Col: 1}, "the operation is on line 5")
		})

		t.Run("positions a definition after a skip of the lexer at its place in the file", func(t *testing.T) {
			t.Parallel()

			gbs, _ := parsedTree(t, fstest.MapFS{appFile: {Data: []byte("\"é open\ntype Query {\n  a: Int\n}\n")}})
			query, _ := declOf(t, fileIn(t, gbs[0], appPkg, appFile), "Query").(*node.Struct)
			assert.Equal(t, query.Fields[0].Pos, position.Pos{File: appFile, Line: 3, Col: 3},
				"the field keeps its position after the blank of the skip")
		})
	})

	t.Run("reportSyntax", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the errors of the lexer and of the parser in the order of the file", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "type A {\n  a:\n}\n\n\"open\n")
			found := findingsOf(sink, frontend.UnparsedFile)
			assert.Length(t, found, 2, "the parser and the lexer each report")
			expect.Equal(t, found[0].Pos.Line, 3, "the error of the parser comes first")
			expect.Equal(t, found[1].Pos.Line, 5, "the error of the lexer follows")
		})
	})

	t.Run("load", func(t *testing.T) {
		t.Parallel()

		t.Run("loads units that extend a scalar of the prelude at once", func(t *testing.T) {
			t.Parallel()

			extension := []byte("extend scalar String @specifiedBy(url: \"https://example.com/text\")\n")
			g := rulestest.Loaded(t, frontend.New(), fstest.MapFS{
				aSchema: {Data: extension},
				bSchema: {Data: extension},
			}).Graph
			for _, pkg := range []string{"a", "b"} {
				_, held := g.Lookup(symbol.Identity{
					Lang: graphql.Lang, Package: pkg, Name: graphql.StringScalar, Kind: symbol.KindAlias,
				})
				expect.True(t, held, "the unit of "+pkg+" declares its extension of String")
			}
		})

		t.Run("reports InvalidDocument at an error of the schema", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "type A {\n  a: Missing\n}\n")
			found := findingsOf(sink, frontend.InvalidDocument)
			assert.Length(t, found, 1, "the schema does not load")
			expect.Equal(t, found[0].Pos, position.Pos{File: appFile, Line: 2, Col: 6}, "the finding is at the type")
			expect.Contains(t, found[0].Msg, "Undefined type Missing", "the finding states the error of the validator")
		})

		t.Run("reports an error of a shared file at the first own file", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedTree(t, fstest.MapFS{
				rootRC:     {Data: []byte(shopProject)},
				depGqlgen:  {Data: []byte(ownSchemas)},
				depSchema:  {Data: []byte("type Money {\n  amount: Missing\n}\n")},
				shopSchema: {Data: []byte("type Order {\n  total: Money\n}\n")},
			})
			found := findingsOf(sink, frontend.InvalidDocument)
			at := make([]position.Pos, 0, len(found))
			for _, d := range found {
				at = append(at, d.Pos)
			}
			assert.Equal(t, at, []position.Pos{
				{File: depSchema, Line: 2, Col: 11},
				{File: shopSchema, Line: 1, Col: 1},
			}, "the unit of dep reports at the error, and the unit of shop at its first file")
		})
	})

	t.Run("documents", func(t *testing.T) {
		t.Parallel()

		t.Run("reports InvalidDocument at an error of a document against the schema", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "type Query {\n  a: Int\n}\n\nquery Q {\n  b\n}\n")
			found := findingsOf(sink, frontend.InvalidDocument)
			assert.Length(t, found, 1, "the field does not exist")
			expect.Equal(t, found[0].Pos, position.Pos{File: appFile, Line: 6, Col: 3}, "the finding is at the field")
		})

		t.Run("reports an anonymous operation beside another operation of its file", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedSource(t, "type Query {\n  a: Int\n}\n\n{\n  a\n}\n\nquery Q {\n  a\n}\n")
			found := findingsOf(sink, frontend.InvalidDocument)
			assert.Length(t, found, 1, "the anonymous operation is not alone")
			expect.Equal(t, found[0].Pos.Line, 5, "the finding is at the anonymous operation")
		})

		t.Run("validates a spread of a fragment of another file of the unit", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedTree(t, fstest.MapFS{
				operationsSchemaFile: {Data: []byte(operationsSchema)},
				appFile:              {Data: []byte("fragment Ids on User {\n  id\n}\n")},
				secondDocument:       {Data: []byte("query Q {\n  users {\n    ...Ids\n  }\n}\n")},
			})
			assert.Empty(t, codesOf(sink), "the unit validates its documents together")
		})

		t.Run("reports an error of a document in the file of its source", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedTree(t, fstest.MapFS{
				operationsSchemaFile: {Data: []byte(operationsSchema)},
				appFile:              {Data: []byte("query A {\n  users {\n    id\n  }\n}\n")},
				secondDocument:       {Data: []byte("query B {\n  users {\n    nope\n  }\n}\n")},
			})
			found := findingsOf(sink, frontend.InvalidDocument)
			assert.Length(t, found, 1, "the second document does not validate")
			expect.Equal(t, found[0].Pos, position.Pos{File: secondDocument, Line: 3, Col: 5},
				"the finding is in the file of the error")
		})

		t.Run("accepts a fragment that no operation spreads", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedTree(t, fstest.MapFS{
				operationsSchemaFile: {Data: []byte(operationsSchema)},
				appFile:              {Data: []byte("fragment Ids on User {\n  id\n}\n")},
			})
			assert.Empty(t, codesOf(sink), "a client defines fragments for the operations of other units")
		})

		t.Run("accepts an anonymous operation in each of two files of the unit", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedTree(t, fstest.MapFS{
				operationsSchemaFile: {Data: []byte(operationsSchema)},
				appFile:              {Data: []byte(anonymousUsers)},
				secondDocument:       {Data: []byte(anonymousUsers)},
			})
			assert.Empty(t, codesOf(sink), "each anonymous operation is alone in its file")
		})

		t.Run("reports InvalidDocument for an operation name that two files of the unit declare", func(t *testing.T) {
			t.Parallel()

			_, sink := parsedTree(t, fstest.MapFS{
				operationsSchemaFile: {Data: []byte(operationsSchema)},
				appFile:              {Data: []byte(namedUsers)},
				secondDocument:       {Data: []byte(namedUsers)},
			})
			found := findingsOf(sink, frontend.InvalidDocument)
			assert.Length(t, found, 1, "the second operation repeats the name")
			expect.Equal(t, found[0].Pos, position.Pos{File: secondDocument, Line: 1, Col: 1},
				"the finding is at the second operation")
		})
	})
}

// BenchmarkParse drives the parse at the canonical scale: every unit of the
// scaled corpus through Parse into a builder of its own, with the partition
// and the comment syntax settled before the loop, so the number measures
// gqlparser, the validator and the lowering, and fails above parseAllocs.
// Only -bench checks the budget, because the 101 calls of an allocation
// check would take minutes.
func BenchmarkParse(b *testing.B) {
	tree := scaledSchema()
	f := frontend.New()
	units, err := f.Partition(b.Context(), claimedIn(tree), treeReader{tree})
	assert.NoError(b, err, "the corpus partitions")
	syntax := f.Syntax()
	c := bench.Start(b).MaxAllocs(parseAllocs)
	defer c.End()
	for c.Loop() {
		for _, unit := range units {
			u := plugin.NewSourceUnit(unit, tree, plugin.DepthFull, syntax, brand, diag.NewSink(), f.Name())
			if err = f.Parse(b.Context(), u); err != nil {
				break
			}
		}
	}
	assert.NoError(b, err, "the corpus parses")
}
