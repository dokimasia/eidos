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

// The markers and the features of the enum cases.
var (
	closedMarker = symbol.Annotation{Name: string(protobuf.ClosedMarker)}
	openEnum     = symbol.Annotation{Name: protobuf.FeatureEnumType, Args: []string{string(protobuf.EnumOpen)}}
)

// TestEnum pins the block of an enum: its options, its ranges and its values
// as written, and its openness in each version.
func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Block", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				head *emit.File
				give *emit.Enum
				want *backend.Block
			}{
				{
					name: "returns the values of an enum of proto3 with their documentation, options and comments",
					head: proto3File,
					give: &emit.Enum{
						Name: phaseName, Doc: []string{"phase names a step."}, Comment: "open set",
						Annotations: symbol.Annotations{{Name: "allow_alias", Args: []string{trueValue}}},
						Variants: emit.SlotOf(
							&emit.EnumVariant{
								Name:    "OPEN",
								Value:   "0",
								Doc:     []string{"OPEN admits writes."},
								Comment: "first",
							},
							&emit.EnumVariant{
								Name: "STARTED", Value: "0x0",
								Annotations: symbol.Annotations{{Name: deprecatedOption, Args: []string{trueValue}}},
							},
							&emit.EnumVariant{
								Name: "SHUT", Value: "1",
								Annotations: symbol.Annotations{
									{
										Name: deprecatedOption,
										Args: []string{trueValue},
									},
									{Name: "(acme.label)", Args: []string{`"shut"`}},
								},
							},
						),
					},
					want: &backend.Block{
						Doc: []string{"phase names a step."}, Head: "enum phase", Open: "{ // open set\n",
						Items: []backend.Item{
							{Text: "  option allow_alias = true;\n"},
							{Text: "  // OPEN admits writes.\n  OPEN = 0; // first\n"},
							{Text: "  STARTED = 0x0 [deprecated = true];\n"},
							{Text: "  SHUT = 1 [\n    deprecated = true,\n    (acme.label) = \"shut\"\n  ];\n"},
						},
					},
				},
				{
					name: "returns a closed enum of proto2 without a feature",
					head: proto2File,
					give: &emit.Enum{
						Name: phaseName, Annotations: symbol.Annotations{closedMarker},
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "SHUT", Value: "1"}),
					},
					want: &backend.Block{
						Head:  "enum phase",
						Open:  "{\n",
						Items: []backend.Item{{Text: "  SHUT = 1;\n"}},
					},
				},
				{
					name: "returns an open enum of an edition without a feature",
					head: edition2023File,
					give: &emit.Enum{
						Name:     phaseName,
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN", Value: "0"}),
					},
					want: &backend.Block{
						Head:  "enum phase",
						Open:  "{\n",
						Items: []backend.Item{{Text: "  OPEN = 0;\n"}},
					},
				},
				{
					name: "returns a closed enum of an edition with the feature enum_type",
					head: edition2023File,
					give: &emit.Enum{
						Name: phaseName, Annotations: symbol.Annotations{closedMarker},
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "SHUT", Value: "1"}),
					},
					want: &backend.Block{Head: "enum phase", Open: "{\n", Items: []backend.Item{
						{Text: "  option features.enum_type = CLOSED;\n"}, {Text: "  SHUT = 1;\n"},
					}},
				},
				{
					name: "returns a closed enum that states its feature without a second one",
					head: edition2023File,
					give: &emit.Enum{
						Name: phaseName, Annotations: symbol.Annotations{closedMarker, closedEnum},
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "SHUT", Value: "1"}),
					},
					want: &backend.Block{Head: "enum phase", Open: "{\n", Items: []backend.Item{
						{Text: "  option features.enum_type = CLOSED;\n"}, {Text: "  SHUT = 1;\n"},
					}},
				},
				{
					name: "returns an open enum of a file of closed enums with the feature enum_type",
					head: &emit.File{
						Annotations: symbol.Annotations{
							{Name: syntaxName, Args: []string{protobuf.Edition2023}},
							closedEnum,
						},
					},
					give: &emit.Enum{
						Name:     phaseName,
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN", Value: "0"}),
					},
					want: &backend.Block{Head: "enum phase", Open: "{\n", Items: []backend.Item{
						{Text: "  option features.enum_type = OPEN;\n"}, {Text: "  OPEN = 0;\n"},
					}},
				},
				{
					name: "returns the reserved ranges of an enum after its options",
					head: edition2023File,
					give: &emit.Enum{
						Name: phaseName,
						Annotations: symbol.Annotations{
							{Name: string(protobuf.ReservedMarker), Args: []string{"-5 to -1", "OLD"}},
							{Name: deprecatedOption, Args: []string{trueValue}},
						},
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN", Value: "0"}),
					},
					want: &backend.Block{Head: "enum phase", Open: "{\n", Items: []backend.Item{
						{
							Text: "  option deprecated = true;\n  reserved -5 to -1;\n  reserved OLD;\n",
						},
						{Text: "  OPEN = 0;\n"},
					}},
				},
				{
					name: "returns an enum of edition 2024 as local under the marker",
					head: edition2024File,
					give: &emit.Enum{
						Name: phaseName, Annotations: symbol.Annotations{localMarker},
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN", Value: "0"}),
					},
					want: &backend.Block{
						Head:  "local enum phase",
						Open:  "{\n",
						Items: []backend.Item{{Text: "  OPEN = 0;\n"}},
					},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.NewVocabulary(&render.ImportSet{}).Block(tt.head, tt.give)
					assert.NoError(t, err, "protobuf writes the enum")
					assert.Equal(t, got, tt.want, "the block of the enum", assert.EquateEmpty())
				})
			}

			refusals := []struct {
				name string
				head *emit.File
				give *emit.Enum
				want string
			}{
				{
					name: "returns an error for an enum of proto2 without the marker closed",
					head: proto2File,
					give: &emit.Enum{
						Name:     phaseName,
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN", Value: "0"}),
					},
					want: "protobuf: every enum of proto2 is closed, and phase does not state the marker protobuf:closed",
				},
				{
					name: "returns an error for an enum of proto3 with the marker closed",
					head: proto3File,
					give: &emit.Enum{
						Name: phaseName, Annotations: symbol.Annotations{closedMarker},
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN", Value: "0"}),
					},
					want: "protobuf: every enum of proto3 is open, and phase states the marker protobuf:closed",
				},
				{
					name: "returns an error for a stated enum type that contradicts the marker",
					head: edition2023File,
					give: &emit.Enum{
						Name: phaseName, Annotations: symbol.Annotations{closedMarker, openEnum},
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN", Value: "0"}),
					},
					want: "protobuf: phase states the enum type OPEN, and its marker needs CLOSED",
				},
				{
					name: "returns an error for an open enum whose first value is not 0",
					head: proto3File,
					give: &emit.Enum{
						Name:     phaseName,
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN", Value: "1"}),
					},
					want: "protobuf: an open enum starts with the value 0, and phase starts with 1",
				},
				{
					name: "returns an error for an open enum whose first value is not a number",
					head: proto3File,
					give: &emit.Enum{
						Name:     phaseName,
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN", Value: "zero"}),
					},
					want: "protobuf: an enum value is a number in decimal, hexadecimal or octal, and OPEN states zero",
				},
				{
					name: "returns an error for a feature of an enum in proto3",
					head: proto3File,
					give: &emit.Enum{Name: phaseName, Annotations: symbol.Annotations{closedEnum}},
					want: "protobuf: a declaration states the feature features.enum_type, and proto3 has no features",
				},
				{
					name: "returns an error for the marker local in edition 2023",
					head: edition2023File,
					give: &emit.Enum{Name: phaseName, Annotations: symbol.Annotations{localMarker}},
					want: "protobuf: 2023 marks no declaration local, and phase states the marker protobuf:local",
				},
				{
					name: "returns an error for a reserved name that is not an identifier",
					head: proto3File,
					give: &emit.Enum{
						Name: phaseName,
						Annotations: symbol.Annotations{
							{Name: string(protobuf.ReservedMarker), Args: []string{"OLD NAME"}},
						},
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN", Value: "0"}),
					},
					want: `protobuf: a reserved name is an identifier, and phase reserves "OLD NAME"`,
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.NewVocabulary(&render.ImportSet{}).Block(tt.head, tt.give)
					assert.HasError(t, err, "protobuf does not write the enum")
					assert.Equal(t, err.Error(), tt.want, "the error states the reason")
					assert.Nil(t, got, "a refusal returns no block")
				})
			}
		})
	})
}
