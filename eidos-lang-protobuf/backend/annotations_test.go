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

// The options that the annotation cases state.
const (
	deprecatedOption = "deprecated"
	goPackageOption  = "go_package"
	trueValue        = "true"
)

// TestAnnotations pins which annotations each kind takes: the markers of
// its kind with their arguments, and options with one value and the name
// of an option.
func TestAnnotations(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		kept := []struct {
			name string
			give symbol.Annotations
		}{
			{
				name: "keeps a header of the syntax, the package, the imports and an option",
				give: symbol.Annotations{
					{Name: syntaxName, Args: []string{protobuf.Edition2024}},
					{Name: string(protobuf.PackageMarker), Args: []string{"acme.svc"}},
					{Name: string(protobuf.ImportMarker), Args: []string{"acme/types.proto"}},
					{Name: string(protobuf.ImportMarker), Args: []string{"public acme/common.proto"}},
					{Name: string(protobuf.ImportMarker), Args: []string{"weak acme/legacy.proto"}},
					{Name: string(protobuf.ImportMarker), Args: []string{"option acme/rules.proto"}},
					{Name: goPackageOption, Args: []string{`"example.com/acme/svcpb"`}},
				},
			},
			{
				name: "keeps a custom option with a path into its message",
				give: symbol.Annotations{{Name: "(buf.validate.message).oneof", Args: []string{`{fields: ["a"]}`}}},
			},
			{
				name: "keeps a custom option with a fully-qualified name",
				give: symbol.Annotations{{Name: "(.acme.rules)", Args: []string{trueValue}}},
			},
			{
				name: "keeps the option of a feature",
				give: symbol.Annotations{implicitPresence},
			},
		}
		for _, tt := range kept {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				out, err := backend.Lower(&emit.File{Annotations: tt.give})
				assert.NoError(t, err, "protobuf writes every annotation of the header")
				assert.Nil(t, out, "the lowering keeps the header as it is")
			})
		}

		refusals := []struct {
			name string
			give symbol.Symbol
			want string
		}{
			{
				name: "returns an error for a marker that protobuf does not have",
				give: &emit.File{Annotations: symbol.Annotations{{Name: protobuf.MarkerPrefix + "lazy"}}},
				want: "protobuf: protobuf:lazy is not a marker of protobuf, and the header states it",
			},
			{
				name: "returns an error for the marker of a field on the header",
				give: &emit.File{Annotations: symbol.Annotations{{Name: string(protobuf.RequiredMarker)}}},
				want: "protobuf: a header does not take the marker protobuf:required, and the header states it",
			},
			{
				name: "returns an error for a syntax marker without an argument",
				give: &emit.File{Annotations: symbol.Annotations{{Name: string(protobuf.SyntaxMarker)}}},
				want: "protobuf: the marker protobuf:syntax takes one argument, and the header states 0",
			},
			{
				name: "returns an error for an import with a modifier that protobuf does not have",
				give: &emit.File{Annotations: symbol.Annotations{
					{Name: string(protobuf.ImportMarker), Args: []string{"lazy acme/types.proto"}},
				}},
				want: "protobuf: an import takes the modifier public, weak or option, and the header states lazy",
			},
			{
				name: "returns an error for an import of a modifier without a path",
				give: &emit.File{Annotations: symbol.Annotations{
					{Name: string(protobuf.ImportMarker), Args: []string{"public "}},
				}},
				want: "protobuf: an import states a path, and the header states the modifier public alone",
			},
			{
				name: "returns an error for an option without a value",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{{Name: deprecatedOption}}},
				want: "protobuf: the option deprecated takes one value, and row states 0",
			},
			{
				name: "returns an error for an option with two values",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{
					{Name: deprecatedOption, Args: []string{trueValue, trueValue}},
				}},
				want: "protobuf: the option deprecated takes one value, and row states 2",
			},
			{
				name: "returns an error for an option name that is not an identifier",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{
					{Name: "deprecated!", Args: []string{trueValue}},
				}},
				want: `protobuf: "deprecated!" is not the name of an option, and row states it`,
			},
			{
				name: "returns an error for a custom option without its closing parenthesis",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{
					{Name: "(acme.rules", Args: []string{trueValue}},
				}},
				want: `protobuf: "(acme.rules" is not the name of an option, and row states it`,
			},
			{
				name: "returns an error for a custom option without a name in its parentheses",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{
					{Name: "().min_len", Args: []string{trueValue}},
				}},
				want: `protobuf: "().min_len" is not the name of an option, and row states it`,
			},
			{
				name: "returns an error for an option name with an empty part",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{
					{Name: "java..package", Args: []string{trueValue}},
				}},
				want: `protobuf: "java..package" is not the name of an option, and row states it`,
			},
			{
				name: "returns an error for an option name that ends in a dot",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{
					{Name: "(acme.rules).", Args: []string{trueValue}},
				}},
				want: `protobuf: "(acme.rules)." is not the name of an option, and row states it`,
			},
			{
				name: "returns an error for a custom option that a name follows without a dot",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{
					{Name: "(acme.rules)min_len", Args: []string{trueValue}},
				}},
				want: `protobuf: "(acme.rules)min_len" is not the name of an option, and row states it`,
			},
			{
				name: "returns an error for an option without a name",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{{Args: []string{trueValue}}}},
				want: `protobuf: "" is not the name of an option, and row states it`,
			},
			{
				name: "returns an error for the marker of an enum on a message",
				give: &emit.Struct{
					Name:        rowName,
					Annotations: symbol.Annotations{{Name: string(protobuf.ClosedMarker)}},
				},
				want: "protobuf: a message does not take the marker protobuf:closed, and row states it",
			},
			{
				name: "returns an error for a marker with an argument that takes none",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{
					{Name: string(protobuf.LocalMarker), Args: []string{trueValue}},
				}},
				want: "protobuf: the marker protobuf:local takes no argument, and row states 1",
			},
			{
				name: "returns an error for a reserved marker without an argument",
				give: &emit.Enum{
					Name:        phaseName,
					Annotations: symbol.Annotations{{Name: string(protobuf.ReservedMarker)}},
				},
				want: "protobuf: the marker protobuf:reserved takes one or more arguments, and phase states 0",
			},
			{
				name: "returns an error for a marker on an enum value",
				give: &emit.Enum{Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{
					Name: "open", Value: "0", Annotations: symbol.Annotations{{Name: string(protobuf.LocalMarker)}},
				})},
				want: "protobuf: an enum value does not take the marker protobuf:local, and open states it",
			},
			{
				name: "returns an error for a marker on a field",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name: "name", Type: &emit.TypeRef{Spelling: protobuf.TypeString},
					Tags:        emit.SlotOf(&emit.TagEntry{Key: protobuf.NumberTag, Value: "1"}),
					Annotations: symbol.Annotations{{Name: string(protobuf.NumberMarker), Args: []string{"1"}}},
				})},
				want: "protobuf: a field does not take the marker protobuf:number, and name states it",
			},
			{
				name: "returns an error for a marker on a service",
				give: &emit.Interface{
					Name:        storeName,
					Annotations: symbol.Annotations{{Name: string(protobuf.LocalMarker)}},
				},
				want: "protobuf: a service does not take the marker protobuf:local, and store states it",
			},
			{
				name: "returns an error for a marker on an rpc",
				give: &emit.Interface{Name: storeName, Methods: emit.SlotOf(&emit.Method{
					Name: "get", Annotations: symbol.Annotations{{Name: string(protobuf.LocalMarker)}},
				})},
				want: "protobuf: an rpc does not take the marker protobuf:local, and get states it",
			},
			{
				name: "returns an error for a marker on a parameter",
				give: &emit.Interface{Name: storeName, Methods: emit.SlotOf(&emit.Method{
					Name: "get",
					Params: emit.SlotOf(&emit.Param{
						Name: "key", Type: &emit.TypeRef{Spelling: protobuf.TypeString},
						Annotations: symbol.Annotations{{Name: string(protobuf.RequiredMarker)}},
					}),
				})},
				want: "protobuf: a parameter does not take the marker protobuf:required, and key states it",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				out, err := backend.Lower(tt.give)
				assert.HasError(t, err, "protobuf has no form of the annotation")
				assert.Equal(t, err.Error(), tt.want, "the error states the annotation")
				assert.Nil(t, out, "a refusal returns no declaration")
			})
		}
	})
}
