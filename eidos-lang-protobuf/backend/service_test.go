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

// The messages of the rpcs of the service cases.
const (
	queryType = "Query"
	rowsType  = "Rows"
)

// TestService pins the block of a service: its options, and one rpc for
// each method with its request, its response and its options.
func TestService(t *testing.T) {
	t.Parallel()

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Block", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give *emit.Interface
				want *backend.Block
			}{
				{
					name: "returns the rpcs of a service with their documentation and comments",
					give: &emit.Interface{
						Name: storeName, Doc: []string{"Store serves rows."}, Comment: "read side",
						Annotations: symbol.Annotations{{Name: deprecatedOption, Args: []string{trueValue}}},
						Methods: emit.SlotOf(
							&emit.Method{
								Name:    "Get",
								Doc:     []string{"Get returns one row."},
								Comment: "by key",
								Params: emit.SlotOf(
									&emit.Param{Name: requestParam, Type: &emit.TypeRef{Spelling: queryType}},
								),
								Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowType}}),
							},
							&emit.Method{
								Name:    "Watch",
								Comment: "until closed",
								Annotations: symbol.Annotations{
									{Name: "idempotency_level", Args: []string{"NO_SIDE_EFFECTS"}},
								},
								Params: emit.SlotOf(&emit.Param{Name: requestParam, Type: &emit.TypeRef{
									Spelling: "stream Query",
									Form:     symbol.FormStream,
									Async:    true,
									Elems:    []*emit.TypeRef{{Spelling: queryType}},
								}}),
								Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{
									Spelling: "stream Rows",
									Form:     symbol.FormStream,
									Async:    true,
									Elems:    []*emit.TypeRef{{Spelling: rowsType}},
								}}),
							},
						),
					},
					want: &backend.Block{
						Doc: []string{"Store serves rows."}, Head: "service store", Open: "{ // read side\n",
						Items: []backend.Item{
							{Text: "  option deprecated = true;\n"},
							{Text: "  // Get returns one row.\n  rpc Get(Query) returns (Row); // by key\n"},
							{Text: "  rpc Watch(stream Query) returns (stream Rows) { // until closed\n" +
								"    option idempotency_level = NO_SIDE_EFFECTS;\n  }\n"},
						},
					},
				},
				{
					name: "returns an rpc with the body marker and without options with an empty body",
					give: &emit.Interface{Name: storeName, Methods: emit.SlotOf(&emit.Method{
						Name:        "Get",
						Comment:     "by key",
						Annotations: symbol.Annotations{{Name: string(protobuf.BodyMarker)}},
						Params: emit.SlotOf(
							&emit.Param{Name: requestParam, Type: &emit.TypeRef{Spelling: queryType}},
						),
						Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowType}}),
					})},
					want: &backend.Block{
						Head: "service store", Open: "{\n",
						Items: []backend.Item{{Text: "  rpc Get(Query) returns (Row) {} // by key\n"}},
					},
				},
				{
					name: "returns a service without an rpc as a block that closes on its line",
					give: &emit.Interface{Name: storeName},
					want: &backend.Block{Head: "service store", Open: "{"},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.NewVocabulary(&render.ImportSet{}).Block(proto3File, tt.give)
					assert.NoError(t, err, "protobuf writes the service")
					assert.Equal(t, got, tt.want, "the block of the service", assert.EquateEmpty())
				})
			}

			refusals := []struct {
				name string
				give *emit.Interface
				want string
			}{
				{
					name: "returns an error for an rpc without exactly one parameter and one result",
					give: &emit.Interface{Name: storeName, Methods: emit.SlotOf(&emit.Method{Name: "Get"})},
					want: "protobuf: an rpc takes one request and returns one response, and Get takes 0 and returns 0",
				},
				{
					name: "returns an error for a request of a type with type arguments",
					give: &emit.Interface{Name: storeName, Methods: emit.SlotOf(&emit.Method{
						Name: "Get",
						Params: emit.SlotOf(
							&emit.Param{Type: &emit.TypeRef{Spelling: "Page", Args: []*emit.TypeRef{int32Ref}}},
						),
						Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowType}}),
					})},
					want: "protobuf: Page states 1 type arguments, and protobuf has none",
				},
				{
					name: "returns an error for a response without a type",
					give: &emit.Interface{Name: storeName, Methods: emit.SlotOf(&emit.Method{
						Name:    "Get",
						Params:  emit.SlotOf(&emit.Param{Type: &emit.TypeRef{Spelling: queryType}}),
						Returns: emit.SlotOf(&emit.Return{}),
					})},
					want: "protobuf: the declaration states no type",
				},
				{
					name: "returns an error for a feature of a service in proto3",
					give: &emit.Interface{Name: storeName, Annotations: symbol.Annotations{closedEnum}},
					want: "protobuf: a declaration states the feature features.enum_type, and proto3 has no features",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := backend.NewVocabulary(&render.ImportSet{}).Block(proto3File, tt.give)
					assert.HasError(t, err, "protobuf does not write the service")
					assert.Equal(t, err.Error(), tt.want, "the error states the rpc")
					assert.Nil(t, got, "a refusal returns no block")
				})
			}
		})
	})
}
