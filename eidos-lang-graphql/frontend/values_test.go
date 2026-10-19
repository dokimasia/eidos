// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// valuesSource declares an input field with a default of each kind of a
// GraphQL value, and the types that the defaults name.
const valuesSource = `input Defaults {
  text: String = "a\"b\\c\nd é"
  block: String = """a "block" """
  int: Int = 1
  float: Float = 1.5
  flag: Boolean = true
  none: String = null
  colour: Colour = RED
  list: [Int] = [1, 2]
  object: Point = {x: 1, y: [true]}
}

enum Colour {
  RED
}

input Point {
  x: Int
  y: [Boolean]
}
`

// annotationsSource applies directives without arguments and with them,
// and a variable as the value of an argument.
const annotationsSource = `type Query {
  plain: Int
  old: Int @deprecated
  reasoned: Int @deprecated(reason: "Use plain.")
}

query Q($on: Boolean!) {
  plain @include(if: $on)
}
`

// A value loads in its canonical form, so two schemas that write one
// default alike load the same text, and a backend writes it back as
// GraphQL.
func TestValues(t *testing.T) {
	t.Parallel()

	t.Run("valueText", func(t *testing.T) {
		t.Parallel()

		gb, sink := parsedSource(t, valuesSource)
		assert.Empty(t, codesOf(sink), "the schema loads clean")
		st, is := declOf(t, fileIn(t, gb, appPkg, appFile), "Defaults").(*node.Struct)
		assert.True(t, is, "the input object is a struct")
		values := map[string]string{}
		for _, f := range st.Fields {
			values[f.Name] = f.Value
		}

		tests := []struct {
			name      string
			giveField string
			want      string
		}{
			{name: "writes a string with the escapes of graphql-js", giveField: "text", want: `"a\"b\\c\nd é"`},
			{name: "writes a block string as a string of its value", giveField: "block", want: `"a \"block\" "`},
			{name: "writes an integer as written", giveField: "int", want: "1"},
			{name: "writes a float as written", giveField: "float", want: "1.5"},
			{name: "writes a boolean as written", giveField: "flag", want: "true"},
			{name: "writes null as written", giveField: "none", want: "null"},
			{name: "writes an enum value as written", giveField: "colour", want: "RED"},
			{name: "writes a list with a comma and a space between its items", giveField: "list", want: "[1, 2]"},
			{
				name:      "writes an object with a colon and a space after the name of each field",
				giveField: "object",
				want:      "{x: 1, y: [true]}",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, values[tt.giveField], tt.want, "the default is the value of the field")
			})
		}

		t.Run("writes a braced escape as its character", func(t *testing.T) {
			t.Parallel()

			gb, sink := parsedSource(t, "input Smile {\n  face: String = \"\\u{1F600}\"\n}\n")
			assert.Empty(t, codesOf(sink), "a braced escape is a string of the September 2025 edition")
			smile, _ := declOf(t, fileIn(t, gb, appPkg, appFile), "Smile").(*node.Struct)
			assert.Equal(t, smile.Fields[0].Value, "\"😀\"", "graphql-js writes the character as it is")
		})
	})

	t.Run("annotations", func(t *testing.T) {
		t.Parallel()

		gb, sink := parsedSource(t, annotationsSource)
		assert.Empty(t, codesOf(sink), "the schema and the document load clean")
		query, is := declOf(t, fileIn(t, gb, appPkg, appFile), "Query").(*node.Struct)
		assert.True(t, is, "the object type is a struct")
		assert.Length(t, query.Fields, 3, "the object type has its three fields")

		t.Run("returns no annotation for a declaration without directives", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, query.Fields[0].Annotations, "the field applies no directive")
		})

		t.Run("returns a directive without arguments as an annotation of its name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, query.Fields[1].Annotations, symbol.Annotations{{Name: "deprecated"}},
				"the annotation has the name of the directive and no arguments")
		})

		t.Run("returns each argument as its name, a colon, a space and its value", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, query.Fields[2].Annotations,
				symbol.Annotations{{Name: "deprecated", Args: []string{`reason: "Use plain."`}}},
				"the argument has the canonical form of its value")
		})

		t.Run("returns a variable as its sigil and its name", func(t *testing.T) {
			t.Parallel()

			result, is := declOf(t, fileIn(t, gb, appDocs, appFile), "QQuery").(*node.Struct)
			assert.True(t, is, "the selection of the operation is a struct")
			assert.Length(t, result.Fields, 1, "the selection has one field")
			assert.Equal(t, result.Fields[0].Annotations,
				symbol.Annotations{{Name: "include", Args: []string{"if: $on"}}},
				"the argument of the directive is the variable")
		})
	})
}
