// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// typesSource declares a field of each form of a GraphQL type expression.
const typesSource = `type T {
  nullable: Int
  required: Int!
  list: [Int]
  strict: [Int!]!
  nested: [[Int!]]!
}
`

// A type expression loads as the forms of the model, with the expression
// as the spelling of each structural reference, so a backend writes the
// type that the schema wrote.
func TestTyperef(t *testing.T) {
	t.Parallel()

	t.Run("typeRef", func(t *testing.T) {
		t.Parallel()

		gb, sink := parsedSource(t, typesSource)
		assert.Empty(t, codesOf(sink), "the schema loads clean")
		st, is := declOf(t, fileIn(t, gb, appPkg, appFile), "T").(*node.Struct)
		assert.True(t, is, "the object type is a struct")
		assert.Length(t, st.Fields, 5, "one field for each form")
		fields := map[string]*node.TypeRef{}
		for _, f := range st.Fields {
			fields[f.Name] = f.Type
		}

		t.Run("loads a nullable type as the optional form of the type", func(t *testing.T) {
			t.Parallel()

			got := fields["nullable"]
			expect.Equal(t, got.Form, symbol.FormOptional, "a nullable type can be absent")
			expect.Equal(t, got.Spelling, "Int", "the optional form spells the type without the mark")
			assert.Length(t, got.Elems, 1, "the optional form has its type")
			expect.Equal(t, got.Elems[0].Spelling, "Int", "the type is the named type")
			expect.Equal(t, got.Elems[0].Form, symbol.FormNamed, "the named type is a named reference")
		})

		t.Run("loads a non-null type as the type itself", func(t *testing.T) {
			t.Parallel()

			got := fields["required"]
			expect.Equal(t, got.Form, symbol.FormNamed, "a non-null type has no form of its own")
			expect.Equal(t, got.Spelling, "Int", "the named reference has the bare name")
		})

		t.Run("loads a nullable list of nullable elements as optional forms around the list", func(t *testing.T) {
			t.Parallel()

			got := fields["list"]
			expect.Equal(t, got.Form, symbol.FormOptional, "the list can be absent")
			expect.Equal(t, got.Spelling, "[Int]", "the optional form spells the list without the mark")
			list := got.Elems[0]
			expect.Equal(t, list.Form, symbol.FormList, "the list is the list form")
			expect.Equal(t, list.Spelling, "[Int]!", "the list spells its element and its own mark")
			expect.Equal(t, list.Elems[0].Form, symbol.FormOptional, "the element can be null")
		})

		t.Run("loads a non-null list of non-null elements as the list form alone", func(t *testing.T) {
			t.Parallel()

			got := fields["strict"]
			expect.Equal(t, got.Form, symbol.FormList, "the list is not optional")
			expect.Equal(t, got.Spelling, "[Int!]!", "the list spells the marks as written")
			expect.Equal(t, got.Elems[0].Form, symbol.FormNamed, "the element is the named type")
		})

		t.Run("spells a list of a nullable list with the expression of the inner list", func(t *testing.T) {
			t.Parallel()

			got := fields["nested"]
			expect.Equal(t, got.Spelling, "[[Int!]]!", "the outer list spells the inner list without its mark")
			inner := got.Elems[0]
			expect.Equal(t, inner.Form, symbol.FormOptional, "the inner list can be null")
			expect.Equal(t, inner.Spelling, "[Int!]", "the optional form spells the inner list without the mark")
		})

		t.Run("positions a reference at its type in the file", func(t *testing.T) {
			t.Parallel()

			expect.Equal(t, fields["required"].Pos, position.Pos{File: appFile, Line: 3, Col: 13},
				"the reference is at the name of its type")
			expect.Equal(t, fields["list"].Elems[0].Elems[0].Pos, position.Pos{File: appFile, Line: 4, Col: 10},
				"the element of a list is at its name inside the brackets")
		})
	})
}
