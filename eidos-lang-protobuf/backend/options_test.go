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

// TestOptions pins how an option writes: a statement at the start of a
// block, and the compact options of a member on its line or, two or more of
// them, on lines of their own.
func TestOptions(t *testing.T) {
	t.Parallel()

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Block", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				head *emit.File
				give *emit.Struct
				want []backend.Item
			}{
				{
					name: "returns the options of a message as statements in their order without its markers",
					head: edition2024File,
					give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{
						{Name: "(acme.table)", Args: []string{`{name: "rows" shards: 4}`}},
						localMarker,
						{Name: protobuf.FeaturesOption, Args: []string{"{field_presence: IMPLICIT}"}},
					}},
					want: []backend.Item{
						{
							Text: "  option (acme.table) = {name: \"rows\" shards: 4};\n  option features = {field_presence: IMPLICIT};\n",
						},
					},
				},
				{
					name: "returns the arguments of an option with more than one value joined",
					head: proto3File,
					give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{
						{Name: deprecatedOption, Args: []string{trueValue, "false"}},
					}},
					want: []backend.Item{{Text: "  option deprecated = true, false;\n"}},
				},
				{
					name: "returns one compact option on the line of its field",
					head: proto3File,
					give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
						Name: countField, Type: int32Ref, Tags: tag1,
						Annotations: symbol.Annotations{{Name: deprecatedOption, Args: []string{trueValue}}},
					})},
					want: []backend.Item{{Text: "  int32 count = 1 [deprecated = true];\n"}},
				},
				{
					name: "returns the default, the options and the added features of a field on lines of their own",
					head: edition2023File,
					give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
						Name: countField, Type: optionalRow, Tags: tag1, Value: "{}", Comment: "after the brackets",
						Annotations: symbol.Annotations{
							delimitedMarker, {Name: deprecatedOption, Args: []string{trueValue}},
						},
					})},
					want: []backend.Item{{Text: "  Row count = 1 [\n    default = {},\n    deprecated = true,\n" +
						"    features.message_encoding = DELIMITED\n  ]; // after the brackets\n"}},
				},
				{
					name: "returns the compact options of an extension in a nested block behind its indentation",
					head: proto2File,
					give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Field{
						Name: extName, Type: optionalInt32, Tags: tag100, Receives: &emit.TypeRef{Spelling: otherName},
						Annotations: symbol.Annotations{
							{
								Name: deprecatedOption,
								Args: []string{trueValue},
							},
							{Name: "json_name", Args: []string{`"x"`}},
						},
					})},
					want: []backend.Item{
						{Text: "  extend Other {\n"},
						{
							Text: "    optional int32 ext = 100 [\n      deprecated = true,\n      json_name = \"x\"\n    ];\n",
						},
						{Text: "  }\n"},
					},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.NewVocabulary(&render.ImportSet{}).Block(tt.head, tt.give)
					assert.NoError(t, err, "protobuf writes the options")
					assert.Equal(t, got.Items, tt.want, "the options of the message and its members")
				})
			}
		})
	})
}
