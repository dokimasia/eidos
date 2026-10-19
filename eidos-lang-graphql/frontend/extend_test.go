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
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// extendSource declares a type of each kind and an extension of each, and
// an extension of a type that the file does not declare.
const extendSource = `directive @tag(name: String!) repeatable on OBJECT | INTERFACE | UNION | ENUM | INPUT_OBJECT | SCALAR

type Query {
  a: Int
}

interface Node {
  id: ID!
}

type User implements Node {
  id: ID!
}

type Group {
  id: ID!
}

union Member = User

enum Role {
  READER
}

input Filter {
  text: String
}

scalar Date

# +fixture:gen:table name=queries
extend type Query implements Node @tag(name: "q") { # the root
  id: ID!
  name: String
  b(arg: Int): Int
}

extend interface Node @tag(name: "n") {
  name: String
}

extend type User {
  name: String
}

extend union Member = Group

extend enum Role {
  WRITER
}

extend input Filter {
  limit: Int
}

extend scalar Date @tag(name: "d")

extend type Review @tag(name: "r") {
  id: ID!
}
`

// The files of the schema statement cases: one with a schema definition and
// one with a schema extension alone.
const (
	definedFile  = "a/one.graphqls"
	extendedFile = "a/two.graphqls"
)

// An extension adds to its type where the unit declares the type, and loads
// as a declaration of its own where it does not, so a generator reads one
// type with every member that the schema gives it.
func TestExtend(t *testing.T) {
	t.Parallel()

	gb, sink := parsedSource(t, extendSource)
	assert.Empty(t, codesOf(sink), "the schema loads clean")
	file := fileIn(t, gb, appPkg, appFile)

	t.Run("merge", func(t *testing.T) {
		t.Parallel()

		t.Run("adds the members of an extension to an object type", func(t *testing.T) {
			t.Parallel()

			query, is := declOf(t, file, "Query").(*node.Struct)
			assert.True(t, is, "the object type is a struct")
			expect.Length(t, query.Fields, 3, "the field of the type and the two fields of the extension")
			expect.Length(t, query.Methods, 1, "the field with arguments of the extension")
			assert.Length(t, query.Implements, 1, "the extension adds an interface")
			expect.Equal(t, query.Implements[0].Spelling, "Node", "the interface of the extension")
		})

		t.Run("adds the directives of an extension to the annotations of its type", func(t *testing.T) {
			t.Parallel()

			query, _ := declOf(t, file, "Query").(*node.Struct)
			assert.Equal(t, query.Annotations, symbol.Annotations{{Name: "tag", Args: []string{`name: "q"`}}},
				"the directive of the extension is an annotation of the type")
		})

		t.Run("takes the comment of an extension for a type without one", func(t *testing.T) {
			t.Parallel()

			query, _ := declOf(t, file, "Query").(*node.Struct)
			assert.Equal(t, query.Comment, "the root", "the comment after the brace of the extension")
		})

		t.Run("attaches the carriers of an extension to its type", func(t *testing.T) {
			t.Parallel()

			query, _ := declOf(t, file, "Query").(*node.Struct)
			assert.Length(t, gb.Attachments(), 1, "the carrier attaches once")
			expect.Equal(t, gb.Attachments()[0].Subject, symbol.Symbol(query), "to the type that the extension extends",
				assert.ByIdentity())
		})

		t.Run("adds the fields of an extension to an interface", func(t *testing.T) {
			t.Parallel()

			nodeType, is := declOf(t, file, "Node").(*node.Interface)
			assert.True(t, is, "the interface is an interface")
			expect.Length(t, nodeType.Fields, 2, "the field of the interface and the field of the extension")
			expect.Equal(t, nodeType.Annotations, symbol.Annotations{{Name: "tag", Args: []string{`name: "n"`}}},
				"the directive of the extension")
		})

		t.Run("adds the members of an extension to a union", func(t *testing.T) {
			t.Parallel()

			member, is := declOf(t, file, "Member").(*node.Alias)
			assert.True(t, is, "the union is an alias")
			expect.Equal(t, member.Target.Spelling, "User | Group", "the target spells every member")
			expect.Length(t, member.Target.Elems, 2, "the member of the union and the member of the extension")
		})

		t.Run("adds the values of an extension to an enum", func(t *testing.T) {
			t.Parallel()

			role, is := declOf(t, file, "Role").(*node.Enum)
			assert.True(t, is, "the enum is an enum")
			assert.Length(t, role.Variants, 2, "the value of the enum and the value of the extension")
			expect.Equal(t, role.Variants[1].Name, "WRITER", "the value of the extension follows")
		})

		t.Run("adds the fields of an extension to an input object", func(t *testing.T) {
			t.Parallel()

			filter, is := declOf(t, file, "Filter").(*node.Struct)
			assert.True(t, is, "the input object is a struct")
			assert.Length(t, filter.Fields, 2, "the field of the input object and the field of the extension")
			expect.Equal(t, filter.Fields[1].Name, "limit", "the field of the extension follows")
		})

		t.Run("adds the directives of an extension to a scalar", func(t *testing.T) {
			t.Parallel()

			date, is := declOf(t, file, "Date").(*node.Alias)
			assert.True(t, is, "the scalar is an alias")
			expect.Equal(t, date.Annotations, symbol.Annotations{{Name: "tag", Args: []string{`name: "d"`}}},
				"the directive of the extension")
		})

		t.Run("adds the interfaces of an extension to an interface", func(t *testing.T) {
			t.Parallel()

			got, sink := parsedSource(t, "interface A {\n  a: Int\n}\n\ninterface B {\n  a: Int\n}\n\n"+
				"extend interface B implements A\n")
			assert.Empty(t, codesOf(sink), "an extension of an interface may add interfaces")
			b, is := declOf(t, fileIn(t, got, appPkg, appFile), "B").(*node.Interface)
			assert.True(t, is, "the interface is an interface")
			assert.Length(t, b.Extends, 1, "the extension adds an interface")
			assert.Equal(t, b.Extends[0].Spelling, "A", "the interface of the extension")
		})

		t.Run("spells the members of an extension of a union without members", func(t *testing.T) {
			t.Parallel()

			got, _ := parsedSource(t, "type A {\n  a: Int\n}\n\nunion U\n\nextend union U = A\n")
			member, is := declOf(t, fileIn(t, got, appPkg, appFile), "U").(*node.Alias)
			assert.True(t, is, "the union is an alias")
			assert.Equal(t, member.Target.Spelling, "A", "the target spells the member of the extension alone")
		})
	})

	t.Run("extensions", func(t *testing.T) {
		t.Parallel()

		t.Run("loads an extension of a type that the unit does not declare as a declaration", func(t *testing.T) {
			t.Parallel()

			review, is := declOf(t, file, "Review").(*node.Struct)
			assert.True(t, is, "the extension of an object type is a struct")
			expect.Equal(t, stampsOn(gb, review, graphql.ExtensionKey), []string{"extend"},
				"the stamp marks the declaration as an extension")
			expect.Length(t, review.Fields, 1, "the declaration has the fields of the extension")
		})

		t.Run("merges a later extension into the declaration of an earlier one", func(t *testing.T) {
			t.Parallel()

			got, sink := parsedSource(t, "extend type Remote {\n  a: Int\n}\n\nextend type Remote {\n  b: Int\n}\n")
			assert.Empty(t, codesOf(sink), "two extensions of one type load clean")
			remote, is := declOf(t, fileIn(t, got, appPkg, appFile), "Remote").(*node.Struct)
			assert.True(t, is, "the first extension is the declaration")
			expect.Length(t, remote.Fields, 2, "the second extension adds its field")
		})

		t.Run("loads an extension of another kind than its type as a declaration of its own", func(t *testing.T) {
			t.Parallel()

			got, sink := parsedSource(t, "type A {\n  a: Int\n}\n\nextend enum A {\n  B\n}\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.InvalidDocument}, "the validator refuses the extension")
			decls := fileIn(t, got, appPkg, appFile).Decls
			kinds := make([]symbol.Kind, 0, len(decls))
			for _, decl := range decls {
				kinds = append(kinds, decl.Kind())
			}
			assert.Equal(t, kinds, []symbol.Kind{symbol.KindStruct, symbol.KindEnum},
				"the type and the extension each load")
		})
	})

	t.Run("schemaStatements", func(t *testing.T) {
		t.Parallel()

		tree := fstest.MapFS{
			definedFile: {Data: []byte(`directive @tag(name: String!) repeatable on SCHEMA

type Root {
  a: Int
}

type Writes {
  b: Int
}

# +fixture:gen:table name=schema
"The schema."
schema @tag(name: "one") {
  query: Root
}
`)},
			extendedFile: {Data: []byte("extend schema @tag(name: \"two\") {\n  mutation: Writes\n}\n")},
		}
		gbs, sink := parsedTree(t, tree)
		assert.Empty(t, codesOf(sink), "the schema loads clean")
		assert.Length(t, gbs, 1, "the two files of one directory are one unit")
		unit := gbs[0]
		defined := fileIn(t, unit, "a", definedFile)
		extended := fileIn(t, unit, "a", extendedFile)

		t.Run("stamps a file with a schema definition as schema", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, stampsOn(unit, defined, graphql.SchemaKey), []string{"schema"},
				"the file defines the schema")
		})

		t.Run("stamps a file with a schema extension alone as extend", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, stampsOn(unit, extended, graphql.SchemaKey), []string{"extend"},
				"the file extends the schema")
		})

		t.Run("stamps the root operation types of a statement as written", func(t *testing.T) {
			t.Parallel()

			expect.Equal(t, stampsOn(unit, defined, graphql.RootsKey), []string{"query: Root"},
				"the roots of the definition")
			expect.Equal(t, stampsOn(unit, extended, graphql.RootsKey), []string{"mutation: Writes"},
				"the roots of the extension")
		})

		t.Run("documents a file with the description of its schema definition", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, defined.Doc, []string{"The schema."}, "the description is the documentation of the file")
		})

		t.Run("annotates a file with the directives of its statements", func(t *testing.T) {
			t.Parallel()

			expect.Equal(t, defined.Annotations, symbol.Annotations{{Name: "tag", Args: []string{`name: "one"`}}},
				"the directive of the definition")
			expect.Equal(t, extended.Annotations, symbol.Annotations{{Name: "tag", Args: []string{`name: "two"`}}},
				"the directive of the extension")
		})

		t.Run("attaches the carriers of a statement to its file", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, unit.Attachments(), 1, "the carrier attaches once")
			assert.Equal(t, unit.Attachments()[0].Subject, symbol.Symbol(defined), "to the file of the statement",
				assert.ByIdentity())
		})
	})

	t.Run("operations", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			giveText string
			wantType string
			want     []string
		}{
			{
				name:     "stamps the root types of their default names without a schema definition",
				giveText: "type Query {\n  a: Int\n}\n\ntype Mutation {\n  b: Int\n}\n",
				wantType: "Mutation",
				want:     []string{"mutation"},
			},
			{
				name:     "stamps the root types that a schema definition names",
				giveText: "schema {\n  query: Root\n}\n\ntype Root {\n  a: Int\n}\n\ntype Query {\n  b: Int\n}\n",
				wantType: "Root",
				want:     []string{"query"},
			},
			{
				name:     "stamps no type of a default name beside a schema definition",
				giveText: "schema {\n  query: Root\n}\n\ntype Root {\n  a: Int\n}\n\ntype Query {\n  b: Int\n}\n",
				wantType: "Query",
			},
			{
				name:     "stamps the default roots of a schema that does not load",
				giveText: "type Query {\n  a: Missing\n}\n",
				wantType: "Query",
				want:     []string{"query"},
			},
			{
				name:     "stamps the roots of the statements of a schema that does not load",
				giveText: "schema {\n  query: Root\n}\n\ntype Root {\n  a: Missing\n}\n",
				wantType: "Root",
				want:     []string{"query"},
			},
			{
				name:     "stamps no root that is not an object type",
				giveText: "schema {\n  query: Root\n}\n\ninterface Root {\n  a: Int\n}\n",
				wantType: "Root",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, _ := parsedSource(t, tt.giveText)
				decl := declOf(t, fileIn(t, got, appPkg, appFile), tt.wantType)
				assert.Equal(t, stampsOn(got, decl, graphql.OperationKey), tt.want,
					"the stamp names the kind of operation of the root")
			})
		}
	})
}
