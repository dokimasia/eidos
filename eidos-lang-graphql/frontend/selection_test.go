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
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// selectionSchema is the schema of the selection cases: an interface, two
// object types that implement it, and a union of both.
const selectionSchema = `interface Node {
  id: ID!
}

type User implements Node {
  id: ID!
  name: String
  friends: [User!]!
  best: User
}

type Group implements Node {
  id: ID!
  title: String!
}

union Member = User | Group

type Query {
  node(id: ID!): Node
  members: [Member!]!
  me: User
}
`

// selectionDocument selects a struct of an object type, a sum of a union
// and a struct of an interface.
const selectionDocument = `fragment Named on User {
  name
}

query Me($on: Boolean!) {
  me { # the user
    id # the id
    handle: name
    __typename
    best {
      friends {
        id
      }
    }
    ...Named
    when: id @include(if: $on)
    kept: name @skip(if: $on)
  }
}

query Members {
  members {
    __typename
    ... on User {
      _best: best {
        id
      }
    }
    ... on Group {
      title
    }
    ...Named
  }
}

query Nodes {
  node(id: "1") {
    id
    ... on Node {
      id
    }
  }
}
`

// A selection set loads as the declaration of the value that a response
// contains for it, so a generator writes the exact shape of each result.
func TestSelection(t *testing.T) {
	t.Parallel()

	unit, sink, fixture := parsedDocument(t, selectionDocument)
	assert.Empty(t, codesOf(sink), "the schema and the document load clean")
	user := nestedStruct(t, declOf(t, fixture, "MeQuery"), "me")
	fields := map[string]*node.Field{}
	for _, f := range user.Fields {
		fields[f.Name] = f
	}

	t.Run("value", func(t *testing.T) {
		t.Parallel()

		t.Run("loads the selection of an object type as a struct of a field for each response key", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldNames(user.Fields), []string{
				"id", "handle", "__typename", "best", "name", "when", "kept",
			}, "the fields are in the order of their first selections")
		})

		t.Run("nests the struct of the selection of a field in the struct of its parent", func(t *testing.T) {
			t.Parallel()

			friends := nestedStruct(t, nestedStruct(t, user, "best"), "friends")
			assert.Equal(t, fieldNames(friends.Fields), []string{"id"}, "the innermost selection has its field")
		})

		t.Run("loads the selection of a union with fragments of its members as a sum", func(t *testing.T) {
			t.Parallel()

			members, is := nested(t, declOf(t, fixture, "MembersQuery"), "members").(*node.Sum)
			assert.True(t, is, "the selection with fragments of particular types is a sum")
			assert.Length(t, members.Variants, 2, "one variant for each possible type")
			expect.Equal(t, members.Variants[0].Name, "User", "in the order of the possible types")
			expect.Equal(t, fieldNames(members.Variants[0].Fields), []string{"__typename", "_best", "name"},
				"the variant has the fields that apply to its type")
			expect.Equal(t, fieldNames(members.Variants[1].Fields), []string{"__typename", "title"},
				"the other variant has its own fields")
		})

		t.Run("declares the struct of a field of a variant beside the sum", func(t *testing.T) {
			t.Parallel()

			best := nestedStruct(t, declOf(t, fixture, "MembersQuery"), "membersUser_best")
			assert.Equal(t, fieldNames(best.Fields), []string{"id"}, "the struct of the field of the variant")
		})

		t.Run("loads the selection of an interface without a narrower fragment as a struct", func(t *testing.T) {
			t.Parallel()

			nodeValue := nestedStruct(t, declOf(t, fixture, "NodesQuery"), "node")
			assert.Equal(t, fieldNames(nodeValue.Fields), []string{"id"}, "the two selections of id are one field")
		})
	})

	t.Run("field", func(t *testing.T) {
		t.Parallel()

		t.Run("names a field after its alias", func(t *testing.T) {
			t.Parallel()

			handle := fields["handle"]
			expect.Equal(t, handle.Type.Spelling, "String", "the alias selects the type of name")
			expect.Equal(t, stampsOn(unit, handle, graphql.CoordinateKey), []string{"User.name"},
				"the coordinate names the schema field that the alias selects")
		})

		t.Run("loads a field of a non-null type as the type itself", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fields["id"].Type.Form, symbol.FormNamed, "the id is never null")
		})

		t.Run("loads a field under @include as optional", func(t *testing.T) {
			t.Parallel()

			when := fields["when"].Type
			expect.Equal(t, when.Form, symbol.FormOptional, "the response can lack the field")
			expect.Equal(t, when.Spelling, "ID", "the optional form spells the type without the mark")
		})

		t.Run("loads a field of a nullable type under @skip as optional once", func(t *testing.T) {
			t.Parallel()

			kept := fields["kept"].Type
			expect.Equal(t, kept.Form, symbol.FormOptional, "the field is optional")
			expect.Equal(t, kept.Elems[0].Form, symbol.FormNamed, "the optional form is not doubled")
		})

		t.Run("returns the comment after a field as its comment", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fields["id"].Comment, "the id", "the comment on the line of the field")
		})

		t.Run("returns the comment after the opening brace of a selection as the comment of its field",
			func(t *testing.T) {
				t.Parallel()

				result, _ := declOf(t, fixture, "MeQuery").(*node.Struct)
				assert.Equal(t, result.Fields[0].Comment, "the user", "the comment after the brace of me")
			})

		t.Run("loads a field that one selection alone makes conditional as required", func(t *testing.T) {
			t.Parallel()

			_, _, docs := parsedDocument(t,
				"query Q($on: Boolean!) {\n  me {\n    id @include(if: $on)\n    id\n  }\n}\n")
			me := nestedStruct(t, declOf(t, docs, "QQuery"), "me")
			assert.Equal(t, me.Fields[0].Type.Form, symbol.FormNamed, "the second selection of id is unconditional")
		})
	})

	t.Run("fieldType", func(t *testing.T) {
		t.Parallel()

		t.Run("loads __typename as a non-null String", func(t *testing.T) {
			t.Parallel()

			typename := fields["__typename"].Type
			expect.Equal(t, typename.Spelling, graphql.StringScalar, "the name of the type is text")
			expect.Equal(t, typename.Form, symbol.FormNamed, "the name of the type is never null")
		})

		t.Run("spells the struct of a selection in place of the named type of its field", func(t *testing.T) {
			t.Parallel()

			best := fields["best"].Type
			expect.Equal(t, best.Form, symbol.FormOptional, "best is nullable")
			expect.Equal(t, best.Elems[0].Spelling, "best", "the reference names the struct of the selection")
		})

		t.Run("positions the type of a field at the field in the document", func(t *testing.T) {
			t.Parallel()

			best := fields["best"]
			expect.Equal(t, best.Type.Pos, best.Pos, "the type is at the field")
			expect.Equal(t, best.Type.Elems[0].Pos, best.Pos, "every node of the type is at the field")
		})

		t.Run("spells a field that the schema does not type with its coordinate", func(t *testing.T) {
			t.Parallel()

			_, sink, docs := parsedDocument(t, "query Q {\n  me {\n    nope\n  }\n}\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.InvalidDocument}, "the validator refuses the field")
			me := nestedStruct(t, declOf(t, docs, "QQuery"), "me")
			assert.Equal(t, me.Fields[0].Type.Spelling, "User.nope", "the reference spells the coordinate")
		})

		t.Run("spells a field without a schema with its name", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "type Query {\n  a: Missing\n}\n\nquery Q {\n  a {\n    b\n  }\n}\n")
			assert.Contains(t, codesOf(sink), frontend.InvalidDocument, "the schema does not load")
			result, _ := declOf(t, fileIn(t, gb, appDocs, appFile), "QQuery").(*node.Struct)
			assert.Equal(t, result.Fields[0].Type.Spelling, "a", "the reference spells the name of the field")
		})

		t.Run("loads a field of a fragment under @defer as optional", func(t *testing.T) {
			t.Parallel()

			_, _, docs := parsedDocument(t, "query Q {\n  me {\n    ... @defer {\n      id\n    }\n  }\n}\n")
			me := nestedStruct(t, declOf(t, docs, "QQuery"), "me")
			assert.Equal(t, me.Fields[0].Type.Form, symbol.FormOptional, "the first payload lacks the field")
		})
	})

	t.Run("collect", func(t *testing.T) {
		t.Parallel()

		t.Run("selects the fields of a fragment that a selection spreads", func(t *testing.T) {
			t.Parallel()

			expect.Equal(t, fields["name"].Type.Spelling, "String", "the field of the fragment")
			expect.Equal(t, stampsOn(unit, fields["name"], graphql.CoordinateKey), []string{"User.name"},
				"the coordinate is on the type condition of the fragment")
		})

		t.Run("skips a spread of a fragment that the unit does not define", func(t *testing.T) {
			t.Parallel()

			_, sink, docs := parsedDocument(t, "query Q {\n  me {\n    id\n    ...Missing\n  }\n}\n")
			assert.Equal(t, codesOf(sink), []diag.Code{frontend.InvalidDocument}, "the validator refuses the spread")
			me := nestedStruct(t, declOf(t, docs, "QQuery"), "me")
			assert.Equal(t, fieldNames(me.Fields), []string{"id"}, "the spread adds no field")
		})

		t.Run("stops at a cycle of fragments", func(t *testing.T) {
			t.Parallel()

			_, sink, docs := parsedDocument(t, "fragment A on User {\n  id\n  ...B\n}\n\n"+
				"fragment B on User {\n  name\n  ...A\n}\n\nquery Q {\n  me {\n    ...A\n  }\n}\n")
			assert.Contains(t, codesOf(sink), frontend.InvalidDocument, "the validator refuses the cycle")
			me := nestedStruct(t, declOf(t, docs, "QQuery"), "me")
			assert.Equal(t, fieldNames(me.Fields), []string{"id", "name"}, "each fragment adds its fields once")
		})

		t.Run("makes the fields of a conditional spread optional", func(t *testing.T) {
			t.Parallel()

			_, _, docs := parsedDocument(t, "fragment Ids on User {\n  id\n}\n\n"+
				"query Q($on: Boolean!) {\n  me {\n    ...Ids @include(if: $on)\n  }\n}\n")
			me := nestedStruct(t, declOf(t, docs, "QQuery"), "me")
			assert.Equal(t, me.Fields[0].Type.Form, symbol.FormOptional, "the response can lack the fragment")
		})
	})

	t.Run("stampSpreads", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps the fragments that a struct spreads", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, stampsOn(unit, user, graphql.SpreadsKey), []string{"Named"}, "the struct spreads Named")
		})

		t.Run("stamps on a variant the fragments whose condition includes its type", func(t *testing.T) {
			t.Parallel()

			members, _ := nested(t, declOf(t, fixture, "MembersQuery"), "members").(*node.Sum)
			expect.Equal(t, stampsOn(unit, members.Variants[0], graphql.SpreadsKey), []string{"Named"},
				"the fragment on User applies to the variant User")
			expect.Empty(t, stampsOn(unit, members.Variants[1], graphql.SpreadsKey),
				"the fragment on User does not apply to the variant Group")
		})
	})

	t.Run("includes", func(t *testing.T) {
		t.Parallel()

		t.Run("applies a fragment on a union or an interface to every possible type", func(t *testing.T) {
			t.Parallel()

			_, sink, docs := parsedDocument(t, "query Q {\n  members {\n    ... on Member {\n      __typename\n    }\n"+
				"    ... on Node {\n      id\n    }\n    ... on Group {\n      title\n    }\n  }\n}\n")
			assert.Empty(t, codesOf(sink), "the document loads clean")
			members, is := nested(t, declOf(t, docs, "QQuery"), "members").(*node.Sum)
			assert.True(t, is, "the selection is a sum")
			expect.Equal(t, fieldNames(members.Variants[0].Fields), []string{"__typename", "id"},
				"the fragments on the union and on the interface apply to User")
			expect.Equal(t, fieldNames(members.Variants[1].Fields), []string{"__typename", "id", "title"},
				"every fragment applies to Group")
		})
	})

	t.Run("selectionStart", func(t *testing.T) {
		t.Parallel()

		t.Run("ends a field before the spread of the fragment after it", func(t *testing.T) {
			t.Parallel()

			_, sink, docs := parsedDocument(t, "query Q {\n  me {\n    id # the id\n    ... on User {\n      name\n"+
				"    }\n  }\n}\n")
			assert.Empty(t, codesOf(sink), "the document loads clean")
			me := nestedStruct(t, declOf(t, docs, "QQuery"), "me")
			assert.Equal(t, me.Fields[0].Comment, "the id", "the comment trails id, which ends before the spread")
		})
	})

	t.Run("titled", func(t *testing.T) {
		t.Parallel()

		t.Run("titles a key that opens with a lower-case letter in a joined name", func(t *testing.T) {
			t.Parallel()

			_, _, docs := parsedDocument(t, "query Q {\n  members {\n    ... on User {\n      best {\n"+
				"        id\n      }\n    }\n    ... on Group {\n      title\n    }\n  }\n}\n")
			best := nestedStruct(t, declOf(t, docs, "QQuery"), "membersUserBest")
			assert.Equal(t, best.Name, "membersUserBest", "the key opens a word of the joined name")
		})

		t.Run("keeps a key that opens with an underscore in a joined name", func(t *testing.T) {
			t.Parallel()

			best := nestedStruct(t, declOf(t, fixture, "MembersQuery"), "membersUser_best")
			assert.Equal(t, best.Name, "membersUser_best", "the underscore has no upper case")
		})
	})
}

// parsedDocument parses a document beside selectionSchema in one unit, and
// returns the builder of the unit, the sink of its findings and the file of
// the document in the package of the documents.
func parsedDocument(tb testing.TB, document string) (*plugin.GraphBuilder, *diag.Sink, *node.File) {
	tb.Helper()

	gbs, sink := parsedTree(tb, fstest.MapFS{
		operationsSchemaFile: {Data: []byte(selectionSchema)},
		appFile:              {Data: []byte(document)},
	})
	assert.Length(tb, gbs, 1, "the schema and the document are one unit")
	return gbs[0], sink, fileIn(tb, gbs[0], appDocs, appFile)
}

// nested returns the nested declaration of a struct with the name, and fails
// where the struct declares none.
func nested(tb assert.TB, owner symbol.Symbol, name string) symbol.Symbol {
	tb.Helper()

	st, is := owner.(*node.Struct)
	assert.True(tb, is, "the owner is a struct")
	for _, t := range st.Types {
		if nameOf(t) == name {
			return t
		}
	}
	tb.Fatalf("the struct %s declares no %s", st.Name, name)
	return nil
}

// nestedStruct returns the nested struct of a struct with the name, and fails
// where the nested declaration is not a struct.
func nestedStruct(tb assert.TB, owner symbol.Symbol, name string) *node.Struct {
	tb.Helper()

	st, is := nested(tb, owner, name).(*node.Struct)
	assert.True(tb, is, "the nested declaration "+name+" is a struct")
	return st
}

// fieldNames returns the names of fields in their order.
func fieldNames(fields []*node.Field) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, f.Name)
	}
	return out
}
