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

// TestRanges pins the reserved and the extensions statements of a message:
// one statement for each run of numbers and of names, the quotes of a name
// in each version, and the options of an extension range.
func TestRanges(t *testing.T) {
	t.Parallel()

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Block", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				head *emit.File
				give symbol.Annotations
				want string
			}{
				{
					name: "returns each run of numbers and of names as one statement in their order",
					head: proto3File,
					give: symbol.Annotations{{Name: string(protobuf.ReservedMarker), Args: []string{
						"2", "15", "foo", "bar", "9 to 11", "40 to max",
					}}},
					want: "  reserved 2, 15;\n  reserved \"foo\", \"bar\";\n  reserved 9 to 11, 40 to max;\n",
				},
				{
					name: "returns a reserved name of proto2 in quotes",
					head: proto2File,
					give: symbol.Annotations{{Name: string(protobuf.ReservedMarker), Args: []string{"foo"}}},
					want: "  reserved \"foo\";\n",
				},
				{
					name: "returns a reserved name of an edition as an identifier",
					head: edition2023File,
					give: symbol.Annotations{{Name: string(protobuf.ReservedMarker), Args: []string{"foo", "bar"}}},
					want: "  reserved foo, bar;\n",
				},
				{
					name: "returns each reserved marker as its own statements",
					head: proto3File,
					give: symbol.Annotations{
						{Name: string(protobuf.ReservedMarker), Args: []string{"1"}},
						{Name: string(protobuf.ReservedMarker), Args: []string{"2"}},
					},
					want: "  reserved 1;\n  reserved 2;\n",
				},
				{
					name: "returns an extensions statement of each marker with its options",
					head: edition2023File,
					give: symbol.Annotations{
						{Name: string(protobuf.ExtensionsMarker), Args: []string{"100 to 199"}},
						{
							Name: string(protobuf.ExtensionsMarker),
							Args: []string{"200", "300 to max", "[verification = UNVERIFIED]"},
						},
					},
					want: "  extensions 100 to 199;\n  extensions 200, 300 to max [verification = UNVERIFIED];\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.NewVocabulary(&render.ImportSet{}).
						Block(tt.head, &emit.Struct{Name: rowName, Annotations: tt.give})
					assert.NoError(t, err, "protobuf writes the ranges")
					assert.Equal(t, got.Items, []backend.Item{{Text: tt.want}}, "the range statements of the message")
				})
			}

			refusals := []struct {
				name string
				head *emit.File
				give symbol.Annotations
				want string
			}{
				{
					name: "returns an error for a reserved name that is empty",
					head: proto3File,
					give: symbol.Annotations{{Name: string(protobuf.ReservedMarker), Args: []string{""}}},
					want: `protobuf: a reserved name is an identifier, and row reserves ""`,
				},
				{
					name: "returns an error for an extension range of proto3",
					head: proto3File,
					give: symbol.Annotations{{Name: string(protobuf.ExtensionsMarker), Args: []string{"100"}}},
					want: "protobuf: proto3 declares no extension range, and row states one",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.NewVocabulary(&render.ImportSet{}).
						Block(tt.head, &emit.Struct{Name: rowName, Annotations: tt.give})
					assert.HasError(t, err, "protobuf does not write the ranges")
					assert.Equal(t, err.Error(), tt.want, "the error states the range")
					assert.Nil(t, got, "a refusal returns no block")
				})
			}
		})
	})
}
