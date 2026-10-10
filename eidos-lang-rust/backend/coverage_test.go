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

// coverageAllocs is the coverage table: the fact map's four, and the
// exception map's two with its six kinds' two each.
const coverageAllocs = 4 + 2 + 6*2

// The suite checks the declaration total and the rendered findings
// against the table, and these cases pin the cells that distinguish
// Rust.
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
				assert.Equal(tb, c.Of(symbol.KindInterface, symbol.FactExtends), render.Renders,
					"Coverage returns Rust's table")
			},
		},
	}
}
