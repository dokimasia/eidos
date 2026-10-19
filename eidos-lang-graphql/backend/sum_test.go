// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// oneOf is the directive of an input object that takes exactly one of its
// fields.
var oneOf = symbol.Annotation{Name: graphql.OneOfDirective}

// TestSum pins the lowering of a sum, which GraphQL writes as a union of
// object types, and under the marker graphql:input as an input object with
// the directive oneOf.
func TestSum(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		outputs := []struct {
			name string
			give *emit.Sum
			want []symbol.Symbol
		}{
			{
				name: "returns a union of an object type for each variant",
				give: &emit.Sum{
					Origin: rowOrigin, Name: "shape", Doc: []string{"shape is one figure."}, Comment: "tagged",
					Annotations: symbol.Annotations{shareable},
					Variants: emit.SlotOf(
						&emit.SumVariant{
							Name:        "circle",
							Doc:         []string{"circle bounds by a radius."},
							Comment:     "round",
							Annotations: symbol.Annotations{deprecated},
							Fields: emit.SlotOf(
								&emit.Field{Name: "radius", Type: &emit.TypeRef{Spelling: intType}},
							),
						},
						&emit.SumVariant{
							Name:   "square",
							Fields: emit.SlotOf(&emit.Field{Name: "side", Type: &emit.TypeRef{Spelling: intType}}),
						},
					),
				},
				want: []symbol.Symbol{
					&emit.Alias{
						Origin: rowOrigin, Name: "shape", Doc: []string{"shape is one figure."}, Comment: "tagged",
						Defined: true, Annotations: symbol.Annotations{shareable},
						Target: &emit.TypeRef{
							Spelling: "ShapeCircle | ShapeSquare", Form: symbol.FormUnion,
							Elems: []*emit.TypeRef{{Spelling: "ShapeCircle"}, {Spelling: "ShapeSquare"}},
						},
					},
					&emit.Struct{
						Origin:      rowOrigin,
						Name:        "ShapeCircle",
						Doc:         []string{"circle bounds by a radius."},
						Comment:     "round",
						Annotations: symbol.Annotations{deprecated},
						Fields:      emit.SlotOf(&emit.Field{Name: "radius", Type: &emit.TypeRef{Spelling: intType}}),
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "ShapeSquare",
						Fields: emit.SlotOf(&emit.Field{Name: "side", Type: &emit.TypeRef{Spelling: intType}}),
					},
				},
			},
			{
				name: "returns the types that the fields of a variant need after the object types",
				give: &emit.Sum{
					Origin: rowOrigin, Name: "shape",
					Variants: emit.SlotOf(&emit.SumVariant{
						Name: "circle",
						Fields: emit.SlotOf(&emit.Field{Name: "tags", Type: &emit.TypeRef{
							Spelling: "map[string]int", Form: symbol.FormMap,
							Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: intType}},
						}}),
					}),
				},
				want: []symbol.Symbol{
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
						Fields: emit.SlotOf(&emit.Field{Name: "tags", Type: &emit.TypeRef{
							Spelling: "ShapeCircleTagsEntry", Form: symbol.FormList,
							Elems: []*emit.TypeRef{{Spelling: "ShapeCircleTagsEntry"}},
						}}),
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "ShapeCircleTagsEntry",
						Fields: emit.SlotOf(
							&emit.Field{Name: "key", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "value", Type: &emit.TypeRef{Spelling: intType}},
						),
					},
				},
			},
			{
				name: "returns a union with the type parameters of a generic sum",
				give: &emit.Sum{
					Origin: rowOrigin, Name: "result", TypeParams: []*emit.TypeParam{{Name: "T"}},
					Variants: emit.SlotOf(&emit.SumVariant{
						Name: "ok", Fields: emit.SlotOf(&emit.Field{Name: "value", Type: &emit.TypeRef{Spelling: "T"}}),
					}),
				},
				want: []symbol.Symbol{
					&emit.Alias{
						Origin: rowOrigin, Name: "result", Defined: true, TypeParams: []*emit.TypeParam{{Name: "T"}},
					},
				},
			},
			{
				name: "returns a union with the type parameters of a generic sum under the marker graphql:input",
				give: &emit.Sum{
					Origin: rowOrigin, Name: "result", TypeParams: []*emit.TypeParam{{Name: "T"}},
					Annotations: symbol.Annotations{inputMarker},
					Variants: emit.SlotOf(&emit.SumVariant{
						Name: "ok", Fields: emit.SlotOf(&emit.Field{Name: "value", Type: &emit.TypeRef{Spelling: "T"}}),
					}),
				},
				want: []symbol.Symbol{
					&emit.Alias{
						Origin: rowOrigin, Name: "result", Defined: true, TypeParams: []*emit.TypeParam{{Name: "T"}},
						Annotations: symbol.Annotations{inputMarker},
					},
				},
			},
			{
				name: "returns an input object with the directive oneOf under the marker graphql:input",
				give: &emit.Sum{
					Origin: rowOrigin, Name: "RowKey", Doc: []string{"RowKey selects one row."}, Comment: "one of",
					Annotations: symbol.Annotations{inputMarker},
					Variants: emit.SlotOf(
						&emit.SumVariant{
							Name: idField,
							Fields: emit.SlotOf(&emit.Field{
								Name: "value", Type: &emit.TypeRef{Spelling: idType}, Doc: []string{"The key."},
								Comment: "opaque", Annotations: symbol.Annotations{deprecated},
							}),
						},
						&emit.SumVariant{
							Name: "slug", Doc: []string{"The slug of a row."}, Comment: "readable",
							Fields: emit.SlotOf(&emit.Field{Name: "text", Type: &emit.TypeRef{Spelling: stringType}}),
						},
						&emit.SumVariant{Name: "latest"},
					),
				},
				want: []symbol.Symbol{
					&emit.Struct{
						Origin: rowOrigin, Name: "RowKey", Doc: []string{"RowKey selects one row."}, Comment: "one of",
						Annotations: symbol.Annotations{inputMarker, oneOf},
						Fields: emit.SlotOf(
							&emit.Field{
								Name: idField, Doc: []string{"The key."}, Comment: "opaque",
								Annotations: symbol.Annotations{deprecated},
								Type: &emit.TypeRef{
									Spelling: idType,
									Form:     symbol.FormOptional,
									Elems:    []*emit.TypeRef{{Spelling: idType}},
								},
							},
							&emit.Field{
								Name: "slug", Doc: []string{"The slug of a row."}, Comment: "readable",
								Type: &emit.TypeRef{
									Spelling: stringType, Form: symbol.FormOptional,
									Elems: []*emit.TypeRef{{Spelling: stringType}},
								},
							},
							&emit.Field{
								Name: "latest",
								Type: &emit.TypeRef{
									Spelling: graphql.BooleanScalar, Form: symbol.FormOptional,
									Elems: []*emit.TypeRef{{Spelling: graphql.BooleanScalar}},
								},
							},
						),
					},
				},
			},
			{
				name: "returns an input object for a variant of two fields under the marker graphql:input",
				give: &emit.Sum{
					Origin: rowOrigin, Name: "RowKey", Annotations: symbol.Annotations{inputMarker},
					Variants: emit.SlotOf(&emit.SumVariant{
						Name: "slug",
						Fields: emit.SlotOf(
							&emit.Field{Name: "text", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "scope", Type: &emit.TypeRef{Spelling: stringType}},
						),
					}),
				},
				want: []symbol.Symbol{
					&emit.Struct{
						Origin: rowOrigin, Name: "RowKey", Annotations: symbol.Annotations{inputMarker, oneOf},
						Fields: emit.SlotOf(&emit.Field{
							Name: "slug",
							Type: &emit.TypeRef{
								Spelling: "RowKeySlugInput", Form: symbol.FormOptional,
								Elems: []*emit.TypeRef{{Spelling: "RowKeySlugInput"}},
							},
						}),
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "RowKeySlugInput", Annotations: symbol.Annotations{inputMarker},
						Fields: emit.SlotOf(
							&emit.Field{Name: "text", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "scope", Type: &emit.TypeRef{Spelling: stringType}},
						),
					},
				},
			},
			{
				name: "returns the field of a variant of one nullable field without a second nullable form",
				give: &emit.Sum{
					Origin: rowOrigin, Name: "RowKey", Annotations: symbol.Annotations{inputMarker},
					Variants: emit.SlotOf(&emit.SumVariant{
						Name: "slug",
						Fields: emit.SlotOf(&emit.Field{Name: "text", Type: &emit.TypeRef{
							Spelling: "String?",
							Form:     symbol.FormOptional,
							Elems:    []*emit.TypeRef{{Spelling: stringType}},
						}}),
					}),
				},
				want: []symbol.Symbol{
					&emit.Struct{
						Origin: rowOrigin, Name: "RowKey", Annotations: symbol.Annotations{inputMarker, oneOf},
						Fields: emit.SlotOf(&emit.Field{Name: "slug", Type: &emit.TypeRef{
							Spelling: "String?",
							Form:     symbol.FormOptional,
							Elems:    []*emit.TypeRef{{Spelling: stringType}},
						}}),
					},
				},
			},
			{
				name: "returns the types that the fields of a variant of two fields need after its input object",
				give: &emit.Sum{
					Origin: rowOrigin, Name: "RowKey", Annotations: symbol.Annotations{inputMarker},
					Variants: emit.SlotOf(&emit.SumVariant{
						Name: "slug",
						Fields: emit.SlotOf(
							&emit.Field{Name: "text", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "tags", Type: &emit.TypeRef{
								Spelling: "map[string]string", Form: symbol.FormMap,
								Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: stringType}},
							}},
						),
					}),
				},
				want: []symbol.Symbol{
					&emit.Struct{
						Origin: rowOrigin, Name: "RowKey", Annotations: symbol.Annotations{inputMarker, oneOf},
						Fields: emit.SlotOf(&emit.Field{
							Name: "slug",
							Type: &emit.TypeRef{
								Spelling: "RowKeySlugInput", Form: symbol.FormOptional,
								Elems: []*emit.TypeRef{{Spelling: "RowKeySlugInput"}},
							},
						}),
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "RowKeySlugInput", Annotations: symbol.Annotations{inputMarker},
						Fields: emit.SlotOf(
							&emit.Field{Name: "text", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "tags", Type: &emit.TypeRef{
								Spelling: "RowKeySlugInputTagsEntryInput", Form: symbol.FormList,
								Elems: []*emit.TypeRef{{Spelling: "RowKeySlugInputTagsEntryInput"}},
							}},
						),
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "RowKeySlugInputTagsEntryInput",
						Annotations: symbol.Annotations{inputMarker},
						Fields: emit.SlotOf(
							&emit.Field{Name: "key", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "value", Type: &emit.TypeRef{Spelling: stringType}},
						),
					},
				},
			},
		}
		for _, tt := range outputs {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Lower(tt.give)
				assert.NoError(t, err, "GraphQL writes the sum")
				assert.Equal(t, got, tt.want, "the declarations of the sum", assert.EquateEmpty())
			})
		}

		refusals := []struct {
			name string
			give *emit.Sum
			want string
		}{
			{
				name: "returns an error for a sum with methods",
				give: &emit.Sum{
					Name: "shape",
					Variants: emit.SlotOf(&emit.SumVariant{
						Name:   "circle",
						Fields: emit.SlotOf(&emit.Field{Name: "radius", Type: &emit.TypeRef{Spelling: intType}}),
					}),
					Methods: emit.SlotOf(&emit.Method{Name: "area"}),
				},
				want: "graphql: a union has no members, and the sum shape states methods",
			},
			{
				name: "returns an error for a sum without a variant",
				give: &emit.Sum{Name: "shape"},
				want: "graphql: a union joins at least one type, and the sum shape states no variant",
			},
			{
				name: "returns an error for a sum with an annotation that is not a GraphQL name",
				give: &emit.Sum{Name: "shape", Annotations: symbol.Annotations{{Name: "not-a-name"}}},
				want: `graphql: the name of a directive is a GraphQL name, and shape states the annotation "not-a-name"`,
			},
			{
				name: "returns an error for a variant without a field",
				give: &emit.Sum{Name: "shape", Variants: emit.SlotOf(&emit.SumVariant{Name: "empty"})},
				want: "graphql: an object type declares at least one field, and the variant empty of shape declares none",
			},
			{
				name: "returns an error for a variant with an annotation that is not a GraphQL name",
				give: &emit.Sum{Name: "shape", Variants: emit.SlotOf(&emit.SumVariant{
					Name: "circle", Annotations: symbol.Annotations{{Name: "not-a-name"}},
					Fields: emit.SlotOf(&emit.Field{Name: "radius", Type: &emit.TypeRef{Spelling: intType}}),
				})},
				want: `graphql: the name of a directive is a GraphQL name, and circle states the annotation "not-a-name"`,
			},
			{
				name: "returns an error for a field of a variant that the lowering refuses",
				give: &emit.Sum{Name: "shape", Variants: emit.SlotOf(&emit.SumVariant{
					Name: "circle", Fields: emit.SlotOf(&emit.Field{Name: "radius"}),
				})},
				want: "graphql: a field of GraphQL has a type, and radius of ShapeCircle states none",
			},
			{
				name: "returns an error for a variant of an input object with an annotation that is not a GraphQL name",
				give: &emit.Sum{
					Name: "RowKey", Annotations: symbol.Annotations{inputMarker},
					Variants: emit.SlotOf(&emit.SumVariant{
						Name: "latest", Annotations: symbol.Annotations{{Name: "not-a-name"}},
					}),
				},
				want: `graphql: the name of a directive is a GraphQL name, and latest states the annotation "not-a-name"`,
			},
			{
				name: "returns an error for a field of a variant of two fields that the lowering refuses",
				give: &emit.Sum{
					Name: "RowKey", Annotations: symbol.Annotations{inputMarker},
					Variants: emit.SlotOf(&emit.SumVariant{
						Name: "slug",
						Fields: emit.SlotOf(
							&emit.Field{Name: "text", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "scope"},
						),
					}),
				},
				want: "graphql: a field of GraphQL has a type, and scope of RowKeySlugInput states none",
			},
			{
				name: "returns an error for a variant whose field the input object refuses",
				give: &emit.Sum{
					Name: "RowKey", Annotations: symbol.Annotations{inputMarker},
					Variants: emit.SlotOf(&emit.SumVariant{
						Name: "page",
						Fields: emit.SlotOf(&emit.Field{Name: "value", Type: &emit.TypeRef{
							Spelling: "Page", Args: []*emit.TypeRef{{Spelling: rowName}},
						}}),
					}),
				},
				want: "graphql: Page states type arguments, and GraphQL has no generic types",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Lower(tt.give)
				assert.HasError(t, err, "GraphQL refuses the sum")
				assert.Equal(t, err.Error(), tt.want, "the error states the reason")
				assert.Nil(t, got, "a refusal returns no declaration")
			})
		}
	})
}
