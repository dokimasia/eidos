// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// excepted is a coverage whose base verdict renders a field's value
// and whose exception refuses it on a field.
var excepted = render.Coverage{
	Facts: map[symbol.Fact]render.Verdict{
		symbol.FactValue: render.Renders,
	},
	Except: map[symbol.Kind]map[symbol.Fact]render.Verdict{
		symbol.KindField: {symbol.FactValue: render.Refuses},
	},
}

// The coverage is the feature table as data: verdicts resolve
// through exceptions then the base, and the render's guard reports
// what the declaration refuses or misses without withholding the
// declaration.
func TestCoverage(t *testing.T) {
	t.Parallel()

	t.Run("Of", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			kind symbol.Kind
			fact symbol.Fact
			want render.Verdict
		}{
			{
				name: "returns the carrier's exception before the base verdict",
				kind: symbol.KindField, fact: symbol.FactValue, want: render.Refuses,
			},
			{
				name: "returns the base verdict for a kind without an exception",
				kind: symbol.KindVariable, fact: symbol.FactValue, want: render.Renders,
			},
			{
				name: "returns VerdictUndeclared for a fact neither names",
				kind: symbol.KindField, fact: symbol.FactAsync, want: render.VerdictUndeclared,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, excepted.Of(tt.kind, tt.fact), tt.want, "the verdict for the fact on the kind")
			})
		}
	})

	t.Run("Declared", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a base map", func(t *testing.T) {
			t.Parallel()

			assert.True(t, excepted.Declared(), "a base map declares the coverage")
		})

		t.Run("reports false for the zero coverage", func(t *testing.T) {
			t.Parallel()

			assert.False(t, render.Coverage{}.Declared(), "an empty declaration leaves the guard off")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		abstract := func() []plugin.Unit {
			u := unitOf("gen", "svc/row.go", "Row")
			u.Decls[0].(*emit.Struct).Abstract = true
			return []plugin.Unit{u}
		}

		t.Run("reports RefusedFact for a refused stated fact", func(t *testing.T) {
			t.Parallel()

			_, sink := runPass(t, covered(map[symbol.Fact]render.Verdict{
				symbol.FactAbstract: render.Refuses,
			}), seeded(t, abstract()...))

			diags := slices.Collect(sink.All())
			assert.Length(t, diags, 1, "one finding per stated refused fact")
			assert.Equal(t, diags[0].Code, render.RefusedFact, "under the refusal code")
			assert.Equal(t, diags[0].Severity, diag.SeverityWarning, "as a warning")
			assert.Contains(t, diags[0].Msg, symbol.FactAbstract.String(), "naming the fact")
			assert.Contains(t, diags[0].Msg, symbol.KindStruct.String(), "and its carrier")
		})

		t.Run("renders the declaration whose stated fact is refused", func(t *testing.T) {
			t.Parallel()

			files, sink := runPass(t, covered(map[symbol.Fact]render.Verdict{
				symbol.FactAbstract: render.Refuses,
			}), seeded(t, abstract()...))
			assert.Length(t, files, 1, "the declaration renders without the fact")
			assert.False(t, sink.Failed(), "a refusal is a warning, not a failure")
		})

		t.Run("reports a member's refused fact under the member's kind", func(t *testing.T) {
			t.Parallel()

			u := unitOf("gen", "svc/row.go", "Row")
			s := u.Decls[0].(*emit.Struct)
			s.Fields.Append(&emit.Field{Name: "key", Value: "1"})
			_, sink := runPass(t, covered(map[symbol.Fact]render.Verdict{
				symbol.FactValue: render.Refuses,
			}), seeded(t, u))

			diags := slices.Collect(sink.All())
			assert.Length(t, diags, 1, "the member's fact reports once")
			assert.Contains(t, diags[0].Msg, symbol.KindField.String(), "under the member's kind")
		})

		t.Run("reports UndeclaredFact for a stated fact the coverage misses", func(t *testing.T) {
			t.Parallel()

			u := unitOf("gen", "svc/row.go", "Row")
			u.Decls[0].(*emit.Struct).Final = true
			_, sink := runPass(t, covered(map[symbol.Fact]render.Verdict{
				symbol.FactAbstract: render.Refuses,
			}), seeded(t, u))

			assert.True(t, sink.Failed(), "a coverage hole is the backend's own defect")
			diags := slices.Collect(sink.All())
			assert.Length(t, diags, 1, "one finding per uncovered fact")
			assert.Equal(t, diags[0].Code, render.UndeclaredFact, "under its code")
			assert.Contains(t, diags[0].Msg, symbol.FactFinal.String(), "naming the fact the declaration misses")
		})

		t.Run("renders the declaration whose stated fact the coverage misses", func(t *testing.T) {
			t.Parallel()

			u := unitOf("gen", "svc/row.go", "Row")
			u.Decls[0].(*emit.Struct).Final = true
			files, _ := runPass(t, covered(map[symbol.Fact]render.Verdict{
				symbol.FactAbstract: render.Refuses,
			}), seeded(t, u))
			assert.Length(t, files, 1, "the declaration still renders")
		})

		silent := []struct {
			name    string
			verdict render.Verdict
		}{
			{name: "reports nothing for a rendered fact", verdict: render.Renders},
			{name: "reports nothing for a held fact", verdict: render.Holds},
		}
		for _, tt := range silent {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, sink := runPass(t, covered(map[symbol.Fact]render.Verdict{
					symbol.FactAbstract: tt.verdict,
				}), seeded(t, abstract()...))
				assert.Empty(t, slices.Collect(sink.All()), "a stance the output honours reports nothing")
			})
		}

		t.Run("reports nothing without a coverage declaration", func(t *testing.T) {
			t.Parallel()

			_, sink := runPass(t, language(), seeded(t, abstract()...))
			assert.Empty(t, slices.Collect(sink.All()), "a language predating the contract renders unguarded")
		})
	})
}

// A verdict and the declaration question allocate nothing in the
// ordinary run, which runs no benchmark.
func TestCoverageAllocs(t *testing.T) {
	var verdict render.Verdict
	assert.MaxAllocs(t, func() { verdict = excepted.Of(symbol.KindField, symbol.FactValue) }, 0,
		"Of allocates nothing")
	assert.Equal(t, verdict, render.Refuses, "Of returns the exception")
	var declared bool
	assert.MaxAllocs(t, func() { declared = excepted.Declared() }, 0, "Declared allocates nothing")
	assert.True(t, declared, "a base map declares the coverage")
}

// BenchmarkCoverage measures the verdict lookup the guard makes for
// every stated fact, and the declaration question it asks once.
func BenchmarkCoverage(b *testing.B) {
	b.Run("Of", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got render.Verdict
		for c.Loop() {
			got = excepted.Of(symbol.KindField, symbol.FactValue)
		}
		assert.Equal(b, got, render.Refuses, "Of returns the exception")
	})

	b.Run("Declared", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = excepted.Declared()
		}
		assert.True(b, got, "a base map declares the coverage")
	})
}

// covered returns the fixture language declaring one verdict per
// stated fact the fixture uses, so the guard runs armed.
func covered(facts map[symbol.Fact]render.Verdict) render.Language {
	l := language()
	l.Coverage = render.Coverage{Facts: facts}
	return l
}
