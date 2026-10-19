// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/lang/graphql/internal/value"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// unparsedValue is a default that is not a constant value of GraphQL.
const unparsedValue = "{"

// TestHost pins the rules of the members of a type: the names, the fields,
// the fields with arguments, the arguments, the payloads of two or more
// results, the forms of a type and the entries of a map.
func TestHost(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		kept := []struct {
			name string
			give symbol.Symbol
			want symbol.Symbol
		}{
			{
				name: "returns nil for a type of more than 16 distinct members",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(fields(20)...)},
				want: &emit.Struct{Name: rowName, Fields: emit.SlotOf(fields(20)...)},
			},
			{
				name: "returns nil for a default of a field of an input object",
				give: &emit.Struct{
					Name:        "Page",
					Annotations: symbol.Annotations{inputMarker},
					Fields: emit.SlotOf(
						&emit.Field{Name: "first", Type: &emit.TypeRef{Spelling: intType}, Value: "10"},
					),
				},
				want: &emit.Struct{
					Name:        "Page",
					Annotations: symbol.Annotations{inputMarker},
					Fields: emit.SlotOf(
						&emit.Field{Name: "first", Type: &emit.TypeRef{Spelling: intType}, Value: "10"},
					),
				},
			},
			{
				name: "returns nil for a stream of a field of the root of subscriptions",
				give: &emit.Struct{
					Name: "Subscription", Annotations: symbol.Annotations{subscriptionOperation},
					Fields: emit.SlotOf(&emit.Field{Name: "changes", Type: &emit.TypeRef{
						Spelling: "stream String", Form: symbol.FormStream,
						Elems: []*emit.TypeRef{{Spelling: stringType}},
					}}),
				},
				want: &emit.Struct{
					Name: "Subscription", Annotations: symbol.Annotations{subscriptionOperation},
					Fields: emit.SlotOf(&emit.Field{Name: "changes", Type: &emit.TypeRef{
						Spelling: "stream String", Form: symbol.FormStream,
						Elems: []*emit.TypeRef{{Spelling: stringType}},
					}}),
				},
			},
			{
				name: "returns nil for a positional variadic parameter and makes it a list argument",
				give: &emit.Struct{
					Name: "Query",
					Methods: emit.SlotOf(&emit.Method{
						Name: "rows",
						Params: emit.SlotOf(&emit.Param{
							Name: "ids", Type: &emit.TypeRef{Spelling: idType}, Variadic: symbol.VariadicPositional,
						}),
						Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowName}}),
					}),
				},
				want: &emit.Struct{
					Name: "Query",
					Methods: emit.SlotOf(&emit.Method{
						Name: "rows",
						Params: emit.SlotOf(&emit.Param{Name: "ids", Type: &emit.TypeRef{
							Spelling: idType, Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: idType}},
						}}),
						Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowName}}),
					}),
				},
			},
		}
		for _, tt := range kept {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Lower(tt.give)
				assert.NoError(t, err, "GraphQL writes the type")
				assert.Nil(t, got, "the lowering keeps the type in place")
				assert.Equal(t, tt.give, tt.want, "the members of the type")
			})
		}

		outputs := []struct {
			name string
			give symbol.Symbol
			want []symbol.Symbol
		}{
			{
				name: "returns the payload type of a method of two results after the type",
				give: &emit.Struct{
					Origin: rowOrigin, Name: rowName,
					Methods: emit.SlotOf(&emit.Method{
						Name: "lookup",
						Returns: emit.SlotOf(
							&emit.Return{Type: &emit.TypeRef{Spelling: stringType}, Comment: "the label"},
							&emit.Return{Name: "count", Type: &emit.TypeRef{Spelling: intType}},
						),
					}),
				},
				want: []symbol.Symbol{
					&emit.Struct{
						Origin: rowOrigin, Name: rowName,
						Methods: emit.SlotOf(&emit.Method{
							Name:    "lookup",
							Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: "RowLookupPayload"}}),
						}),
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "RowLookupPayload",
						Fields: emit.SlotOf(
							&emit.Field{
								Name:    "result1",
								Type:    &emit.TypeRef{Spelling: stringType},
								Comment: "the label",
							},
							&emit.Field{Name: "count", Type: &emit.TypeRef{Spelling: intType}},
						),
					},
				},
			},
			{
				name: "returns the types that a result of a payload needs after the payload",
				give: &emit.Struct{
					Origin: rowOrigin, Name: rowName,
					Methods: emit.SlotOf(&emit.Method{
						Name: "lookup",
						Returns: emit.SlotOf(
							&emit.Return{Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Return{Type: &emit.TypeRef{
								Spelling: "map[string]int", Form: symbol.FormMap,
								Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: intType}},
							}},
						),
					}),
				},
				want: []symbol.Symbol{
					&emit.Struct{
						Origin: rowOrigin, Name: rowName,
						Methods: emit.SlotOf(&emit.Method{
							Name:    "lookup",
							Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: "RowLookupPayload"}}),
						}),
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "RowLookupPayload",
						Fields: emit.SlotOf(
							&emit.Field{Name: "result1", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "result2", Type: &emit.TypeRef{
								Spelling: "RowLookupPayloadResult2Entry", Form: symbol.FormList,
								Elems: []*emit.TypeRef{{Spelling: "RowLookupPayloadResult2Entry"}},
							}},
						),
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "RowLookupPayloadResult2Entry",
						Fields: emit.SlotOf(
							&emit.Field{Name: "key", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "value", Type: &emit.TypeRef{Spelling: intType}},
						),
					},
				},
			},
			{
				name: "returns an input type of entries for a map argument after the type",
				give: &emit.Struct{
					Origin: rowOrigin, Name: rowName,
					Methods: emit.SlotOf(&emit.Method{
						Name: "rows",
						Params: emit.SlotOf(&emit.Param{Name: "tags", Type: &emit.TypeRef{
							Spelling: "map[string]string", Form: symbol.FormMap,
							Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: stringType}},
						}}),
						Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowName}}),
					}),
				},
				want: []symbol.Symbol{
					&emit.Struct{
						Origin: rowOrigin, Name: rowName,
						Methods: emit.SlotOf(&emit.Method{
							Name: "rows",
							Params: emit.SlotOf(&emit.Param{Name: "tags", Type: &emit.TypeRef{
								Spelling: "RowRowsTagsEntryInput", Form: symbol.FormList,
								Elems: []*emit.TypeRef{{Spelling: "RowRowsTagsEntryInput"}},
							}}),
							Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowName}}),
						}),
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "RowRowsTagsEntryInput", Annotations: symbol.Annotations{inputMarker},
						Fields: emit.SlotOf(
							&emit.Field{Name: "key", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "value", Type: &emit.TypeRef{Spelling: stringType}},
						),
					},
				},
			},
			{
				name: "returns an input type of entries for a map field of an input object after the type",
				give: &emit.Struct{
					Origin: rowOrigin, Name: "RowFilter", Annotations: symbol.Annotations{inputMarker},
					Fields: emit.SlotOf(&emit.Field{Name: "tags", Type: &emit.TypeRef{
						Spelling: "map[string]string", Form: symbol.FormMap,
						Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: stringType}},
					}}),
				},
				want: []symbol.Symbol{
					&emit.Struct{
						Origin: rowOrigin, Name: "RowFilter", Annotations: symbol.Annotations{inputMarker},
						Fields: emit.SlotOf(&emit.Field{Name: "tags", Type: &emit.TypeRef{
							Spelling: "RowFilterTagsEntryInput", Form: symbol.FormList,
							Elems: []*emit.TypeRef{{Spelling: "RowFilterTagsEntryInput"}},
						}}),
					},
					&emit.Struct{
						Origin:      rowOrigin,
						Name:        "RowFilterTagsEntryInput",
						Annotations: symbol.Annotations{inputMarker},
						Fields: emit.SlotOf(
							&emit.Field{Name: "key", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "value", Type: &emit.TypeRef{Spelling: stringType}},
						),
					},
				},
			},
			{
				name: "returns the type of entries of the values of a map of maps before the outer type of entries",
				give: &emit.Struct{
					Origin: rowOrigin, Name: rowName,
					Fields: emit.SlotOf(&emit.Field{Name: "tags", Type: &emit.TypeRef{
						Spelling: "map[string]map[string]int", Form: symbol.FormMap,
						Elems: []*emit.TypeRef{{Spelling: stringType}, {
							Spelling: "map[string]int", Form: symbol.FormMap,
							Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: intType}},
						}},
					}}),
				},
				want: []symbol.Symbol{
					&emit.Struct{
						Origin: rowOrigin, Name: rowName,
						Fields: emit.SlotOf(&emit.Field{Name: "tags", Type: &emit.TypeRef{
							Spelling: "RowTagsEntry",
							Form:     symbol.FormList,
							Elems:    []*emit.TypeRef{{Spelling: "RowTagsEntry"}},
						}}),
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "RowTagsEntryValueEntry",
						Fields: emit.SlotOf(
							&emit.Field{Name: "key", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "value", Type: &emit.TypeRef{Spelling: intType}},
						),
					},
					&emit.Struct{
						Origin: rowOrigin, Name: "RowTagsEntry",
						Fields: emit.SlotOf(
							&emit.Field{Name: "key", Type: &emit.TypeRef{Spelling: stringType}},
							&emit.Field{Name: "value", Type: &emit.TypeRef{
								Spelling: "RowTagsEntryValueEntry", Form: symbol.FormList,
								Elems: []*emit.TypeRef{{Spelling: "RowTagsEntryValueEntry"}},
							}},
						),
					},
				},
			},
		}
		for _, tt := range outputs {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Lower(tt.give)
				assert.NoError(t, err, "GraphQL writes the type")
				assert.Equal(t, got, tt.want, "the type and the types that its members need", assert.EquateEmpty())
			})
		}

		_, valueErr := value.Parse(unparsedValue)
		assert.HasError(t, valueErr, "the default is not a value of GraphQL")
		refusals := []struct {
			name string
			give symbol.Symbol
			want string
		}{
			{
				name: "returns an error for a field name that a type declares twice",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(
					&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}},
					&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: stringType}},
				)},
				want: "graphql: a type declares the name of a member once, and Row declares id twice",
			},
			{
				name: "returns an error for a method that declares the name of a field",
				give: &emit.Struct{
					Name:   rowName,
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: &emit.TypeRef{Spelling: idType}}),
					Methods: emit.SlotOf(&emit.Method{
						Name: idField, Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: idType}}),
					}),
				},
				want: "graphql: a type declares the name of a member once, and Row declares id twice",
			},
			{
				name: "returns an error for a name after the first 16 members that a type declares twice",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(append(fields(18), fields(18)[17])...)},
				want: "graphql: a type declares the name of a member once, and Row declares f17 twice",
			},
			{
				name: "returns an error for a name of the first 16 members that a type of more members declares again",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(append(fields(18), fields(1)...)...)},
				want: "graphql: a type declares the name of a member once, and Row declares f0 twice",
			},
			{
				name: "returns an error for a field that receives a type inside a type",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name:     "label",
					Receives: &emit.TypeRef{Spelling: rowName},
					Type:     &emit.TypeRef{Spelling: stringType},
				})},
				want: "graphql: an extension of GraphQL is a block of its own, and label receives Row inside Row",
			},
			{
				name: "returns an error for a default of a field of an object type",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name: "first", Type: &emit.TypeRef{Spelling: intType}, Value: "10",
				})},
				want: "graphql: a field of an object type takes no default, and first of Row states 10",
			},
			{
				name: "returns an error for a default of a field that is not a value",
				give: &emit.Struct{
					Name: "Page", Annotations: symbol.Annotations{inputMarker},
					Fields: emit.SlotOf(&emit.Field{
						Name: "first", Type: &emit.TypeRef{Spelling: intType}, Value: unparsedValue,
					}),
				},
				want: "graphql: a default is a value of GraphQL, and the default of first is not: " + valueErr.Error(),
			},
			{
				name: "returns an error for a field with an annotation that is not a GraphQL name",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name:        "label",
					Type:        &emit.TypeRef{Spelling: stringType},
					Annotations: symbol.Annotations{{Name: "not-a-name"}},
				})},
				want: `graphql: the name of a directive is a GraphQL name, and label states the annotation "not-a-name"`,
			},
			{
				name: "returns an error for a method of an input object",
				give: &emit.Struct{
					Name: "RowFilter", Annotations: symbol.Annotations{inputMarker},
					Methods: emit.SlotOf(&emit.Method{
						Name: "match", Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: "Boolean"}}),
					}),
				},
				want: "graphql: an input field takes no arguments, and the input object RowFilter declares the method match",
			},
			{
				name: "returns an error for a method that receives a type inside a type",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{
					Name: "track", Receives: &emit.TypeRef{Spelling: rowName},
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: stringType}}),
				})},
				want: "graphql: an extension of GraphQL is a block of its own, and track receives Row inside Row",
			},
			{
				name: "returns an error for a result without a type",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{
					Name: "fetch", Returns: emit.SlotOf(&emit.Return{}),
				})},
				want: "graphql: a field of GraphQL has a type, and the result of fetch of Row states none",
			},
			{
				name: "returns an error for a result that the lowering refuses",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{
					Name: "fetch", Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{
						Spelling: "Page", Args: []*emit.TypeRef{{Spelling: rowName}},
					}}),
				})},
				want: "graphql: Page states type arguments, and GraphQL has no generic types",
			},
			{
				name: "returns an error for a method with an annotation that is not a GraphQL name",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{
					Name: "fetch", Annotations: symbol.Annotations{{Name: "not-a-name"}},
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: stringType}}),
				})},
				want: `graphql: the name of a directive is a GraphQL name, and fetch states the annotation "not-a-name"`,
			},
			{
				name: "returns an error for a payload of two results of one name",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{
					Name: "lookup",
					Returns: emit.SlotOf(
						&emit.Return{Type: &emit.TypeRef{Spelling: stringType}},
						&emit.Return{Name: "result1", Type: &emit.TypeRef{Spelling: intType}},
					),
				})},
				want: "graphql: a type declares the name of a member once, and RowLookupPayload declares result1 twice",
			},
			{
				name: "returns an error for a parameter without a name",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{
					Name:    "peer",
					Params:  emit.SlotOf(&emit.Param{Type: &emit.TypeRef{Spelling: idType}}),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowName}}),
				})},
				want: "graphql: an argument of GraphQL has a name, and a parameter of peer states none",
			},
			{
				name: "returns an error for a parameter without a type",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{
					Name:    "peer",
					Params:  emit.SlotOf(&emit.Param{Name: idField}),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowName}}),
				})},
				want: "graphql: an argument of GraphQL has a type, and id of peer states none",
			},
			{
				name: "returns an error for a keyword variadic parameter",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{
					Name: "peer",
					Params: emit.SlotOf(&emit.Param{
						Name: "options", Type: &emit.TypeRef{Spelling: stringType}, Variadic: symbol.VariadicKeyword,
					}),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowName}}),
				})},
				want: "graphql: GraphQL has no keyword arguments, and options of peer takes them",
			},
			{
				name: "returns an error for a default of a parameter that is not a value",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{
					Name: "peers",
					Params: emit.SlotOf(&emit.Param{
						Name: "first", Type: &emit.TypeRef{Spelling: intType}, Default: unparsedValue,
					}),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowName}}),
				})},
				want: "graphql: a default is a value of GraphQL, and the default of first is not: " + valueErr.Error(),
			},
			{
				name: "returns an error for a parameter with an annotation that is not a GraphQL name",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{
					Name: "peer",
					Params: emit.SlotOf(&emit.Param{
						Name:        idField,
						Type:        &emit.TypeRef{Spelling: idType},
						Annotations: symbol.Annotations{{Name: "not-a-name"}},
					}),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowName}}),
				})},
				want: `graphql: the name of a directive is a GraphQL name, and id states the annotation "not-a-name"`,
			},
			{
				name: "returns an error for a parameter that the lowering refuses",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{
					Name: "peer",
					Params: emit.SlotOf(&emit.Param{Name: "page", Type: &emit.TypeRef{
						Spelling: "Page", Args: []*emit.TypeRef{{Spelling: rowName}},
					}}),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowName}}),
				})},
				want: "graphql: Page states type arguments, and GraphQL has no generic types",
			},
			{
				name: "returns an error for an optional of an optional",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: nameField, Type: &emit.TypeRef{
					Spelling: "String??", Form: symbol.FormOptional,
					Elems: []*emit.TypeRef{{
						Spelling: "String?", Form: symbol.FormOptional, Elems: []*emit.TypeRef{{Spelling: stringType}},
					}},
				}})},
				want: "graphql: a GraphQL type has one nullability, and String?? is optional twice",
			},
			{
				name: "returns an error for a list of two children",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: "names", Type: &emit.TypeRef{
					Spelling: "[String, Int]", Form: symbol.FormList,
					Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: intType}},
				}})},
				want: "graphql: [String, Int] is a list of 2 children, and the form has one",
			},
			{
				name: "returns an error for a stream outside the root of subscriptions",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: "changes", Type: &emit.TypeRef{
					Spelling: "stream String", Form: symbol.FormStream, Elems: []*emit.TypeRef{{Spelling: stringType}},
				}})},
				want: "graphql: a field streams on the root of subscriptions alone, and stream String of Row is a stream",
			},
			{
				name: "returns an error for a stream inside a list of the root of subscriptions",
				give: &emit.Struct{
					Name: "Subscription", Annotations: symbol.Annotations{subscriptionOperation},
					Fields: emit.SlotOf(&emit.Field{Name: "changes", Type: &emit.TypeRef{
						Spelling: "[stream String]", Form: symbol.FormList,
						Elems: []*emit.TypeRef{
							{
								Spelling: "stream String",
								Form:     symbol.FormStream,
								Elems:    []*emit.TypeRef{{Spelling: stringType}},
							},
						},
					}}),
				},
				want: "graphql: a field streams on the root of subscriptions alone, " +
					"and stream String of Subscription is a stream",
			},
			{
				name: "returns an error for a stream of an argument of the root of subscriptions",
				give: &emit.Struct{
					Name: "Subscription", Annotations: symbol.Annotations{subscriptionOperation},
					Methods: emit.SlotOf(&emit.Method{
						Name: "changes",
						Params: emit.SlotOf(&emit.Param{Name: "since", Type: &emit.TypeRef{
							Spelling: "stream Int",
							Form:     symbol.FormStream,
							Elems:    []*emit.TypeRef{{Spelling: intType}},
						}}),
						Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: stringType}}),
					}),
				},
				want: "graphql: a field streams on the root of subscriptions alone, " +
					"and stream Int of Subscription is a stream",
			},
			{
				name: "returns an error for a stream of a field of an input object",
				give: &emit.Struct{
					Name: "Filter", Annotations: symbol.Annotations{inputMarker},
					Fields: emit.SlotOf(&emit.Field{Name: "changes", Type: &emit.TypeRef{
						Spelling: "stream String",
						Form:     symbol.FormStream,
						Elems:    []*emit.TypeRef{{Spelling: stringType}},
					}}),
				},
				want: "graphql: a field streams on the root of subscriptions alone, " +
					"and stream String of Filter is a stream",
			},
			{
				name: "returns an error for a map of one child",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: "tags", Type: &emit.TypeRef{
					Spelling: "map", Form: symbol.FormMap, Elems: []*emit.TypeRef{{Spelling: stringType}},
				}})},
				want: "graphql: a map has a key and a value, and map has 1 children",
			},
			{
				name: "returns an error for a form that GraphQL does not spell",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: "pair", Type: &emit.TypeRef{
					Spelling: "(int, string)", Form: symbol.FormTuple,
					Elems: []*emit.TypeRef{{Spelling: intType}, {Spelling: stringType}},
				}})},
				want: "graphql: (int, string) is a tuple, which GraphQL does not spell",
			},
			{
				name: "returns an error for an element that the lowering refuses",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: "pages", Type: &emit.TypeRef{
					Spelling: "[Page<Row>]", Form: symbol.FormList,
					Elems: []*emit.TypeRef{{Spelling: "Page", Args: []*emit.TypeRef{{Spelling: rowName}}}},
				}})},
				want: "graphql: Page states type arguments, and GraphQL has no generic types",
			},
			{
				name: "returns an error for a key of a map that the lowering refuses",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: "tags", Type: &emit.TypeRef{
					Spelling: "map[Page<Row>]int", Form: symbol.FormMap,
					Elems: []*emit.TypeRef{
						{Spelling: "Page", Args: []*emit.TypeRef{{Spelling: rowName}}}, {Spelling: intType},
					},
				}})},
				want: "graphql: Page states type arguments, and GraphQL has no generic types",
			},
			{
				name: "returns an error for a value of a map that the lowering refuses",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: "tags", Type: &emit.TypeRef{
					Spelling: "map[string]Page<Row>", Form: symbol.FormMap,
					Elems: []*emit.TypeRef{
						{Spelling: stringType}, {Spelling: "Page", Args: []*emit.TypeRef{{Spelling: rowName}}},
					},
				}})},
				want: "graphql: Page states type arguments, and GraphQL has no generic types",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Lower(tt.give)
				assert.HasError(t, err, "GraphQL refuses the type")
				assert.Equal(t, err.Error(), tt.want, "the error states the reason")
				assert.Nil(t, got, "a refusal returns no declaration")
			})
		}
	})
}

// fields returns n fields of the type Int named f0 to fn-1, which a case of
// a type of many members declares. Each call returns new fields, because the
// lowering writes the type of each field.
func fields(n int) []*emit.Field {
	out := make([]*emit.Field, n)
	for k := range out {
		out[k] = &emit.Field{Name: "f" + strconv.Itoa(k), Type: &emit.TypeRef{Spelling: intType}}
	}
	return out
}
