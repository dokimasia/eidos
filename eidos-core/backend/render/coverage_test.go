// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// rejectedCode is the persisted spelling of [render.RejectedFact], which
// a suppression names.
const rejectedCode = "EID-0074"

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

// TestCoverage pins the resolution of a verdict through the exceptions
// and then the base, and the findings of the guard of the render. The
// guard renders a declaration whose stated fact is refused or missed, and
// leaves out a declaration whose stated fact is rejected.
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

		unpromoted := render.Coverage{
			Facts: map[symbol.Fact]render.Verdict{
				symbol.FactComment: render.Renders, symbol.FactEmbeds: render.Renders,
			},
			RefusesEmbeds: true,
		}
		embeds := []struct {
			name string
			kind symbol.Kind
			fact symbol.Fact
			want render.Verdict
		}{
			{
				name: "returns Refuses for a fact on an embed where the coverage refuses embeds",
				kind: symbol.KindEmbed, fact: symbol.FactComment, want: render.Refuses,
			},
			{
				name: "returns Refuses for the embeds of a struct where the coverage refuses embeds",
				kind: symbol.KindStruct, fact: symbol.FactEmbeds, want: render.Refuses,
			},
			{
				name: "returns the base verdict for a fact off an embed where the coverage refuses embeds",
				kind: symbol.KindStruct, fact: symbol.FactComment, want: render.Renders,
			},
		}
		for _, tt := range embeds {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, unpromoted.Of(tt.kind, tt.fact), tt.want, "the verdict for the fact on the kind")
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

		rejecting := map[symbol.Fact]render.Verdict{
			symbol.FactAbstract: render.Rejects, symbol.FactFinal: render.Refuses,
		}

		t.Run("reports RejectedFact for a rejected stated fact", func(t *testing.T) {
			t.Parallel()

			_, sink := runPass(t, covered(rejecting), seeded(t, abstract()...))
			diags := slices.Collect(sink.All())
			assert.Length(t, diags, 1, "one finding per stated rejected fact")
			expect.Equal(t, diags[0].Code, render.RejectedFact, "the finding has the rejection code")
			expect.Equal(t, diags[0].Code.String(), rejectedCode, "the code keeps its persisted spelling")
			expect.Equal(t, diags[0].Severity, diag.SeverityError, "a rejection is an error")
			expect.Contains(t, diags[0].Msg, "with or without "+symbol.FactAbstract.String(),
				"the finding names the rejected fact")
		})

		t.Run("leaves out the declaration whose stated fact is rejected", func(t *testing.T) {
			t.Parallel()

			u := unitOf("gen", "svc/row.go", "Row", "Other")
			u.Decls[0].(*emit.Struct).Abstract = true
			files, sink := runPass(t, covered(rejecting), seeded(t, u))
			assert.True(t, sink.Failed(), "a rejection fails the run")
			assert.Length(t, files, 1, "the file renders with the other declaration")
			expect.NotContains(t, string(files[0].Body), "Row", "the rejected declaration does not render")
			expect.Contains(t, string(files[0].Body), "Other", "the declaration beside it renders")
		})

		t.Run("reports every refusal and rejection of one declaration", func(t *testing.T) {
			t.Parallel()

			u := unitOf("gen", "svc/row.go", "Row")
			s := u.Decls[0].(*emit.Struct)
			s.Abstract, s.Final = true, true
			_, sink := runPass(t, covered(rejecting), seeded(t, u))
			assert.Permutation(t, coretest.Codes(sink), []diag.Code{render.RejectedFact, render.RefusedFact},
				"the guard reports each stated fact before it answers")
		})

		t.Run("leaves out a cluster with a rejected member", func(t *testing.T) {
			t.Parallel()

			l := covered(rejecting)
			l.Cluster = func(decls []symbol.Symbol) []render.Clustered {
				return []render.Clustered{{Group: blockGroup, Decls: decls}}
			}
			l.Groups = map[render.GroupName]string{blockGroup: "{{range .Decls}}type {{.Name}} struct{}\n{{end}}"}
			u := unitOf("gen", "svc/row.go", "Row", "Other")
			u.Decls[0].(*emit.Struct).Abstract = true
			files, sink := runPass(t, l, seeded(t, u))
			assert.True(t, sink.Failed(), "a rejection fails the run")
			assert.Empty(t, files, "the cluster does not render without its rejected member")
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
