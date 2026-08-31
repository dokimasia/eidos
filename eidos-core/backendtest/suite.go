// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backendtest

import (
	"cmp"
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/render"
)

// RunBackendSuite holds a renderer to the checks a render answers
// as values: the fixture is inhabited, two runs produce
// byte-identical files, every emit kind the fixture carries
// renders, every body lands whole, and a file's failure reports
// positioned and attributed while the render continues. The
// header and trailer checks are the output contract's and join the
// suite with it.
func RunBackendSuite(t *testing.T, setup Setup) {
	t.Helper()

	t.Run("an inhabited fixture", func(t *testing.T) {
		t.Parallel()
		AssertInhabitedFixture(t, setup)
	})
	t.Run("byte-stable render", func(t *testing.T) {
		t.Parallel()
		AssertDeterministicRender(t, setup)
	})
	t.Run("every kind renders", func(t *testing.T) {
		t.Parallel()
		AssertSpeltKinds(t, setup)
	})
	t.Run("content lands whole", func(t *testing.T) {
		t.Parallel()
		AssertPlacedContent(t, setup)
	})
	t.Run("a file failure continues", func(t *testing.T) {
		t.Parallel()
		AssertContinuedRender(t, setup)
	})
}

// runRender is one check's render call: a fresh setup, a fresh sink
// and the fatality law held, because a renderer returns an error
// for a defect in the pass's own inputs, never for a problem with
// one file.
func runRender(tb assert.TB, setup Setup) ([]plugin.RenderedFile, []diag.Diag) {
	tb.Helper()

	r, f := setup(tb)
	sink := diag.NewSink()
	files, err := r.Render(f.context(sink))
	assert.NoError(tb, err,
		"a file's problem attaches to the sink and the render continues")
	return files, slices.Collect(sink.All())
}

// AssertInhabitedFixture refuses an empty world: a suite over a
// store holding no units passes every check vacuously and proves
// nothing about the backend.
func AssertInhabitedFixture(tb assert.TB, setup Setup) {
	tb.Helper()

	_, f := setup(tb)
	assert.True(tb, f.Emit != nil, "the fixture carries a store")
	if f.Emit == nil {
		return
	}
	units := 0
	for range f.Emit.Units() {
		units++
	}
	assert.True(tb, units > 0, "the fixture emits at least one unit")
}

// AssertDeterministicRender renders two isolated setups and holds
// the files byte-equal: the same names, the same packages, the
// same bytes, which is the byte-identity contract as values. The
// findings must match as a set too; only their order is the run's,
// because the pass reports in completion order.
func AssertDeterministicRender(tb assert.TB, setup Setup) {
	tb.Helper()

	first, firstDiags := runRender(tb, setup)
	second, secondDiags := runRender(tb, setup)
	assert.Equal(tb, first, second,
		"two isolated renders answer the same bytes")
	assert.Equal(tb, sorted(firstDiags), sorted(secondDiags),
		"and report the same findings")
}

// sorted orders findings canonically, so two lawful runs reporting
// one set in two completion orders compare equal.
func sorted(diags []diag.Diag) []diag.Diag {
	slices.SortFunc(diags, func(a, b diag.Diag) int {
		return cmp.Or(
			cmp.Compare(a.Pos.File, b.Pos.File),
			cmp.Compare(a.Pos.Line, b.Pos.Line),
			cmp.Compare(a.Pos.Col, b.Pos.Col),
			cmp.Compare(a.Code.String(), b.Code.String()),
			cmp.Compare(a.Msg, b.Msg),
			cmp.Compare(string(a.Origin), string(b.Origin)),
		)
	})
	return diags
}

// AssertSpeltKinds renders once and refuses a kind the language
// cannot spell: whatever inventory the fixture emits, the backend
// holds a spelling for it. The fixture owns the coverage, so a
// backend claims the full kind set by emitting the full kind set.
func AssertSpeltKinds(tb assert.TB, setup Setup) {
	tb.Helper()

	_, diags := runRender(tb, setup)
	for _, d := range diags {
		assert.True(tb, d.Code != render.UnspeltKind,
			"every kind the fixture emits has a spelling: "+d.Msg)
	}
}

// AssertPlacedContent renders once and holds every body to landing
// whole: no conflicting forms, no reference resolving to nothing,
// no pending slot content dropped by its template.
func AssertPlacedContent(tb assert.TB, setup Setup) {
	tb.Helper()

	_, diags := runRender(tb, setup)
	for _, d := range diags {
		assert.True(tb,
			d.Code != render.BodyConflict &&
				d.Code != render.UnresolvedRef &&
				d.Code != render.DroppedSlots,
			"every body lands whole: "+d.Msg)
	}
}

// AssertContinuedRender renders once and holds the failure
// semantics: the call answers no error, every finding carries a
// position and the suite's origin, and a file reported unformatted
// is withheld from the values.
func AssertContinuedRender(tb assert.TB, setup Setup) {
	tb.Helper()

	files, diags := runRender(tb, setup)
	names := make(map[string]struct{}, len(files))
	for _, f := range files {
		names[f.Name] = struct{}{}
	}
	for _, d := range diags {
		assert.True(tb, !d.Pos.IsZero(),
			"every finding carries a position: "+d.Msg)
		assert.Equal(tb, d.Origin, origin,
			"every finding carries the context's plugin as origin")
		if d.Code == render.UnformattedFile {
			_, returned := names[d.Pos.File]
			assert.False(tb, returned,
				"a file the formatter refused stays withheld: "+d.Pos.File)
		}
	}
}
