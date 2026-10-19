// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust/backend"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// coverageAllocs is the count of the coverage table: four for the fact
// map, and two for the exception map with two for each of its five kinds.
const coverageAllocs = 4 + 2 + 5*2

// TestCoverage pins the verdicts that distinguish Rust. The conformance
// suite checks that the table is total and that the rendered findings
// match it.
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
				name: "returns Renders for a trait's supertraits",
				kind: symbol.KindInterface, fact: symbol.FactExtends, want: render.Renders,
			},
			{
				name: "returns Refuses for a struct's supertypes",
				kind: symbol.KindStruct, fact: symbol.FactExtends, want: render.Refuses,
			},
			{
				name: "returns Holds for a struct's final",
				kind: symbol.KindStruct, fact: symbol.FactFinal, want: render.Holds,
			},
			{
				name: "returns Renders for a struct's methods",
				kind: symbol.KindStruct, fact: symbol.FactMethods, want: render.Renders,
			},
			{
				name: "returns Refuses for a sum's methods",
				kind: symbol.KindSum, fact: symbol.FactMethods, want: render.Refuses,
			},
			{
				name: "returns Holds for a method's final",
				kind: symbol.KindMethod, fact: symbol.FactFinal, want: render.Holds,
			},
			{
				name: "returns Renders for a const parameter",
				kind: symbol.KindTypeParam, fact: symbol.FactConstParam, want: render.Renders,
			},
			{
				name: "returns Renders for a type parameter's default",
				kind: symbol.KindTypeParam, fact: symbol.FactTypeParamDefault, want: render.Renders,
			},
			{
				name: "returns Refuses for a variadic parameter",
				kind: symbol.KindParam, fact: symbol.FactVariadic, want: render.Refuses,
			},
			{
				name: "returns Renders for an enum variant's value",
				kind: symbol.KindEnumVariant, fact: symbol.FactValue, want: render.Renders,
			},
			{
				name: "returns Refuses for a field's level",
				kind: symbol.KindField, fact: symbol.FactLevel, want: render.Refuses,
			},
			{
				name: "returns Renders for a trait's nested types",
				kind: symbol.KindInterface, fact: symbol.FactTypes, want: render.Renders,
			},
			{
				name: "returns Refuses for a struct's nested types",
				kind: symbol.KindStruct, fact: symbol.FactTypes, want: render.Refuses,
			},
			{
				name: "returns Renders for a method's throws",
				kind: symbol.KindMethod, fact: symbol.FactThrows, want: render.Renders,
			},
			{
				name: "returns Renders for a parameter's attributes",
				kind: symbol.KindParam, fact: symbol.FactAnnotations, want: render.Renders,
			},
			{
				name: "returns Refuses for a field that another type receives",
				kind: symbol.KindField, fact: symbol.FactReceives, want: render.Refuses,
			},
			{
				name: "returns Refuses for a file's annotations",
				kind: symbol.KindFile, fact: symbol.FactAnnotations, want: render.Refuses,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, backend.Coverage().Of(tt.kind, tt.fact), tt.want,
					"Coverage returns the verdict of Rust")
			})
		}
	})
}

// TestCoverageAllocs checks the ceiling of the table in the ordinary run,
// which runs no benchmark.
func TestCoverageAllocs(t *testing.T) {
	checkAllocs(t, coverageCalls())
}

// BenchmarkCoverage measures the table the backend reads once per
// build.
func BenchmarkCoverage(b *testing.B) {
	benchCalls(b, coverageCalls())
}

// coverageCalls returns a call of Coverage.
func coverageCalls() []allocCall {
	var c render.Coverage
	return []allocCall{
		{
			name: "Coverage", allocs: coverageAllocs,
			call: func() { c = backend.Coverage() },
			check: func(tb assert.TB) {
				assert.Equal(tb, c.Of(symbol.KindInterface, symbol.FactExtends), render.Renders,
					"Coverage returns Rust's table")
			},
		},
	}
}
