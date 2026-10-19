// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestCoverage pins the verdicts of the coverage that differ by kind, and
// the verdicts that the docblock of Coverage gives a reason for. The kernel
// suite checks that the coverage is total and that the render reports what
// it refuses.
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
				name: "returns Renders for the methods of an object type",
				kind: symbol.KindStruct, fact: symbol.FactMethods, want: render.Renders,
			},
			{
				name: "returns Refuses for the methods of an enum",
				kind: symbol.KindEnum, fact: symbol.FactMethods, want: render.Refuses,
			},
			{
				name: "returns Refuses for the supertypes of an object type",
				kind: symbol.KindStruct, fact: symbol.FactExtends, want: render.Refuses,
			},
			{
				name: "returns Renders for the supertypes of an interface",
				kind: symbol.KindInterface, fact: symbol.FactExtends, want: render.Renders,
			},
			{
				name: "returns Holds for the level of an object type",
				kind: symbol.KindStruct, fact: symbol.FactLevel, want: render.Holds,
			},
			{
				name: "returns Rejects for the level of a field",
				kind: symbol.KindField, fact: symbol.FactLevel, want: render.Rejects,
			},
			{
				name: "returns Holds for an abstract field with arguments",
				kind: symbol.KindMethod, fact: symbol.FactAbstract, want: render.Holds,
			},
			{
				name: "returns Refuses for an abstract object type",
				kind: symbol.KindStruct, fact: symbol.FactAbstract, want: render.Refuses,
			},
			{
				name: "returns Holds for the results of an operation",
				kind: symbol.KindFunction, fact: symbol.FactMultiReturn, want: render.Holds,
			},
			{
				name: "returns Renders for the results of a field with arguments",
				kind: symbol.KindMethod, fact: symbol.FactMultiReturn, want: render.Renders,
			},
			{
				name: "returns Renders for the named results of a field with arguments",
				kind: symbol.KindMethod, fact: symbol.FactNamedReturn, want: render.Renders,
			},
			{
				name: "returns Refuses for the value of an enum value",
				kind: symbol.KindEnumVariant, fact: symbol.FactValue, want: render.Refuses,
			},
			{
				name: "returns Renders for the default of an input field",
				kind: symbol.KindField, fact: symbol.FactValue, want: render.Renders,
			},
			{
				name: "returns Refuses for the comment of a result",
				kind: symbol.KindReturn, fact: symbol.FactComment, want: render.Refuses,
			},
			{
				name: "returns Renders for the comment of a field",
				kind: symbol.KindField, fact: symbol.FactComment, want: render.Renders,
			},
			{
				name: "returns Holds for the visibility of an object type",
				kind: symbol.KindStruct, fact: symbol.FactVisibility, want: render.Holds,
			},
			{
				name: "returns Rejects for type parameters",
				kind: symbol.KindStruct, fact: symbol.FactTypeParams, want: render.Rejects,
			},
			{
				name: "returns Renders for the extended type of a field at file level",
				kind: symbol.KindField, fact: symbol.FactReceives, want: render.Renders,
			},
			{
				name: "returns Refuses for an embed",
				kind: symbol.KindEmbed, fact: symbol.FactComment, want: render.Refuses,
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
