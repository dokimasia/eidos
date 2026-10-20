// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// typeSystemSource declares a definition of each kind of the type system of
// GraphQL, with descriptions, arguments, defaults and directives.
const typeSystemSource = `"An instant."
scalar Date @specifiedBy(url: "https://example.com/date")

"A thing with an identifier."
interface Node {
  id: ID!
}

"A named thing."
interface Named implements Node {
  id: ID!
  "The name."
  name(locale: String = "en"): String
}

"A user."
type User implements Named & Node @key(fields: "id") {
  id: ID!
  name(locale: String = "en"): String
  "The friends of the user."
  friends("The number of friends." first: Int = 10 @deprecated): [User!]!
}

type Group {
  id: ID!
}

"A user or a group."
union Member = User | Group

enum Role {
  "Reads."
  READER
  WRITER @deprecated(reason: "Use EDITOR.")
  EDITOR
}

input Filter {
  "The text to find."
  text: String = "a"
  role: Role @deprecated
}

"""
Marks a key.
Repeat it for each key.
"""
directive @key("The fields of the key." fields: String! = "id") repeatable on OBJECT | INTERFACE

directive @internal on FIELD_DEFINITION
`

// The type system lowers into the kinds of the model, so a generator reads a
// schema as it reads the declarations of any other language.
func TestTypes(t *testing.T) {
	t.Parallel()

	gb, sink := parsedSource(t, typeSystemSource)
	assert.Empty(t, codesOf(sink), "the schema loads clean")
	file := fileIn(t, gb, appPkg, appFile)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		t.Run("loads a scalar as a defined alias without a target", func(t *testing.T) {
			t.Parallel()

			date, is := declOf(t, file, "Date").(*node.Alias)
			assert.True(t, is, "the scalar is an alias")
			expect.Equal(t, date.Visibility, symbol.VisibilityPublic,
				"GraphQL has no visibility, so every name is public")
			expect.True(t, date.Defined, "the alias defines a type of its own")
			expect.Nil(t, date.Target, "a scalar has no structure")
			expect.Equal(t, date.Doc, []string{"An instant."}, "the description is the documentation")
			specified := symbol.Annotation{
				Name: graphql.SpecifiedByDirective, Args: []string{`url: "https://example.com/date"`},
			}
			expect.Equal(t, date.Annotations, symbol.Annotations{specified},
				"the directive of the scalar is its annotation")
		})

		t.Run("loads an interface with its interfaces in Extends", func(t *testing.T) {
			t.Parallel()

			named, is := declOf(t, file, "Named").(*node.Interface)
			assert.True(t, is, "the interface is an interface")
			assert.Length(t, named.Extends, 1, "the interface implements one interface")
			expect.Equal(t, named.Extends[0].Spelling, "Node", "the interface extends Node")
			expect.Equal(t, named.Extends[0].Pos, position.Pos{File: appFile, Line: 10, Col: 28},
				"the reference is at the name in the implements clause")
		})

		t.Run("loads a field of an interface with arguments as an abstract method", func(t *testing.T) {
			t.Parallel()

			named, _ := declOf(t, file, "Named").(*node.Interface)
			assert.Length(t, named.Fields, 1, "the field without arguments is a field")
			assert.Length(t, named.Methods, 1, "the field with arguments is a method")
			expect.True(t, named.Methods[0].Abstract, "the method of an interface has no body")
			expect.Equal(t, named.Methods[0].Doc, []string{"The name."},
				"the description of the field is its documentation")
		})

		t.Run("loads an object type as a struct with its interfaces in Implements", func(t *testing.T) {
			t.Parallel()

			user, is := declOf(t, file, "User").(*node.Struct)
			assert.True(t, is, "the object type is a struct")
			assert.Length(t, user.Implements, 2, "the object type implements two interfaces")
			expect.Equal(t, user.Implements[0].Spelling, "Named", "in the order of the implements clause")
			expect.Equal(t, user.Implements[1].Spelling, "Node", "the second interface follows the &")
			expect.Equal(t, user.Annotations, symbol.Annotations{{Name: "key", Args: []string{`fields: "id"`}}},
				"the directive of the type is its annotation")
		})

		t.Run("loads a field with arguments as a method with one result", func(t *testing.T) {
			t.Parallel()

			user, _ := declOf(t, file, "User").(*node.Struct)
			assert.Length(t, user.Methods, 2, "the two fields with arguments are methods")
			friends := user.Methods[1]
			expect.False(t, friends.Abstract, "the method of an object type is not abstract")
			assert.Length(t, friends.Returns, 1, "the method has one result")
			expect.Equal(t, friends.Returns[0].Type.Spelling, "[User!]!", "the result is the type of the field")
			expect.Equal(t, friends.Doc, []string{"The friends of the user."}, "the method has its documentation")
		})

		t.Run("loads a union as a defined alias of the union of its members", func(t *testing.T) {
			t.Parallel()

			member, is := declOf(t, file, "Member").(*node.Alias)
			assert.True(t, is, "the union is an alias")
			expect.True(t, member.Defined, "the alias defines a type of its own")
			assert.NotNil(t, member.Target, "the alias has the union as its target")
			expect.Equal(t, member.Target.Form, symbol.FormUnion, "the target is the union form")
			expect.Equal(t, member.Target.Spelling, "User | Group", "the target spells the members")
			assert.Length(t, member.Target.Elems, 2, "the union has two members")
			expect.Equal(t, member.Target.Elems[1].Pos, position.Pos{File: appFile, Line: 29, Col: 23},
				"a member is at its name")
		})

		t.Run("loads an enum with a variant for each value", func(t *testing.T) {
			t.Parallel()

			role, is := declOf(t, file, "Role").(*node.Enum)
			assert.True(t, is, "the enum is an enum")
			assert.Length(t, role.Variants, 3, "one variant for each value")
			expect.Equal(t, role.Variants[0].Doc, []string{"Reads."}, "a value has its documentation")
			expect.Equal(t, role.Variants[1].Annotations,
				symbol.Annotations{{Name: graphql.DeprecatedDirective, Args: []string{`reason: "Use EDITOR."`}}},
				"a value has its directives")
		})

		t.Run("loads an input object as a struct with the input stamp", func(t *testing.T) {
			t.Parallel()

			filter, is := declOf(t, file, "Filter").(*node.Struct)
			assert.True(t, is, "the input object is a struct")
			expect.Equal(t, stampsOn(gb, filter, graphql.InputKey), []string{"input"},
				"the stamp tells the input object from an object type")
		})
	})

	t.Run("inputFields", func(t *testing.T) {
		t.Parallel()

		t.Run("loads the default of an input field as its value", func(t *testing.T) {
			t.Parallel()

			filter, _ := declOf(t, file, "Filter").(*node.Struct)
			assert.Length(t, filter.Fields, 2, "the input object has its two fields")
			expect.Equal(t, filter.Fields[0].Value, `"a"`, "the default is the value of the field")
			expect.Equal(t, filter.Fields[0].Doc, []string{"The text to find."}, "the field has its documentation")
			expect.Equal(t, filter.Fields[1].Value, "", "a field without a default has no value")
		})
	})

	t.Run("params", func(t *testing.T) {
		t.Parallel()

		t.Run("loads each argument as a parameter with its default, its documentation and its directives",
			func(t *testing.T) {
				t.Parallel()

				user, _ := declOf(t, file, "User").(*node.Struct)
				assert.Length(t, user.Methods[1].Params, 1, "the method has one parameter")
				first := user.Methods[1].Params[0]
				expect.Equal(t, first.Name, "first", "the parameter has the name of the argument")
				expect.Equal(t, first.Default, "10", "the parameter has the default of the argument")
				expect.Equal(t, first.Doc, []string{"The number of friends."}, "the parameter has the description")
				expect.Equal(t, first.Annotations, symbol.Annotations{{Name: graphql.DeprecatedDirective}},
					"the parameter has the directive of the argument")
				expect.Equal(t, first.Type.Form, symbol.FormOptional, "the argument is nullable")
			})
	})

	t.Run("directive", func(t *testing.T) {
		t.Parallel()

		t.Run("loads a directive definition as an interface with a field for each argument", func(t *testing.T) {
			t.Parallel()

			key, is := declOf(t, file, "key").(*node.Interface)
			assert.True(t, is, "the directive definition is an interface")
			expect.Equal(t, key.Doc, []string{"Marks a key.", "Repeat it for each key."},
				"each line of the description is a line of the documentation")
			assert.Length(t, key.Fields, 1, "the directive has one argument")
			fields := key.Fields[0]
			expect.Equal(t, fields.Name, "fields", "the field has the name of the argument")
			expect.Equal(t, fields.Value, `"id"`, "the field has the default of the argument")
			expect.Equal(t, fields.Type.Spelling, "String", "the field has the type of the argument")
			expect.Equal(t, fields.Doc, []string{"The fields of the key."}, "the field has the description")
		})

		t.Run("loads the directives of a directive definition as the annotations of its interface", func(t *testing.T) {
			t.Parallel()

			got, sink := parsedSource(t, "directive @experimental on DIRECTIVE_DEFINITION\n\n"+
				"directive @cached(ttl: Int) @experimental on FIELD_DEFINITION\n")
			assert.Empty(t, codesOf(sink), "a directive may apply to a directive definition")
			cached, is := declOf(t, fileIn(t, got, appPkg, appFile), "cached").(*node.Interface)
			assert.True(t, is, "the directive definition is an interface")
			assert.Equal(t, cached.Annotations, symbol.Annotations{{Name: "experimental"}},
				"the directive of the definition is an annotation of the interface")
		})

		t.Run("stamps the locations of a directive as written", func(t *testing.T) {
			t.Parallel()

			key, _ := declOf(t, file, "key").(*node.Interface)
			expect.Equal(t, stampsOn(gb, key, graphql.DirectiveKey), []string{"OBJECT", "INTERFACE"},
				"the locations are in the order of the definition")
		})

		t.Run("stamps a repeatable directive", func(t *testing.T) {
			t.Parallel()

			key, _ := declOf(t, file, "key").(*node.Interface)
			expect.Equal(t, stampsOn(gb, key, graphql.RepeatableKey), []string{"repeatable"},
				"the stamp marks the directive as repeatable")
		})

		t.Run("stamps a directive that is not repeatable with its locations alone", func(t *testing.T) {
			t.Parallel()

			internal, is := declOf(t, file, "internal").(*node.Interface)
			assert.True(t, is, "the directive definition is an interface")
			expect.Empty(t, stampsOn(gb, internal, graphql.RepeatableKey), "the directive is not repeatable")
			expect.Empty(t, internal.Fields, "the directive has no arguments")
		})
	})

	t.Run("typeSystem", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the definitions of a file in the order of the file", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, declNames(file), []string{
				"Date", "Node", "Named", "User", "Group", "Member", "Role", "Filter", "key", "internal",
			}, "the directives follow the types in the parse, and the file sorts them by position")
		})
	})
}
