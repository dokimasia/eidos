// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The allocations of the lowering of one declaration. A declaration that
// GraphQL writes as it is allocates nothing.
const (
	// defaultLowerAllocs is the parse of the default of one input field.
	defaultLowerAllocs = 1 + 1
	// operationLowerAllocs is the source and the parse of gqlparser, which
	// checks the selection set of an operation.
	operationLowerAllocs = 1 + 8
	// sumLowerAllocs is the union of a sum of two variants: its alias, the
	// spelling of its sum and of each variant in Pascal case, the name of
	// each object type, the lists of its declarations, its object types, its
	// members and their references, the spelling of its target and the
	// target.
	sumLowerAllocs = 1 + 1 + 2 + 2 + 4 + 1 + 1
)

// unparsed is a selection set that gqlparser does not parse.
const unparsed = "{ a( }"

// TestLower pins the dispatch of the lowering by kind, the rules of each
// kind, the rewrites that GraphQL needs, and the declarations that it
// synthesizes. The specs of the host, the sum and the annotations pin the
// rules of the members, of a sum and of the markers.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		kept := []struct {
			name string
			give symbol.Symbol
		}{
			{
				name: "returns nil for an object type that GraphQL writes as it is",
				give: &emit.Struct{
					Name:   rowName,
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
			},
			{
				name: "returns nil for a struct with type parameters",
				give: &emit.Struct{Name: rowName, TypeParams: []*emit.TypeParam{{Name: "T"}}},
			},
			{
				name: "returns nil for an interface with type parameters",
				give: &emit.Interface{Name: nodeName, TypeParams: []*emit.TypeParam{{Name: "T"}}},
			},
			{
				name: "returns nil for an alias with type parameters",
				give: &emit.Alias{
					Name: "Page", TypeParams: []*emit.TypeParam{{Name: "T"}}, Target: &emit.TypeRef{Spelling: "T"},
				},
			},
			{
				name: "returns nil for a function with type parameters",
				give: &emit.Function{Name: "GetRow", TypeParams: []*emit.TypeParam{{Name: "T"}}},
			},
			{
				name: "returns nil for a constant",
				give: &emit.Constant{Name: "limit", Value: "8"},
			},
			{
				name: "returns nil for a header with a schema definition",
				give: &emit.File{Annotations: symbol.Annotations{queryRoot}},
			},
			{
				name: "returns nil for an extension of the schema that adds a directive",
				give: &emit.File{Annotations: symbol.Annotations{extendMarker, shareable}},
			},
			{
				name: "returns nil for a header without a statement",
				give: &emit.File{},
			},
			{
				name: "returns nil for an extension of a type that adds a directive alone",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{extendMarker, shareable}},
			},
			{
				name: "returns nil for an extension of a type that adds an interface alone",
				give: &emit.Struct{
					Name: rowName, Annotations: symbol.Annotations{extendMarker},
					Implements: []*emit.TypeRef{{Spelling: nodeName}},
				},
			},
			{
				name: "returns nil for an extension of an interface that adds an interface alone",
				give: &emit.Interface{
					Name: "Named", Annotations: symbol.Annotations{extendMarker},
					Extends: []*emit.TypeRef{{Spelling: nodeName}},
				},
			},
			{
				name: "returns nil for an interface that GraphQL writes as it is",
				give: &emit.Interface{
					Name:   nodeName,
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
			},
			{
				name: "returns nil for an enum",
				give: &emit.Enum{Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN"})},
			},
			{
				name: "returns nil for an extension of an enum that adds a directive alone",
				give: &emit.Enum{Name: phaseName, Annotations: symbol.Annotations{extendMarker, shareable}},
			},
			{
				name: "returns nil for a scalar",
				give: &emit.Alias{Name: "DateTime"},
			},
			{
				name: "returns nil for a union of named references",
				give: &emit.Alias{Name: "SearchResult", Target: &emit.TypeRef{
					Spelling: "Book | Author", Form: symbol.FormUnion,
					Elems: []*emit.TypeRef{{Spelling: "Book"}, {Spelling: "Author"}},
				}},
			},
			{
				name: "returns nil for a field at file level that receives a type",
				give: &emit.Field{
					Name:     "label",
					Receives: &emit.TypeRef{Spelling: rowName},
					Type:     &emit.TypeRef{Spelling: stringType},
				},
			},
			{
				name: "returns nil for a method at file level that receives a type",
				give: &emit.Method{
					Name: "track", Receives: &emit.TypeRef{Spelling: rowName},
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: stringType}}),
				},
			},
			{
				name: "returns nil for a directive definition whose name has the sigil of a directive",
				give: &emit.Interface{Name: "@cached", Annotations: symbol.Annotations{cachedLocations}},
			},
			{
				name: "returns nil for an operation whose selection set has no white space around it",
				give: &emit.Function{
					Name:        "GetRow",
					Annotations: symbol.Annotations{queryOperation},
					Body:        emit.Body{Verbatim: selection},
				},
			},
		}
		for _, tt := range kept {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Lower(tt.give)
				assert.NoError(t, err, "GraphQL writes the declaration")
				assert.Nil(t, got, "the lowering keeps the declaration in place")
			})
		}

		inPlace := []struct {
			name string
			give symbol.Symbol
			want symbol.Symbol
		}{
			{
				name: "returns nil for a directive definition and gives its name the sigil of a directive",
				give: &emit.Interface{
					Name: "cached", Annotations: symbol.Annotations{cachedLocations},
					Fields: emit.SlotOf(&emit.Field{Name: "maxAge", Type: &emit.TypeRef{Spelling: intType}}),
				},
				want: &emit.Interface{
					Name: "@cached", Annotations: symbol.Annotations{cachedLocations},
					Fields: emit.SlotOf(&emit.Field{Name: "maxAge", Type: &emit.TypeRef{Spelling: intType}}),
				},
			},
			{
				name: "returns nil for an operation and removes the white space around its selection set",
				give: &emit.Function{
					Name: "GetRow", Annotations: symbol.Annotations{queryOperation},
					Body: emit.Body{Verbatim: "\n  " + selection + "\n"},
				},
				want: &emit.Function{
					Name:        "GetRow",
					Annotations: symbol.Annotations{queryOperation},
					Body:        emit.Body{Verbatim: selection},
				},
			},
		}
		for _, tt := range inPlace {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Lower(tt.give)
				assert.NoError(t, err, "GraphQL writes the declaration")
				assert.Nil(t, got, "the lowering keeps the declaration in place")
				assert.Equal(t, tt.give, tt.want, "the lowering rewrites the declaration in place")
			})
		}

		outputs := []struct {
			name string
			give symbol.Symbol
			want []symbol.Symbol
		}{
			{
				name: "returns the nested declarations of a type after the type with the origin of the type",
				give: &emit.Struct{
					Origin: rowOrigin, Name: rowName,
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					Types: emit.SlotOf[symbol.Symbol](&emit.Enum{
						Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN"}),
					}),
				},
				want: []symbol.Symbol{
					&emit.Struct{
						Origin: rowOrigin, Name: rowName,
						Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					},
					&emit.Enum{
						Origin:   rowOrigin,
						Name:     phaseName,
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN"}),
					},
				},
			},
			{
				name: "returns the lowered nested declarations of a type after the type",
				give: &emit.Struct{
					Origin: rowOrigin, Name: rowName,
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					Types: emit.SlotOf[symbol.Symbol](&emit.Sum{
						Name: "shape",
						Variants: emit.SlotOf(&emit.SumVariant{
							Name:   "circle",
							Fields: emit.SlotOf(&emit.Field{Name: "radius", Type: &emit.TypeRef{Spelling: intType}}),
						}),
					}),
				},
				want: []symbol.Symbol{
					&emit.Struct{
						Origin: rowOrigin, Name: rowName,
						Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					},
					&emit.Alias{
						Origin: rowOrigin, Name: "shape", Defined: true,
						Target: &emit.TypeRef{
							Spelling: "ShapeCircle",
							Form:     symbol.FormUnion,
							Elems:    []*emit.TypeRef{{Spelling: "ShapeCircle"}},
						},
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "ShapeCircle",
						Fields: emit.SlotOf(&emit.Field{Name: "radius", Type: &emit.TypeRef{Spelling: intType}}),
					},
				},
			},
			{
				name: "returns the nested declarations of an interface after the interface",
				give: &emit.Interface{
					Origin: rowOrigin, Name: nodeName,
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					Types: emit.SlotOf[symbol.Symbol](&emit.Struct{
						Name:   "Edge",
						Fields: emit.SlotOf(&emit.Field{Name: "cursor", Type: &emit.TypeRef{Spelling: stringType}}),
					}),
				},
				want: []symbol.Symbol{
					&emit.Interface{
						Origin: rowOrigin, Name: nodeName,
						Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "Edge",
						Fields: emit.SlotOf(&emit.Field{Name: "cursor", Type: &emit.TypeRef{Spelling: stringType}}),
					},
				},
			},
			{
				name: "returns a nested interface of a type after the type with the origin of the type",
				give: &emit.Struct{
					Origin: rowOrigin, Name: rowName,
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					Types: emit.SlotOf[symbol.Symbol](&emit.Interface{
						Name:   nodeName,
						Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					}),
				},
				want: []symbol.Symbol{
					&emit.Struct{
						Origin: rowOrigin, Name: rowName,
						Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					},
					&emit.Interface{
						Origin: rowOrigin, Name: nodeName,
						Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					},
				},
			},
			{
				name: "returns a nested scalar of a type after the type with the origin of the type",
				give: &emit.Struct{
					Origin: rowOrigin, Name: rowName,
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					Types:  emit.SlotOf[symbol.Symbol](&emit.Alias{Name: "Cursor"}),
				},
				want: []symbol.Symbol{
					&emit.Struct{
						Origin: rowOrigin, Name: rowName,
						Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					},
					&emit.Alias{Origin: rowOrigin, Name: "Cursor"},
				},
			},
			{
				name: "returns the types that the variables of an operation need after the operation",
				give: &emit.Function{
					Origin: rowOrigin, Name: "GetRow", Annotations: symbol.Annotations{queryOperation},
					Params: emit.SlotOf(&emit.Param{Name: "tags", Type: &emit.TypeRef{
						Spelling: "map[string]string", Form: symbol.FormMap,
						Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: stringType}},
					}}),
					Body: emit.Body{Verbatim: selection},
				},
				want: []symbol.Symbol{
					&emit.Function{
						Origin: rowOrigin, Name: "GetRow", Annotations: symbol.Annotations{queryOperation},
						Params: emit.SlotOf(&emit.Param{Name: "tags", Type: &emit.TypeRef{
							Spelling: "GetRowTagsEntryInput", Form: symbol.FormList,
							Elems: []*emit.TypeRef{{Spelling: "GetRowTagsEntryInput"}},
						}}),
						Body: emit.Body{Verbatim: selection},
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "GetRowTagsEntryInput", Annotations: symbol.Annotations{inputMarker},
						Fields: emit.SlotOf(
							&emit.Field{Name: "key", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "value", Type: &emit.TypeRef{Spelling: stringType}},
						),
					},
				},
			},
			{
				name: "returns the types that a field at file level needs after the field",
				give: &emit.Field{
					Origin: rowOrigin, Name: "tags", Receives: &emit.TypeRef{Spelling: rowName},
					Type: &emit.TypeRef{
						Spelling: "map[string]int", Form: symbol.FormMap,
						Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: intType}},
					},
				},
				want: []symbol.Symbol{
					&emit.Field{
						Origin: rowOrigin, Name: "tags", Receives: &emit.TypeRef{Spelling: rowName},
						Type: &emit.TypeRef{
							Spelling: "RowTagsEntry",
							Form:     symbol.FormList,
							Elems:    []*emit.TypeRef{{Spelling: "RowTagsEntry"}},
						},
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "RowTagsEntry",
						Fields: emit.SlotOf(
							&emit.Field{Name: "key", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "value", Type: &emit.TypeRef{Spelling: intType}},
						),
					},
				},
			},
			{
				name: "returns the payload type of a method at file level after the method",
				give: &emit.Method{
					Origin: rowOrigin, Name: "lookup", Receives: &emit.TypeRef{Spelling: rowName},
					Returns: emit.SlotOf(
						&emit.Return{Type: &emit.TypeRef{Spelling: stringType}},
						&emit.Return{Name: "count", Type: &emit.TypeRef{Spelling: intType}},
					),
				},
				want: []symbol.Symbol{
					&emit.Method{
						Origin: rowOrigin, Name: "lookup", Receives: &emit.TypeRef{Spelling: rowName},
						Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: "RowLookupPayload"}}),
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "RowLookupPayload",
						Fields: emit.SlotOf(
							&emit.Field{Name: "result1", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "count", Type: &emit.TypeRef{Spelling: intType}},
						),
					},
				},
			},
		}
		for _, tt := range outputs {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Lower(tt.give)
				assert.NoError(t, err, "GraphQL writes the declaration")
				assert.Equal(t, got, tt.want, "the declarations of the lowering", assert.EquateEmpty())
			})
		}

		_, parseErr := parser.ParseQuery(&ast.Source{Input: unparsed})
		assert.HasError(t, parseErr, "gqlparser does not parse the selection set")
		refusals := []struct {
			name string
			give symbol.Symbol
			want string
		}{
			{
				name: "returns an error for an extension of the schema with documentation",
				give: &emit.File{Doc: []string{"The shop."}, Annotations: symbol.Annotations{extendMarker, queryRoot}},
				want: "graphql: an extension of GraphQL has no description, and the header of the file states documentation",
			},
			{
				name: "returns an error for an extension of the schema without a directive or a root operation type",
				give: &emit.File{Annotations: symbol.Annotations{extendMarker}},
				want: "graphql: an extension of the schema adds a directive or a root operation type, " +
					"and the header of the file states neither",
			},
			{
				name: "returns an error for a schema definition with a description and without a root operation type",
				give: &emit.File{Doc: []string{"The shop."}},
				want: "graphql: a schema definition names a root operation type, " +
					"and the header of the file states a description or a directive without one",
			},
			{
				name: "returns an error for a schema definition with a directive and without a root operation type",
				give: &emit.File{Annotations: symbol.Annotations{shareable}},
				want: "graphql: a schema definition names a root operation type, " +
					"and the header of the file states a description or a directive without one",
			},
			{
				name: "returns an error for a header with an annotation that is not a GraphQL name",
				give: &emit.File{Annotations: symbol.Annotations{{Name: "not-a-name"}}},
				want: `graphql: the name of a directive is a GraphQL name, and the header of the file states ` +
					`the annotation "not-a-name"`,
			},
			{
				name: "returns an error for an input object with the marker graphql:operation",
				give: &emit.Struct{
					Name: "Query", Annotations: symbol.Annotations{inputMarker, queryOperation},
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
				want: "graphql: an input object is no root operation type, and Query states the marker graphql:operation",
			},
			{
				name: "returns an error for an extension of a type with documentation",
				give: &emit.Struct{
					Name: rowName, Doc: []string{"Row is one record."}, Annotations: symbol.Annotations{extendMarker},
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
				want: "graphql: an extension of GraphQL has no description, and Row states documentation",
			},
			{
				name: "returns an error for a type without a field",
				give: &emit.Struct{Name: rowName},
				want: "graphql: a type declares at least one field, and Row declares none",
			},
			{
				name: "returns an error for an extension of a type that adds nothing",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{extendMarker}},
				want: "graphql: a type declares at least one field, and Row declares none",
			},
			{
				name: "returns an error for an input object that implements an interface",
				give: &emit.Struct{
					Name: "RowInput", Annotations: symbol.Annotations{inputMarker},
					Implements: []*emit.TypeRef{{Spelling: nodeName}},
					Fields:     emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
				want: "graphql: an input object implements no interface, and RowInput implements Node",
			},
			{
				name: "returns an error for a type with an annotation that is not a GraphQL name",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{{Name: "not-a-name"}}},
				want: `graphql: the name of a directive is a GraphQL name, and Row states the annotation "not-a-name"`,
			},
			{
				name: "returns an error for a field of a type that the lowering refuses",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: "label"})},
				want: "graphql: a field of GraphQL has a type, and label of Row states none",
			},
			{
				name: "returns an error for a method of a type that the lowering refuses",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{Name: "fetch"})},
				want: "graphql: a field of GraphQL has a type, and the method fetch of Row returns nothing",
			},
			{
				name: "returns an error for a nested declaration of a kind that a type does not nest",
				give: &emit.Struct{
					Name:   rowName,
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					Types:  emit.SlotOf[symbol.Symbol](&emit.Function{Name: "GetRow"}),
				},
				want: "graphql: a type nests types alone, and a type nests a Function",
			},
			{
				name: "returns an error for a nested declaration that the lowering refuses",
				give: &emit.Struct{
					Name:   rowName,
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					Types:  emit.SlotOf[symbol.Symbol](&emit.Enum{Name: phaseName}),
				},
				want: "graphql: an enum declares at least one value, and Phase declares none",
			},
			{
				name: "returns an error for the marker graphql:repeatable without the marker graphql:directive",
				give: &emit.Interface{Name: nodeName, Annotations: symbol.Annotations{repeatable}},
				want: "graphql: the marker graphql:repeatable marks the definition of a directive, " +
					"and Node states no marker graphql:directive",
			},
			{
				name: "returns an error for an extension of an interface with documentation",
				give: &emit.Interface{
					Name:        nodeName,
					Doc:         []string{"Node has an identity."},
					Annotations: symbol.Annotations{extendMarker},
					Fields:      emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
				want: "graphql: an extension of GraphQL has no description, and Node states documentation",
			},
			{
				name: "returns an error for an interface without a field",
				give: &emit.Interface{Name: nodeName},
				want: "graphql: a type declares at least one field, and Node declares none",
			},
			{
				name: "returns an error for an interface with an annotation that is not a GraphQL name",
				give: &emit.Interface{Name: nodeName, Annotations: symbol.Annotations{{Name: "not-a-name"}}},
				want: `graphql: the name of a directive is a GraphQL name, and Node states the annotation "not-a-name"`,
			},
			{
				name: "returns an error for a field of an interface that the lowering refuses",
				give: &emit.Interface{Name: nodeName, Fields: emit.SlotOf(&emit.Field{Name: "label"})},
				want: "graphql: a field of GraphQL has a type, and label of Node states none",
			},
			{
				name: "returns an error for a method of an interface that the lowering refuses",
				give: &emit.Interface{Name: nodeName, Methods: emit.SlotOf(&emit.Method{Name: "peer"})},
				want: "graphql: a field of GraphQL has a type, and the method peer of Node returns nothing",
			},
			{
				name: "returns an error for a nested declaration of an interface that the lowering refuses",
				give: &emit.Interface{
					Name:   nodeName,
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					Types:  emit.SlotOf[symbol.Symbol](&emit.Enum{Name: phaseName}),
				},
				want: "graphql: an enum declares at least one value, and Phase declares none",
			},
			{
				name: "returns an error for an extension of a directive definition",
				give: &emit.Interface{Name: "cached", Annotations: symbol.Annotations{cachedLocations, extendMarker}},
				want: "graphql: a directive definition has no extension, and cached states the marker graphql:extend",
			},
			{
				name: "returns an error for a directive definition with methods",
				give: &emit.Interface{
					Name: "cached", Annotations: symbol.Annotations{cachedLocations},
					Methods: emit.SlotOf(&emit.Method{Name: "peer"}),
				},
				want: "graphql: a directive definition declares its arguments as the fields of cached alone, " +
					"and cached declares methods, interfaces or nested declarations",
			},
			{
				name: "returns an error for a directive definition with interfaces",
				give: &emit.Interface{
					Name: "cached", Annotations: symbol.Annotations{cachedLocations},
					Extends: []*emit.TypeRef{{Spelling: nodeName}},
				},
				want: "graphql: a directive definition declares its arguments as the fields of cached alone, " +
					"and cached declares methods, interfaces or nested declarations",
			},
			{
				name: "returns an error for a directive definition with nested declarations",
				give: &emit.Interface{
					Name: "cached", Annotations: symbol.Annotations{cachedLocations},
					Types: emit.SlotOf[symbol.Symbol](&emit.Enum{Name: phaseName}),
				},
				want: "graphql: a directive definition declares its arguments as the fields of cached alone, " +
					"and cached declares methods, interfaces or nested declarations",
			},
			{
				name: "returns an error for an argument of a directive definition of a map type",
				give: &emit.Interface{
					Name: "cached", Annotations: symbol.Annotations{cachedLocations},
					Fields: emit.SlotOf(&emit.Field{Name: "tags", Type: &emit.TypeRef{
						Spelling: "map[string]string", Form: symbol.FormMap,
						Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: stringType}},
					}}),
				},
				want: "graphql: an argument of a directive is a value of a type that the schema declares, " +
					"and cached takes a map",
			},
			{
				name: "returns an error for an argument of a directive definition that the lowering refuses",
				give: &emit.Interface{
					Name: "@cached", Annotations: symbol.Annotations{cachedLocations},
					Fields: emit.SlotOf(&emit.Field{Name: "maxAge"}),
				},
				want: "graphql: a field of GraphQL has a type, and maxAge of cached states none",
			},
			{
				name: "returns an error for an enum without a value",
				give: &emit.Enum{Name: phaseName},
				want: "graphql: an enum declares at least one value, and Phase declares none",
			},
			{
				name: "returns an error for an extension of an enum without a value or a directive",
				give: &emit.Enum{Name: phaseName, Annotations: symbol.Annotations{extendMarker}},
				want: "graphql: an enum declares at least one value, and Phase declares none",
			},
			{
				name: "returns an error for a value that an enum declares twice",
				give: &emit.Enum{
					Name:     phaseName,
					Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN"}, &emit.EnumVariant{Name: "OPEN"}),
				},
				want: "graphql: a type declares the name of a member once, and Phase declares OPEN twice",
			},
			{
				name: "returns an error for an extension of an enum with documentation",
				give: &emit.Enum{
					Name:        phaseName,
					Doc:         []string{"Phase names a step."},
					Annotations: symbol.Annotations{extendMarker},
					Variants:    emit.SlotOf(&emit.EnumVariant{Name: "PAUSED"}),
				},
				want: "graphql: an extension of GraphQL has no description, and Phase states documentation",
			},
			{
				name: "returns an error for an enum with an annotation that is not a GraphQL name",
				give: &emit.Enum{
					Name: phaseName, Annotations: symbol.Annotations{{Name: "not-a-name"}},
					Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN"}),
				},
				want: `graphql: the name of a directive is a GraphQL name, and Phase states the annotation "not-a-name"`,
			},
			{
				name: "returns an error for a value with an annotation that is not a GraphQL name",
				give: &emit.Enum{
					Name: phaseName,
					Variants: emit.SlotOf(&emit.EnumVariant{
						Name: "OPEN", Annotations: symbol.Annotations{{Name: "not-a-name"}},
					}),
				},
				want: `graphql: the name of a directive is a GraphQL name, and OPEN states the annotation "not-a-name"`,
			},
			{
				name: "returns an error for an alias of a type",
				give: &emit.Alias{Name: "Key", Target: &emit.TypeRef{Spelling: stringType}},
				want: "graphql: GraphQL has no type alias, and Key aliases String",
			},
			{
				name: "returns an error for a union without a member",
				give: &emit.Alias{Name: "SearchResult", Target: &emit.TypeRef{Form: symbol.FormUnion}},
				want: "graphql: a union has at least one member, and SearchResult has none",
			},
			{
				name: "returns an error for a member of a union with type arguments",
				give: &emit.Alias{Name: "SearchResult", Target: &emit.TypeRef{
					Spelling: "Page<Book>", Form: symbol.FormUnion,
					Elems: []*emit.TypeRef{{Spelling: "Page", Args: []*emit.TypeRef{{Spelling: "Book"}}}},
				}},
				want: "graphql: a member of a union is an object type, and SearchResult joins Page",
			},
			{
				name: "returns an error for a member of a union that is not a named reference",
				give: &emit.Alias{Name: "SearchResult", Target: &emit.TypeRef{
					Spelling: "[Book]", Form: symbol.FormUnion,
					Elems: []*emit.TypeRef{{
						Spelling: "[Book]", Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: "Book"}},
					}},
				}},
				want: "graphql: a member of a union is an object type, and SearchResult joins [Book]",
			},
			{
				name: "returns an error for an extension of a scalar with documentation",
				give: &emit.Alias{
					Name: "DateTime", Doc: []string{"DateTime is an instant."},
					Annotations: symbol.Annotations{extendMarker, shareable},
				},
				want: "graphql: an extension of GraphQL has no description, and DateTime states documentation",
			},
			{
				name: "returns an error for an alias with an annotation that is not a GraphQL name",
				give: &emit.Alias{Name: "DateTime", Annotations: symbol.Annotations{{Name: "not-a-name"}}},
				want: `graphql: the name of a directive is a GraphQL name, and DateTime states the annotation "not-a-name"`,
			},
			{
				name: "returns an error for a function without the marker of an operation or of a fragment",
				give: &emit.Function{Name: "GetRow", Body: emit.Body{Verbatim: selection}},
				want: "graphql: a document declares no callable but an operation and a fragment, " +
					"and GetRow states neither the marker graphql:operation nor graphql:fragment",
			},
			{
				name: "returns an error for a function with the markers of an operation and of a fragment",
				give: &emit.Function{
					Name: "GetRow", Annotations: symbol.Annotations{queryOperation, rowFragment},
					Body: emit.Body{Verbatim: selection},
				},
				want: "graphql: a definition is an operation or a fragment, and GetRow states both markers",
			},
			{
				name: "returns an error for an anonymous fragment",
				give: &emit.Function{
					Name: "RowFields", Annotations: symbol.Annotations{rowFragment, anonymous},
					Body: emit.Body{Verbatim: selection},
				},
				want: "graphql: a fragment has a name, and RowFields states the marker graphql:anonymous",
			},
			{
				name: "returns an error for a function with an annotation that is not a GraphQL name",
				give: &emit.Function{
					Name: "GetRow", Annotations: symbol.Annotations{{Name: "not-a-name"}},
					Body: emit.Body{Verbatim: selection},
				},
				want: `graphql: the name of a directive is a GraphQL name, and GetRow states the annotation "not-a-name"`,
			},
			{
				name: "returns an error for a body of statements",
				give: &emit.Function{
					Name: "GetRow", Annotations: symbol.Annotations{queryOperation},
					Body: emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}},
				},
				want: "graphql: the body of an executable definition is its selection set, " +
					"and GetRow states no verbatim body",
			},
			{
				name: "returns an error for a body of two forms",
				give: &emit.Function{
					Name: "GetRow", Annotations: symbol.Annotations{queryOperation},
					Body: emit.Body{Verbatim: selection, Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}},
				},
				want: "graphql: the body of an executable definition is its selection set, " +
					"and GetRow states no verbatim body",
			},
			{
				name: "returns an error for a body with the content of a slot",
				give: &emit.Function{
					Name: "GetRow", Annotations: symbol.Annotations{queryOperation},
					Body: emit.Body{Verbatim: selection, Prologue: emit.SlotOf(emit.Stmt{Kind: emit.StmtReturn})},
				},
				want: "graphql: a selection set is text alone, and the body of GetRow has the content of a slot",
			},
			{
				name: "returns an error for a body that is not a selection set",
				give: &emit.Function{
					Name: "GetRow", Annotations: symbol.Annotations{queryOperation},
					Body: emit.Body{Verbatim: "query " + selection},
				},
				want: "graphql: the body of GetRow is not one selection set",
			},
			{
				name: "returns an error for a selection set that gqlparser does not parse",
				give: &emit.Function{
					Name: "GetRow", Annotations: symbol.Annotations{queryOperation},
					Body: emit.Body{Verbatim: unparsed},
				},
				want: "graphql: the selection set of GetRow does not parse: " + parseErr.Error(),
			},
			{
				name: "returns an error for a body of two selection sets",
				give: &emit.Function{
					Name: "GetRow", Annotations: symbol.Annotations{queryOperation},
					Body: emit.Body{Verbatim: "{ a } { b }"},
				},
				want: "graphql: the body of GetRow is not one selection set",
			},
			{
				name: "returns an error for a body with a fragment",
				give: &emit.Function{
					Name: "GetRow", Annotations: symbol.Annotations{queryOperation},
					Body: emit.Body{Verbatim: "{ a } fragment F on Row { b }"},
				},
				want: "graphql: the body of GetRow is not one selection set",
			},
			{
				name: "returns an error for a variable that the lowering refuses",
				give: &emit.Function{
					Name: "GetRow", Annotations: symbol.Annotations{queryOperation},
					Params: emit.SlotOf(&emit.Param{Type: &emit.TypeRef{Spelling: idType}}),
					Body:   emit.Body{Verbatim: selection},
				},
				want: "graphql: an argument of GraphQL has a name, and a parameter of GetRow states none",
			},
			{
				name: "returns an error for a field at file level that receives no type",
				give: &emit.Field{Name: "label", Type: &emit.TypeRef{Spelling: stringType}},
				want: "graphql: a document declares a field inside a type, and label receives no type",
			},
			{
				name: "returns an error for a method at file level that receives no type",
				give: &emit.Method{
					Name: "track", Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: stringType}}),
				},
				want: "graphql: a document declares a field with arguments inside a type, and track receives no type",
			},
			{
				name: "returns an error for a field at file level that the lowering refuses",
				give: &emit.Field{Name: "label", Receives: &emit.TypeRef{Spelling: rowName}},
				want: "graphql: a field of GraphQL has a type, and label of Row states none",
			},
			{
				name: "returns an error for a method at file level that the lowering refuses",
				give: &emit.Method{Name: "track", Receives: &emit.TypeRef{Spelling: rowName}},
				want: "graphql: a field of GraphQL has a type, and the method track of Row returns nothing",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Lower(tt.give)
				assert.HasError(t, err, "GraphQL refuses the declaration")
				assert.Equal(t, err.Error(), tt.want, "the error states the reason")
				assert.Nil(t, got, "a refusal returns no declaration")
			})
		}
	})
}

// A declaration that GraphQL writes as it is allocates nothing, a default
// allocates its parse, and an operation the parse of its selection set. A
// sum allocates its union and its object types. The ordinary run, which
// runs no benchmark, checks those ceilings here.
func TestLowerAllocs(t *testing.T) {
	checkAllocs(t, lowerCalls())
}

// BenchmarkLower measures the lowering of every declaration of a plan in
// its settle: one case for each kind.
func BenchmarkLower(b *testing.B) {
	benchCalls(b, lowerCalls())
}

// lowerCalls returns a call of Lower for a declaration of each kind. Each
// call lowers its declaration again, which changes nothing after the first
// call.
func lowerCalls() []allocCall {
	row := &emit.Struct{
		Name: rowName,
		Fields: emit.SlotOf(
			&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}},
			&emit.Field{Name: nameField, Type: &emit.TypeRef{Spelling: stringType}, Optional: true},
		),
		Methods: emit.SlotOf(&emit.Method{
			Name:    "peers",
			Params:  emit.SlotOf(&emit.Param{Name: "first", Type: &emit.TypeRef{Spelling: intType}}),
			Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowName}}),
		}),
	}
	page := &emit.Struct{
		Name: "Page", Annotations: symbol.Annotations{inputMarker},
		Fields: emit.SlotOf(&emit.Field{Name: "first", Type: &emit.TypeRef{Spelling: intType}, Value: "10"}),
	}
	node := &emit.Interface{
		Name: nodeName, Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
	}
	phase := &emit.Enum{
		Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN"}, &emit.EnumVariant{Name: "CLOSED"}),
	}
	scalar := &emit.Alias{Name: "DateTime"}
	header := &emit.File{Annotations: symbol.Annotations{queryRoot}}
	query := &emit.Function{
		Name: "GetRow", Annotations: symbol.Annotations{queryOperation}, Body: emit.Body{Verbatim: selection},
	}
	shape := &emit.Sum{
		Name: "shape",
		Variants: emit.SlotOf(
			&emit.SumVariant{
				Name:   "circle",
				Fields: emit.SlotOf(&emit.Field{Name: "radius", Type: &emit.TypeRef{Spelling: intType}}),
			},
			&emit.SumVariant{
				Name: "square", Fields: emit.SlotOf(&emit.Field{Name: "side", Type: &emit.TypeRef{Spelling: intType}}),
			},
		),
	}
	var (
		got []symbol.Symbol
		err error
	)
	kept := func(tb assert.TB) {
		assert.NoError(tb, err, "Lower lowers the declaration")
		assert.Nil(tb, got, "Lower keeps the declaration in place")
	}
	return []allocCall{
		{name: "Lower", caseName: "an object type", call: func() { got, err = backend.Lower(row) }, check: kept},
		{
			name: "Lower", caseName: "an input object with a default", allocs: defaultLowerAllocs,
			call: func() { got, err = backend.Lower(page) }, check: kept,
		},
		{name: "Lower", caseName: "an interface", call: func() { got, err = backend.Lower(node) }, check: kept},
		{name: "Lower", caseName: "an enum", call: func() { got, err = backend.Lower(phase) }, check: kept},
		{name: "Lower", caseName: "a scalar", call: func() { got, err = backend.Lower(scalar) }, check: kept},
		{name: "Lower", caseName: "a header", call: func() { got, err = backend.Lower(header) }, check: kept},
		{
			name: "Lower", caseName: "an operation", allocs: operationLowerAllocs,
			call: func() { got, err = backend.Lower(query) }, check: kept,
		},
		{
			name: "Lower", caseName: "a sum", allocs: sumLowerAllocs,
			call: func() { got, err = backend.Lower(shape) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Lower lowers the sum")
				assert.Length(tb, got, 3, "Lower returns the union and an object type for each variant")
			},
		},
	}
}
