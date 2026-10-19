// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// operationsSchema is the schema of the operation cases.
const operationsSchema = `directive @tag on QUERY | FRAGMENT_DEFINITION | VARIABLE_DEFINITION

type Query {
  user(id: ID!): User
  users: [User!]!
}

type Mutation {
  rename(id: ID!, name: String!): User
}

type Subscription {
  renamed: User
}

type User {
  id: ID!
  name: String
}
`

// operationsDocument declares an operation of each kind and a fragment.
const operationsDocument = `# +fixture:gen:table name=users
query GetUser(
  $id: ID! = "1" @tag # the user
  $flag: Boolean = false
) @tag {
  user(id: $id) {
    id
    name @include(if: $flag)
  }
}

mutation Rename($id: ID!, $name: String!) {
  rename(id: $id, name: $name) {
    name
  }
}

subscription OnRename {
  renamed {
    id
  }
}

fragment UserFields on User @tag {
  id
  name
}
`

// The files of the operation cases.
const (
	operationsSchemaFile = "app/schema.graphqls"
	anonymousFile        = "app/anon.graphql"
)

// An operation loads as a function whose result is the struct of its
// selection, so a generator writes a typed client of each operation.
func TestOperations(t *testing.T) {
	t.Parallel()

	gbs, sink := parsedTree(t, fstest.MapFS{
		operationsSchemaFile: {Data: []byte(operationsSchema)},
		appFile:              {Data: []byte(operationsDocument)},
		anonymousFile:        {Data: []byte("{\n  users {\n    id\n  }\n}\n")},
	})
	assert.Empty(t, codesOf(sink), "the schema and the documents load clean")
	assert.Length(t, gbs, 1, "the files of one directory are one unit")
	gb := gbs[0]
	docs := fileIn(t, gb, appDocs, appFile)

	t.Run("operation", func(t *testing.T) {
		t.Parallel()

		t.Run("loads a query as a function whose result is the struct of its selection", func(t *testing.T) {
			t.Parallel()

			fn, is := declOf(t, docs, "GetUser").(*node.Function)
			assert.True(t, is, "the operation is a function")
			assert.Length(t, fn.Returns, 1, "the function has one result")
			expect.Equal(t, fn.Returns[0].Type.Spelling, "GetUserQuery", "the result is named after the operation")
			_, isStruct := declOf(t, docs, "GetUserQuery").(*node.Struct)
			expect.True(t, isStruct, "the selection of the operation is a struct of the file")
		})

		t.Run("stamps the kind of an operation and its selection set as written", func(t *testing.T) {
			t.Parallel()

			fn := declOf(t, docs, "GetUser")
			expect.Equal(t, stampsOn(gb, fn, graphql.OperationKey), []string{"query"}, "the kind of the operation")
			expect.Equal(t, stampsOn(gb, fn, graphql.SelectionKey),
				[]string{"{\n  user(id: $id) {\n    id\n    name @include(if: $flag)\n  }\n}"},
				"the selection set from its opening brace to its closing brace")
		})

		t.Run("annotates the function with the directives of its operation", func(t *testing.T) {
			t.Parallel()

			fn, _ := declOf(t, docs, "GetUser").(*node.Function)
			assert.Equal(t, fn.Annotations, symbol.Annotations{{Name: "tag"}}, "the directive of the operation")
		})

		t.Run("attaches the carriers above an operation to its function", func(t *testing.T) {
			t.Parallel()

			fn := declOf(t, docs, "GetUser")
			assert.Length(t, gb.Attachments(), 1, "the carrier attaches")
			assert.Equal(t, gb.Attachments()[0].Subject, fn, "to the function of the operation", assert.ByIdentity())
		})

		t.Run("names the result of a mutation after the mutation", func(t *testing.T) {
			t.Parallel()

			fn, _ := declOf(t, docs, "Rename").(*node.Function)
			expect.Equal(t, fn.Returns[0].Type.Spelling, "RenameMutation", "the result has the suffix Mutation")
			expect.Equal(t, stampsOn(gb, fn, graphql.OperationKey), []string{"mutation"}, "the kind of the operation")
		})

		t.Run("returns an asynchronous stream of the result of a subscription", func(t *testing.T) {
			t.Parallel()

			fn, _ := declOf(t, docs, "OnRename").(*node.Function)
			ret := fn.Returns[0].Type
			expect.Equal(t, ret.Form, symbol.FormStream, "a subscription responds with a sequence of results")
			expect.True(t, ret.Async, "the stream arrives over time")
			expect.Equal(t, ret.Spelling, "stream OnRenameSubscription", "the stream spells its element")
			assert.Length(t, ret.Elems, 1, "the stream has its element")
			expect.Equal(t, ret.Elems[0].Spelling, "OnRenameSubscription", "the element is the result")
		})

		t.Run("names an anonymous operation after the base name of its file", func(t *testing.T) {
			t.Parallel()

			anon := fileIn(t, gb, appDocs, anonymousFile)
			fn, is := declOf(t, anon, "anon").(*node.Function)
			assert.True(t, is, "the anonymous operation is a function")
			expect.Equal(t, fn.Returns[0].Type.Spelling, "anonQuery", "the result is named after the function")
			expect.Equal(t, stampsOn(gb, fn, graphql.AnonymousKey), []string{"anonymous"},
				"the stamp marks the operation without a name")
		})

		t.Run("documents an operation and its variables with their descriptions", func(t *testing.T) {
			t.Parallel()

			gbs, sink := parsedTree(t, fstest.MapFS{
				operationsSchemaFile: {Data: []byte(operationsSchema)},
				appFile: {Data: []byte("\"Fetches a user.\"\nquery GetUser(\"The user.\" $id: ID!) {\n" +
					"  user(id: $id) {\n    id\n  }\n}\n")},
			})
			assert.Empty(t, codesOf(sink), "the September 2025 edition describes executable definitions")
			fn, _ := declOf(t, fileIn(t, gbs[0], appDocs, appFile), "GetUser").(*node.Function)
			expect.Equal(t, fn.Doc, []string{"Fetches a user."}, "the description of the operation")
			assert.Length(t, fn.Params, 1, "the operation has one variable")
			expect.Equal(t, fn.Params[0].Doc, []string{"The user."}, "the description of the variable")
		})

		t.Run("stamps no named operation as anonymous", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, stampsOn(gb, declOf(t, docs, "GetUser"), graphql.AnonymousKey),
				"the operation has a name")
		})
	})

	t.Run("variables", func(t *testing.T) {
		t.Parallel()

		t.Run("loads each variable as a parameter with its type, its default and its directives", func(t *testing.T) {
			t.Parallel()

			fn, _ := declOf(t, docs, "GetUser").(*node.Function)
			assert.Length(t, fn.Params, 2, "one parameter for each variable")
			id := fn.Params[0]
			expect.Equal(t, id.Name, "id", "the parameter has the name of the variable without its sigil")
			expect.Equal(t, id.Type.Spelling, "ID", "the parameter has the type of the variable")
			expect.Equal(t, id.Default, `"1"`, "the parameter has the default of the variable")
			expect.Equal(t, id.Annotations, symbol.Annotations{{Name: "tag"}}, "the directive of the variable")
			expect.Equal(t, fn.Params[1].Type.Form, symbol.FormOptional, "a nullable variable is optional")
		})

		t.Run("returns the comment on the line of a variable as its comment", func(t *testing.T) {
			t.Parallel()

			fn, _ := declOf(t, docs, "GetUser").(*node.Function)
			assert.Equal(t, fn.Params[0].Comment, "the user", "the comment after the variable")
		})

		t.Run("loads no parameter for an operation without variables", func(t *testing.T) {
			t.Parallel()

			fn, _ := declOf(t, docs, "OnRename").(*node.Function)
			assert.Empty(t, fn.Params, "the subscription has no variables")
		})
	})

	t.Run("fragment", func(t *testing.T) {
		t.Parallel()

		t.Run("loads a fragment as a function whose result is named after the fragment", func(t *testing.T) {
			t.Parallel()

			fn, is := declOf(t, docs, "UserFields").(*node.Function)
			assert.True(t, is, "the fragment is a function")
			expect.Equal(t, fn.Returns[0].Type.Spelling, "UserFieldsFragment", "the result has the suffix Fragment")
			expect.Equal(t, fn.Annotations, symbol.Annotations{{Name: "tag"}}, "the directive of the fragment")
		})

		t.Run("stamps the type condition of a fragment and its selection set", func(t *testing.T) {
			t.Parallel()

			fn := declOf(t, docs, "UserFields")
			expect.Equal(t, stampsOn(gb, fn, graphql.FragmentKey), []string{"User"}, "the type condition")
			expect.Equal(t, stampsOn(gb, fn, graphql.SelectionKey), []string{"{\n  id\n  name\n}"},
				"the selection set as written")
		})

		t.Run("loads the variables of a fragment as its parameters", func(t *testing.T) {
			t.Parallel()

			gbs, _ := parsedTree(t, fstest.MapFS{
				operationsSchemaFile: {Data: []byte(operationsSchema)},
				appFile:              {Data: []byte("fragment F($n: Int) on User {\n  id\n}\n")},
			})
			fn, _ := declOf(t, fileIn(t, gbs[0], appDocs, appFile), "F").(*node.Function)
			assert.Length(t, fn.Params, 1, "the variable of the fragment is a parameter")
			assert.Equal(t, fn.Params[0].Name, "n", "the parameter has the name of the variable")
		})
	})

	t.Run("declare", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the struct of a field of a variant of a fragment in the file of the documents",
			func(t *testing.T) {
				t.Parallel()

				gbs, sink := parsedTree(t, fstest.MapFS{
					operationsSchemaFile: {Data: []byte(selectionSchema)},
					appFile: {Data: []byte("fragment M on Member {\n  ... on User {\n    best {\n      id\n    }\n" +
						"  }\n  ... on Group {\n    title\n  }\n}\n")},
				})
				assert.Empty(t, codesOf(sink), "the fragment loads clean")
				assert.Equal(t, declNames(fileIn(t, gbs[0], appDocs, appFile)),
					[]string{"M", "MFragment", "MFragmentUserBest"},
					"a sum declares no nested types, so the struct of best is a declaration of the file")
			})
	})

	t.Run("executable", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the fragments before the operations of a file in the order of their positions",
			func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, declNames(docs), []string{
					"GetUser", "GetUserQuery", "Rename", "RenameMutation", "OnRename", "OnRenameSubscription",
					"UserFields", "UserFieldsFragment",
				}, "each function precedes the declaration of its result")
			})
	})
}
