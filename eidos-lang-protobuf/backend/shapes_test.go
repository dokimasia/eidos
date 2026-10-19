// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the fields and the types that the shape cases write.
const (
	tagsField    = "tags"
	headersField = "httpHeaders"
	rowType      = "Row"
	int32Wrapper = protobuf.WellKnownPackage + protobuf.NameSep + "Int32Value"
)

// TestShapes pins the wrapper messages through which protobuf writes a
// field of a form that it has no syntax of.
func TestShapes(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name         string
			give         *emit.TypeRef
			want         *emit.TypeRef
			wantWrappers []symbol.Symbol
		}{
			{
				name: "keeps a list of messages",
				give: &emit.TypeRef{
					Spelling: "repeated Row",
					Form:     symbol.FormList,
					Elems:    []*emit.TypeRef{{Spelling: rowType}},
				},
				want: &emit.TypeRef{
					Spelling: "repeated Row",
					Form:     symbol.FormList,
					Elems:    []*emit.TypeRef{{Spelling: rowType}},
				},
			},
			{
				name: "keeps a map from string to a message",
				give: &emit.TypeRef{
					Spelling: "map<string, Row>", Form: symbol.FormMap,
					Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: rowType}},
				},
				want: &emit.TypeRef{
					Spelling: "map<string, Row>", Form: symbol.FormMap,
					Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: rowType}},
				},
			},
			{
				name: "keeps a list without a child, which the writer refuses",
				give: &emit.TypeRef{Spelling: "[]", Form: symbol.FormList},
				want: &emit.TypeRef{Spelling: "[]", Form: symbol.FormList},
			},
			{
				name: "returns a list of a list as a list of the wrapper Item",
				give: &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{
					{
						Spelling: "repeated string",
						Form:     symbol.FormList,
						Elems:    []*emit.TypeRef{{Spelling: stringType}},
					},
				}},
				want: &emit.TypeRef{
					Spelling: "repeated TagsItem",
					Form:     symbol.FormList,
					Elems:    []*emit.TypeRef{{Spelling: "TagsItem"}},
				},
				wantWrappers: []symbol.Symbol{&emit.Struct{Name: "TagsItem", Fields: emit.SlotOf(&emit.Field{
					Name: "values",
					Tags: tag1,
					Type: &emit.TypeRef{
						Spelling: "repeated string",
						Form:     symbol.FormList,
						Elems:    []*emit.TypeRef{{Spelling: stringType}},
					},
				})}},
			},
			{
				name: "returns a list of a list of a list as a list of a wrapper with a wrapper of its own",
				give: &emit.TypeRef{
					Form: symbol.FormList,
					Elems: []*emit.TypeRef{{Form: symbol.FormList, Elems: []*emit.TypeRef{
						{
							Spelling: "repeated string",
							Form:     symbol.FormList,
							Elems:    []*emit.TypeRef{{Spelling: stringType}},
						},
					}}},
				},
				want: &emit.TypeRef{
					Spelling: "repeated TagsItem",
					Form:     symbol.FormList,
					Elems:    []*emit.TypeRef{{Spelling: "TagsItem"}},
				},
				wantWrappers: []symbol.Symbol{&emit.Struct{
					Name: "TagsItem",
					Fields: emit.SlotOf(&emit.Field{
						Name: "values", Tags: tag1,
						Type: &emit.TypeRef{
							Spelling: "repeated ValuesItem",
							Form:     symbol.FormList,
							Elems:    []*emit.TypeRef{{Spelling: "ValuesItem"}},
						},
					}),
					Types: emit.SlotOf[symbol.Symbol](&emit.Struct{Name: "ValuesItem", Fields: emit.SlotOf(&emit.Field{
						Name: "values",
						Tags: tag1,
						Type: &emit.TypeRef{
							Spelling: "repeated string",
							Form:     symbol.FormList,
							Elems:    []*emit.TypeRef{{Spelling: stringType}},
						},
					})}),
				}},
			},
			{
				name: "returns a list of an optional scalar as a list of its well-known wrapper",
				give: &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{
					{Form: symbol.FormOptional, Elems: []*emit.TypeRef{{Spelling: int32Type}}},
				}},
				want: &emit.TypeRef{
					Spelling: "repeated " + int32Wrapper, Form: symbol.FormList,
					Elems: []*emit.TypeRef{{Spelling: int32Wrapper, Package: protobuf.WrappersFile}},
				},
			},
			{
				name: "returns a list of an optional message as a list of the wrapper Item of an optional value",
				give: &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{
					{Spelling: "optional Row", Form: symbol.FormOptional, Elems: []*emit.TypeRef{{Spelling: rowType}}},
				}},
				want: &emit.TypeRef{
					Spelling: "repeated TagsItem",
					Form:     symbol.FormList,
					Elems:    []*emit.TypeRef{{Spelling: "TagsItem"}},
				},
				wantWrappers: []symbol.Symbol{&emit.Struct{Name: "TagsItem", Fields: emit.SlotOf(&emit.Field{
					Name: "value",
					Tags: tag1,
					Type: &emit.TypeRef{
						Spelling: "optional Row",
						Form:     symbol.FormOptional,
						Elems:    []*emit.TypeRef{{Spelling: rowType}},
					},
				})}},
			},
			{
				name: "returns a map to a list as a map to the wrapper Value",
				give: &emit.TypeRef{Form: symbol.FormMap, Elems: []*emit.TypeRef{
					{Spelling: stringType},
					{
						Spelling: "repeated string",
						Form:     symbol.FormList,
						Elems:    []*emit.TypeRef{{Spelling: stringType}},
					},
				}},
				want: &emit.TypeRef{
					Spelling: "map<string, TagsValue>", Form: symbol.FormMap,
					Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: "TagsValue"}},
				},
				wantWrappers: []symbol.Symbol{&emit.Struct{Name: "TagsValue", Fields: emit.SlotOf(&emit.Field{
					Name: "values",
					Tags: tag1,
					Type: &emit.TypeRef{
						Spelling: "repeated string",
						Form:     symbol.FormList,
						Elems:    []*emit.TypeRef{{Spelling: stringType}},
					},
				})}},
			},
			{
				name: "returns a map to an optional scalar as a map to its well-known wrapper",
				give: &emit.TypeRef{Form: symbol.FormMap, Elems: []*emit.TypeRef{
					{Spelling: stringType}, {Form: symbol.FormOptional, Elems: []*emit.TypeRef{{Spelling: int32Type}}},
				}},
				want: &emit.TypeRef{
					Spelling: "map<string, " + int32Wrapper + ">",
					Form:     symbol.FormMap,
					Elems: []*emit.TypeRef{
						{Spelling: stringType},
						{Spelling: int32Wrapper, Package: protobuf.WrappersFile},
					},
				},
			},
			{
				name: "returns a map to an optional message as a map to the wrapper Value of an optional value",
				give: &emit.TypeRef{Form: symbol.FormMap, Elems: []*emit.TypeRef{
					{Spelling: stringType},
					{Spelling: "optional Row", Form: symbol.FormOptional, Elems: []*emit.TypeRef{{Spelling: rowType}}},
				}},
				want: &emit.TypeRef{
					Spelling: "map<string, TagsValue>", Form: symbol.FormMap,
					Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: "TagsValue"}},
				},
				wantWrappers: []symbol.Symbol{&emit.Struct{Name: "TagsValue", Fields: emit.SlotOf(&emit.Field{
					Name: "value",
					Tags: tag1,
					Type: &emit.TypeRef{
						Spelling: "optional Row",
						Form:     symbol.FormOptional,
						Elems:    []*emit.TypeRef{{Spelling: rowType}},
					},
				})}},
			},
			{
				name: "returns a map with the key of a message as a list of the wrapper Entry",
				give: &emit.TypeRef{
					Form:  symbol.FormMap,
					Elems: []*emit.TypeRef{{Spelling: rowType}, {Spelling: int32Type}},
				},
				want: &emit.TypeRef{
					Spelling: "repeated TagsEntry",
					Form:     symbol.FormList,
					Elems:    []*emit.TypeRef{{Spelling: "TagsEntry"}},
				},
				wantWrappers: []symbol.Symbol{&emit.Struct{Name: "TagsEntry", Fields: emit.SlotOf(
					&emit.Field{Name: "key", Type: &emit.TypeRef{Spelling: rowType}, Tags: tag1},
					&emit.Field{Name: "value", Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag2},
				)}},
			},
			{
				name: "returns a map with the key of bytes as a list of the wrapper Entry",
				give: &emit.TypeRef{Form: symbol.FormMap, Elems: []*emit.TypeRef{
					{Spelling: protobuf.TypeBytes},
					{
						Spelling: "repeated string",
						Form:     symbol.FormList,
						Elems:    []*emit.TypeRef{{Spelling: stringType}},
					},
				}},
				want: &emit.TypeRef{
					Spelling: "repeated TagsEntry",
					Form:     symbol.FormList,
					Elems:    []*emit.TypeRef{{Spelling: "TagsEntry"}},
				},
				wantWrappers: []symbol.Symbol{&emit.Struct{Name: "TagsEntry", Fields: emit.SlotOf(
					&emit.Field{Name: "key", Type: &emit.TypeRef{Spelling: protobuf.TypeBytes}, Tags: tag1},
					&emit.Field{
						Name: "value",
						Tags: tag2,
						Type: &emit.TypeRef{
							Spelling: "repeated string",
							Form:     symbol.FormList,
							Elems:    []*emit.TypeRef{{Spelling: stringType}},
						},
					},
				)}},
			},
			{
				name: "returns an entry whose value needs a wrapper with the wrapper nested in the entry",
				give: &emit.TypeRef{Form: symbol.FormMap, Elems: []*emit.TypeRef{
					{Spelling: rowType},
					{Form: symbol.FormList, Elems: []*emit.TypeRef{
						{
							Spelling: "repeated string",
							Form:     symbol.FormList,
							Elems:    []*emit.TypeRef{{Spelling: stringType}},
						},
					}},
				}},
				want: &emit.TypeRef{
					Spelling: "repeated TagsEntry",
					Form:     symbol.FormList,
					Elems:    []*emit.TypeRef{{Spelling: "TagsEntry"}},
				},
				wantWrappers: []symbol.Symbol{&emit.Struct{
					Name: "TagsEntry",
					Fields: emit.SlotOf(
						&emit.Field{Name: "key", Type: &emit.TypeRef{Spelling: rowType}, Tags: tag1},
						&emit.Field{
							Name: "value", Tags: tag2,
							Type: &emit.TypeRef{
								Spelling: "repeated ValueItem",
								Form:     symbol.FormList,
								Elems:    []*emit.TypeRef{{Spelling: "ValueItem"}},
							},
						},
					),
					Types: emit.SlotOf[symbol.Symbol](&emit.Struct{Name: "ValueItem", Fields: emit.SlotOf(&emit.Field{
						Name: "values",
						Tags: tag1,
						Type: &emit.TypeRef{
							Spelling: "repeated string",
							Form:     symbol.FormList,
							Elems:    []*emit.TypeRef{{Spelling: stringType}},
						},
					})}),
				}},
			},
			{
				name: "returns an optional of a list as the list",
				give: &emit.TypeRef{Form: symbol.FormOptional, Elems: []*emit.TypeRef{
					{
						Spelling: "repeated string",
						Form:     symbol.FormList,
						Elems:    []*emit.TypeRef{{Spelling: stringType}},
					},
				}},
				want: &emit.TypeRef{
					Spelling: "repeated string",
					Form:     symbol.FormList,
					Elems:    []*emit.TypeRef{{Spelling: stringType}},
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				field := &emit.Field{Name: tagsField, Type: tt.give, Tags: tag1}
				row := &emit.Struct{Name: rowName, Fields: emit.SlotOf(field)}
				out, err := backend.Lower(row)
				assert.NoError(t, err, "protobuf writes the field")
				assert.Nil(t, out, "the lowering rewrites the message in place")
				assert.Equal(t, field.Type, tt.want, "the type of the field")
				assert.Equal(
					t,
					row.Types.Items(),
					tt.wantWrappers,
					"the wrapper messages of the field",
					assert.EquateEmpty(),
				)
			})
		}

		t.Run("returns a wrapper whose name writes an abbreviation as one word", func(t *testing.T) {
			t.Parallel()

			field := &emit.Field{Name: headersField, Tags: tag1, Type: &emit.TypeRef{
				Form: symbol.FormList,
				Elems: []*emit.TypeRef{
					{
						Spelling: "repeated string",
						Form:     symbol.FormList,
						Elems:    []*emit.TypeRef{{Spelling: stringType}},
					},
				},
			}}
			_, err := backend.Lower(&emit.Struct{Name: rowName, Fields: emit.SlotOf(field)})
			assert.NoError(t, err, "protobuf writes the field")
			assert.Equal(t, field.Type.Spelling, "repeated HttpHeadersItem",
				"the wrapper has the name of the field in Pascal case")
		})

		t.Run("returns a wrapper of a member of a list as a member of the wrapper", func(t *testing.T) {
			t.Parallel()

			member := &emit.Field{
				Name: tagsField,
				Tags: tag1,
				Type: &emit.TypeRef{
					Spelling: "repeated string",
					Form:     symbol.FormList,
					Elems:    []*emit.TypeRef{{Spelling: stringType}},
				},
			}
			row := &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Sum{
				Name: "choice", Variants: emit.SlotOf(&emit.SumVariant{Name: tagsField, Fields: emit.SlotOf(member)}),
			})}
			_, err := backend.Lower(row)
			assert.NoError(t, err, "protobuf writes the member")
			assert.Equal(
				t,
				member.Type,
				&emit.TypeRef{Spelling: "TagsValue"},
				"the member takes the wrapper of the list",
			)
			assert.Equal(
				t,
				row.Types.Items()[1],
				symbol.Symbol(&emit.Struct{Name: "TagsValue", Fields: emit.SlotOf(&emit.Field{
					Name: "values",
					Tags: tag1,
					Type: &emit.TypeRef{
						Spelling: "repeated string",
						Form:     symbol.FormList,
						Elems:    []*emit.TypeRef{{Spelling: stringType}},
					},
				})}),
				"the wrapper of the member is a nested message of the message of the oneof",
			)
		})

		t.Run("returns a wrapper of an extension at file level beside the extension", func(t *testing.T) {
			t.Parallel()

			origin := symbol.Identity{Lang: protobuf.Lang, Package: "acme", Name: tagsField, Kind: symbol.KindField}
			ext := &emit.Field{
				Origin: origin, Name: tagsField, Tags: tag100, Receives: &emit.TypeRef{Spelling: otherName},
				Type: &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{
					{
						Spelling: "repeated string",
						Form:     symbol.FormList,
						Elems:    []*emit.TypeRef{{Spelling: stringType}},
					},
				}},
			}
			out, err := backend.Lower(ext)
			assert.NoError(t, err, "protobuf writes the extension")
			assert.Equal(
				t,
				out,
				[]symbol.Symbol{ext, &emit.Struct{Origin: origin, Name: "TagsItem", Fields: emit.SlotOf(&emit.Field{
					Name: "values",
					Tags: tag1,
					Type: &emit.TypeRef{
						Spelling: "repeated string",
						Form:     symbol.FormList,
						Elems:    []*emit.TypeRef{{Spelling: stringType}},
					},
				})}},
				"the extension and its wrapper, which takes the origin of the extension",
			)
		})
	})
}
