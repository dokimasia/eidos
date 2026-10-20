// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python/backend"
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
				name: "returns Renders for the visibility of a method",
				kind: symbol.KindMethod, fact: symbol.FactVisibility, want: render.Renders,
			},
			{
				name: "returns Renders for the results of a function",
				kind: symbol.KindFunction, fact: symbol.FactMultiReturn, want: render.Renders,
			},
			{
				name: "returns Renders for the embeds of a struct",
				kind: symbol.KindStruct, fact: symbol.FactEmbeds, want: render.Renders,
			},
			{
				name: "returns Refuses for the comment of an embed",
				kind: symbol.KindEmbed, fact: symbol.FactComment, want: render.Refuses,
			},
			{
				name: "returns Refuses for the annotations of an embed",
				kind: symbol.KindEmbed, fact: symbol.FactAnnotations, want: render.Refuses,
			},
			{
				name: "returns Holds for the level of a struct",
				kind: symbol.KindStruct, fact: symbol.FactLevel, want: render.Holds,
			},
			{
				name: "returns Renders for the level of a method",
				kind: symbol.KindMethod, fact: symbol.FactLevel, want: render.Renders,
			},
			{
				name: "returns Holds for the failures of a signature",
				kind: symbol.KindMethod, fact: symbol.FactThrows, want: render.Holds,
			},
			{
				name: "returns Holds for a const enum",
				kind: symbol.KindEnum, fact: symbol.FactConstEnum, want: render.Holds,
			},
			{
				name: "returns Holds for a sealed type",
				kind: symbol.KindInterface, fact: symbol.FactSealed, want: render.Holds,
			},
			{
				name: "returns Rejects for a field that another type receives",
				kind: symbol.KindField, fact: symbol.FactReceives, want: render.Rejects,
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
				name: "returns Renders for the passing of a parameter",
				kind: symbol.KindParam, fact: symbol.FactPassing, want: render.Renders,
			},
			{
				name: "returns Refuses for the methods of a sum",
				kind: symbol.KindSum, fact: symbol.FactMethods, want: render.Refuses,
			},
			{
				name: "returns Renders for the methods of a struct",
				kind: symbol.KindStruct, fact: symbol.FactMethods, want: render.Renders,
			},
			{
				name: "returns Refuses for an optional method",
				kind: symbol.KindMethod, fact: symbol.FactOptional, want: render.Refuses,
			},
			{
				name: "returns Renders for an optional field",
				kind: symbol.KindField, fact: symbol.FactOptional, want: render.Renders,
			},
			{
				name: "returns Refuses for the tag of a field",
				kind: symbol.KindField, fact: symbol.FactTag, want: render.Refuses,
			},
			{
				name: "returns Refuses for a value parameter",
				kind: symbol.KindTypeParam, fact: symbol.FactConstParam, want: render.Refuses,
			},
			{
				name: "returns Renders for the fields of an enum",
				kind: symbol.KindEnum, fact: symbol.FactFields, want: render.Renders,
			},
			{
				name: "returns Refuses for the contracts of an enum",
				kind: symbol.KindEnum, fact: symbol.FactImplements, want: render.Refuses,
			},
			{
				name: "returns Renders for the contracts of a sum",
				kind: symbol.KindSum, fact: symbol.FactImplements, want: render.Renders,
			},
			{
				name: "returns Renders for an overload signature of a function",
				kind: symbol.KindFunction, fact: symbol.FactOverload, want: render.Renders,
			},
		}
		coverage := backend.Coverage()
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, coverage.Of(tt.kind, tt.fact), tt.want, "the verdict of the fact")
			})
		}

		t.Run("returns a table of the caller's own", func(t *testing.T) {
			t.Parallel()

			delete(backend.Coverage().Facts, symbol.FactComment)
			assert.Equal(t, backend.Coverage().Of(symbol.KindField, symbol.FactComment), render.Renders,
				"a change of one table leaves the next table unchanged")
		})
	})
}
