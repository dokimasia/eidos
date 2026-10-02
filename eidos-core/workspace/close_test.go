// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// promisedKey is the fixture key whose contract the audit checks.
const promisedKey meta.KeyName = "audit.seen"

// promise is the fixture key a composition registers, and its handle
// once the registration ran at Build.
type promise struct{ key meta.Key[bool] }

// registration returns the registration of promisedKey, promising it
// on every struct by the end of the annotate phase at a severity.
func (p *promise) registration(severity diag.Severity) func(r *meta.Registry) error {
	return func(r *meta.Registry) error {
		if err := r.ClaimNamespace("audit"); err != nil {
			return err
		}
		k, err := meta.Register[bool](r, meta.KeySpec{
			Name: promisedKey,
			Contract: &meta.Completeness{
				On: []symbol.Kind{symbol.KindStruct}, By: diag.PhaseAnnotate, Severity: severity,
			},
			Doc: "marks a struct the fixture's annotator saw",
		})
		p.key = k
		return err
	}
}

// keeper returns an annotator stamping the promised key on every
// struct, through the handle the registration fills.
func (p *promise) keeper() plugin.Annotator {
	return stamper("keeper", func(_ *eidos.StructMatch, st *eidos.Stamper) error {
		eidos.Stamp(st, p.key, true)
		return nil
	})
}

// keptAndGone returns a composition of two plans writing under a and
// b: the first run of every case that removes the second plan.
func keptAndGone(tb assert.TB, root string) *workspace.Workspace {
	tb.Helper()

	return built(tb, onDisk(tb, root, diskPlan(tb, "kept", centralised("a")), diskPlan(tb, "gone", centralised("b"))))
}

// Close runs over the plans' records on one goroutine: the collisions
// between plans, the sweep of the plans the composition no longer
// declares, and the audit of the completeness contracts.
func TestClose(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("reports PlanCollision for two plans that route a file to one path", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "left", layout.Config{}), diskPlan(t, "right", layout.Config{})))
			report, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the collision fails the run")
			collided := findings(report.Sink, workspace.PlanCollision)
			assert.Length(t, collided, 1, "one finding names both plans")
			assert.Contains(t, collided[0].Msg, `"left"`, "the first plan")
			assert.Contains(t, collided[0].Msg, `"right"`, "and the second")
			assert.Equal(t, collided[0].Related, []position.Pos{alphaAt}, "the first plan's file is related")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "neither plan commits")
			assert.Equal(t, report.Plans[1].Status, workspace.PlanFailed, "neither plan commits")
			assert.True(t, absent(root, storeGen), "nothing is written")
		})

		t.Run("reports PlanCollision for two paths that differ only in case", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			upper := diskPlan(t, "upper", layout.Config{})
			upper.Backend = printerAs(t, "upper-printer", "fixture", "GEN.txt")
			w := built(t, onDisk(t, root, diskPlan(t, "lower", layout.Config{}), upper))
			report, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the clash fails the run")
			assert.Length(t, findings(report.Sink, workspace.PlanCollision), 1, "one finding for the clash")
		})

		t.Run("removes the outputs of a plan the composition no longer declares", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			cleanRun(t, keptAndGone(t, root), routedIn(t, coretest.StorePath))

			report := cleanRun(t, built(t, onDisk(t, root, diskPlan(t, "kept", centralised("a")))),
				routedIn(t, coretest.StorePath))
			assert.True(t, absent(root, "b/"+storeGen), "the removed plan's file is removed")
			assert.Equal(t, report.Swept, []output.Written{{Path: "b/" + storeGen, Action: output.ActionDeleted}},
				"the report records the removal")
			assert.Equal(t, paths(recorded(t, root)), []string{"a/" + storeGen},
				"the record lists the plan that remains")
		})

		t.Run("keeps a removed plan's output edited since its stamp", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			cleanRun(t, keptAndGone(t, root), routedIn(t, coretest.StorePath))
			changed := edited(read(t, root, "b/"+storeGen))
			place(t, root, "b/"+storeGen, changed)

			report := cleanRun(t, built(t, onDisk(t, root, diskPlan(t, "kept", centralised("a")))),
				routedIn(t, coretest.StorePath))
			assert.Length(t, findings(report.Sink, workspace.KeptOutput), 1, "one warning for the kept file")
			assert.Equal(t, read(t, root, "b/"+storeGen), changed, "the edited file remains")
			assert.Empty(t, report.Swept, "nothing is removed")
			assert.Equal(t, paths(recorded(t, root)), []string{"a/" + storeGen, "b/" + storeGen},
				"and keeps its entry")
		})

		t.Run("removes nothing of a removed plan in a dry run", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			cleanRun(t, keptAndGone(t, root), routedIn(t, coretest.StorePath))

			report, err := built(t, onDisk(t, root, diskPlan(t, "kept", centralised("a")))).
				Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.StorePath), Dry: true})
			assert.NoError(t, err, "the dry run is clean")
			assert.False(t, absent(root, "b/"+storeGen), "the removed plan's file remains")
			assert.Empty(t, report.Swept, "nothing is removed")
			assert.Equal(t, paths(report.Manifest), []string{"a/" + storeGen}, "the record it would commit drops it")
		})

		sweeps := []struct {
			name   string
			output func() (output.Sink, error)
			dry    bool
			marker string
		}{
			{
				name:   "returns an error for a sweep whose output fails to open",
				output: func() (output.Sink, error) { return nil, errNoDevice },
				marker: "sweep: open the output",
			},
			{
				name:   "returns an error for a sweep whose sink fails to prepare",
				output: faultyOutput(func(f *faulty) { f.prepareErr = errDiskFull }),
				marker: "sweep: prepare the output",
			},
			{
				name:   "returns an error for a sweep whose sink fails to commit",
				output: faultyOutput(func(f *faulty) { f.commitErr = errDiskFull }),
				marker: "sweep: commit the output",
			},
			{
				name:   "returns an error for a dry sweep whose sink fails to discard",
				output: faultyOutput(func(f *faulty) { f.discardErr = errReadOnly }),
				dry:    true,
				marker: "sweep: discard the output",
			},
		}
		for _, tt := range sweeps {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gone := manifest.Entry{Path: "b/" + storeGen, Plan: "gone", Hash: "sha256:" + strings.Repeat("ab", 32)}
				mem := recording(t, gone)
				w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "kept", centralised("a"))).
					Ledger(func() (ledger.Ledger, error) { return mem, nil }).
					Output(tt.output))
				_, err := w.Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.StorePath), Dry: tt.dry})
				assert.HasError(t, err, "the sweep fails the run")
				assert.Contains(t, err.Error(), tt.marker, "the error names the step")
			})
		}

		t.Run("returns an error for a removed plan's entry the sink refuses to remove", func(t *testing.T) {
			t.Parallel()

			mem := recording(t, stageEntry("gone"))
			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "kept", centralised("a"))).
				Ledger(func() (ledger.Ledger, error) { return mem, nil }))
			_, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.HasError(t, err, "the sweep fails the run")
			assert.Contains(t, err.Error(), "sweep: stage the removal of svc/old.txt.stage",
				"the error names the entry")
		})

		t.Run("reports UnmetContract for a struct its key's contract promises", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			var p promise
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})).
				Keys(p.registration(diag.SeverityError)))
			report, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the unmet contract fails the run")
			unmet := findings(report.Sink, workspace.UnmetContract)
			assert.Length(t, unmet, 1, "one finding for the struct")
			assert.Equal(t, unmet[0].Pos, alphaAt, "at the declaration")
			assert.Contains(t, unmet[0].Msg, string(promisedKey), "naming the key")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "no plan commits")
			assert.True(t, absent(root, storeGen), "nothing is written")
		})

		t.Run("reports UnmetContract at the contract's severity", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			var p promise
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})).
				Keys(p.registration(diag.SeverityWarning)))
			report := cleanRun(t, w, routedIn(t, coretest.StorePath))
			unmet := findings(report.Sink, workspace.UnmetContract)
			assert.Length(t, unmet, 1, "one finding for the struct")
			assert.Equal(t, unmet[0].Severity, diag.SeverityWarning, "at the declared severity")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanCommitted, "a warning commits")
		})

		t.Run("reports nothing for a key stamped on every struct it promises", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			var p promise
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})).
				Keys(p.registration(diag.SeverityError)).Annotators(p.keeper()))
			report := cleanRun(t, w, routedIn(t, coretest.StorePath))
			assert.Empty(t, findings(report.Sink, workspace.UnmetContract), "the contract is met")
		})

		t.Run("audits the declarations of the units the load parsed at full depth", func(t *testing.T) {
			t.Parallel()

			var p promise
			w := built(t, workspace.New().
				Brand(fixtureBrand).
				Frontends(frontendtest.NewScriptedDependent()).
				Keys(p.registration(diag.SeverityWarning)).
				Targets("fixture").
				Plans(planTo("plan", "fixture", mirror("mirror"))))
			report, err := w.Run(t.Context(), workspace.Input{
				Tree: fstest.MapFS{
					"svc/a/a.zz": {Data: []byte("package svc/a\nimport dep dep/b\ntype A dep.B\n")},
				},
				Stores: map[string]fs.FS{
					frontendtest.ScriptedStore: fstest.MapFS{"dep/b/b.zz": {Data: []byte("package dep/b\ntype B\n")}},
				},
			})
			assert.NoError(t, err, "the run is clean")
			unmet := findings(report.Sink, workspace.UnmetContract)
			assert.Length(t, unmet, 1, "the workspace's struct alone")
			assert.Equal(t, unmet[0].Pos.File, "svc/a/a.zz", "the dependency's struct is not audited")
		})
	})
}
