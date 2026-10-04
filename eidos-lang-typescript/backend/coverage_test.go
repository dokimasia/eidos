// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// coverageAllocs is the table: the fact map's four allocations, and the
// exception map's four with its twelve kinds' two each.
const coverageAllocs = 4 + 4 + 12*2

// The suite checks the declaration total and the rendered findings
// against the table; this twin pins the cells that distinguish
// TypeScript.
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
				name: "returns Refuses for a field's tag",
				kind: symbol.KindField, fact: symbol.FactTag, want: render.Refuses,
			},
			{
				name: "returns Renders for a type parameter's variance",
				kind: symbol.KindTypeParam, fact: symbol.FactVariance, want: render.Renders,
			},
			{
				name: "returns Renders for a class's annotations",
				kind: symbol.KindStruct, fact: symbol.FactAnnotations, want: render.Renders,
			},
			{
				name: "returns Refuses for an interface's annotations",
				kind: symbol.KindInterface, fact: symbol.FactAnnotations, want: render.Refuses,
			},
			{
				name: "returns Renders for a parameter's default",
				kind: symbol.KindParam, fact: symbol.FactParamDefault, want: render.Renders,
			},
			{
				name: "returns Refuses for a class's level",
				kind: symbol.KindStruct, fact: symbol.FactLevel, want: render.Refuses,
			},
			{
				name: "returns Refuses for a function's throws",
				kind: symbol.KindFunction, fact: symbol.FactThrows, want: render.Refuses,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, backend.Coverage().Of(tt.kind, tt.fact), tt.want, "the table's verdict")
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
				assert.Equal(tb, c.Of(symbol.KindStruct, symbol.FactAnnotations), render.Renders,
					"Coverage returns TypeScript's table")
			},
		},
	}
}
