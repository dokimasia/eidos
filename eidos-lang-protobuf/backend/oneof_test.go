// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// choiceName is the name of the oneof of the oneof cases.
const choiceName = "choice"

// The features of the encoding of a message field.
var (
	delimitedEncoding = symbol.Annotation{
		Name: protobuf.FeatureEncoding,
		Args: []string{string(protobuf.EncodingDelimited)},
	}
	prefixedEncoding = symbol.Annotation{
		Name: protobuf.FeatureEncoding, Args: []string{string(protobuf.EncodingLengthPrefixed)},
	}
)

// TestOneof pins the block of a oneof: one member for each variant under
// the name of the variant, without a label or a presence.
func TestOneof(t *testing.T) {
	t.Parallel()

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Block", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				head *emit.File
				give *emit.Sum
				want *backend.Block
			}{
				{
					name: "returns each variant as a member under its name with its options",
					head: proto3File,
					give: &emit.Sum{
						Name: choiceName, Doc: []string{"choice is one of two."}, Comment: "tagged",
						Annotations: symbol.Annotations{{Name: "(acme.required)", Args: []string{trueValue}}},
						Variants: emit.SlotOf(
							&emit.SumVariant{Name: "circle", Fields: emit.SlotOf(&emit.Field{
								Name:        "radius",
								Type:        int32Ref,
								Tags:        tag1,
								Doc:         []string{"circle has a radius."},
								Comment:     "round",
								Annotations: symbol.Annotations{{Name: deprecatedOption, Args: []string{trueValue}}},
							})},
							&emit.SumVariant{
								Name: "row", Doc: []string{"row is a record."}, Comment: "of a store",
								Annotations: symbol.Annotations{{Name: deprecatedOption, Args: []string{trueValue}}},
								Fields: emit.SlotOf(&emit.Field{
									Name: "row",
									Tags: tag2,
									Type: &emit.TypeRef{
										Spelling: "optional Row",
										Form:     symbol.FormOptional,
										Elems:    []*emit.TypeRef{{Spelling: rowType}},
									},
								}),
							},
						),
					},
					want: &backend.Block{
						Doc: []string{"choice is one of two."}, Head: "oneof choice", Open: "{ // tagged\n",
						Items: []backend.Item{
							{Text: "  option (acme.required) = true;\n"},
							{Text: "  // circle has a radius.\n  int32 circle = 1 [deprecated = true]; // round\n"},
							{Text: "  // row is a record.\n  Row row = 2 [deprecated = true]; // of a store\n"},
						},
					},
				},
				{
					name: "returns a delimited member of an edition with the feature message_encoding",
					head: edition2023File,
					give: &emit.Sum{
						Name: choiceName,
						Variants: emit.SlotOf(&emit.SumVariant{Name: "row", Fields: emit.SlotOf(&emit.Field{
							Name:        "row",
							Type:        &emit.TypeRef{Spelling: rowType},
							Tags:        tag1,
							Annotations: symbol.Annotations{delimitedMarker},
						})}),
					},
					want: &backend.Block{Head: "oneof choice", Open: "{\n", Items: []backend.Item{
						{Text: "  Row row = 1 [features.message_encoding = DELIMITED];\n"},
					}},
				},
				{
					name: "returns a member of a message under a delimited file with the feature message_encoding",
					head: &emit.File{
						Annotations: symbol.Annotations{
							{Name: syntaxName, Args: []string{protobuf.Edition2023}},
							delimitedEncoding,
						},
					},
					give: &emit.Sum{
						Name: choiceName,
						Variants: emit.SlotOf(&emit.SumVariant{Name: "row", Fields: emit.SlotOf(&emit.Field{
							Name: "row", Type: &emit.TypeRef{Spelling: rowType}, Tags: tag1,
						})}),
					},
					want: &backend.Block{Head: "oneof choice", Open: "{\n", Items: []backend.Item{
						{Text: "  Row row = 1 [features.message_encoding = LENGTH_PREFIXED];\n"},
					}},
				},
				{
					name: "returns a member of proto2 with its default",
					head: proto2File,
					give: &emit.Sum{
						Name: choiceName,
						Variants: emit.SlotOf(&emit.SumVariant{Name: "count", Fields: emit.SlotOf(&emit.Field{
							Name: "count", Type: int32Ref, Tags: tag1, Value: "8",
						})}),
					},
					want: &backend.Block{Head: "oneof choice", Open: "{\n", Items: []backend.Item{
						{Text: "  int32 count = 1 [default = 8];\n"},
					}},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.NewVocabulary(&render.ImportSet{}).Block(tt.head, tt.give)
					assert.NoError(t, err, "protobuf writes the oneof")
					assert.Equal(t, got, tt.want, "the block of the oneof", assert.EquateEmpty())
				})
			}

			t.Run("returns a delimited member of proto2 as the block of its group", func(t *testing.T) {
				t.Parallel()

				member := &emit.Field{
					Name:        "result",
					Tags:        tag1,
					Annotations: symbol.Annotations{delimitedMarker},
					Type:        &emit.TypeRef{Spelling: resultName},
				}
				group := &emit.Struct{Name: resultName}
				oneof := &emit.Sum{
					Name:     choiceName,
					Variants: emit.SlotOf(&emit.SumVariant{Name: "result", Fields: emit.SlotOf(member)}),
				}
				v := backend.NewVocabulary(&render.ImportSet{})
				_, err := v.Block(
					proto2File,
					&emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](oneof, group)},
				)
				assert.NoError(t, err, "protobuf writes the message of the oneof")
				got, err := v.Block(proto2File, oneof)
				assert.NoError(t, err, "protobuf writes the oneof")
				assert.Length(t, got.Items, 1, "the group is the one member of the oneof")
				assert.Equal(t, got.Items[0].Nested, symbol.Symbol(group), "the member is the message of the group",
					assert.ByIdentity())
				head, err := v.Block(proto2File, group)
				assert.NoError(t, err, "protobuf writes the group")
				assert.Equal(t, head.Head, "group Result = 1", "the group of a member takes no label")
			})

			refusals := []struct {
				name string
				head *emit.File
				give *emit.SumVariant
				want string
			}{
				{
					name: "returns an error for a variant without exactly one field",
					head: proto3File,
					give: &emit.SumVariant{
						Name:   "point",
						Fields: emit.SlotOf(&emit.Field{Name: "x"}, &emit.Field{Name: "y"}),
					},
					want: "protobuf: a member of a oneof is one field, and point has 2",
				},
				{
					name: "returns an error for a member without a type",
					head: proto3File,
					give: &emit.SumVariant{Name: "empty", Fields: emit.SlotOf(&emit.Field{Name: "empty", Tags: tag1})},
					want: "protobuf: the declaration states no type",
				},
				{
					name: "returns an error for a member of a list",
					head: proto3File,
					give: &emit.SumVariant{
						Name:   "ids",
						Fields: emit.SlotOf(&emit.Field{Name: "ids", Type: repeatedInt32, Tags: tag1}),
					},
					want: "protobuf: a member of a oneof is no list and no map, and ids states repeated int32",
				},
				{
					name: "returns an error for a required member",
					head: proto2File,
					give: &emit.SumVariant{Name: "id", Fields: emit.SlotOf(&emit.Field{
						Name: "id", Type: int32Ref, Tags: tag1, Annotations: symbol.Annotations{requiredMarker},
					})},
					want: "protobuf: a member of a oneof is not required, and id states the marker protobuf:required",
				},
				{
					name: "returns an error for a default of a member of proto3",
					head: proto3File,
					give: &emit.SumVariant{
						Name:   "id",
						Fields: emit.SlotOf(&emit.Field{Name: "id", Type: int32Ref, Tags: tag1, Value: "8"}),
					},
					want: "protobuf: proto3 has no default, and id states one",
				},
				{
					name: "returns an error for a feature of a member in proto2",
					head: proto2File,
					give: &emit.SumVariant{Name: "id", Fields: emit.SlotOf(&emit.Field{
						Name: "id", Type: int32Ref, Tags: tag1, Annotations: symbol.Annotations{implicitPresence},
					})},
					want: "protobuf: a declaration states the feature features.field_presence, and proto2 has no features",
				},
				{
					name: "returns an error for a stated encoding that contradicts the marker",
					head: edition2023File,
					give: &emit.SumVariant{Name: "row", Fields: emit.SlotOf(&emit.Field{
						Name: "row", Type: &emit.TypeRef{Spelling: rowType}, Tags: tag1,
						Annotations: symbol.Annotations{delimitedMarker, prefixedEncoding},
					})},
					want: "protobuf: row states the encoding LENGTH_PREFIXED, and the field needs DELIMITED",
				},
				{
					name: "returns an error for a member without a number",
					head: proto3File,
					give: &emit.SumVariant{Name: "id", Fields: emit.SlotOf(&emit.Field{Name: "id", Type: int32Ref})},
					want: "protobuf: a field of a message takes a number, and id states none",
				},
				{
					name: "returns an error for a member of a type with type arguments",
					head: proto3File,
					give: &emit.SumVariant{Name: "page", Fields: emit.SlotOf(&emit.Field{
						Name: "page",
						Tags: tag1,
						Type: &emit.TypeRef{Spelling: "Page", Args: []*emit.TypeRef{int32Ref}},
					})},
					want: "protobuf: Page states 1 type arguments, and protobuf has none",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.NewVocabulary(&render.ImportSet{}).Block(tt.head, &emit.Sum{
						Name: choiceName, Variants: emit.SlotOf(tt.give),
					})
					assert.HasError(t, err, "protobuf does not write the oneof")
					assert.Equal(t, err.Error(), tt.want, "the error states the member")
					assert.Nil(t, got, "a refusal returns no block")
				})
			}

			t.Run("returns an error for a feature of a oneof in proto3", func(t *testing.T) {
				t.Parallel()

				got, err := backend.NewVocabulary(&render.ImportSet{}).Block(proto3File, &emit.Sum{
					Name: choiceName, Annotations: symbol.Annotations{implicitPresence},
				})
				assert.HasError(t, err, "proto3 has no features")
				assert.Equal(t, err.Error(),
					"protobuf: a declaration states the feature features.field_presence, and proto3 has no features",
					"the error states the feature")
				assert.Nil(t, got, "a refusal returns no block")
			})
		})
	})
}
