// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
)

// covered returns the fixture language declaring one verdict per
// stated fact the fixture uses, so the guard runs armed.
func covered(facts map[symbol.Fact]render.Verdict) render.Language {
	l := language()
	l.Coverage = render.Coverage{Facts: facts}
	return l
}

// The coverage is the feature table as data: verdicts resolve
// through exceptions then the base, and the render's guard reports
// what the declaration refuses or misses without withholding the
// declaration.
func TestCoverage(t *testing.T) {
	t.Parallel()

	t.Run("Of resolves the exception before the base", func(t *testing.T) {
		t.Parallel()

		c := render.Coverage{
			Facts: map[symbol.Fact]render.Verdict{
				symbol.FactValue: render.Renders,
			},
			Except: map[symbol.Kind]map[symbol.Fact]render.Verdict{
				symbol.KindField: {symbol.FactValue: render.Refuses},
			},
		}
		assert.Equal(t, c.Of(symbol.KindField, symbol.FactValue), render.Refuses,
			"the carrier's exception wins")
		assert.Equal(t, c.Of(symbol.KindVariable, symbol.FactValue), render.Renders,
			"everything else takes the base verdict")
		assert.Equal(t, c.Of(symbol.KindField, symbol.FactAsync),
			render.VerdictUndeclared, "a fact neither names stays undeclared")
		assert.True(t, c.Declared(), "a base map declares the coverage")
		assert.False(t, render.Coverage{}.Declared(),
			"an empty declaration leaves the guard off")
	})

	t.Run("a refused stated fact warns and still renders", func(t *testing.T) {
		t.Parallel()

		u := unitOf("gen", "svc/row.go", "Row")
		u.Decls[0].(*emit.Struct).Abstract = true
		files, sink := runPass(t, covered(map[symbol.Fact]render.Verdict{
			symbol.FactAbstract: render.Refuses,
		}), seeded(t, u))

		assert.Length(t, files, 1, "the declaration renders without the fact")
		assert.False(t, sink.Failed(), "a refusal is a warning, not a failure")
		diags := slices.Collect(sink.All())
		assert.Length(t, diags, 1, "one finding per stated refused fact")
		assert.Equal(t, diags[0].Code, render.RefusedFact, "under the refusal code")
		assert.Equal(t, diags[0].Severity, diag.SeverityWarning, "as a warning")
		assert.Contains(t, diags[0].Msg, symbol.FactAbstract.String(),
			"naming the fact")
		assert.Contains(t, diags[0].Msg, symbol.KindStruct.String(),
			"and its carrier")
	})

	t.Run("a member's fact reports under its own kind", func(t *testing.T) {
		t.Parallel()

		u := unitOf("gen", "svc/row.go", "Row")
		s := u.Decls[0].(*emit.Struct)
		s.Fields.Append(&emit.Field{Name: "key", Value: "1"})
		_, sink := runPass(t, covered(map[symbol.Fact]render.Verdict{
			symbol.FactValue: render.Refuses,
		}), seeded(t, u))

		diags := slices.Collect(sink.All())
		assert.Length(t, diags, 1, "the member's fact reports once")
		assert.Contains(t, diags[0].Msg, symbol.KindField.String(),
			"under the member's kind")
	})

	t.Run("an undeclared stated fact is a defect", func(t *testing.T) {
		t.Parallel()

		u := unitOf("gen", "svc/row.go", "Row")
		u.Decls[0].(*emit.Struct).Final = true
		files, sink := runPass(t, covered(map[symbol.Fact]render.Verdict{
			symbol.FactAbstract: render.Refuses,
		}), seeded(t, u))

		assert.Length(t, files, 1, "the declaration still renders")
		assert.True(t, sink.Failed(),
			"a coverage hole is the backend's own defect")
		diags := slices.Collect(sink.All())
		assert.Length(t, diags, 1, "one finding per uncovered fact")
		assert.Equal(t, diags[0].Code, render.UndeclaredFact, "under its code")
		assert.Contains(t, diags[0].Msg, symbol.FactFinal.String(),
			"naming the fact the declaration misses")
	})

	t.Run("a rendered or held fact stays silent", func(t *testing.T) {
		t.Parallel()

		u := unitOf("gen", "svc/row.go", "Row")
		s := u.Decls[0].(*emit.Struct)
		s.Abstract = true
		s.Final = true
		_, sink := runPass(t, covered(map[symbol.Fact]render.Verdict{
			symbol.FactAbstract: render.Renders,
			symbol.FactFinal:    render.Holds,
		}), seeded(t, u))
		assert.Length(t, slices.Collect(sink.All()), 0,
			"a stance the output honours reports nothing")
	})

	t.Run("no declaration, no guard", func(t *testing.T) {
		t.Parallel()

		u := unitOf("gen", "svc/row.go", "Row")
		u.Decls[0].(*emit.Struct).Abstract = true
		_, sink := runPass(t, language(), seeded(t, u))
		assert.Length(t, slices.Collect(sink.All()), 0,
			"a language predating the contract renders unguarded")
	})
}
