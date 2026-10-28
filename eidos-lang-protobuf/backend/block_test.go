// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the nested declarations of the block cases.
const (
	innerName  = "inner"
	kindName   = "kind"
	resultName = "Result"
)

// blockAllocs is the block of a message of proto3 with one field: the block,
// its items, its head and the text of its field.
const blockAllocs = 4

// The markers that the block cases state.
var (
	localMarker     = symbol.Annotation{Name: string(protobuf.LocalMarker)}
	delimitedMarker = symbol.Annotation{Name: string(protobuf.DelimitedMarker)}
	requiredMarker  = symbol.Annotation{Name: string(protobuf.RequiredMarker)}
)

// TestBlock pins the block of a message: its parts in their order, the
// visibility keyword of each edition, the groups of proto2, and the scope
// that it passes to its nested declarations.
func TestBlock(t *testing.T) {
	t.Parallel()

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Block", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				head *emit.File
				give *emit.Struct
				want *backend.Block
			}{
				{
					name: "returns the options, the ranges and the fields of a message in that order",
					head: proto3File,
					give: &emit.Struct{
						Name: rowName, Doc: []string{"Row is one record."}, Comment: "one per fetch",
						Annotations: symbol.Annotations{
							{Name: string(protobuf.ReservedMarker), Args: []string{"2", "9 to 11", "foo"}},
							{Name: deprecatedOption, Args: []string{trueValue}},
						},
						Fields: emit.SlotOf(&emit.Field{Name: nameField, Type: stringRef, Tags: tag1}),
					},
					want: &backend.Block{
						Doc: []string{"Row is one record."}, Head: "message row", Open: "{ // one per fetch\n",
						Items: []backend.Item{
							{Text: "  option deprecated = true;\n  reserved 2, 9 to 11;\n  reserved \"foo\";\n"},
							{Text: "  string name = 1;\n"},
						},
					},
				},
				{
					name: "returns an empty message as a block that closes on its line",
					head: proto3File,
					give: &emit.Struct{Name: rowName},
					want: &backend.Block{Head: "message row", Open: "{"},
				},
				{
					name: "returns an empty message with a comment as a block whose comment ends its line",
					head: proto3File,
					give: &emit.Struct{Name: rowName, Comment: "nothing yet"},
					want: &backend.Block{Head: "message row", Open: "{ // nothing yet\n"},
				},
				{
					name: "returns the extension ranges of a message of proto2",
					head: proto2File,
					give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{
						{Name: string(protobuf.ExtensionsMarker), Args: []string{"100 to 199", "500"}},
						{
							Name: string(protobuf.ExtensionsMarker),
							Args: []string{"1000 to max", "[verification = UNVERIFIED]"},
						},
					}},
					want: &backend.Block{Head: "message row", Open: "{\n", Items: []backend.Item{{
						Text: "  extensions 100 to 199, 500;\n  extensions 1000 to max [verification = UNVERIFIED];\n",
					}}},
				},
				{
					name: "places each nested declaration after the member that its follows marker names",
					head: proto3File,
					give: &emit.Struct{
						Name: rowName,
						Fields: emit.SlotOf(
							&emit.Field{Name: nameField, Type: stringRef, Tags: tag1},
							&emit.Field{
								Name: "count",
								Type: stringRef,
								Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "2"}),
							},
						),
						Types: emit.SlotOf[symbol.Symbol](
							&emit.Enum{Name: kindName, Annotations: symbol.Annotations{
								{Name: string(protobuf.FollowsMarker), Args: []string{""}},
							}},
							&emit.Struct{Name: innerName, Annotations: symbol.Annotations{
								{Name: string(protobuf.FollowsMarker), Args: []string{nameField}},
							}},
						),
					},
					want: &backend.Block{Head: "message row", Open: "{\n", Items: []backend.Item{
						{Nested: &emit.Enum{Name: kindName, Annotations: symbol.Annotations{
							{Name: string(protobuf.FollowsMarker), Args: []string{""}},
						}}, Indent: "  "},
						{Text: "  string name = 1;\n"},
						{Nested: &emit.Struct{Name: innerName, Annotations: symbol.Annotations{
							{Name: string(protobuf.FollowsMarker), Args: []string{nameField}},
						}}, Indent: "  "},
						{Text: "  string count = 2;\n"},
					}},
				},
				{
					name: "returns the nested declarations of a message as items behind the indentation of a member",
					head: proto3File,
					give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](
						&emit.Struct{Name: innerName}, &emit.Enum{Name: kindName},
					)},
					want: &backend.Block{Head: "message row", Open: "{\n", Items: []backend.Item{
						{Nested: &emit.Struct{Name: innerName}, Indent: "  "},
						{Nested: &emit.Enum{Name: kindName}, Indent: "  "},
					}},
				},
				{
					name: "returns a message of edition 2024 as local under the marker where its default exports it",
					head: edition2024File,
					give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{localMarker}},
					want: &backend.Block{Head: "local message row", Open: "{"},
				},
				{
					name: "returns a message of edition 2026 without a keyword under the marker local",
					head: edition2026File,
					give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{localMarker}},
					want: &backend.Block{Head: "message row", Open: "{"},
				},
				{
					name: "returns a message of edition 2026 without the marker as exported",
					head: edition2026File,
					give: &emit.Struct{Name: rowName},
					want: &backend.Block{Head: "export message row", Open: "{"},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.NewVocabulary(&render.ImportSet{}).Block(tt.head, tt.give)
					assert.NoError(t, err, "protobuf writes the message")
					assert.Equal(t, got, tt.want, "the block of the message", assert.EquateEmpty())
				})
			}

			nested := []struct {
				name    string
				head    *emit.File
				host    *emit.Struct
				give    symbol.Symbol
				want    string
				wantErr string
			}{
				{
					name: "returns a nested message of edition 2024 as exported where its default makes it local",
					head: edition2024File,
					host: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Struct{Name: innerName})},
					want: "export message inner",
				},
				{
					name: "returns a nested message of edition 2024 without a keyword under the marker local",
					head: edition2024File,
					host: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](
						&emit.Struct{Name: innerName, Annotations: symbol.Annotations{localMarker}},
					)},
					want: "message inner",
				},
				{
					name: "returns the nested enum of a message that reserves every number as exported under STRICT",
					head: edition2026File,
					host: &emit.Struct{
						Name: rowName,
						Annotations: symbol.Annotations{
							{Name: string(protobuf.ReservedMarker), Args: []string{"1 to max"}},
						},
						Types: emit.SlotOf[symbol.Symbol](&emit.Enum{
							Name: kindName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "plain", Value: "0"}),
						}),
					},
					want: "export enum kind",
				},
				{
					name: "returns an error for an exported nested message under the visibility STRICT",
					head: edition2026File,
					host: &emit.Struct{
						Name:        rowName,
						Annotations: symbol.Annotations{localMarker},
						Types:       emit.SlotOf[symbol.Symbol](&emit.Struct{Name: innerName}),
					},
					wantErr: "protobuf: the visibility STRICT exports no nested message, and inner is not marked local",
				},
				{
					name: "returns an error for the exported nested enum of a message that reserves some numbers",
					head: edition2026File,
					host: &emit.Struct{
						Name: rowName,
						Annotations: symbol.Annotations{
							localMarker,
							{Name: string(protobuf.ReservedMarker), Args: []string{"1 to 10", "20 to max"}},
						},
						Types: emit.SlotOf[symbol.Symbol](&emit.Enum{
							Name: kindName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "plain", Value: "0"}),
						}),
					},
					wantErr: "protobuf: the visibility STRICT exports the nested enum of a message that reserves 1 to max alone, " +
						"and kind is not marked local",
				},
			}
			for _, tt := range nested {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					v := backend.NewVocabulary(&render.ImportSet{})
					host, err := v.Block(tt.head, tt.host)
					assert.NoError(t, err, "protobuf writes the message of the nested declaration")
					last := host.Items[len(host.Items)-1]
					assert.NotNil(t, last.Nested, "the message lists its nested declaration last")
					got, err := v.Block(tt.head, last.Nested)
					if tt.wantErr != "" {
						assert.HasError(t, err, "protobuf does not write the nested declaration")
						assert.Equal(t, err.Error(), tt.wantErr, "the error states the visibility")
						return
					}
					assert.NoError(t, err, "protobuf writes the nested declaration")
					assert.Equal(t, got.Head, tt.want, "the head of the nested declaration")
				})
			}

			groups := []struct {
				name      string
				field     *emit.Field
				member    bool
				wantHost  []backend.Item
				wantGroup *backend.Block
			}{
				{
					name: "returns an optional delimited field as the block of its group",
					field: &emit.Field{
						Name:    "result",
						Tags:    tag1,
						Doc:     []string{"Result is the outcome."},
						Comment: "after the brace",
						Annotations: symbol.Annotations{
							delimitedMarker,
							{Name: deprecatedOption, Args: []string{trueValue}},
						},
						Type: &emit.TypeRef{
							Spelling: "optional Result",
							Form:     symbol.FormOptional,
							Elems: []*emit.TypeRef{{
								Spelling: resultName,
							}},
						},
					},
					wantGroup: &backend.Block{
						Doc: []string{"Result is the outcome."}, Head: "optional group Result = 1 [deprecated = true]",
						Open: "{ // after the brace\n", Items: []backend.Item{{Text: "  optional int32 code = 1;\n"}},
					},
				},
				{
					name: "returns a repeated delimited field as the block of its group",
					field: &emit.Field{
						Name: "result", Tags: tag1, Annotations: symbol.Annotations{delimitedMarker},
						Type: &emit.TypeRef{Spelling: "repeated Result", Form: symbol.FormList, Elems: []*emit.TypeRef{{
							Spelling: resultName,
						}}},
					},
					wantGroup: &backend.Block{
						Head:  "repeated group Result = 1",
						Open:  "{\n",
						Items: []backend.Item{{Text: "  optional int32 code = 1;\n"}},
					},
				},
				{
					name: "returns a required delimited field as the block of its group",
					field: &emit.Field{
						Name: "result", Tags: tag1, Annotations: symbol.Annotations{delimitedMarker, requiredMarker},
						Type: &emit.TypeRef{Spelling: resultName},
					},
					wantGroup: &backend.Block{
						Head:  "required group Result = 1",
						Open:  "{\n",
						Items: []backend.Item{{Text: "  optional int32 code = 1;\n"}},
					},
				},
			}
			for _, tt := range groups {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					group := &emit.Struct{
						Name:   resultName,
						Fields: emit.SlotOf(&emit.Field{Name: "code", Type: optionalInt32, Tags: tag1}),
					}
					v := backend.NewVocabulary(&render.ImportSet{})
					host, err := v.Block(proto2File, &emit.Struct{
						Name: rowName, Fields: emit.SlotOf(tt.field), Types: emit.SlotOf[symbol.Symbol](group),
					})
					assert.NoError(t, err, "protobuf writes the message of the group")
					assert.Length(t, host.Items, 1, "the group is the one member of its message")
					assert.Equal(t, host.Items[0].Nested, symbol.Symbol(group),
						"the member is the message of the group, at the position of its field", assert.ByIdentity())
					got, err := v.Block(proto2File, group)
					assert.NoError(t, err, "protobuf writes the group")
					assert.Equal(t, got, tt.wantGroup, "the block of the group", assert.EquateEmpty())
				})
			}

			refusals := []struct {
				name string
				head *emit.File
				give symbol.Symbol
				want string
			}{
				{
					name: "returns an error for a header without a version of protobuf",
					head: &emit.File{Annotations: symbol.Annotations{{Name: syntaxName, Args: []string{"2025"}}}},
					give: &emit.Struct{Name: rowName},
					want: "protobuf: the header states the version 2025, which protobuf does not have",
				},
				{
					name: "returns an error for a declaration of another kind",
					head: proto3File,
					give: &emit.Function{Name: "boot"},
					want: "protobuf: protobuf writes no block of a declaration of the kind Function",
				},
				{
					name: "returns an error for a feature of a message in proto3",
					head: proto3File,
					give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{closedEnum}},
					want: "protobuf: a declaration states the feature features.enum_type, and proto3 has no features",
				},
				{
					name: "returns an error for the marker local in proto3",
					head: proto3File,
					give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{localMarker}},
					want: "protobuf: proto3 marks no declaration local, and row states the marker protobuf:local",
				},
				{
					name: "returns an error for a feature of a oneof in proto2",
					head: proto2File,
					give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Sum{
						Name: "choice", Annotations: symbol.Annotations{implicitPresence},
					})},
					want: "protobuf: a declaration states the feature features.field_presence, and proto2 has no features",
				},
				{
					name: "returns an error for a reserved name that is not an identifier",
					head: proto3File,
					give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{
						{Name: string(protobuf.ReservedMarker), Args: []string{"foo bar"}},
					}},
					want: `protobuf: a reserved name is an identifier, and row reserves "foo bar"`,
				},
				{
					name: "returns an error for an extension range in proto3",
					head: proto3File,
					give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{
						{Name: string(protobuf.ExtensionsMarker), Args: []string{"100 to 199"}},
					}},
					want: "protobuf: proto3 declares no extension range, and row states one",
				},
				{
					name: "returns an error for a field that its version refuses",
					head: proto2File,
					give: &emit.Struct{
						Name:   rowName,
						Fields: emit.SlotOf(&emit.Field{Name: nameField, Type: stringRef, Tags: tag1}),
					},
					want: "protobuf: proto2 writes a singular field with a label, and name is neither optional nor required",
				},
				{
					name: "returns an error for a nested extension that its version refuses",
					head: proto3File,
					give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Field{
						Name: extName, Type: stringRef, Tags: tag100, Receives: &emit.TypeRef{Spelling: otherName},
					})},
					want: "protobuf: proto3 extends the options of descriptor.proto alone, and an extension extends Other",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.NewVocabulary(&render.ImportSet{}).Block(tt.head, tt.give)
					assert.HasError(t, err, "protobuf does not write the declaration")
					assert.Equal(t, err.Error(), tt.want, "the error states the reason")
					assert.Nil(t, got, "a refusal returns no block")
				})
			}

			groupRefusals := []struct {
				name  string
				field *emit.Field
				want  string
			}{
				{
					name: "returns an error for a delimited field of proto2 without a label",
					field: &emit.Field{
						Name: "result", Tags: tag1, Annotations: symbol.Annotations{delimitedMarker},
						Type: &emit.TypeRef{Spelling: resultName},
					},
					want: "protobuf: proto2 writes a singular field with a label, and result is neither optional nor required",
				},
				{
					name: "returns an error for a delimited field of proto2 without a number",
					field: &emit.Field{
						Name: "result", Annotations: symbol.Annotations{delimitedMarker, requiredMarker},
						Type: &emit.TypeRef{Spelling: resultName},
					},
					want: "protobuf: a field of a message takes a number, and result states none",
				},
			}
			for _, tt := range groupRefusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					group := &emit.Struct{Name: resultName}
					v := backend.NewVocabulary(&render.ImportSet{})
					_, err := v.Block(proto2File, &emit.Struct{
						Name: rowName, Fields: emit.SlotOf(tt.field), Types: emit.SlotOf[symbol.Symbol](group),
					})
					assert.NoError(t, err, "protobuf lists the group in its message")
					_, err = v.Block(proto2File, group)
					assert.HasError(t, err, "protobuf does not write the group")
					assert.Equal(t, err.Error(), tt.want, "the error states the field of the group")
				})
			}
		})
	})
}

// Block allocates the block, its items and the text of each member. The
// ordinary run, which runs no benchmark, checks that ceiling here.
func TestBlockAllocs(t *testing.T) {
	v := backend.NewVocabulary(&render.ImportSet{})
	msg := &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: nameField, Type: stringRef, Tags: tag1})}
	var (
		got *backend.Block
		err error
	)
	assert.MaxAllocs(t, func() { got, err = v.Block(proto3File, msg) }, blockAllocs,
		"Block allocates the block, its items and the text of its field")
	assert.NoError(t, err, "Block writes the message")
	assert.Length(t, got.Items, 1, "Block writes the field of the message")
}

// BenchmarkBlock measures the block that every message of a file writes.
func BenchmarkBlock(b *testing.B) {
	b.Run("Vocabulary", func(b *testing.B) {
		b.Run("Block", func(b *testing.B) {
			v := backend.NewVocabulary(&render.ImportSet{})
			msg := &emit.Struct{
				Name:   rowName,
				Fields: emit.SlotOf(&emit.Field{Name: nameField, Type: stringRef, Tags: tag1}),
			}
			c := bench.Start(b).MaxAllocs(blockAllocs)
			defer c.End()
			var (
				got *backend.Block
				err error
			)
			for c.Loop() {
				got, err = v.Block(proto3File, msg)
			}
			assert.NoError(b, err, "Block writes the message")
			assert.Length(b, got.Items, 1, "Block writes the field of the message")
		})
	})
}
