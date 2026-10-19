// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The allocations of the definitions of the type system: the text that each
// writer returns.
const (
	structAllocs    = 1
	interfaceAllocs = 1
)

// keyDirective is the applied directive of an entity of Apollo Federation,
// with its argument.
var keyDirective = symbol.Annotation{Name: "key", Args: []string{`fields: "id"`}}

// The results of the fields with arguments: one row, and a list of rows.
var (
	rowRef  = &emit.TypeRef{Spelling: rowName}
	rowsRef = &emit.TypeRef{Spelling: "[Row!]!", Form: symbol.FormList, Elems: []*emit.TypeRef{rowRef}}
)

// TestTypes pins the definitions of an object type, an input object, an
// interface and a directive, byte for byte in the layout of graphql-js.
func TestTypes(t *testing.T) {
	t.Parallel()

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Struct", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give *emit.Struct
				want string
			}{
				{
					name: "returns an object type with its description, its fields and its trailing comment",
					give: &emit.Struct{
						Name: rowName, Doc: []string{"Row is one record."}, Comment: "one per fetch",
						Fields: emit.SlotOf(
							&emit.Field{Name: idField, Type: idRef},
							&emit.Field{Name: nameField, Type: stringRef, Optional: true},
						),
					},
					want: "\"Row is one record.\"\ntype Row { # one per fetch\n  id: ID!\n  name: String\n}\n",
				},
				{
					name: "returns an input object under the marker graphql:input",
					give: &emit.Struct{
						Name: "RowFilter", Annotations: symbol.Annotations{inputMarker},
						Fields: emit.SlotOf(&emit.Field{Name: nameField, Type: stringRef, Value: `"a"`}),
					},
					want: "input RowFilter {\n  name: String! = \"a\"\n}\n",
				},
				{
					name: "returns an extension of an object type under the marker graphql:extend",
					give: &emit.Struct{
						Name: rowName, Annotations: symbol.Annotations{extendMarker},
						Fields: emit.SlotOf(&emit.Field{Name: nameField, Type: stringRef}),
					},
					want: "extend type Row {\n  name: String!\n}\n",
				},
				{
					name: "returns an extension of an input object under both markers",
					give: &emit.Struct{
						Name: "RowFilter", Annotations: symbol.Annotations{extendMarker, inputMarker},
						Fields: emit.SlotOf(&emit.Field{Name: nameField, Type: stringRef}),
					},
					want: "extend input RowFilter {\n  name: String!\n}\n",
				},
				{
					name: "returns an extension without a body for a type without members",
					give: &emit.Struct{
						Name: rowName, Comment: "keyed", Annotations: symbol.Annotations{extendMarker, keyDirective},
					},
					want: "extend type Row @key(fields: \"id\") # keyed\n",
				},
				{
					name: "returns an extension that adds an interface without a body",
					give: &emit.Struct{
						Name: rowName, Annotations: symbol.Annotations{extendMarker},
						Implements: []*emit.TypeRef{{Spelling: nodeName}},
					},
					want: "extend type Row implements Node\n",
				},
				{
					name: "returns the interfaces of an object type after implements",
					give: &emit.Struct{
						Name: rowName, Implements: []*emit.TypeRef{{Spelling: nodeName}, {Spelling: "Entity"}},
						Fields: emit.SlotOf(&emit.Field{Name: idField, Type: idRef}),
					},
					want: "type Row implements Node & Entity {\n  id: ID!\n}\n",
				},
				{
					name: "returns the directives of an object type after its interfaces",
					give: &emit.Struct{
						Name: rowName, Implements: []*emit.TypeRef{{Spelling: nodeName}},
						Annotations: symbol.Annotations{keyDirective, shareable},
						Fields:      emit.SlotOf(&emit.Field{Name: idField, Type: idRef}),
					},
					want: "type Row implements Node @key(fields: \"id\") @shareable {\n  id: ID!\n}\n",
				},
				{
					name: "returns a method as a field with arguments after the fields",
					give: &emit.Struct{
						Name:   "Query",
						Fields: emit.SlotOf(&emit.Field{Name: "version", Type: stringRef}),
						Methods: emit.SlotOf(&emit.Method{
							Name: "row", Optional: true,
							Params:  emit.SlotOf(&emit.Param{Name: idField, Type: idRef}),
							Returns: emit.SlotOf(&emit.Return{Type: rowRef}),
						}),
					},
					want: "type Query {\n  version: String!\n  row(id: ID!): Row\n}\n",
				},
				{
					name: "returns the arguments of a field on lines of their own where an argument has a description",
					give: &emit.Struct{
						Name: "Query",
						Methods: emit.SlotOf(&emit.Method{
							Name: "rows",
							Params: emit.SlotOf(
								&emit.Param{Name: idField, Type: idRef, Doc: []string{"The key."}},
								&emit.Param{Name: "first", Type: intRef, Default: "10"},
							),
							Returns: emit.SlotOf(&emit.Return{Type: rowsRef}),
						}),
					},
					want: "type Query {\n  rows(\n    \"The key.\"\n    id: ID!\n    first: Int! = 10\n  ): [Row!]!\n}\n",
				},
				{
					name: "returns the arguments of a field on lines of their own where an argument has a comment",
					give: &emit.Struct{
						Name: "Query",
						Methods: emit.SlotOf(&emit.Method{
							Name: "row", Optional: true,
							Params:  emit.SlotOf(&emit.Param{Name: idField, Type: idRef, Comment: "the key"}),
							Returns: emit.SlotOf(&emit.Return{Type: rowRef}),
						}),
					},
					want: "type Query {\n  row(\n    id: ID! # the key\n  ): Row\n}\n",
				},
				{
					name: "returns the directives of an argument after its default",
					give: &emit.Struct{
						Name: "Query",
						Methods: emit.SlotOf(&emit.Method{
							Name: "rows",
							Params: emit.SlotOf(&emit.Param{
								Name: "first", Type: intRef, Default: "10", Annotations: symbol.Annotations{deprecated},
							}),
							Returns: emit.SlotOf(&emit.Return{Type: rowsRef}),
						}),
					},
					want: "type Query {\n  rows(first: Int! = 10 @deprecated): [Row!]!\n}\n",
				},
				{
					name: "returns the description, the directives and the comment of a method",
					give: &emit.Struct{
						Name: "Query",
						Methods: emit.SlotOf(&emit.Method{
							Name: "row", Doc: []string{"Reads a row."}, Comment: "by key", Optional: true,
							Annotations: symbol.Annotations{deprecated},
							Returns:     emit.SlotOf(&emit.Return{Type: rowRef}),
						}),
					},
					want: "type Query {\n  \"Reads a row.\"\n  row: Row @deprecated # by key\n}\n",
				},
				{
					name: "returns the directives and the comment of a field after its default",
					give: &emit.Struct{
						Name: "Page", Annotations: symbol.Annotations{inputMarker},
						Fields: emit.SlotOf(&emit.Field{
							Name: "first", Type: intRef, Value: "10", Comment: "page size",
							Annotations: symbol.Annotations{deprecated},
						}),
					},
					want: "input Page {\n  first: Int! = 10 @deprecated # page size\n}\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, (&backend.Vocabulary{}).Struct(tt.give), tt.want, "the definition of the struct")
				})
			}
		})

		t.Run("Interface", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give *emit.Interface
				want string
			}{
				{
					name: "returns an interface with the interfaces that it extends",
					give: &emit.Interface{
						Name: nodeName, Extends: []*emit.TypeRef{{Spelling: "Entity"}},
						Fields: emit.SlotOf(&emit.Field{Name: idField, Type: idRef}),
					},
					want: "interface Node implements Entity {\n  id: ID!\n}\n",
				},
				{
					name: "returns the description, the directives, the comment and the methods of an interface",
					give: &emit.Interface{
						Name: nodeName, Doc: []string{"Node has an identity."}, Comment: "relay",
						Annotations: symbol.Annotations{shareable},
						Methods: emit.SlotOf(&emit.Method{
							Name: "peer", Optional: true, Returns: emit.SlotOf(&emit.Return{Type: rowRef}),
						}),
					},
					want: "\"Node has an identity.\"\ninterface Node @shareable { # relay\n  peer: Row\n}\n",
				},
				{
					name: "returns an extension of an interface under the marker graphql:extend",
					give: &emit.Interface{
						Name: nodeName, Annotations: symbol.Annotations{extendMarker},
						Fields: emit.SlotOf(&emit.Field{Name: "label", Type: stringRef}),
					},
					want: "extend interface Node {\n  label: String!\n}\n",
				},
				{
					name: "returns a directive definition under the marker graphql:directive",
					give: &emit.Interface{
						Name: "@cached", Annotations: symbol.Annotations{cachedLocations},
						Fields: emit.SlotOf(&emit.Field{Name: "maxAge", Type: intRef, Value: "60"}),
					},
					want: "directive @cached(maxAge: Int! = 60) on FIELD_DEFINITION | OBJECT\n",
				},
				{
					name: "returns a repeatable directive definition under the marker graphql:repeatable",
					give: &emit.Interface{
						Name: "@tag", Annotations: symbol.Annotations{cachedLocations, repeatable},
						Fields: emit.SlotOf(&emit.Field{Name: nameField, Type: stringRef}),
					},
					want: "directive @tag(name: String!) repeatable on FIELD_DEFINITION | OBJECT\n",
				},
				{
					name: "returns the description and the comment of a directive definition without arguments",
					give: &emit.Interface{
						Name: "@cached", Doc: []string{"Caches a field."}, Comment: "edge",
						Annotations: symbol.Annotations{cachedLocations},
					},
					want: "\"Caches a field.\"\ndirective @cached on FIELD_DEFINITION | OBJECT # edge\n",
				},
				{
					name: "returns the arguments of a directive definition on lines of their own " +
						"where an argument has a description",
					give: &emit.Interface{
						Name: "@cached", Annotations: symbol.Annotations{cachedLocations},
						Fields: emit.SlotOf(&emit.Field{Name: "maxAge", Type: intRef, Doc: []string{"Seconds."}}),
					},
					want: "directive @cached(\n  \"Seconds.\"\n  maxAge: Int!\n) on FIELD_DEFINITION | OBJECT\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, (&backend.Vocabulary{}).Interface(tt.give), tt.want,
						"the definition of the interface")
				})
			}
		})
	})
}

// Struct and Interface allocate the text of the definition, once, because
// the buffer of the vocabulary fits it after the first call. The ordinary
// run, which runs no benchmark, checks those ceilings here.
func TestTypesAllocs(t *testing.T) {
	v := &backend.Vocabulary{}
	row := &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: idField, Type: idRef})}
	node := &emit.Interface{Name: nodeName, Fields: emit.SlotOf(&emit.Field{Name: idField, Type: idRef})}
	var got string
	assert.MaxAllocs(t, func() { got = v.Struct(row) }, structAllocs, "Struct allocates the text of the definition")
	assert.Equal(t, got, "type Row {\n  id: ID!\n}\n", "Struct writes the object type")
	assert.MaxAllocs(t, func() { got = v.Interface(node) }, interfaceAllocs,
		"Interface allocates the text of the definition")
	assert.Equal(t, got, "interface Node {\n  id: ID!\n}\n", "Interface writes the interface")
}

// BenchmarkTypes measures the definitions that every object type and every
// interface of a file write.
func BenchmarkTypes(b *testing.B) {
	b.Run("Vocabulary", func(b *testing.B) {
		b.Run("Struct", func(b *testing.B) {
			v := &backend.Vocabulary{}
			row := &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: idField, Type: idRef})}
			c := bench.Start(b).Warmup(1).MaxAllocs(structAllocs)
			defer c.End()
			var got string
			for c.Loop() {
				got = v.Struct(row)
			}
			assert.Equal(b, got, "type Row {\n  id: ID!\n}\n", "Struct writes the object type")
		})

		b.Run("Interface", func(b *testing.B) {
			v := &backend.Vocabulary{}
			node := &emit.Interface{Name: nodeName, Fields: emit.SlotOf(&emit.Field{Name: idField, Type: idRef})}
			c := bench.Start(b).Warmup(1).MaxAllocs(interfaceAllocs)
			defer c.End()
			var got string
			for c.Loop() {
				got = v.Interface(node)
			}
			assert.Equal(b, got, "interface Node {\n  id: ID!\n}\n", "Interface writes the interface")
		})
	})
}
