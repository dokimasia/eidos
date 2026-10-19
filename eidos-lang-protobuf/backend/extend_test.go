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

// The extended messages of the extend cases.
const (
	fieldOptions = "google.protobuf.FieldOptions"
	descriptor   = "google/protobuf/descriptor.proto"
	anotherName  = "Another"
)

// extensionAllocs is the extend block of one extension of proto3: the
// block, its head, its items and the text of the field.
const extensionAllocs = 4

// optionsCluster is a cluster of proto3 with one extension of the options
// of a field, which the allocation cases write.
var optionsCluster = render.Clustered{Group: backend.ExtendGroup, Decls: []symbol.Symbol{&emit.Field{
	Name: "max_len", Type: int32Ref, Tags: tag100,
	Receives: &emit.TypeRef{Spelling: fieldOptions, Package: descriptor},
}}}

// optionsBlock is the extend block of optionsCluster.
var optionsBlock = &backend.Block{Head: "extend " + fieldOptions, Open: "{\n", Items: []backend.Item{
	{Text: "  int32 max_len = 100;\n"},
}}

// TestExtend pins the extend blocks: at file level, the block of a cluster,
// and inside a message, one block for each extended message among its
// nested declarations.
func TestExtend(t *testing.T) {
	t.Parallel()

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Extension", func(t *testing.T) {
			t.Parallel()

			t.Run(
				"returns the fields of a cluster as the members of its block and imports the extended message",
				func(t *testing.T) {
					t.Parallel()

					set := &render.ImportSet{}
					receives := &emit.TypeRef{Spelling: fieldOptions, Package: descriptor}
					got, err := backend.NewVocabulary(set).
						Extension(proto3File, render.Clustered{Group: backend.ExtendGroup, Decls: []symbol.Symbol{
							&emit.Field{
								Name:     "max_len",
								Type:     int32Ref,
								Tags:     tag100,
								Receives: receives,
								Doc:      []string{"max_len bounds a string."},
							},
							&emit.Field{
								Name:     "min_len",
								Type:     int32Ref,
								Tags:     emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "101"}),
								Receives: receives,
							},
						}})
					assert.NoError(t, err, "protobuf writes the extensions")
					assert.Equal(
						t,
						got,
						&backend.Block{Head: "extend " + fieldOptions, Open: "{\n", Items: []backend.Item{
							{Text: "  // max_len bounds a string.\n  int32 max_len = 100;\n"},
							{Text: "  int32 min_len = 101;\n"},
						}},
						"the block of the extensions",
						assert.EquateEmpty(),
					)
					assert.Equal(
						t,
						set.Paths(),
						[]string{descriptor},
						"the block imports the file of the extended message",
					)
				},
			)

			t.Run("returns a delimited extension of proto2 as the block of its group", func(t *testing.T) {
				t.Parallel()

				group := &emit.Struct{Name: resultName}
				v := backend.NewVocabulary(&render.ImportSet{})
				got, err := v.Extension(proto2File, render.Clustered{Group: backend.ExtendGroup, Decls: []symbol.Symbol{
					&emit.Field{
						Name:        "result",
						Tags:        tag100,
						Receives:    &emit.TypeRef{Spelling: otherName},
						Annotations: symbol.Annotations{delimitedMarker},
						Type: &emit.TypeRef{
							Spelling: "optional Result",
							Form:     symbol.FormOptional,
							Elems:    []*emit.TypeRef{{Spelling: resultName}},
						},
					},
					group,
				}})
				assert.NoError(t, err, "protobuf writes the extension")
				assert.Length(t, got.Items, 1, "the group is the one member of the block")
				assert.Equal(
					t,
					got.Items[0].Nested,
					symbol.Symbol(group),
					"the member is the message of the group",
					assert.ByIdentity(),
				)
				assert.Empty(t, got.Next, "the message of the group renders inside the block")
				head, err := v.Block(proto2File, group)
				assert.NoError(t, err, "protobuf writes the group")
				assert.Equal(t, head.Head, "optional group Result = 100", "the head of the group of the extension")
			})

			t.Run("returns the message of a delimited extension of an edition after its block", func(t *testing.T) {
				t.Parallel()

				message := &emit.Struct{Name: resultName}
				v := backend.NewVocabulary(&render.ImportSet{})
				got, err := v.Extension(
					edition2023File,
					render.Clustered{Group: backend.ExtendGroup, Decls: []symbol.Symbol{
						&emit.Field{
							Name: "result", Tags: tag100, Receives: &emit.TypeRef{Spelling: otherName},
							Annotations: symbol.Annotations{delimitedMarker}, Type: &emit.TypeRef{Spelling: resultName},
						},
						message,
					}},
				)
				assert.NoError(t, err, "protobuf writes the extension")
				assert.Equal(t, got.Items, []backend.Item{
					{Text: "  Result result = 100 [features.message_encoding = DELIMITED];\n"},
				}, "the extension is a field of the block")
				assert.Length(t, got.Next, 1, "the message follows the block")
				assert.Equal(
					t,
					got.Next[0],
					symbol.Symbol(message),
					"the message is a message of the file",
					assert.ByIdentity(),
				)
				block, err := v.Block(edition2023File, message)
				assert.NoError(t, err, "protobuf writes the message")
				assert.Equal(t, block.Head, "message Result", "the message is at file level")
			})

			refusals := []struct {
				name string
				head *emit.File
				give []symbol.Symbol
				want string
			}{
				{
					name: "returns an error for a cluster without an extension",
					head: proto2File,
					give: []symbol.Symbol{&emit.Struct{Name: resultName}},
					want: "protobuf: an extend block extends a message, and the cluster states no extension",
				},
				{
					name: "returns an error for an extension of proto3 of a message outside descriptor.proto",
					head: proto3File,
					give: []symbol.Symbol{
						&emit.Field{
							Name:     extName,
							Type:     int32Ref,
							Tags:     tag100,
							Receives: &emit.TypeRef{Spelling: otherName},
						},
					},
					want: "protobuf: proto3 extends the options of descriptor.proto alone, and an extension extends Other",
				},
				{
					name: "returns an error for a header without a version of protobuf",
					head: &emit.File{Annotations: symbol.Annotations{{Name: syntaxName, Args: []string{"2025"}}}},
					give: []symbol.Symbol{
						&emit.Field{
							Name:     extName,
							Type:     int32Ref,
							Tags:     tag100,
							Receives: &emit.TypeRef{Spelling: otherName},
						},
					},
					want: "protobuf: the header states the version 2025, which protobuf does not have",
				},
				{
					name: "returns an error for an extended message of a type with type arguments",
					head: proto2File,
					give: []symbol.Symbol{&emit.Field{
						Name: extName, Type: optionalInt32, Tags: tag100,
						Receives: &emit.TypeRef{Spelling: otherName, Args: []*emit.TypeRef{int32Ref}},
					}},
					want: "protobuf: Other states 1 type arguments, and protobuf has none",
				},
				{
					name: "returns an error for a required extension",
					head: proto2File,
					give: []symbol.Symbol{&emit.Field{
						Name: extName, Type: int32Ref, Tags: tag100, Receives: &emit.TypeRef{Spelling: otherName},
						Annotations: symbol.Annotations{requiredMarker},
					}},
					want: "protobuf: an extension is not required, and ext states the marker protobuf:required",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.NewVocabulary(&render.ImportSet{}).Extension(tt.head, render.Clustered{
						Group: backend.ExtendGroup, Decls: tt.give,
					})
					assert.HasError(t, err, "protobuf does not write the extensions")
					assert.Equal(t, err.Error(), tt.want, "the error states the reason")
					assert.Nil(t, got, "a refusal returns no block")
				})
			}
		})

		t.Run("Block", func(t *testing.T) {
			t.Parallel()

			t.Run(
				"returns the extensions of a message as one extend block for each extended message",
				func(t *testing.T) {
					t.Parallel()

					other := &emit.TypeRef{Spelling: otherName}
					got, err := backend.NewVocabulary(&render.ImportSet{}).Block(proto2File, &emit.Struct{
						Name: rowName,
						Types: emit.SlotOf[symbol.Symbol](
							&emit.Field{Name: "first", Type: optionalInt32, Tags: tag100, Receives: other},
							&emit.Field{
								Name:     "another",
								Type:     optionalInt32,
								Tags:     tag100,
								Receives: &emit.TypeRef{Spelling: anotherName},
							},
							&emit.Field{
								Name: "second", Type: optionalInt32, Receives: other,
								Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "101"}),
							},
						),
					})
					assert.NoError(t, err, "protobuf writes the extensions of the message")
					assert.Equal(t, got.Items, []backend.Item{
						{Text: "  extend Other {\n"},
						{Text: "    optional int32 first = 100;\n"},
						{Text: "    optional int32 second = 101;\n"},
						{Text: "  }\n"},
						{Text: "  extend Another {\n"},
						{Text: "    optional int32 another = 100;\n"},
						{Text: "  }\n"},
					}, "one block for each extended message, at the position of its first extension")
				},
			)

			t.Run(
				"returns a delimited extension of a message of proto2 as the block of its group in its extend block",
				func(t *testing.T) {
					t.Parallel()

					group := &emit.Struct{Name: resultName}
					v := backend.NewVocabulary(&render.ImportSet{})
					got, err := v.Block(proto2File, &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](
						&emit.Field{
							Name:        "result",
							Tags:        tag100,
							Receives:    &emit.TypeRef{Spelling: otherName},
							Annotations: symbol.Annotations{delimitedMarker},
							Type: &emit.TypeRef{
								Spelling: "repeated Result",
								Form:     symbol.FormList,
								Elems:    []*emit.TypeRef{{Spelling: resultName}},
							},
						},
						group,
					)})
					assert.NoError(t, err, "protobuf writes the extension of the message")
					assert.Length(t, got.Items, 3, "the extend block has the group as its one member")
					assert.Equal(
						t,
						got.Items[1].Nested,
						symbol.Symbol(group),
						"the member is the message of the group",
						assert.ByIdentity(),
					)
					assert.Equal(t, got.Items[1].Indent, "    ", "the group is a member of a nested block")
				},
			)

			t.Run("returns an error for an extension of a message whose field its version refuses", func(t *testing.T) {
				t.Parallel()

				got, err := backend.NewVocabulary(&render.ImportSet{}).Block(proto2File, &emit.Struct{
					Name: rowName,
					Types: emit.SlotOf[symbol.Symbol](
						&emit.Field{
							Name:     extName,
							Type:     int32Ref,
							Tags:     tag100,
							Receives: &emit.TypeRef{Spelling: otherName},
						},
					),
				})
				assert.HasError(t, err, "protobuf does not write the extension")
				assert.Equal(t, err.Error(),
					"protobuf: proto2 writes a singular field with a label, and ext is neither optional nor required",
					"the error states the extension")
				assert.Nil(t, got, "a refusal returns no block")
			})
		})
	})
}

// Extension allocates the block of a cluster and the text of its fields.
// The ordinary run, which runs no benchmark, checks that ceiling here.
func TestExtendAllocs(t *testing.T) {
	v := backend.NewVocabulary(&render.ImportSet{})
	var (
		got *backend.Block
		err error
	)
	assert.MaxAllocs(t, func() { got, err = v.Extension(proto3File, optionsCluster) }, extensionAllocs,
		"Extension allocates the block and the text of its field")
	assert.NoError(t, err, "Extension writes the cluster")
	assert.Equal(t, got, optionsBlock, "Extension writes the block of the extension", assert.EquateEmpty())
}

// BenchmarkExtend measures the extend block of each cluster of a file.
func BenchmarkExtend(b *testing.B) {
	b.Run("Vocabulary", func(b *testing.B) {
		b.Run("Extension", func(b *testing.B) {
			v := backend.NewVocabulary(&render.ImportSet{})
			c := bench.Start(b).Warmup(1).MaxAllocs(extensionAllocs)
			defer c.End()
			var (
				got *backend.Block
				err error
			)
			for c.Loop() {
				got, err = v.Extension(proto3File, optionsCluster)
			}
			assert.NoError(b, err, "Extension writes the cluster")
			assert.Equal(b, got, optionsBlock, "Extension writes the block of the extension", assert.EquateEmpty())
		})
	})
}
