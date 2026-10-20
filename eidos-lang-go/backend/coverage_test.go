// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// coverageAllocs is the count of the coverage table: four for the fact
// map, and two for the exception map with two for each of its five kinds.
const coverageAllocs = 4 + 2 + 5*2

// TestCoverage pins the verdicts that distinguish Go. The conformance
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
				name: "returns Holds for a struct's final",
				kind: symbol.KindStruct, fact: symbol.FactFinal, want: render.Holds,
			},
			{
				name: "returns Holds for a struct's implements",
				kind: symbol.KindStruct, fact: symbol.FactImplements, want: render.Holds,
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
				name: "returns Renders for a field's tag",
				kind: symbol.KindField, fact: symbol.FactTag, want: render.Renders,
			},
			{
				name: "returns Renders for a method's throws",
				kind: symbol.KindMethod, fact: symbol.FactThrows, want: render.Renders,
			},
			{
				name: "returns Refuses for a sealed interface",
				kind: symbol.KindInterface, fact: symbol.FactSealed, want: render.Refuses,
			},
			{
				name: "returns Refuses for a field's initializer",
				kind: symbol.KindField, fact: symbol.FactValue, want: render.Refuses,
			},
			{
				name: "returns Renders for a variable's initializer",
				kind: symbol.KindVariable, fact: symbol.FactValue, want: render.Renders,
			},
			{
				name: "returns Refuses for a parameter's annotations",
				kind: symbol.KindParam, fact: symbol.FactAnnotations, want: render.Refuses,
			},
			{
				name: "returns Renders for a function's annotations",
				kind: symbol.KindFunction, fact: symbol.FactAnnotations, want: render.Renders,
			},
			{
				name: "returns Rejects for an async function",
				kind: symbol.KindFunction, fact: symbol.FactAsync, want: render.Rejects,
			},
			{
				name: "returns Rejects for a method's override",
				kind: symbol.KindMethod, fact: symbol.FactOverride, want: render.Rejects,
			},
			{
				name: "returns Rejects for a method's default body",
				kind: symbol.KindMethod, fact: symbol.FactDefaultBody, want: render.Rejects,
			},
			{
				name: "returns Rejects for a field at the type level",
				kind: symbol.KindField, fact: symbol.FactLevel, want: render.Rejects,
			},
			{
				name: "returns Rejects for a computed field",
				kind: symbol.KindField, fact: symbol.FactComputed, want: render.Rejects,
			},
			{
				name: "returns Rejects for a method's mutability",
				kind: symbol.KindMethod, fact: symbol.FactMutability, want: render.Rejects,
			},
			{
				name: "returns Renders for a variable's mutability",
				kind: symbol.KindVariable, fact: symbol.FactMutability, want: render.Renders,
			},
			{
				name: "returns Refuses for an abstract method",
				kind: symbol.KindMethod, fact: symbol.FactAbstract, want: render.Refuses,
			},
			{
				name: "returns Renders for an enum's underlying type",
				kind: symbol.KindEnum, fact: symbol.FactUnderlying, want: render.Renders,
			},
			{
				name: "returns Refuses for a field that another type receives",
				kind: symbol.KindField, fact: symbol.FactReceives, want: render.Refuses,
			},
			{
				name: "returns Refuses for a file's annotations",
				kind: symbol.KindFile, fact: symbol.FactAnnotations, want: render.Refuses,
			},
			{
				name: "returns Holds for a parameter's passing",
				kind: symbol.KindParam, fact: symbol.FactPassing, want: render.Holds,
			},
			{
				name: "returns Rejects for a wire name of a field",
				kind: symbol.KindField, fact: symbol.FactWire, want: render.Rejects,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, backend.Coverage().Of(tt.kind, tt.fact), tt.want,
					"Coverage returns the verdict of Go")
			})
		}
	})
}

// TestCoverageAllocs checks the ceiling of the table in the ordinary run,
// which runs no benchmark.
func TestCoverageAllocs(t *testing.T) {
	checkAllocs(t, coverageCalls())
}

// BenchmarkCoverage measures the table that the backend reads once per
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
				assert.Equal(tb, c.Of(symbol.KindStruct, symbol.FactFinal), render.Holds, "Coverage returns Go's table")
			},
		},
	}
}
