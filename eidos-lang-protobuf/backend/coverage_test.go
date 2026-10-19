// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/protobuf/backend"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestCoverage pins the verdicts of the coverage that the docblock of
// Coverage gives a reason for. The kernel suite checks that the coverage is
// total and that the render reports what it refuses.
func TestCoverage(t *testing.T) {
	t.Parallel()

	t.Run("Coverage", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			kind symbol.Kind
			fact symbol.Fact
			want render.Verdict
		}{
			{
				name: "returns Holds for the visibility of a message",
				kind: symbol.KindStruct,
				fact: symbol.FactVisibility,
				want: render.Holds,
			},
			{
				name: "returns Holds for the mutability of a field",
				kind: symbol.KindField,
				fact: symbol.FactMutability,
				want: render.Holds,
			},
			{
				name: "returns Holds for an asynchronous rpc",
				kind: symbol.KindMethod,
				fact: symbol.FactAsync,
				want: render.Holds,
			},
			{
				name: "returns Holds for the failures of an rpc",
				kind: symbol.KindMethod,
				fact: symbol.FactThrows,
				want: render.Holds,
			},
			{
				name: "returns Holds for an abstract rpc",
				kind: symbol.KindMethod,
				fact: symbol.FactAbstract,
				want: render.Holds,
			},
			{
				name: "returns Refuses for an abstract message",
				kind: symbol.KindStruct,
				fact: symbol.FactAbstract,
				want: render.Refuses,
			},
			{
				name: "returns Holds for the level of a nested message",
				kind: symbol.KindStruct,
				fact: symbol.FactLevel,
				want: render.Holds,
			},
			{
				name: "returns Rejects for a static field",
				kind: symbol.KindField,
				fact: symbol.FactLevel,
				want: render.Rejects,
			},
			{
				name: "returns Refuses for the methods of a message",
				kind: symbol.KindStruct,
				fact: symbol.FactMethods,
				want: render.Refuses,
			},
			{
				name: "returns Renders for the presence of a field",
				kind: symbol.KindField,
				fact: symbol.FactOptional,
				want: render.Renders,
			},
			{
				name: "returns Refuses for an optional parameter",
				kind: symbol.KindParam,
				fact: symbol.FactOptional,
				want: render.Refuses,
			},
			{
				name: "returns Renders for the results of an rpc",
				kind: symbol.KindMethod,
				fact: symbol.FactMultiReturn,
				want: render.Renders,
			},
			{
				name: "returns Renders for the nested types of a message",
				kind: symbol.KindStruct,
				fact: symbol.FactTypes,
				want: render.Renders,
			},
			{
				name: "returns Refuses for the nested types of a service",
				kind: symbol.KindInterface,
				fact: symbol.FactTypes,
				want: render.Refuses,
			},
			{
				name: "returns Refuses for an embed",
				kind: symbol.KindEmbed,
				fact: symbol.FactComment,
				want: render.Refuses,
			},
			{
				name: "returns Rejects for type parameters",
				kind: symbol.KindStruct,
				fact: symbol.FactTypeParams,
				want: render.Rejects,
			},
			{
				name: "returns Renders for the extended message of a field",
				kind: symbol.KindField,
				fact: symbol.FactReceives,
				want: render.Renders,
			},
			{
				name: "returns Renders for the tag of a field",
				kind: symbol.KindField,
				fact: symbol.FactTag,
				want: render.Renders,
			},
			{
				name: "returns Renders for the annotations of a header",
				kind: symbol.KindFile,
				fact: symbol.FactAnnotations,
				want: render.Renders,
			},
		}
		coverage := backend.Coverage()
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, coverage.Of(tt.kind, tt.fact), tt.want, "the verdict of the fact")
			})
		}
	})
}
