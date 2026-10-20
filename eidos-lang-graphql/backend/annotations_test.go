// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"github.com/dokimasia/gqlparser/v2/ast"
	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/lang/graphql/internal/value"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestAnnotations pins the rules of the annotations of a declaration: a
// marker of the satellite on a declaration of its kinds with the arguments
// of its contract, once, and an applied directive with a GraphQL name and
// arguments of a name and a constant value.
func TestAnnotations(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		kept := []struct {
			name string
			give symbol.Symbol
		}{
			{
				name: "returns nil for a directive that a declaration applies twice",
				give: &emit.Struct{
					Name: rowName, Annotations: symbol.Annotations{shareable, shareable},
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
			},
			{
				name: "returns nil for a directive with arguments of names and values",
				give: &emit.Struct{
					Name: rowName,
					Annotations: symbol.Annotations{{
						Name: "cost", Args: []string{"weight: 2", `tags: ["a", "b"]`, "options: {limit: 3}"},
					}},
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
			},
			{
				name: "returns nil for a directive definition of every location of GraphQL",
				give: &emit.Interface{Name: "@everywhere", Annotations: symbol.Annotations{{
					Name: string(graphql.DirectiveMarker),
					Args: []string{
						string(ast.LocationQuery), string(ast.LocationMutation), string(ast.LocationSubscription),
						string(ast.LocationField), string(ast.LocationFragmentDefinition),
						string(ast.LocationFragmentSpread), string(ast.LocationInlineFragment),
						string(ast.LocationVariableDefinition), string(ast.LocationSchema), string(ast.LocationScalar),
						string(ast.LocationObject), string(ast.LocationFieldDefinition),
						string(ast.LocationArgumentDefinition), string(ast.LocationInterface),
						string(ast.LocationUnion), string(ast.LocationEnum), string(ast.LocationEnumValue),
						string(ast.LocationInputObject), string(ast.LocationInputFieldDefinition),
						string(ast.LocationDirectiveDefinition),
					},
				}}},
			},
			{
				name: "returns nil for a fragment on a type with a GraphQL name",
				give: &emit.Function{
					Name:        "RowFields",
					Annotations: symbol.Annotations{rowFragment},
					Body:        emit.Body{Verbatim: selection},
				},
			},
			{
				name: "returns nil for an operation of each kind",
				give: &emit.Function{
					Name: "Changed", Annotations: symbol.Annotations{subscriptionOperation},
					Body: emit.Body{Verbatim: selection},
				},
			},
		}
		for _, tt := range kept {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Lower(tt.give)
				assert.NoError(t, err, "GraphQL writes the annotations")
				assert.Nil(t, got, "the lowering keeps the declaration in place")
			})
		}

		_, valueErr := value.Parse("{")
		assert.HasError(t, valueErr, "the argument is not a value of GraphQL")
		refusals := []struct {
			name string
			give symbol.Symbol
			want string
		}{
			{
				name: "returns an error for a marker that the satellite does not declare",
				give: &emit.Struct{
					Name: rowName, Annotations: symbol.Annotations{{Name: graphql.MarkerPrefix + "bogus"}},
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
				want: "graphql: graphql:bogus is no marker of the satellite, and Row states it",
			},
			{
				name: "returns an error for a marker on a declaration of another kind",
				give: &emit.Enum{
					Name: phaseName, Annotations: symbol.Annotations{inputMarker},
					Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN"}),
				},
				want: "graphql: the marker graphql:input marks no Enum, and Phase is one",
			},
			{
				name: "returns an error for an argument of a marker that takes none",
				give: &emit.Struct{
					Name:        rowName,
					Annotations: symbol.Annotations{{Name: string(graphql.InputMarker), Args: []string{"all"}}},
					Fields:      emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
				want: `graphql: the marker graphql:input takes no argument, and Row states ["all"]`,
			},
			{
				name: "returns an error for a marker of an operation of a kind that GraphQL does not declare",
				give: &emit.Function{
					Name: "GetRow", Annotations: symbol.Annotations{{
						Name: string(graphql.OperationMarker), Args: []string{"read"},
					}},
					Body: emit.Body{Verbatim: selection},
				},
				want: `graphql: the marker graphql:operation takes one argument, query, mutation or subscription, ` +
					`and GetRow states ["read"]`,
			},
			{
				name: "returns an error for a marker of an operation without its kind",
				give: &emit.Function{
					Name: "GetRow", Annotations: symbol.Annotations{{Name: string(graphql.OperationMarker)}},
					Body: emit.Body{Verbatim: selection},
				},
				want: `graphql: the marker graphql:operation takes one argument, query, mutation or subscription, ` +
					`and GetRow states []`,
			},
			{
				name: "returns an error for a fragment on a type that is not a GraphQL name",
				give: &emit.Function{
					Name: "RowFields", Annotations: symbol.Annotations{{
						Name: string(graphql.FragmentMarker), Args: []string{"1Row"},
					}},
					Body: emit.Body{Verbatim: selection},
				},
				want: `graphql: the marker graphql:fragment takes one argument, the name of a type, ` +
					`and RowFields states ["1Row"]`,
			},
			{
				name: "returns an error for a fragment on two types",
				give: &emit.Function{
					Name: "RowFields", Annotations: symbol.Annotations{{
						Name: string(graphql.FragmentMarker), Args: []string{rowName, nodeName},
					}},
					Body: emit.Body{Verbatim: selection},
				},
				want: `graphql: the marker graphql:fragment takes one argument, the name of a type, ` +
					`and RowFields states ["Row" "Node"]`,
			},
			{
				name: "returns an error for a location that GraphQL does not declare",
				give: &emit.Interface{Name: "cached", Annotations: symbol.Annotations{{
					Name: string(graphql.DirectiveMarker), Args: []string{string(ast.LocationObject), "ANYWHERE"},
				}}},
				want: `graphql: the marker graphql:directive takes locations of a directive, ` +
					`and cached states ["OBJECT" "ANYWHERE"]`,
			},
			{
				name: "returns an error for a marker that a declaration states twice",
				give: &emit.Struct{
					Name: rowName, Annotations: symbol.Annotations{extendMarker, extendMarker},
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
				want: "graphql: Row states the marker graphql:extend twice",
			},
			{
				name: "returns an error for a directive whose name is not a GraphQL name",
				give: &emit.Struct{
					Name: rowName, Annotations: symbol.Annotations{{Name: "not-a-name"}},
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
				want: `graphql: the name of a directive is a GraphQL name, and Row states the annotation "not-a-name"`,
			},
			{
				name: "returns an error for an argument of a directive without a colon",
				give: &emit.Struct{
					Name: rowName, Annotations: symbol.Annotations{{Name: "key", Args: []string{`"id"`}}},
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
				want: `graphql: an argument of a directive is a name, a colon and a value, ` +
					`and the directive key of Row states "\"id\""`,
			},
			{
				name: "returns an error for an argument of a directive whose name is not a GraphQL name",
				give: &emit.Struct{
					Name: rowName, Annotations: symbol.Annotations{{Name: "key", Args: []string{`key-fields: "id"`}}},
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
				want: `graphql: an argument of a directive is a name, a colon and a value, ` +
					`and the directive key of Row states "key-fields: \"id\""`,
			},
			{
				name: "returns an error for an argument of a directive that is not a value",
				give: &emit.Struct{
					Name: rowName, Annotations: symbol.Annotations{{Name: "key", Args: []string{"fields: {"}}},
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
				},
				want: "graphql: the argument fields of the directive key of Row is no value of GraphQL: " +
					valueErr.Error(),
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Lower(tt.give)
				assert.HasError(t, err, "GraphQL refuses the annotations")
				assert.Equal(t, err.Error(), tt.want, "the error states the reason")
				assert.Nil(t, got, "a refusal returns no declaration")
			})
		}
	})
}
