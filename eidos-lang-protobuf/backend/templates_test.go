// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"maps"
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/backend"
	kit "go.dokimi.dev/eidos/sdk/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The unit of the template cases.
const (
	templateWord = "gen"
	templateKey  = "acme/store/schema.src"
)

// TestTemplates pins the templates of the backend: the kinds that they
// spell and refuse, and the whole files that the skeleton, the block
// template and the extend template write, in the layout of buf format.
func TestTemplates(t *testing.T) {
	t.Parallel()

	t.Run("KindTemplates", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the block template for a message, an enum, a oneof and a service", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.KindTemplates(), map[symbol.Kind]string{
				symbol.KindStruct:    backend.BlockTemplate,
				symbol.KindEnum:      backend.BlockTemplate,
				symbol.KindSum:       backend.BlockTemplate,
				symbol.KindInterface: backend.BlockTemplate,
			}, "the kinds that protobuf declares")
		})
	})

	t.Run("RefusedKinds", func(t *testing.T) {
		t.Parallel()

		t.Run(
			"returns a reason for each kind of the canonical fixture that protobuf does not declare",
			func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, slices.Sorted(maps.Keys(backend.RefusedKinds())), []symbol.Kind{
					symbol.KindFunction, symbol.KindMethod, symbol.KindVariable, symbol.KindConstant, symbol.KindAlias,
				}, "the kinds that protobuf refuses")
			},
		)
	})

	t.Run("FileTemplate", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			pkg   string
			decls []symbol.Symbol
			want  string
		}{
			{
				name: "returns a file of proto2 with every construct of a message",
				pkg:  "acme.store",
				decls: []symbol.Symbol{
					&emit.File{Doc: []string{"Schema of the store."}, Annotations: symbol.Annotations{
						{Name: syntaxName, Args: []string{protobuf.Proto2}},
						{Name: importName, Args: []string{typesProto}},
						{Name: "(acme.file)", Args: []string{trueValue}},
						{Name: goPackageOption, Args: []string{`"x"`}},
					}},
					// The message has options, ranges, a field, a group, a oneof
					// with a group, a nested message, a nested enum and an
					// extension.
					&emit.Struct{
						Name: "Row", Doc: []string{"Row is one record."}, Comment: "one per key",
						Annotations: symbol.Annotations{
							{Name: deprecatedOption, Args: []string{trueValue}},
							{Name: string(protobuf.ReservedMarker), Args: []string{"5"}},
							{Name: string(protobuf.ExtensionsMarker), Args: []string{"100 to 199"}},
						},
						Fields: emit.SlotOf(
							&emit.Field{Name: "id", Type: &emit.TypeRef{
								Spelling: optionalInt32Type,
								Form:     symbol.FormOptional,
								Elems:    []*emit.TypeRef{{Spelling: int32Type}},
							}, Tags: tag1, Comment: "the key"},
							&emit.Field{
								Name: "result", Tags: tag2, Annotations: symbol.Annotations{delimitedMarker},
								Type: &emit.TypeRef{
									Spelling: "optional Result",
									Form:     symbol.FormOptional,
									Elems:    []*emit.TypeRef{{Spelling: resultName}},
								},
							},
						),
						Types: emit.SlotOf[symbol.Symbol](
							&emit.Struct{Name: resultName, Fields: emit.SlotOf(&emit.Field{
								Name: "code",
								Tags: tag1,
								Type: &emit.TypeRef{
									Spelling: "optional string",
									Form:     symbol.FormOptional,
									Elems:    []*emit.TypeRef{{Spelling: protobuf.TypeString}},
								},
							})},
							&emit.Sum{Name: "choice", Variants: emit.SlotOf(
								&emit.SumVariant{
									Name: "name",
									Fields: emit.SlotOf(
										&emit.Field{
											Name: "name",
											Type: &emit.TypeRef{Spelling: protobuf.TypeString},
											Tags: tag3,
										},
									),
								},
								&emit.SumVariant{Name: "pick", Fields: emit.SlotOf(&emit.Field{
									Name:        "pick",
									Type:        &emit.TypeRef{Spelling: "Pick"},
									Annotations: symbol.Annotations{delimitedMarker},
									Tags:        emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "4"}),
								})},
							)},
							&emit.Struct{
								Name: "Pick",
								Fields: emit.SlotOf(&emit.Field{Name: "at", Type: &emit.TypeRef{
									Spelling: optionalInt32Type,
									Form:     symbol.FormOptional,
									Elems:    []*emit.TypeRef{{Spelling: int32Type}},
								}, Tags: tag1}),
							},
							&emit.Struct{
								Name: "Inner",
								Fields: emit.SlotOf(&emit.Field{Name: "x", Type: &emit.TypeRef{
									Spelling: optionalInt32Type,
									Form:     symbol.FormOptional,
									Elems:    []*emit.TypeRef{{Spelling: int32Type}},
								}, Tags: tag1}),
							},
							&emit.Enum{
								Name: "Kind", Annotations: symbol.Annotations{closedMarker},
								Variants: emit.SlotOf(&emit.EnumVariant{Name: "PLAIN", Value: "0"}),
							},
							&emit.Field{
								Name: "ext",
								Type: &emit.TypeRef{
									Spelling: optionalInt32Type,
									Form:     symbol.FormOptional,
									Elems:    []*emit.TypeRef{{Spelling: int32Type}},
								},
								Tags:     tag100,
								Receives: &emit.TypeRef{Spelling: otherName},
							},
						),
					},
					&emit.Field{
						Name: "tag", Type: &emit.TypeRef{
							Spelling: optionalInt32Type,
							Form:     symbol.FormOptional,
							Elems:    []*emit.TypeRef{{Spelling: int32Type}},
						}, Receives: &emit.TypeRef{Spelling: "Row"},
						Tags: emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "150"}),
					},
				},
				want: "// Schema of the store.\nsyntax = \"proto2\";\n\npackage acme.store;\n\n" +
					"import \"acme/types.proto\";\n\noption go_package = \"x\";\noption (acme.file) = true;\n\n" +
					"// Row is one record.\nmessage Row { // one per key\n" +
					"  option deprecated = true;\n  reserved 5;\n  extensions 100 to 199;\n" +
					"  optional int32 id = 1; // the key\n" +
					"  optional group Result = 2 {\n    optional string code = 1;\n  }\n" +
					"  oneof choice {\n    string name = 3;\n    group Pick = 4 {\n      optional int32 at = 1;\n    }\n  }\n" +
					"  message Inner {\n    optional int32 x = 1;\n  }\n" +
					"  enum Kind {\n    PLAIN = 0;\n  }\n" +
					"  extend Other {\n    optional int32 ext = 100;\n  }\n}\n\n" +
					"extend Row {\n  optional int32 tag = 150;\n}\n",
			},
			{
				name: "returns a file of edition 2024 with its visibility and its features",
				pkg:  "acme.v2",
				decls: []symbol.Symbol{
					&emit.File{Annotations: symbol.Annotations{
						{Name: syntaxName, Args: []string{protobuf.Edition2024}},
						{Name: importName, Args: []string{"option " + rulesProto}},
						implicitPresence,
					}},
					&emit.Struct{
						Name: "Row",
						Fields: emit.SlotOf(
							&emit.Field{Name: "count", Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag1},
							&emit.Field{Name: "limit", Type: &emit.TypeRef{
								Spelling: optionalInt32Type,
								Form:     symbol.FormOptional,
								Elems:    []*emit.TypeRef{{Spelling: int32Type}},
							}, Tags: tag2},
							&emit.Field{Name: "parent", Type: &emit.TypeRef{Spelling: "Row"}, Tags: tag3},
						),
						Types: emit.SlotOf[symbol.Symbol](
							&emit.Struct{
								Name: "Inner",
								Fields: emit.SlotOf(
									&emit.Field{Name: "x", Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag1},
								),
							},
							&emit.Enum{
								Name: "Kind", Annotations: symbol.Annotations{localMarker},
								Variants: emit.SlotOf(&emit.EnumVariant{Name: "PLAIN", Value: "0"}),
							},
						),
					},
					&emit.Enum{
						Name: "State", Annotations: symbol.Annotations{localMarker},
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "STATE_UNSPECIFIED", Value: "0"}),
					},
					&emit.Interface{Name: "Store", Methods: emit.SlotOf(&emit.Method{
						Name:    "Get",
						Params:  emit.SlotOf(&emit.Param{Name: requestParam, Type: &emit.TypeRef{Spelling: "Row"}}),
						Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: "Row"}}),
					})},
				},
				want: "edition = \"2024\";\n\npackage acme.v2;\n\nimport option \"acme/rules.proto\";\n\n" +
					"option features.field_presence = IMPLICIT;\n\n" +
					"message Row {\n  int32 count = 1;\n  int32 limit = 2 [features.field_presence = EXPLICIT];\n" +
					"  Row parent = 3;\n  export message Inner {\n    int32 x = 1;\n  }\n  enum Kind {\n    PLAIN = 0;\n  }\n}\n\n" +
					"local enum State {\n  STATE_UNSPECIFIED = 0;\n}\n\n" +
					"service Store {\n  rpc Get(Row) returns (Row);\n}\n",
			},
			{
				name: "returns a file of edition 2026 with its edition and its strict visibility",
				pkg:  "acme.v3",
				decls: []symbol.Symbol{
					edition2026File,
					&emit.Struct{
						Name: "Row",
						Fields: emit.SlotOf(
							&emit.Field{Name: "id", Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag1},
						),
						Types: emit.SlotOf[symbol.Symbol](&emit.Struct{
							Name:        "Inner",
							Annotations: symbol.Annotations{localMarker},
							Fields: emit.SlotOf(
								&emit.Field{Name: "x", Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag1},
							),
						}),
					},
				},
				want: "edition = \"2026\";\n\npackage acme.v3;\n\n" +
					"export message Row {\n  int32 id = 1 [features.field_presence = IMPLICIT];\n" +
					"  message Inner {\n    int32 x = 1 [features.field_presence = IMPLICIT];\n  }\n}\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				files, findings := backendtest.Rendered(t, func(tb assert.TB) (kit.Backend, *backendtest.Fixture) {
					e := plugin.NewEmit()
					assert.NoError(tb, e.Add(plugin.Unit{
						Plugin: templateWord,
						Per:    plugin.PerSource,
						Word:   templateWord,
						Key:    templateKey,
						Pkg:    symbol.Identity{Lang: protobuf.Lang, Package: tt.pkg, Kind: symbol.KindPackage},
						Decls:  tt.decls,
					}), "the unit is added")
					return backend.New(), &backendtest.Fixture{Emit: e}
				})
				assert.Empty(t, findings, "the file renders clean")
				assert.Length(t, files, 1, "the unit writes one file")
				body := string(files[0].Body)
				assert.That(t, body).
					Equal(tt.want, "the written file").
					Equal(formatted(t, body), "the written file is in the layout of buf format")
			})
		}
	})
}
