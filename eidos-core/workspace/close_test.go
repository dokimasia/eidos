// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// promisedKey is the fixture key whose contract the audit checks.
const promisedKey meta.KeyName = "audit.seen"

// followerID names the annotator that stamps the promised key on the user.
const followerID plugin.ID = "follower"

// narrowWidth is the number of fields the row has before an edit widens
// it.
const narrowWidth = 2

// userID is the struct of the rounds tree's api package.
var userID = symbol.Identity{Lang: frontendtest.ScriptedLang, Package: "svc/api", Name: "User", Kind: symbol.KindStruct}

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

// follower is an annotator that implements its role directly. It stamps
// the promised key on the user where the row is wider than narrowWidth,
// so an edit of the row changes a fact of a struct the edit leaves alone.
type follower struct{ p *promise }

// Name returns the follower's name.
func (follower) Name() plugin.ID { return followerID }

// Annotate looks the row up, and stamps the promised key on the user
// where the row is wide.
func (f follower) Annotate(ctx *plugin.AnnotatorContext) error {
	row, held := ctx.Reader.Lookup(rowID)
	s, is := row.(*node.Struct)
	if !held || !is || len(s.Fields) <= narrowWidth {
		return nil
	}
	return meta.Stamp(ctx.Facts, f.p.key, true, meta.Claim{Subject: userID, Bucket: ctx.Bucket, Plugin: followerID})
}

// errCheck is the error the returning check returns.
var errCheck = errors.New("the check cannot read its records")

// recordingCheck is a workspace check that reads the plans it names,
// records each context it is called with, appends its name to a log
// checks share, runs a script over the context and returns err.
type recordingCheck struct {
	name   plugin.ID
	reads  []string
	called []*plugin.CheckContext
	log    *[]plugin.ID
	script func(ctx *plugin.CheckContext)
	err    error
}

// Name returns the check's name.
func (c *recordingCheck) Name() plugin.ID { return c.name }

// Reads returns the plans the check names.
func (c *recordingCheck) Reads() []string { return c.reads }

// Check records the call, runs the script and returns the check's
// error.
func (c *recordingCheck) Check(ctx *plugin.CheckContext) error {
	c.called = append(c.called, ctx)
	if c.log != nil {
		*c.log = append(*c.log, c.name)
	}
	if c.script != nil {
		c.script(ctx)
	}
	return c.err
}

// Close runs over the plans' records on one goroutine: the collisions
// between plans, the sweep of the plans the composition no longer
// declares, the audit of the completeness contracts, and the workspace
// checks.
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
			files.Absent(t, filepath.Join(root, storeGen), "nothing is written")
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

		t.Run("reports PlanCollision for a file at the path of a skipped plan's file", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			left := diskPlan(t, "left", layout.Config{})
			cleanRun(t, built(t, onDisk(t, root, left)), routedIn(t, coretest.StorePath))
			w := built(t, onDisk(t, root, left, diskPlan(t, "right", layout.Config{})))
			report, err := w.Run(t.Context(), workspace.Input{
				Graph: routedIn(t, coretest.StorePath), Plans: []string{"right"},
			})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the collision fails the run")
			assert.Length(t, findings(report.Sink, workspace.PlanCollision), 1, "one finding names both plans")
		})

		t.Run("relates the PlanCollision of a warm run to the path of a skipped plan's file", func(t *testing.T) {
			t.Parallel()

			w := built(t, casePlans(t, ledger.NewMem()))
			before := roundsTree(rowLine, userLine)
			sealedRun(t, w, workspace.Input{Tree: before})
			report, err := w.Run(t.Context(), workspace.Input{
				Tree: editedAfter(before, caseTree()), Plans: []string{"upper"},
			})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the collision fails the run")
			assert.False(t, report.Stats.Cold, "the run reads the sealed state")
			collided := findings(report.Sink, workspace.PlanCollision)
			assert.Length(t, collided, 1, "one finding names both plans")
			assert.Equal(t, collided[0].Related, []position.Pos{{File: storeGenerated}},
				"the skipped plan's file is related at its path")
		})

		t.Run("reports the PlanCollision of a new path that clashes with another plan's kept file", func(t *testing.T) {
			t.Parallel()

			after := caseTree()
			warm, err := warmAfter(t, built(t, casePlans(t, ledger.NewMem())), roundsTree(rowLine, userLine), after)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the collision fails the run")
			got := findings(warm.Sink, workspace.PlanCollision)
			assert.Length(t, got, 1, "one finding names both plans")
			cold, _ := built(t, casePlans(t, ledger.NewMem())).Run(t.Context(), workspace.Input{Tree: after})
			assert.Permutation(t, got, findings(cold.Sink, workspace.PlanCollision),
				"the warm run reports the same finding as the cold run")
		})

		t.Run("removes the outputs of a plan the composition no longer declares", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			cleanRun(t, keptAndGone(t, root), routedIn(t, coretest.StorePath))

			report := cleanRun(t, built(t, onDisk(t, root, diskPlan(t, "kept", centralised("a")))),
				routedIn(t, coretest.StorePath))
			files.Absent(t, filepath.Join(root, "b", storeGen), "the removed plan's file is removed")
			assert.Equal(t, report.Swept, []output.Written{{Path: "b/" + storeGen, Action: output.ActionDeleted}},
				"the report records the removal")
			assert.Equal(t, paths(recorded(t, root)), []string{"a/" + storeGen},
				"the record lists the plan that remains")
		})

		t.Run("keeps a removed plan's output edited since its stamp", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			cleanRun(t, keptAndGone(t, root), routedIn(t, coretest.StorePath))
			changed := edited(files.Read(t, filepath.Join(root, "b", storeGen)))
			place(t, root, "b/"+storeGen, changed)

			report := cleanRun(t, built(t, onDisk(t, root, diskPlan(t, "kept", centralised("a")))),
				routedIn(t, coretest.StorePath))
			assert.Length(t, findings(report.Sink, workspace.KeptOutput), 1, "one warning for the kept file")
			files.HasContent(t, filepath.Join(root, "b", storeGen), changed, "the edited file remains")
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
			files.IsFile(t, filepath.Join(root, "b", storeGen), "the removed plan's file remains")
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

		t.Run("reports OutOfDate for a file that a run would create under Check", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})))
			report, err := w.Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.StorePath), Check: true})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the check fails the run")
			stale := findings(report.Sink, workspace.OutOfDate)
			assert.Length(t, stale, 1, "one finding for the new file")
			expect.Equal(t, stale[0].Pos, position.Pos{File: storeGen}, "at the file's path")
			expect.Equal(t, stale[0].Severity, diag.SeverityError, "as an Error")
		})

		t.Run("writes nothing under Check", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			files.Unchanged(t, os.DirFS(root), func() {
				_, _ = w.Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.CachePath), Check: true})
			}, "the check writes nothing")
		})

		t.Run("reports no OutOfDate under Check for the files that a run leaves as they are", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			report, err := w.Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.StorePath), Check: true})
			assert.NoError(t, err, "the check passes")
			assert.Empty(t, findings(report.Sink, workspace.OutOfDate), "no file is out of date")
		})

		t.Run("reports OutOfDate for a generated file edited since its stamp under Check", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			place(t, root, storeGen, edited(files.Read(t, filepath.Join(root, storeGen))))
			report, _ := w.Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.StorePath), Check: true})
			stale := findings(report.Sink, workspace.OutOfDate)
			assert.Length(t, stale, 1, "one finding for the edited file")
			assert.Equal(t, stale[0].Pos, position.Pos{File: storeGen}, "at the file's path")
		})

		t.Run("reports OutOfDate under Check for a file of a plan that the composition no longer declares",
			func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				cleanRun(t, keptAndGone(t, root), routedIn(t, coretest.StorePath))
				report, err := built(t, onDisk(t, root, diskPlan(t, "kept", centralised("a")))).
					Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.StorePath), Check: true})
				assert.ErrorIs(t, err, workspace.ErrRunFailed, "the check fails the run")
				stale := findings(report.Sink, workspace.OutOfDate)
				assert.Length(t, stale, 1, "one finding for the removed plan's file")
				expect.Equal(t, stale[0].Pos, position.Pos{File: "b/" + storeGen}, "at the file's path")
				expect.That(t, stale[0].Msg).Contains(`plan "gone"`, "naming the plan that the composition dropped")
			})

		t.Run("reports OutOfDate under Check for the changes inside the patterns alone", func(t *testing.T) {
			t.Parallel()

			root, w, _ := onDiskRun(t)
			grown(t, root)
			report, err := w.Run(t.Context(), workspace.Input{
				Tree: os.DirFS(root), Patterns: []string{storePattern}, Check: true,
			})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the check fails the run")
			stale := findings(report.Sink, workspace.OutOfDate)
			assert.Length(t, stale, 1, "one finding for the store package's file")
			assert.Equal(t, stale[0].Pos, position.Pos{File: storeGenerated}, "at the file's path")
		})

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
			files.Absent(t, filepath.Join(root, storeGen), "nothing is written")
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

		t.Run("reports the UnmetContract findings that a cold run over the edited tree reports", func(t *testing.T) {
			t.Parallel()

			var p, q promise
			edited := roundsTree(widerRow, userLine)
			warm, err := warmAfter(t, built(t, sealingBuilder(t, ledger.NewMem(), "plan").
				Keys(p.registration(diag.SeverityWarning))), roundsTree(rowLine, userLine), edited)
			assert.NoError(t, err, "a warning fails no run")
			cold := sealedRun(t, built(t, sealingBuilder(t, ledger.NewMem(), "plan").
				Keys(q.registration(diag.SeverityWarning))), workspace.Input{Tree: edited})
			got := findings(warm.Sink, workspace.UnmetContract)
			assert.Length(t, got, 2, "the edited row and the kept user lack the key")
			assert.Permutation(t, got, findings(cold.Sink, workspace.UnmetContract),
				"the warm run reports the same findings as the cold run")
		})

		t.Run("decodes no region for the audit of a struct that a warm run keeps", func(t *testing.T) {
			t.Parallel()

			var p promise
			before, edited := roundsTree(rowLine, userLine), roundsTree(widerRow, userLine)
			audited, err := warmAfter(t, built(t, sealingBuilder(t, ledger.NewMem(), "plan").
				Keys(p.registration(diag.SeverityWarning))), before, edited)
			assert.NoError(t, err, "a warning fails no run")
			plain, err := warmAfter(t, sealing(t, ledger.NewMem(), "plan"), before, edited)
			assert.NoError(t, err, "the run without a contract is clean")
			assert.Equal(t, audited.Stats.Decoded, plain.Stats.Decoded,
				"the audit decodes the same regions as the run without a contract")
		})

		t.Run("reports the UnmetContract of a kept struct again on the next warm run", func(t *testing.T) {
			t.Parallel()

			var p promise
			w := built(t, sealingBuilder(t, ledger.NewMem(), "plan").Keys(p.registration(diag.SeverityWarning)))
			edited := roundsTree(widerRow, userLine)
			_, err := warmAfter(t, w, roundsTree(rowLine, userLine), edited)
			assert.NoError(t, err, "a warning fails no run")
			report := sealedRun(t, w, workspace.Input{Tree: edited})
			assert.False(t, report.Stats.Cold, "the third run reads the sealed state")
			assert.Length(t, findings(report.Sink, workspace.UnmetContract), 2, "the record keeps both findings")
		})

		t.Run("reports no UnmetContract for a struct that an edit removed", func(t *testing.T) {
			t.Parallel()

			var p promise
			w := built(t, sealingBuilder(t, ledger.NewMem(), "plan").Keys(p.registration(diag.SeverityWarning)))
			report, err := warmAfter(t, w, roundsTree(rowLine+colLine, userLine), roundsTree(rowLine, userLine))
			assert.NoError(t, err, "a warning fails no run")
			assert.Length(t, findings(report.Sink, workspace.UnmetContract), 2, "the column's finding is gone")
		})

		t.Run("reports no UnmetContract for a kept struct that gains its key", func(t *testing.T) {
			t.Parallel()

			var p promise
			w := built(t, sealingBuilder(t, ledger.NewMem(), "plan").
				Keys(p.registration(diag.SeverityWarning)).Annotators(follower{p: &p}))
			report, err := warmAfter(t, w, roundsTree(rowLine, userLine), roundsTree(widerRow, userLine))
			assert.NoError(t, err, "a warning fails no run")
			unmet := findings(report.Sink, workspace.UnmetContract)
			assert.Length(t, unmet, 1, "the follower stamps the user")
			assert.Equal(t, unmet[0].Pos.File, sealedSource, "the row lacks the key")
		})

		t.Run("reports no UnmetContract for a removed struct whose key the warm run withdraws", func(t *testing.T) {
			t.Parallel()

			var p promise
			w := built(t, sealingBuilder(t, ledger.NewMem(), "plan").
				Keys(p.registration(diag.SeverityWarning)).Annotators(p.keeper()))
			report, err := warmAfter(t, w, roundsTree(rowLine+colLine, userLine), roundsTree(rowLine, userLine))
			assert.NoError(t, err, "the run is clean")
			assert.Empty(t, findings(report.Sink, workspace.UnmetContract), "every remaining struct has the key")
		})

		t.Run("hands a check the records of the plans it reads", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			c := &recordingCheck{name: "stubbed", reads: []string{"plan"}}
			cleanRun(t, built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})).Checks(c)),
				routedIn(t, coretest.StorePath))
			assert.Length(t, c.called, 1, "the check runs once")
			plans := c.called[0].Plans
			assert.Length(t, plans, 1, "over one plan's record")
			assert.Equal(t, plans[0].Name, "plan", "the plan it reads")
			assert.Equal(t, plans[0].Files, []manifest.Entry{{
				Path:    storeGen,
				Plan:    "plan",
				Hash:    digestOf(files.Read(t, filepath.Join(root, storeGen))),
				Plugins: []plugin.ID{"plan-mirror"},
				Sources: []string{coretest.Struct(coretest.StorePath, "Alpha").ID.String()},
			}}, "the plan's files as the record lists them")
			assert.Equal(t, plans[0].Export.Symbols[0].Name, "ForAlpha", "the plan's export")
		})

		t.Run("hands a check that names no plan the record of every plan in composition order", func(t *testing.T) {
			t.Parallel()

			c := &recordingCheck{name: "everything"}
			cleanRun(t, built(t, onDisk(t, t.TempDir(),
				diskPlan(t, "b", centralised("b")), diskPlan(t, "a", centralised("a"))).Checks(c)),
				routedIn(t, coretest.StorePath))
			names := make([]string, 0, 2)
			for _, p := range c.called[0].Plans {
				names = append(names, p.Name)
			}
			assert.Equal(t, names, []string{"b", "a"}, "the plans the check reads")
		})

		t.Run("hands a check a reader over the whole graph", func(t *testing.T) {
			t.Parallel()

			var seen, indexed bool
			alphaID := coretest.Struct(coretest.StorePath, "Alpha").ID
			c := &recordingCheck{name: "reading", reads: []string{"plan"}, script: func(ctx *plugin.CheckContext) {
				_, seen = ctx.Reader.Lookup(alphaID)
				_, indexed = ctx.Index.Lookup(alphaID)
			}}
			scoped := diskPlan(t, "plan", layout.Config{})
			scoped.Sources = workspace.Sources{Packages: []string{"./other/..."}}
			cleanRun(t, built(t, onDisk(t, t.TempDir(), scoped).Checks(c)), routedIn(t, coretest.StorePath))
			assert.True(t, seen, "the reader returns a declaration outside the plan's scope")
			assert.True(t, indexed, "and so does the index")
		})

		t.Run("reports a check's findings in the run's sink", func(t *testing.T) {
			t.Parallel()

			c := &recordingCheck{name: "warning", reads: []string{"plan"}, script: func(ctx *plugin.CheckContext) {
				ctx.Sink.Warnf(runCode, alphaAt, ctx.Plugin, "the claim is advisory")
			}}
			report := cleanRun(t, built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).Checks(c)),
				routedIn(t, coretest.StorePath))
			warned := findings(report.Sink, runCode)
			assert.Length(t, warned, 1, "the check's one finding")
			assert.Equal(t, warned[0].Origin, diag.Origin("warning"), "under the check's origin")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanCommitted, "a warning blocks no commit")
		})

		t.Run("blocks every commit for a check's Error", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			c := &recordingCheck{name: "refusing", reads: []string{"plan"}, script: func(ctx *plugin.CheckContext) {
				ctx.Sink.Errorf(runCode, alphaAt, ctx.Plugin, "no stub derives from Alpha")
			}}
			report, err := runOver(t, built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})).Checks(c)),
				routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the check's Error fails the run")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "the plan commits nothing")
			files.Absent(t, filepath.Join(root, storeGen), "nothing is written")
		})

		t.Run("returns a check's error wrapped with its name", func(t *testing.T) {
			t.Parallel()

			c := &recordingCheck{name: "returning", reads: []string{"plan"}, err: errCheck}
			report, err := runOver(t, built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).Checks(c)),
				routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, errCheck, "the check's error is returned")
			assert.Contains(t, err.Error(), "check returning", "naming the check")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "the plan commits nothing")
		})

		t.Run("runs no check after a check returned an error", func(t *testing.T) {
			t.Parallel()

			later := &recordingCheck{name: "later", reads: []string{"plan"}}
			_, err := runOver(t, built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Checks(&recordingCheck{name: "returning", reads: []string{"plan"}, err: errCheck}, later)),
				routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, errCheck, "the first check's error is returned")
			assert.Empty(t, later.called, "the later check does not run")
		})

		t.Run("does not call a check that reads a failed plan", func(t *testing.T) {
			t.Parallel()

			c := &recordingCheck{name: "stubbed", reads: []string{"plan"}}
			_, err := runOver(t, built(t, onDisk(t, t.TempDir(), failing(t, "plan")).Checks(c)),
				routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the failing plan fails the run")
			assert.Empty(t, c.called, "the check does not run")
		})

		t.Run("reports FailedDependency at the first Error of a failed plan a check reads", func(t *testing.T) {
			t.Parallel()

			c := &recordingCheck{name: "stubbed", reads: []string{"plan"}}
			report, _ := runOver(t, built(t, onDisk(t, t.TempDir(), failing(t, "plan")).Checks(c)),
				routedIn(t, coretest.StorePath))
			stood := findings(report.Sink, workspace.FailedDependency)
			assert.Length(t, stood, 1, "one finding for the check")
			assert.Equal(t, stood[0].Pos, alphaAt, "at the failed plan's first Error")
			assert.Equal(t, stood[0].Severity, diag.SeverityInfo, "at Info")
			assert.Contains(t, stood[0].Msg, `check stubbed reads plan "plan"`, "naming the check and the plan")
		})

		t.Run("calls a check that reads no failed plan", func(t *testing.T) {
			t.Parallel()

			c := &recordingCheck{name: "stubbed", reads: []string{"fine"}}
			_, err := runOver(t, built(t, onDisk(t, t.TempDir(),
				failing(t, "plan"), diskPlan(t, "fine", centralised("fine"))).Checks(c)),
				routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the failing plan fails the run")
			assert.Length(t, c.called, 1, "the check over the clean plan runs")
		})

		t.Run("does not call a check that reads a skipped plan", func(t *testing.T) {
			t.Parallel()

			c := &recordingCheck{name: "stubbed", reads: []string{"b"}}
			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "a", centralised("a")),
				diskPlan(t, "b", centralised("b"))).Checks(c))
			_, err := w.Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.StorePath), Plans: []string{"a"}})
			assert.NoError(t, err, "the run is clean")
			assert.Empty(t, c.called, "the check does not run")
		})

		t.Run("reports no FailedDependency for a check whose plan returned an error alone", func(t *testing.T) {
			t.Parallel()

			c := &recordingCheck{name: "stubbed", reads: []string{"plan"}}
			report, err := runOver(t, built(t, onDisk(t, t.TempDir(), broken(t, "plan")).Checks(c)),
				routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, errBroken, "the generator's error is returned")
			assert.Empty(t, c.called, "the check does not run")
			assert.Empty(t, findings(report.Sink, workspace.FailedDependency), "and nothing explains it")
		})

		t.Run("runs no check after an Error in a phase every plan shares", func(t *testing.T) {
			t.Parallel()

			c := &recordingCheck{name: "stubbed", reads: []string{"plan"}}
			g := routedIn(t, coretest.StorePath)
			assert.NoError(t, g.AttachDirectives(coretest.Struct(coretest.StorePath, "Alpha").ID,
				[]directive.Raw{{Name: unclaimedName}}), "an unclaimed directive attaches before the seal")
			_, err := runOver(t, built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).Checks(c)), g)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the shared Error fails the run")
			assert.Empty(t, c.called, "the check does not run")
		})

		t.Run("runs the checks in registration order", func(t *testing.T) {
			t.Parallel()

			var log []plugin.ID
			cleanRun(t, built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Checks(&recordingCheck{name: "zeta", log: &log}).
				Checks(&recordingCheck{name: "alpha", log: &log})),
				routedIn(t, coretest.StorePath))
			assert.Equal(t, log, []plugin.ID{"zeta", "alpha"}, "the order the checks ran in")
		})

		t.Run("calls no check on a warm run without a change", func(t *testing.T) {
			t.Parallel()

			c := &recordingCheck{name: "stubbed", reads: []string{"plan"}}
			w := built(t, sealingBuilder(t, ledger.NewMem(), "plan").Checks(c))
			before := roundsTree(rowLine, userLine)
			sealedRun(t, w, workspace.Input{Tree: before})
			report := sealedRun(t, w, workspace.Input{Tree: before})
			assert.False(t, report.Stats.Cold, "the second run reads the sealed state")
			assert.Length(t, c.called, 1, "the first run alone calls the check")
		})

		t.Run("calls a check again on a warm run after an edit", func(t *testing.T) {
			t.Parallel()

			c := &recordingCheck{name: "stubbed", reads: []string{"plan"}}
			w := built(t, sealingBuilder(t, ledger.NewMem(), "plan").Checks(c))
			_, err := warmAfter(t, w, roundsTree(rowLine, userLine), roundsTree(widerRow, userLine))
			assert.NoError(t, err, "the run is clean")
			assert.Length(t, c.called, 2, "both runs call the check")
		})

		t.Run("calls a check again on a warm run that renders a generated file that vanished", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			before := roundsTree(rowLine, userLine)
			assert.NoError(t, os.CopyFS(root, before), "the tree copies into the run's directory")
			c := &recordingCheck{name: "stubbed", reads: []string{"plan"}}
			w := built(t, sealingOnDisk(t, root).Checks(c))
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root)})
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root)})
			assert.NoError(t, os.Remove(filepath.Join(root, filepath.FromSlash(apiGenerated))),
				"the generated file is removed after a run that recorded its stat")
			report := sealedRun(t, w, workspace.Input{Tree: os.DirFS(root)})
			assert.False(t, report.Stats.Cold, "the third run reads the sealed state")
			assert.Equal(t, report.Stats.Rendered, 1, "the run renders the removed file again")
			assert.Length(t, c.called, 2, "the plan's rendered file calls the check again")
		})

		t.Run("hands a check on a warm run the same export as a cold run", func(t *testing.T) {
			t.Parallel()

			warmCheck := &recordingCheck{name: "stubbed", reads: []string{"plan"}}
			edited := roundsTree(widerRow, userLine)
			_, err := warmAfter(t, built(t, sealingBuilder(t, ledger.NewMem(), "plan").Checks(warmCheck)),
				roundsTree(rowLine, userLine), edited)
			assert.NoError(t, err, "the warm run is clean")
			coldCheck := &recordingCheck{name: "stubbed", reads: []string{"plan"}}
			sealedRun(t, built(t, sealingBuilder(t, ledger.NewMem(), "plan").Checks(coldCheck)),
				workspace.Input{Tree: edited})
			assert.Length(t, warmCheck.called, 2, "both runs call the check")
			assert.Equal(t, warmCheck.called[1].Plans[0].Export, coldCheck.called[0].Plans[0].Export,
				"the warm run hands the check the kept file's rows and the rendered file's rows")
		})

		t.Run("reports the findings of a check that a warm run does not call", func(t *testing.T) {
			t.Parallel()

			c := &recordingCheck{name: "warning", reads: []string{"plan"}, script: func(ctx *plugin.CheckContext) {
				ctx.Sink.Warnf(runCode, alphaAt, ctx.Plugin, "the claim is advisory")
			}}
			w := built(t, sealingBuilder(t, ledger.NewMem(), "plan").Checks(c))
			before := roundsTree(rowLine, userLine)
			sealedRun(t, w, workspace.Input{Tree: before})
			report := sealedRun(t, w, workspace.Input{Tree: before})
			assert.Length(t, c.called, 1, "the second run does not call the check")
			assert.Length(t, findings(report.Sink, runCode), 1, "the second run reports the recorded warning")
		})
	})
}

// casePlans returns the builder of a composition of two plans that
// mirror into files of one name beside their sources: the lower plan
// scoped to the store package and the api package, and the upper plan
// scoped to a package whose directory differs from the store's only in
// case.
func casePlans(tb assert.TB, l ledger.Ledger) *workspace.Builder {
	tb.Helper()

	lower := workspace.Plan{
		Name: "lower", Sources: workspace.Sources{Packages: []string{"svc/store", "svc/api"}},
		Generators: []plugin.Generator{mirror("lower-mirror")}, Backend: printerAs(tb, "lower-printer", "fixture", ""),
	}
	upper := workspace.Plan{
		Name: "upper", Sources: workspace.Sources{Packages: []string{"svc/Store"}},
		Generators: []plugin.Generator{mirror("upper-mirror")}, Backend: printerAs(tb, "upper-printer", "fixture", ""),
	}
	return sealingPlans(l, lower, upper)
}

// keptAndGone returns a composition of two plans writing under a and
// b: the first run of every case that removes the second plan.
func keptAndGone(tb assert.TB, root string) *workspace.Workspace {
	tb.Helper()

	return built(tb, onDisk(tb, root, diskPlan(tb, "kept", centralised("a")), diskPlan(tb, "gone", centralised("b"))))
}
