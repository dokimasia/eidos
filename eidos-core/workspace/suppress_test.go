// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/workspace"
)

// The generators of the suppression cases: one that reports a Warning at
// every struct, and one that reports an Error.
const (
	flaggingID plugin.ID = "flagging-mirror"
	faultingID plugin.ID = "faulting-mirror"
)

// The position of the suppression cases' diag directives: the file that
// rawDiag positions an instance in, and the line.
const (
	alphaDirectiveFile = "alpha.go"
	directiveLine      = 2
)

// suppressible is the code of the suppression cases' findings, which
// their diag directives name.
var suppressible = diag.MustRegister(diag.Prefix("SUPPRESSTEST"), diag.CodeSpec{
	Number: 1, Meaning: "a finding that the suppression cases suppress",
})

// alphaID is the struct of the graphs that routedIn returns, which the
// suppression cases' directives annotate.
var alphaID = coretest.Struct(coretest.StorePath, "Alpha").ID

// suppressingLine is the scripted line that suppresses suppressible at
// the struct on the line before it.
var suppressingLine = "+" + string(directive.KernelDiag) + " " + string(directive.DiagOff) + "=" +
	suppressible.String() + "\n"

// A diag directive removes the findings of its code at its declaration
// from every sink that decides an outcome, except a kernel Error, and the
// report counts what each directive removed. Strict reports every Warning
// that remains as an Error.
func TestSuppress(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("removes the Warning of a plan at the declaration that the directive annotates", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), flaggedPlan(t, flagging())))
			report := cleanRun(t, w, suppressedAlpha(t, suppressible))
			assert.Empty(t, findings(report.Sink, suppressible), "the directive removes the finding")
		})

		t.Run("commits a plan whose Error the directive removes", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), flaggedPlan(t, faulting())))
			report := cleanRun(t, w, suppressedAlpha(t, suppressible))
			assert.Equal(t, report.Plans[0].Status, workspace.PlanCommitted, "the removed Error fails nothing")
		})

		t.Run("keeps the finding at a declaration that no directive annotates", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), flaggedPlan(t, flagging())))
			g := routedIn(t, coretest.StorePath, coretest.CachePath)
			assert.NoError(t, g.AttachDirectives(alphaID, []directive.Raw{rawDiag(suppressible, directiveLine)}),
				"the directive attaches before the seal")
			report := cleanRun(t, w, g)
			kept := findings(report.Sink, suppressible)
			assert.Length(t, kept, 1, "the other package's struct keeps its finding")
			assert.Equal(t, kept[0].Pos.File, coretest.CachePath+"/alpha.go", "at the struct without a directive")
		})

		t.Run("removes the Warning of an annotator that the directive names", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).Annotators(warningAnnotator()))
			report := cleanRun(t, w, suppressedAlpha(t, suppressible))
			assert.Empty(t, findings(report.Sink, suppressible), "the directive removes the annotator's finding")
		})

		t.Run("removes a kernel Warning that the directive names", func(t *testing.T) {
			t.Parallel()

			var p promise
			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Keys(p.registration(diag.SeverityWarning)))
			report := cleanRun(t, w, suppressedAlpha(t, workspace.UnmetContract))
			assert.Empty(t, findings(report.Sink, workspace.UnmetContract),
				"the directive removes the kernel's Warning")
		})

		t.Run("keeps a kernel Error that the directive names", func(t *testing.T) {
			t.Parallel()

			var p promise
			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Keys(p.registration(diag.SeverityError)))
			report, err := runOver(t, w, suppressedAlpha(t, workspace.UnmetContract))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the kernel's Error fails the run")
			assert.Length(t, findings(report.Sink, workspace.UnmetContract), 1, "the directive removes no kernel Error")
		})

		t.Run("counts the removed findings by code in Suppressed", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), flaggedPlan(t, flagging())).Annotators(warningAnnotator()))
			report := cleanRun(t, w, suppressedAlpha(t, suppressible))
			assert.Equal(t, report.Suppressed, map[diag.Code]int{suppressible: 2},
				"the annotator's finding and the plan's finding")
		})

		t.Run("lists each directive with the findings that it removed in Suppressions", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), flaggedPlan(t, flagging())))
			report := cleanRun(t, w, suppressedAlpha(t, suppressible))
			assert.Equal(t, report.Suppressions, []workspace.Suppression{{
				Subject: alphaID,
				At:      position.Pos{File: alphaDirectiveFile, Line: directiveLine, Col: 1},
				Code:    suppressible,
				Count:   1,
			}}, "the one directive and its one removed finding")
		})

		t.Run("counts no finding for a second directive of one code on one declaration", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), flaggedPlan(t, flagging())))
			g := routedIn(t, coretest.StorePath)
			assert.NoError(t, g.AttachDirectives(alphaID, []directive.Raw{
				rawDiag(suppressible, directiveLine), rawDiag(suppressible, directiveLine+1),
			}), "the directives attach before the seal")
			report := cleanRun(t, w, g)
			assert.Length(t, report.Suppressions, 2, "both directives are listed")
			expect.Equal(t, report.Suppressions[0].Count, 1, "the first directive removed the finding")
			expect.Equal(t, report.Suppressions[1].Count, 0, "and the second one removed nothing")
		})

		t.Run("reports UnusedSuppression at a directive that removed nothing", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})))
			report := cleanRun(t, w, suppressedAlpha(t, suppressible))
			unused := findings(report.Sink, workspace.UnusedSuppression)
			assert.Length(t, unused, 1, "one finding for the directive")
			expect.Equal(t, unused[0].Pos, position.Pos{File: alphaDirectiveFile, Line: directiveLine, Col: 1},
				"at the directive")
			expect.Equal(t, unused[0].Severity, diag.SeverityInfo, "as an Info")
		})

		t.Run("reports no UnusedSuppression for a run that skips a plan", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "a", centralised("a")),
				diskPlan(t, "b", centralised("b"))))
			report, err := w.Run(t.Context(), workspace.Input{
				Graph: suppressedAlpha(t, suppressible), Plans: []string{"a"},
			})
			assert.NoError(t, err, "the run is clean")
			assert.Empty(t, findings(report.Sink, workspace.UnusedSuppression),
				"the skipped plan's findings are not counted, so no directive reads as unused")
		})

		t.Run("reports no UnusedSuppression for a cancelled run", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			cancelling := generator("canceller", func(m *eidos.StructMatch, e *eidos.Emitter) error {
				cancel()
				return mirrored(m, e)
			})
			w := built(t, onDisk(t, t.TempDir(), flaggedPlan(t, cancelling)))
			report, err := w.Run(ctx, workspace.Input{Graph: suppressedAlpha(t, suppressible)})
			assert.ErrorIs(t, err, context.Canceled, "the cancellation is returned")
			assert.Empty(t, findings(report.Sink, workspace.UnusedSuppression),
				"the cancelled run did not count every finding")
		})

		t.Run("removes a recorded finding that a warm run reports again", func(t *testing.T) {
			t.Parallel()

			report := keptSuppression(t)
			kept := findings(report.Sink, suppressible)
			assert.Length(t, kept, 1, "the row's finding remains")
			assert.Equal(t, kept[0].Pos.File, sealedSource, "and the user's recorded finding is removed")
		})

		t.Run("counts a recorded finding that a warm run removes", func(t *testing.T) {
			t.Parallel()

			report := keptSuppression(t)
			assert.Equal(t, report.Suppressed, map[diag.Code]int{suppressible: 1},
				"the record keeps the finding as the generator reported it")
		})

		t.Run("reports the finding again after an edit removes its directive", func(t *testing.T) {
			t.Parallel()

			w := built(t, sealingOf(t, ledger.NewMem(), "plan", flagging()))
			report, err := warmAfter(t, w, roundsTree(rowLine, userLine+suppressingLine), roundsTree(rowLine, userLine))
			assert.NoError(t, err, "a warning fails no run")
			assert.Length(t, findings(report.Sink, suppressible), 2, "the user's finding is reported again")
		})

		t.Run("fails the plan that reports a Warning under Strict", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), flaggedPlan(t, flagging())))
			report, err := w.Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.StorePath), Strict: true})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the promoted Warning fails the run")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "and the plan that reported it")
		})

		t.Run("returns a Warning at Error under Strict", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), flaggedPlan(t, flagging())))
			report, _ := w.Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.StorePath), Strict: true})
			promoted := findings(report.Sink, suppressible)
			assert.Length(t, promoted, 1, "the plan's finding")
			assert.Equal(t, promoted[0].Severity, diag.SeverityError, "at Error")
		})

		t.Run("fails every plan for a Warning of an annotator under Strict", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).Annotators(warningAnnotator()))
			report, err := w.Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.StorePath), Strict: true})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the promoted Warning fails the run")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "and blocks every plan")
		})

		t.Run("commits a plan whose Warning the directive removes under Strict", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), flaggedPlan(t, flagging())))
			report, err := w.Run(t.Context(), workspace.Input{Graph: suppressedAlpha(t, suppressible), Strict: true})
			assert.NoError(t, err, "the directive removes the Warning before Strict promotes it")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanCommitted, "and the plan commits")
		})
	})
}

// flagging returns a generator that mirrors every struct and reports a
// Warning under suppressible at it.
func flagging() plugin.Generator {
	return generator(flaggingID, func(m *eidos.StructMatch, e *eidos.Emitter) error {
		m.Warnf(suppressible, "%s is flagged", m.Struct.Name)
		return mirrored(m, e)
	})
}

// faulting returns a generator that mirrors every struct and reports an
// Error under suppressible at it.
func faulting() plugin.Generator {
	return generator(faultingID, func(m *eidos.StructMatch, e *eidos.Emitter) error {
		m.Errorf(suppressible, "%s is refused", m.Struct.Name)
		return mirrored(m, e)
	})
}

// warningAnnotator returns an annotator that reports a Warning under
// suppressible at every struct.
func warningAnnotator() plugin.Annotator {
	return stamper("warning-annotator", func(m *eidos.StructMatch, _ *eidos.Stamper) error {
		m.Warnf(suppressible, "%s is flagged by an annotator", m.Struct.Name)
		return nil
	})
}

// flaggedPlan returns the plan of the suppression cases, which runs gen
// through a printer of its own and writes each file beside its source.
func flaggedPlan(tb assert.TB, gen plugin.Generator) workspace.Plan {
	tb.Helper()

	return workspace.Plan{
		Name: "plan", Generators: []plugin.Generator{gen}, Backend: printerAs(tb, "plan-printer", "fixture", ""),
	}
}

// keptSuppression runs the flagging plan twice over the rounds tree with a
// diag directive on the user, and returns the report of the second run,
// which keeps every invocation and reports the recorded findings again.
// It stops the test where the second run executes an invocation of the
// generator.
func keptSuppression(t *testing.T) *workspace.Report {
	t.Helper()

	w := built(t, sealingOf(t, ledger.NewMem(), "plan", flagging()))
	tree := roundsTree(rowLine, userLine+suppressingLine)
	sealedRun(t, w, workspace.Input{Tree: tree})
	report := sealedRun(t, w, workspace.Input{Tree: tree})
	assert.False(t, report.Stats.Cold, "the second run reads the sealed state")
	assert.Equal(t, generatedBy(report, flaggingID), 0, "and executes no invocation of the generator")
	return report
}

// suppressedAlpha returns the graph of routedIn over the store package,
// with a diag directive on Alpha that suppresses code.
func suppressedAlpha(tb assert.TB, code diag.Code) *store.Graph {
	tb.Helper()

	g := routedIn(tb, coretest.StorePath)
	assert.NoError(tb, g.AttachDirectives(alphaID, []directive.Raw{rawDiag(code, directiveLine)}),
		"the directive attaches before the seal")
	return g
}
