// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// coverageAllocs is the coverage table: the fact map's four, and the
// exception map's two with its four kinds' two each.
const coverageAllocs = 4 + 2 + 4*2

// The suite checks the declaration total and the rendered findings
// against the table, and these cases pin the cells that distinguish
// Java.
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
				name: "returns Renders for a method's throws",
				kind: symbol.KindMethod, fact: symbol.FactThrows, want: render.Renders,
			},
			{
				name: "returns Refuses for a method's several results",
				kind: symbol.KindMethod, fact: symbol.FactMultiReturn, want: render.Refuses,
			},
			{
				name: "returns Refuses for an enum constant's value",
				kind: symbol.KindEnumVariant, fact: symbol.FactValue, want: render.Refuses,
			},
			{
				name: "returns Renders for an enum's methods",
				kind: symbol.KindEnum, fact: symbol.FactMethods, want: render.Renders,
			},
			{
				name: "returns Refuses for a sum's methods",
				kind: symbol.KindSum, fact: symbol.FactMethods, want: render.Refuses,
			},
			{
				name: "returns Renders for a class's nested types",
				kind: symbol.KindStruct, fact: symbol.FactTypes, want: render.Renders,
			},
			{
				name: "returns Renders for a sealed interface",
				kind: symbol.KindInterface, fact: symbol.FactSealed, want: render.Renders,
			},
			{
				name: "returns Renders for a class's level",
				kind: symbol.KindStruct, fact: symbol.FactLevel, want: render.Renders,
			},
			{
				name: "returns Renders for an interface's properties",
				kind: symbol.KindInterface, fact: symbol.FactProperties, want: render.Renders,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, backend.Coverage().Of(tt.kind, tt.fact), tt.want, "the verdict")
			})
		}
	})
}

// The table allocates its maps. The ordinary run, which runs no
// benchmark, checks that ceiling here.
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
				assert.Equal(tb, c.Of(symbol.KindMethod, symbol.FactThrows), render.Renders,
					"Coverage returns Java's table")
			},
		},
	}
}
