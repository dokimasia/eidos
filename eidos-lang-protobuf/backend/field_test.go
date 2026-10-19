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

// countField is the name of the field of the field cases.
const countField = "count"

// The types of the field cases that the shared types of the specs do not
// cover.
var (
	rowRef      = &emit.TypeRef{Spelling: rowType}
	optionalRow = &emit.TypeRef{Spelling: "optional Row", Form: symbol.FormOptional, Elems: []*emit.TypeRef{rowRef}}
	countsMap   = &emit.TypeRef{
		Spelling: "map<string, int32>", Form: symbol.FormMap, Elems: []*emit.TypeRef{stringRef, int32Ref},
	}
	explicitPresence = symbol.Annotation{
		Name: protobuf.FeaturePresence,
		Args: []string{string(protobuf.PresenceExplicit)},
	}
)

// TestField pins how a field writes its label, its presence and its
// encoding in each version, and its default.
func TestField(t *testing.T) {
	t.Parallel()

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Block", func(t *testing.T) {
			t.Parallel()

			implicitFile := &emit.File{Annotations: symbol.Annotations{
				{Name: syntaxName, Args: []string{protobuf.Edition2023}}, implicitPresence,
			}}
			delimitedFile := &emit.File{Annotations: symbol.Annotations{
				{Name: syntaxName, Args: []string{protobuf.Edition2023}}, delimitedEncoding,
			}}
			tests := []struct {
				name string
				head *emit.File
				give *emit.Field
				want string
			}{
				{
					name: "returns an optional of proto2 under the label optional",
					head: proto2File, give: &emit.Field{Name: countField, Type: optionalInt32, Tags: tag1},
					want: "  optional int32 count = 1;\n",
				},
				{
					name: "returns a required field of proto2 under the label required",
					head: proto2File,
					give: &emit.Field{
						Name:        countField,
						Type:        int32Ref,
						Tags:        tag1,
						Annotations: symbol.Annotations{requiredMarker},
					},
					want: "  required int32 count = 1;\n",
				},
				{
					name: "returns a list under the label repeated",
					head: proto2File, give: &emit.Field{Name: countField, Type: repeatedInt32, Tags: tag1},
					want: "  repeated int32 count = 1;\n",
				},
				{
					name: "returns a map without a label",
					head: proto2File, give: &emit.Field{Name: countField, Type: countsMap, Tags: tag1},
					want: "  map<string, int32> count = 1;\n",
				},
				{
					name: "returns a field of proto3 without a label",
					head: proto3File, give: &emit.Field{Name: countField, Type: int32Ref, Tags: tag1},
					want: "  int32 count = 1;\n",
				},
				{
					name: "returns an optional of proto3 under the label optional",
					head: proto3File, give: &emit.Field{Name: countField, Type: optionalInt32, Tags: tag1},
					want: "  optional int32 count = 1;\n",
				},
				{
					name: "returns a field that a value of its message may lack under the label optional",
					head: proto3File, give: &emit.Field{Name: countField, Type: int32Ref, Tags: tag1, Optional: true},
					want: "  optional int32 count = 1;\n",
				},
				{
					name: "returns an optional of an edition without a feature",
					head: edition2023File, give: &emit.Field{Name: countField, Type: optionalInt32, Tags: tag1},
					want: "  int32 count = 1;\n",
				},
				{
					name: "returns a scalar of an edition with implicit presence",
					head: edition2023File, give: &emit.Field{Name: countField, Type: int32Ref, Tags: tag1},
					want: "  int32 count = 1 [features.field_presence = IMPLICIT];\n",
				},
				{
					name: "returns a message of an edition without a feature",
					head: edition2023File, give: &emit.Field{Name: "row", Type: rowRef, Tags: tag1},
					want: "  Row row = 1;\n",
				},
				{
					name: "returns a required field of an edition with legacy required presence",
					head: edition2023File,
					give: &emit.Field{
						Name:        countField,
						Type:        int32Ref,
						Tags:        tag1,
						Annotations: symbol.Annotations{requiredMarker},
					},
					want: "  int32 count = 1 [features.field_presence = LEGACY_REQUIRED];\n",
				},
				{
					name: "returns an optional of a file of implicit presence with explicit presence",
					head: implicitFile, give: &emit.Field{Name: countField, Type: optionalInt32, Tags: tag1},
					want: "  int32 count = 1 [features.field_presence = EXPLICIT];\n",
				},
				{
					name: "returns a scalar of a file of implicit presence without a feature",
					head: implicitFile, give: &emit.Field{Name: countField, Type: int32Ref, Tags: tag1},
					want: "  int32 count = 1;\n",
				},
				{
					name: "returns a field that states the presence that it needs without a second feature",
					head: implicitFile,
					give: &emit.Field{
						Name:        countField,
						Type:        optionalInt32,
						Tags:        tag1,
						Annotations: symbol.Annotations{explicitPresence},
					},
					want: "  int32 count = 1 [features.field_presence = EXPLICIT];\n",
				},
				{
					name: "returns an extension of an edition without a presence",
					head: edition2023File,
					give: &emit.Field{
						Name:     countField,
						Type:     int32Ref,
						Tags:     tag1,
						Receives: &emit.TypeRef{Spelling: otherName},
					},
					want: "  int32 count = 1;\n",
				},
				{
					name: "returns an optional extension of a file of implicit presence without a presence",
					head: implicitFile,
					give: &emit.Field{
						Name:     countField,
						Type:     optionalInt32,
						Tags:     tag1,
						Receives: &emit.TypeRef{Spelling: otherName},
					},
					want: "  int32 count = 1;\n",
				},
				{
					name: "returns a delimited message of an edition with the encoding DELIMITED",
					head: edition2023File,
					give: &emit.Field{
						Name:        "row",
						Type:        optionalRow,
						Tags:        tag1,
						Annotations: symbol.Annotations{delimitedMarker},
					},
					want: "  Row row = 1 [features.message_encoding = DELIMITED];\n",
				},
				{
					name: "returns a message of a delimited file without the marker with the encoding LENGTH_PREFIXED",
					head: delimitedFile, give: &emit.Field{Name: "row", Type: optionalRow, Tags: tag1},
					want: "  Row row = 1 [features.message_encoding = LENGTH_PREFIXED];\n",
				},
				{
					name: "returns a scalar of a delimited file without an encoding",
					head: delimitedFile, give: &emit.Field{Name: countField, Type: optionalInt32, Tags: tag1},
					want: "  int32 count = 1;\n",
				},
				{
					name: "returns the default of a field of proto2 as its first option",
					head: proto2File,
					give: &emit.Field{
						Name: countField, Type: optionalInt32, Tags: tag1, Value: "8",
						Annotations: symbol.Annotations{{Name: "json_name", Args: []string{`"total"`}}},
					},
					want: "  optional int32 count = 1 [\n    default = 8,\n    json_name = \"total\"\n  ];\n",
				},
				{
					name: "returns the default of an optional of an edition",
					head: edition2023File,
					give: &emit.Field{Name: countField, Type: optionalInt32, Tags: tag1, Value: "8"},
					want: "  int32 count = 1 [default = 8];\n",
				},
				{
					name: "returns the documentation and the trailing comment of a field",
					head: proto3File,
					give: &emit.Field{
						Name:    countField,
						Type:    int32Ref,
						Tags:    tag1,
						Doc:     []string{"count is the total."},
						Comment: "of rows",
					},
					want: "  // count is the total.\n  int32 count = 1; // of rows\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.NewVocabulary(&render.ImportSet{}).Block(tt.head, &emit.Struct{
						Name: rowName, Fields: emit.SlotOf(tt.give),
					})
					assert.NoError(t, err, "protobuf writes the field")
					assert.Equal(t, got.Items, []backend.Item{{Text: tt.want}}, "the field of the message")
				})
			}

			refusals := []struct {
				name string
				head *emit.File
				give *emit.Field
				want string
			}{
				{
					name: "returns an error for a field without a type",
					head: proto3File, give: &emit.Field{Name: countField, Tags: tag1},
					want: "protobuf: the declaration states no type",
				},
				{
					name: "returns an error for a field of a stream",
					head: proto3File,
					give: &emit.Field{Name: countField, Tags: tag1, Type: &emit.TypeRef{
						Spelling: "stream int32", Form: symbol.FormStream, Elems: []*emit.TypeRef{int32Ref},
					}},
					want: "protobuf: a field of a message is not a stream, and count states one",
				},
				{
					name: "returns an error for a required list",
					head: proto2File,
					give: &emit.Field{
						Name:        countField,
						Type:        repeatedInt32,
						Tags:        tag1,
						Annotations: symbol.Annotations{requiredMarker},
					},
					want: "protobuf: a required field is singular and present, and count is not",
				},
				{
					name: "returns an error for a required optional",
					head: proto2File,
					give: &emit.Field{
						Name:        countField,
						Type:        optionalInt32,
						Tags:        tag1,
						Annotations: symbol.Annotations{requiredMarker},
					},
					want: "protobuf: a required field is singular and present, and count is not",
				},
				{
					name: "returns an error for a required extension",
					head: proto2File,
					give: &emit.Field{
						Name: countField, Type: int32Ref, Tags: tag1, Receives: &emit.TypeRef{Spelling: otherName},
						Annotations: symbol.Annotations{requiredMarker},
					},
					want: "protobuf: an extension is not required, and count states the marker protobuf:required",
				},
				{
					name: "returns an error for a required field of proto3",
					head: proto3File,
					give: &emit.Field{
						Name:        countField,
						Type:        int32Ref,
						Tags:        tag1,
						Annotations: symbol.Annotations{requiredMarker},
					},
					want: "protobuf: proto3 has no required field, and count states the marker protobuf:required",
				},
				{
					name: "returns an error for a field of proto2 without a label",
					head: proto2File,
					give: &emit.Field{Name: countField, Type: int32Ref, Tags: tag1},
					want: "protobuf: proto2 writes a singular field with a label, and count is neither optional nor required",
				},
				{
					name: "returns an error for a default of proto3",
					head: proto3File, give: &emit.Field{Name: countField, Type: optionalInt32, Tags: tag1, Value: "8"},
					want: "protobuf: proto3 has no default, and count states one",
				},
				{
					name: "returns an error for a default of a list",
					head: proto2File, give: &emit.Field{Name: countField, Type: repeatedInt32, Tags: tag1, Value: "8"},
					want: "protobuf: a repeated field has no default, and count states one",
				},
				{
					name: "returns an error for a default of a field without presence",
					head: edition2023File, give: &emit.Field{Name: countField, Type: int32Ref, Tags: tag1, Value: "8"},
					want: "protobuf: a field without presence has no default, and count states one",
				},
				{
					name: "returns an error for a stated presence that contradicts the field",
					head: edition2023File,
					give: &emit.Field{
						Name:        countField,
						Type:        int32Ref,
						Tags:        tag1,
						Annotations: symbol.Annotations{explicitPresence},
					},
					want: "protobuf: count states the presence EXPLICIT, and the field needs IMPLICIT",
				},
				{
					name: "returns an error for a delimited map",
					head: edition2023File,
					give: &emit.Field{
						Name:        countField,
						Type:        countsMap,
						Tags:        tag1,
						Annotations: symbol.Annotations{delimitedMarker},
					},
					want: "protobuf: a map is not delimited, and count states the marker protobuf:delimited",
				},
				{
					name: "returns an error for a delimited field of proto2 without the message of its group",
					head: proto2File,
					give: &emit.Field{
						Name:        "row",
						Type:        optionalRow,
						Tags:        tag1,
						Annotations: symbol.Annotations{delimitedMarker},
					},
					want: "protobuf: proto2 writes a delimited field as a group, and row has no message of its group",
				},
				{
					name: "returns an error for a delimited field of proto3",
					head: proto3File,
					give: &emit.Field{
						Name:        "row",
						Type:        optionalRow,
						Tags:        tag1,
						Annotations: symbol.Annotations{delimitedMarker},
					},
					want: "protobuf: proto3 has no delimited field, and row states the marker protobuf:delimited",
				},
				{
					name: "returns an error for a feature of a field in proto3",
					head: proto3File,
					give: &emit.Field{
						Name:        countField,
						Type:        int32Ref,
						Tags:        tag1,
						Annotations: symbol.Annotations{implicitPresence},
					},
					want: "protobuf: a declaration states the feature features.field_presence, and proto3 has no features",
				},
				{
					name: "returns an error for a field without a number",
					head: proto3File, give: &emit.Field{Name: countField, Type: int32Ref},
					want: "protobuf: a field of a message takes a number, and count states none",
				},
				{
					name: "returns an error for a field of a type with type arguments",
					head: proto3File,
					give: &emit.Field{
						Name: countField,
						Tags: tag1,
						Type: &emit.TypeRef{Spelling: "Page", Args: []*emit.TypeRef{int32Ref}},
					},
					want: "protobuf: Page states 1 type arguments, and protobuf has none",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.NewVocabulary(&render.ImportSet{}).Block(tt.head, &emit.Struct{
						Name: rowName, Fields: emit.SlotOf(tt.give),
					})
					assert.HasError(t, err, "protobuf does not write the field")
					assert.Equal(t, err.Error(), tt.want, "the error states the field")
					assert.Nil(t, got, "a refusal returns no block")
				})
			}
		})
	})
}
