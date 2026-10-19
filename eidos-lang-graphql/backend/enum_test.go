// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// enumAllocs is the definition of an enum: the text that the writer
// returns.
const enumAllocs = 1

// TestEnum pins the definition of an enum and of its values, byte for byte
// in the layout of graphql-js.
func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Enum", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give *emit.Enum
				want string
			}{
				{
					name: "returns an enum with its values",
					give: &emit.Enum{
						Name:     phaseName,
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN"}, &emit.EnumVariant{Name: "CLOSED"}),
					},
					want: "enum Phase {\n  OPEN\n  CLOSED\n}\n",
				},
				{
					name: "returns the description, the directives and the comment of a value",
					give: &emit.Enum{
						Name: phaseName,
						Variants: emit.SlotOf(&emit.EnumVariant{
							Name: "OPEN", Doc: []string{"OPEN admits writes."}, Comment: "first",
							Annotations: symbol.Annotations{{Name: "deprecated", Args: []string{`reason: "old"`}}},
						}),
					},
					want: "enum Phase {\n  \"OPEN admits writes.\"\n  OPEN @deprecated(reason: \"old\") # first\n}\n",
				},
				{
					name: "returns the description, the directives and the comment of an enum",
					give: &emit.Enum{
						Name: phaseName, Doc: []string{"Phase names a step."}, Comment: "closed set",
						Annotations: symbol.Annotations{shareable},
						Variants:    emit.SlotOf(&emit.EnumVariant{Name: "OPEN"}),
					},
					want: "\"Phase names a step.\"\nenum Phase @shareable { # closed set\n  OPEN\n}\n",
				},
				{
					name: "returns an extension of an enum under the marker graphql:extend",
					give: &emit.Enum{
						Name: phaseName, Annotations: symbol.Annotations{extendMarker},
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "PAUSED"}),
					},
					want: "extend enum Phase {\n  PAUSED\n}\n",
				},
				{
					name: "returns an extension without a body for an enum without values",
					give: &emit.Enum{
						Name: phaseName, Comment: "shared", Annotations: symbol.Annotations{extendMarker, shareable},
					},
					want: "extend enum Phase @shareable # shared\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, (&backend.Vocabulary{}).Enum(tt.give), tt.want, "the definition of the enum")
				})
			}
		})
	})
}

// Enum allocates the text of the definition, once. The ordinary run, which
// runs no benchmark, checks that ceiling here.
func TestEnumAllocs(t *testing.T) {
	v := &backend.Vocabulary{}
	phase := &emit.Enum{Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN"})}
	var got string
	assert.MaxAllocs(t, func() { got = v.Enum(phase) }, enumAllocs, "Enum allocates the text of the definition")
	assert.Equal(t, got, "enum Phase {\n  OPEN\n}\n", "Enum writes the enum")
}

// BenchmarkEnum measures the definition that every enum of a file writes.
func BenchmarkEnum(b *testing.B) {
	b.Run("Vocabulary", func(b *testing.B) {
		b.Run("Enum", func(b *testing.B) {
			v := &backend.Vocabulary{}
			phase := &emit.Enum{Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN"})}
			c := bench.Start(b).Warmup(1).MaxAllocs(enumAllocs)
			defer c.End()
			var got string
			for c.Loop() {
				got = v.Enum(phase)
			}
			assert.Equal(b, got, "enum Phase {\n  OPEN\n}\n", "Enum writes the enum")
		})
	})
}
