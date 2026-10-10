// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The paths the disk fixtures write: the store package's file beside
// its source, and the cache package's, where the store's moves.
const (
	storeGen = coretest.StorePath + "/gen.txt"
	cacheGen = coretest.CachePath + "/gen.txt"
)

// alphaAt is where every disk fixture declares Alpha.
var alphaAt = position.Pos{File: alphaFile, Line: 3, Col: 1}

// faulty is an in-memory sink with scripted faults: a staging, a
// preparation, a commit or a discard that fails, a verdict it reports
// on every path, and a cancellation it delivers when it stages a file.
type faulty struct {
	*output.Mem
	writeErr, prepareErr, commitErr, discardErr error
	found                                       output.Found
	cancel                                      context.CancelFunc
}

// Write delivers the cancellation, then stages or fails.
func (f *faulty) Write(path string, body []byte) error {
	if f.cancel != nil {
		f.cancel()
	}
	if f.writeErr != nil {
		return f.writeErr
	}
	return f.Mem.Write(path, body)
}

// Prepare fails, or reports the memory sink's changes with the
// scripted verdict.
func (f *faulty) Prepare() ([]output.Change, error) {
	if f.prepareErr != nil {
		return nil, f.prepareErr
	}
	changes, err := f.Mem.Prepare()
	if f.found != 0 {
		for i := range changes {
			changes[i].Found = f.found
		}
	}
	return changes, err
}

// Commit fails, or commits into memory.
func (f *faulty) Commit() ([]output.Written, error) {
	if f.commitErr != nil {
		return nil, f.commitErr
	}
	return f.Mem.Commit()
}

// Discard discards, then fails where it is scripted to.
func (f *faulty) Discard() error {
	if err := f.Mem.Discard(); err != nil {
		return err
	}
	return f.discardErr
}

// plainSink is a sink that does not write over a drifted or foreign
// file: it offers the methods of the sink it wraps that [output.Sink]
// declares, and not Overwrite.
type plainSink struct{ output.Sink }

// errBroken is the error the broken plan's generator returns.
var errBroken = errors.New("the generator is broken")

// exportReader is a generator recording whether it ran and the
// exports its context handed over.
type exportReader struct {
	name plugin.ID
	ran  bool
	got  map[string]plugin.ExportDoc
}

// Name returns the reader's name.
func (r *exportReader) Name() plugin.ID { return r.name }

// Generate records the run and the exports.
func (r *exportReader) Generate(ctx *plugin.GeneratorContext) error {
	r.ran, r.got = true, ctx.Exports
	return nil
}

// contextReader is a generator recording the target, the spoke and the
// policy that its context handed over.
type contextReader struct {
	name   plugin.ID
	target plugin.Target
	types  plugin.TypeSpeller
	policy plugin.Policy
}

// Name returns the reader's name.
func (r *contextReader) Name() plugin.ID { return r.name }

// Generate records the target, the spoke and the policy.
func (r *contextReader) Generate(ctx *plugin.GeneratorContext) error {
	r.target, r.types, r.policy = ctx.Target, ctx.Types, ctx.Policy
	return nil
}

// A plan stages its files and the removal of its stale outputs into a
// sink of its own, and what the destination contains decides whether
// the plan may commit.
func TestPlan(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the target, the spoke and the policy of the plan to each generator", func(t *testing.T) {
			t.Parallel()

			reader := &contextReader{name: "reader"}
			printer := lowerBackend(t, clientBackend, lowerTarget, camel, widthPolicy)
			w := built(t, workspace.New().Brand(fixtureBrand).Targets(lowerTarget).Plans(workspace.Plan{
				Name: clientPlan, Generators: []plugin.Generator{reader}, Backend: printer,
			}))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			expect.Equal(t, reader.target, lowerTarget, "the generator reads the plan's target")
			expect.Equal(t, reader.types, printer.(plugin.TypeSpeller), "the generator reads the backend's spoke",
				assert.ByIdentity())
			expect.Equal(t, reader.policy.Choice(widthKey), narrow, "the generator reads the plan's policy")
		})

		t.Run("passes a nil spoke for a backend without the spoke role", func(t *testing.T) {
			t.Parallel()

			reader := &contextReader{name: "reader"}
			printer := fakeBackend{name: "printer", target: "fixture"}
			w := built(t, workspace.New().Brand(fixtureBrand).Targets("fixture").Plans(workspace.Plan{
				Name: "plan", Generators: []plugin.Generator{reader}, Backend: printer,
			}))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			assert.Nil(t, reader.types, "the generator does not receive a spoke")
		})

		t.Run("commits a plan's file beside its source", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			report := cleanRun(t, w, routedIn(t, coretest.StorePath))
			assert.Equal(t, report.Plans[0].Status, workspace.PlanCommitted, "the plan commits")
			assert.Equal(t, report.Plans[0].Changes[0].Action, output.ActionCreated, "the file is new")
			assert.Contains(
				t,
				files.Read(t, filepath.Join(root, storeGen)),
				"type ForAlpha struct{}",
				"the file is on disk",
			)
		})

		t.Run("reports DriftedOutput for a generated file edited since its stamp", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			changed := edited(files.Read(t, filepath.Join(root, storeGen)))
			place(t, root, storeGen, changed)

			report, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the drift fails the run")
			drifted := findings(report.Sink, workspace.DriftedOutput)
			assert.Length(t, drifted, 1, "one finding for the edited file")
			assert.Equal(t, drifted[0].Pos, alphaAt, "at the file's first declaration's origin")
			assert.Contains(t, drifted[0].Msg, `plan "plan"`, "naming the plan that generated it")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "the plan commits nothing")
			files.HasContent(t, filepath.Join(root, storeGen), changed, "the edit remains")
		})

		t.Run("lists a write over a drifted file in Refused", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			place(t, root, storeGen, edited(files.Read(t, filepath.Join(root, storeGen))))

			report, _ := runOver(t, w, routedIn(t, coretest.StorePath))
			refused := report.Plans[0].Refused
			assert.Length(t, refused, 1, "the run refuses one write")
			expect.Equal(t, refused[0].Path, storeGen, "the change has the path of the edited file")
			expect.Equal(t, refused[0].Found, output.FoundDrifted, "the change found the drifted file")
		})

		t.Run("names the plan the record lists for a drifted file", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			cleanRun(t, built(t, onDisk(t, root, diskPlan(t, "old", layout.Config{}))), routedIn(t, coretest.StorePath))
			place(t, root, storeGen, edited(files.Read(t, filepath.Join(root, storeGen))))

			report, err := runOver(t, built(t, onDisk(t, root, diskPlan(t, "new", layout.Config{}))),
				routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the drift fails the run")
			drifted := findings(report.Sink, workspace.DriftedOutput)
			assert.Length(t, drifted, 1, "one finding for the edited file")
			assert.Contains(t, drifted[0].Msg, `plan "old"`, "naming the plan the record lists")
		})

		t.Run("names the routing plan in a drift finding the record lists no file for", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			cleanRun(t, built(t, onDisk(t, root, diskPlan(t, "old", layout.Config{}))), routedIn(t, coretest.StorePath))
			place(t, root, storeGen, edited(files.Read(t, filepath.Join(root, storeGen))))
			assert.NoError(t, os.RemoveAll(filepath.Join(root, ledger.StateDir(fixtureBrand))),
				"a fresh clone has no state directory")

			report, _ := runOver(t, built(t, onDisk(t, root, diskPlan(t, "new", layout.Config{}))),
				routedIn(t, coretest.StorePath))
			drifted := findings(report.Sink, workspace.DriftedOutput)
			assert.Length(t, drifted, 1, "one finding for the edited file")
			assert.Contains(t, drifted[0].Msg, `plan "new"`, "naming the plan that routes it")
		})

		t.Run("stops a plan before its layout after a cancellation", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			var o opener
			cancelling := generator("canceller", func(m *eidos.StructMatch, e *eidos.Emitter) error {
				cancel()
				return mirrored(m, e)
			})
			plan := diskPlan(t, "plan", layout.Config{})
			plan.Generators = []plugin.Generator{cancelling}
			w := built(t, onDisk(t, t.TempDir(), plan).Output(o.open))
			report, err := w.Run(ctx, workspace.Input{Graph: routedIn(t, coretest.StorePath)})
			assert.ErrorIs(t, err, context.Canceled, "the cancellation is returned")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanCancelled, "the plan is cancelled")
			assert.Empty(t, o.opened(), "and opens no sink")
		})

		t.Run("stops a plan before its preparation after a cancellation", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Output(faultyOutput(func(f *faulty) { f.cancel = cancel })))
			report, err := w.Run(ctx, workspace.Input{Graph: routedIn(t, coretest.StorePath)})
			assert.ErrorIs(t, err, context.Canceled, "the cancellation is returned")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanCancelled, "the plan is cancelled")
		})

		t.Run("returns an error for a sink that fails to prepare", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Output(faultyOutput(func(f *faulty) { f.prepareErr = errDiskFull })))
			report, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, errDiskFull, "the preparation's error fails the run")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "the plan fails")
		})

		t.Run("returns an error for a sink that fails to discard a refused plan's staging", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Output(faultyOutput(func(f *faulty) { f.found, f.discardErr = output.FoundDrifted, errReadOnly })))
			report, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, errReadOnly, "the discard's error is returned")
			assert.Length(t, findings(report.Sink, workspace.DriftedOutput), 1, "beside the finding")
		})

		t.Run("returns both errors for a sink that fails to stage and to discard", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Output(faultyOutput(func(f *faulty) { f.writeErr, f.discardErr = errReadOnly, errNoDevice })))
			_, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.That(t, err).
				ErrorIs(errReadOnly, "the staging's error is returned").
				ErrorIs(errNoDevice, "and so is the discard's")
		})

		t.Run("returns an error for an output that fails to open the sink of the withheld changes", func(t *testing.T) {
			t.Parallel()

			var opens atomic.Int64
			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Output(func() (output.Sink, error) {
					if opens.Add(1) > 1 {
						return nil, errNoDevice
					}
					return output.NewMem(), nil
				}))
			report, err := w.Run(t.Context(), workspace.Input{
				Graph: routedIn(t, coretest.StorePath), Patterns: []string{storePattern},
			})
			assert.ErrorIs(t, err, errNoDevice, "the open's error fails the run")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "the plan fails")
		})

		t.Run("returns an error for a sink that fails to prepare the withheld changes", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Output(faultyOutput(func(f *faulty) { f.prepareErr = errDiskFull })))
			_, err := w.Run(t.Context(), workspace.Input{
				Graph: routedIn(t, coretest.StorePath), Patterns: []string{storePattern},
			})
			assert.ErrorIs(t, err, errDiskFull, "the preparation's error fails the run")
			assert.Contains(t, err.Error(), "prepare the withheld changes", "the error names the step")
		})

		t.Run("returns an error for a sink that fails to discard the withheld changes", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Output(faultyOutput(func(f *faulty) { f.discardErr = errReadOnly })))
			_, err := w.Run(t.Context(), workspace.Input{
				Graph: routedIn(t, coretest.StorePath), Patterns: []string{storePattern},
			})
			assert.ErrorIs(t, err, errReadOnly, "the discard's error fails the run")
			assert.Contains(t, err.Error(), "discard the withheld changes", "the error names the step")
		})

		t.Run("returns both errors for a sink that fails to stage a withheld change and to discard it",
			func(t *testing.T) {
				t.Parallel()

				w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
					Output(faultyOutput(func(f *faulty) { f.writeErr, f.discardErr = errReadOnly, errNoDevice })))
				_, err := w.Run(t.Context(), workspace.Input{
					Graph: routedIn(t, coretest.StorePath), Patterns: []string{coretest.CachePath},
				})
				assert.That(t, err).
					ErrorIs(errReadOnly, "the staging's error is returned").
					ErrorIs(errNoDevice, "and so is the discard's")
			})

		t.Run("returns an error for a stale entry the sink refuses to stage among the withheld changes",
			func(t *testing.T) {
				t.Parallel()

				entry := stageEntry("plan")
				entry.Plugins = []plugin.ID{"plan-mirror"}
				mem := recording(t, entry)
				w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
					Ledger(func() (ledger.Ledger, error) { return mem, nil }))
				_, err := w.Run(t.Context(), workspace.Input{
					Graph: routedIn(t, coretest.StorePath), Patterns: []string{storePattern},
				})
				assert.HasError(t, err, "the run fails")
				assert.Contains(t, err.Error(), "the removal of svc/old.txt.stage", "the error names the entry")
			})

		t.Run("returns an error for a stale entry the sink refuses to remove", func(t *testing.T) {
			t.Parallel()

			mem := recording(t, stageEntry("plan"))
			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Ledger(func() (ledger.Ledger, error) { return mem, nil }))
			_, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.HasError(t, err, "the run fails")
			assert.Contains(t, err.Error(), "the removal of svc/old.txt.stage", "the error names the entry")
		})

		t.Run("reports ForeignFile for a hand-written file at a generated path", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{storeGen: files.Text("written by hand\n")})
			report, err := runOver(t, built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{}))),
				routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the foreign file fails the run")
			foreign := findings(report.Sink, workspace.ForeignFile)
			assert.Length(t, foreign, 1, "one finding for the hand-written file")
			assert.Equal(t, foreign[0].Pos, alphaAt, "at the file's first declaration's origin")
			assert.Contains(t, foreign[0].Msg, "Struct ForAlpha", "naming the declaration routed there by its kind")
			files.HasContent(t, filepath.Join(root, storeGen), "written by hand\n", "the file remains")
		})

		t.Run("lists a write over a foreign file in Refused", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{storeGen: files.Text("written by hand\n")})
			report, _ := runOver(t, built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{}))),
				routedIn(t, coretest.StorePath))
			refused := report.Plans[0].Refused
			assert.Length(t, refused, 1, "the run refuses one write")
			expect.Equal(t, refused[0].Path, storeGen, "the change has the path of the hand-written file")
			expect.Equal(t, refused[0].Found, output.FoundForeign, "the change found the foreign file")
		})

		t.Run("writes over a generated file edited since its stamp under OverwriteDrift", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			generated := files.Read(t, filepath.Join(root, storeGen))
			place(t, root, storeGen, edited(generated))

			in := workspace.Input{Graph: routedIn(t, coretest.StorePath), OverwriteDrift: true}
			report, err := w.Run(t.Context(), in)
			assert.NoError(t, err, "the run is clean")
			expect.Empty(t, findings(report.Sink, workspace.DriftedOutput), "the run reports no drift")
			expect.Equal(t, report.Plans[0].Status, workspace.PlanCommitted, "the plan commits")
			files.HasContent(t, filepath.Join(root, storeGen), generated, "the generated bytes replace the edit")
		})

		t.Run("writes over a hand-written file at a generated path under Adopt", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{storeGen: files.Text("written by hand\n")})
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			report, err := w.Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.StorePath), Adopt: true})
			assert.NoError(t, err, "the run is clean")
			expect.Empty(t, findings(report.Sink, workspace.ForeignFile), "the run reports no foreign file")
			expect.Equal(t, report.Plans[0].Status, workspace.PlanCommitted, "the plan commits")
			expect.That(t, files.Read(t, filepath.Join(root, storeGen))).
				Contains("type ForAlpha struct{}", "the generated file replaces the hand-written one")
		})

		t.Run("reports ForeignFile for a hand-written file under OverwriteDrift", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{storeGen: files.Text("written by hand\n")})
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			in := workspace.Input{Graph: routedIn(t, coretest.StorePath), OverwriteDrift: true}
			report, err := w.Run(t.Context(), in)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the foreign file fails the run")
			assert.Length(t, findings(report.Sink, workspace.ForeignFile), 1, "the drift input adopts nothing")
		})

		t.Run("reports DriftedOutput under OverwriteDrift for a sink that does not overwrite", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})).
				Output(func() (output.Sink, error) {
					d, err := output.NewDisk(root, fixtureBrand)
					return plainSink{d}, err
				}))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			place(t, root, storeGen, edited(files.Read(t, filepath.Join(root, storeGen))))

			in := workspace.Input{Graph: routedIn(t, coretest.StorePath), OverwriteDrift: true}
			report, err := w.Run(t.Context(), in)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the drift fails the run")
			assert.Length(t, findings(report.Sink, workspace.DriftedOutput), 1, "the sink refuses as before")
		})

		t.Run("returns the sink's error for an overwrite it refuses", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Output(func() (output.Sink, error) {
					return output.NewTee(output.NewMem(), plainSink{output.NewMem()}), nil
				}))
			report, err := w.Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.StorePath), Adopt: true})
			assert.HasError(t, err, "the tee refuses the overwrite")
			expect.Contains(t, err.Error(), "does not write over", "the error names the refusal")
			expect.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "and the plan fails")
		})

		t.Run("adopts a file with the staged bytes that no record lists", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			assert.NoError(t, os.RemoveAll(filepath.Join(root, ledger.StateDir(fixtureBrand))),
				"a fresh clone has no state directory")

			report := cleanRun(t, w, routedIn(t, coretest.StorePath))
			change := report.Plans[0].Changes[0]
			assert.Equal(t, change.Found, output.FoundSame, "the file has the staged bytes")
			assert.Equal(t, change.Action, output.ActionUnchanged, "and is not written")
			assert.Equal(t, paths(recorded(t, root)), []string{storeGen}, "the record adopts it")
		})

		t.Run("removes a stale output of its own plan", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			cleanRun(t, w, routedIn(t, coretest.StorePath))

			report := cleanRun(t, w, routedIn(t, coretest.CachePath))
			files.Absent(t, filepath.Join(root, storeGen), "the stale output is removed")
			assert.Contains(t, report.Plans[0].Changes, output.Change{
				Path: storeGen, Action: output.ActionDeleted, Found: output.FoundIntact,
			}, "the report records the removal")
			assert.Equal(t, paths(recorded(t, root)), []string{cacheGen}, "the record lists the new file alone")
		})

		t.Run("keeps a stale output edited since its stamp under KeptOutput", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			changed := edited(files.Read(t, filepath.Join(root, storeGen)))
			place(t, root, storeGen, changed)

			report := cleanRun(t, w, routedIn(t, coretest.CachePath))
			kept := findings(report.Sink, workspace.KeptOutput)
			assert.Length(t, kept, 1, "one warning for the kept file")
			assert.Equal(t, kept[0].Severity, diag.SeverityWarning, "as a warning")
			assert.Equal(t, kept[0].Pos, position.Pos{File: storeGen}, "at the kept file")
			files.HasContent(t, filepath.Join(root, storeGen), changed, "the edited file remains")
			assert.Equal(t, paths(recorded(t, root)), []string{cacheGen, storeGen}, "and keeps its entry")
		})

		t.Run("keeps a stale output that lost the brand's frame under KeptOutput", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			place(t, root, storeGen, "written by hand\n")

			report := cleanRun(t, w, routedIn(t, coretest.CachePath))
			assert.Length(t, findings(report.Sink, workspace.KeptOutput), 1, "one warning for the kept file")
			files.HasContent(t, filepath.Join(root, storeGen), "written by hand\n", "the hand-written file remains")
			assert.Equal(t, paths(recorded(t, root)), []string{cacheGen}, "and leaves the record")
		})

		t.Run("commits a plan whose sibling's file drifted", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root,
				diskPlan(t, "drifting", centralised("a")), diskPlan(t, "steady", centralised("b"))))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			place(t, root, "a/"+storeGen, edited(files.Read(t, filepath.Join(root, "a", storeGen))))

			report, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the drift fails the run")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "the drifting plan commits nothing")
			assert.Equal(t, report.Plans[1].Status, workspace.PlanCommitted, "its sibling commits")
		})

		t.Run("writes a file another plan generated before without removing it", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			cleanRun(t, built(t, onDisk(t, root,
				diskPlan(t, "first", centralised("out")), diskPlan(t, "second", centralised("other")))),
				routedIn(t, coretest.StorePath))

			report := cleanRun(t, built(t, onDisk(t, root,
				diskPlan(t, "first", centralised("moved")), diskPlan(t, "second", centralised("out")))),
				routedIn(t, coretest.StorePath))
			assert.Contains(t, files.Read(t, filepath.Join(root, "out", storeGen)), "second-mirror",
				"the new plan's file is on disk")
			assert.Contains(t, report.Plans[1].Changes, output.Change{
				Path: "out/" + storeGen, Action: output.ActionUpdated, Found: output.FoundIntact,
				Hash: report.Plans[1].Changes[1].Hash,
			}, "the new plan updates it")
			assert.Equal(t, report.Plans[0].Changes[0].Path, "moved/"+storeGen,
				"and the old plan removes nothing of it")
			for _, e := range recorded(t, root).Files {
				if e.Path == "out/"+storeGen {
					assert.Equal(t, e.Plan, "second", "the record lists it under the new plan")
				}
			}
		})

		t.Run("hands a dependent the export of the plan it depends on", func(t *testing.T) {
			t.Parallel()

			reader := &exportReader{name: "bindings-reader"}
			w := built(t, onDisk(t, t.TempDir(),
				diskPlan(t, "plan", layout.Config{}), dependent(t, "bindings", reader, "plan")))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			assert.Contains(t, reader.got, "plan", "the dependent reads the plan's export")
			assert.Length(t, reader.got["plan"].Symbols, 1, "the export lists the plan's one declaration")
			got := reader.got["plan"].Symbols[0]
			assert.Equal(t, got.ExportKey, plugin.ExportKey{
				Origin: coretest.Struct(coretest.StorePath, "Alpha").ID, Plugin: "plan-mirror", Name: "ForAlpha",
			}, "the declaration's key")
			assert.Equal(t, got.Spelling, "ForAlpha", "the spelling the settle left")
			assert.Equal(t, got.File, storeGen, "the file the declaration was rendered into")
			assert.Equal(t, got.Kind, symbol.KindStruct, "the declaration's kind")
		})

		t.Run("hands a dependent the export of each plan it depends on", func(t *testing.T) {
			t.Parallel()

			reader := &exportReader{name: "bindings-reader"}
			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "a", centralised("a")),
				diskPlan(t, "b", centralised("b")), dependent(t, "bindings", reader, "a", "b")))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			assert.Equal(t, reader.got["a"].Symbols[0].File, "a/"+storeGen, "the first plan's export")
			assert.Equal(t, reader.got["b"].Symbols[0].File, "b/"+storeGen, "the second plan's export")
		})

		t.Run("hands a plan without dependencies no export", func(t *testing.T) {
			t.Parallel()

			reader := &exportReader{name: "alone-reader"}
			w := built(t, onDisk(t, t.TempDir(), dependent(t, "alone", reader)))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			assert.True(t, reader.ran, "the plan generates")
			assert.Empty(t, reader.got, "the exports the plan reads")
		})

		t.Run("generates nothing for a plan whose dependency failed", func(t *testing.T) {
			t.Parallel()

			reader := &exportReader{name: "bindings-reader"}
			w := built(t, onDisk(t, t.TempDir(), failing(t, "plan"), dependent(t, "bindings", reader, "plan")))
			report, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the failing plan fails the run")
			assert.False(t, reader.ran, "the dependent generates nothing")
			assert.Equal(t, report.Plans[1].Status, workspace.PlanFailed, "the dependent commits nothing")
		})

		t.Run("reports FailedDependency at the first Error of the plan a dependent depends on", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), failing(t, "plan"),
				dependent(t, "bindings", &exportReader{name: "bindings-reader"}, "plan")))
			report, _ := runOver(t, w, routedIn(t, coretest.StorePath))
			stood := findings(report.Sink, workspace.FailedDependency)
			assert.Length(t, stood, 1, "one finding for the dependent")
			assert.Equal(t, stood[0].Pos, alphaAt, "at the failed plan's first Error")
			assert.Equal(t, stood[0].Severity, diag.SeverityInfo, "at Info")
			assert.Contains(t, stood[0].Msg, `plan "bindings" depends on plan "plan"`, "naming both plans")
		})

		t.Run("reports FailedDependency at the cause of a chain of dependencies", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), failing(t, "a"),
				dependent(t, "b", &exportReader{name: "b-reader"}, "a"),
				dependent(t, "c", &exportReader{name: "c-reader"}, "b")))
			report, _ := runOver(t, w, routedIn(t, coretest.StorePath))
			stood := findings(report.Sink, workspace.FailedDependency)
			assert.Length(t, stood, 2, "one finding for each dependent")
			assert.Contains(t, stood[1].Msg, `plan "c" depends on plan "b"`, "naming the plan the last one depends on")
			assert.Equal(t, stood[1].Pos, alphaAt, "at the first Error of the chain")
		})

		t.Run("reports no FailedDependency for a dependency that failed on a returned error alone", func(t *testing.T) {
			t.Parallel()

			reader := &exportReader{name: "bindings-reader"}
			w := built(t, onDisk(t, t.TempDir(), broken(t, "plan"), dependent(t, "bindings", reader, "plan")))
			report, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, errBroken, "the generator's error is returned")
			assert.False(t, reader.ran, "the dependent generates nothing")
			assert.Equal(t, report.Plans[1].Status, workspace.PlanFailed, "the dependent commits nothing")
			assert.Empty(t, findings(report.Sink, workspace.FailedDependency), "and nothing explains it")
		})

		t.Run("cancels a plan whose dependency was cancelled", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			cancelled := workspace.Plan{
				Name: "plan",
				Generators: []plugin.Generator{generator("plan-canceller",
					func(*eidos.StructMatch, *eidos.Emitter) error {
						cancel()
						return context.Canceled
					})},
				Backend: printerAs(t, "plan-printer", "fixture", ""),
			}
			reader := &exportReader{name: "bindings-reader"}
			w := built(t, onDisk(t, t.TempDir(), cancelled, dependent(t, "bindings", reader, "plan")))
			report, err := w.Run(ctx, workspace.Input{Graph: routedIn(t, coretest.StorePath)})
			assert.ErrorIs(t, err, context.Canceled, "the cancellation is returned")
			assert.False(t, reader.ran, "the dependent generates nothing")
			assert.Equal(t, report.Plans[1].Status, workspace.PlanCancelled, "the dependent is cancelled")
		})
	})
}

// diskPlan returns a plan mirroring every struct through a printer of
// its own under a layout.
func diskPlan(tb assert.TB, name string, cfg layout.Config) workspace.Plan {
	tb.Helper()

	return workspace.Plan{
		Name:       name,
		Generators: []plugin.Generator{mirror(plugin.ID(name + "-mirror"))},
		Backend:    printerAs(tb, plugin.ID(name+"-printer"), "fixture", ""),
		Layout:     cfg,
	}
}

// centralised returns the layout that writes every file under dir.
func centralised(dir string) layout.Config {
	return layout.Config{Policy: layout.PolicyCentralised, Dir: dir}
}

// onDisk returns a composition writing its plans into a disk sink at
// root, with the state directory's ledger under root.
func onDisk(tb assert.TB, root string, plans ...workspace.Plan) *workspace.Builder {
	tb.Helper()

	return workspace.New().
		Brand(fixtureBrand).
		Annotators(stamper("noter", quiet)).
		Targets("fixture").
		Plans(plans...).
		Output(func() (output.Sink, error) { return output.NewDisk(root, fixtureBrand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, fixtureBrand) })
}

// built builds a composition, failing the test where it does not
// compose.
func built(tb assert.TB, b *workspace.Builder) *workspace.Workspace {
	tb.Helper()

	w, err := b.Build()
	assert.NoError(tb, err, "the fixture composes")
	return w
}

// routedIn returns an unfrozen graph that declares Alpha in each package
// of pkgs, in a file of the package's own directory.
func routedIn(tb assert.TB, pkgs ...string) *store.Graph {
	tb.Helper()

	g := store.New()
	for _, pkg := range pkgs {
		s := coretest.Struct(pkg, "Alpha")
		s.Pos = position.Pos{File: pkg + "/alpha.go", Line: 3, Col: 1}
		p := coretest.Package(pkg, s)
		p.Files[0].Path = pkg + "/alpha.go"
		assert.NoError(tb, g.AddPackage(p), "the fixture package is admitted")
	}
	return g
}

// runOver runs a composition over a graph and returns the report and
// the run's error.
func runOver(t *testing.T, w *workspace.Workspace, g *store.Graph) (*workspace.Report, error) {
	t.Helper()

	return w.Run(t.Context(), workspace.Input{Graph: g})
}

// cleanRun runs a composition over a graph and fails the test where
// the run is not clean.
func cleanRun(t *testing.T, w *workspace.Workspace, g *store.Graph) *workspace.Report {
	t.Helper()

	report, err := runOver(t, w, g)
	assert.NoError(t, err, "the run is clean")
	return report
}

// place writes content at a path of the root, the way a person edits a
// file.
func place(t *testing.T, root, path, content string) {
	t.Helper()

	at := filepath.Join(root, filepath.FromSlash(path))
	assert.NoError(t, os.MkdirAll(filepath.Dir(at), 0o755), "the file's directory is made")
	assert.NoError(t, os.WriteFile(at, []byte(content), 0o644), "the file is placed")
}

// edited returns a generated file's bytes with its body changed and its
// trailer kept: what a person's edit of a generated file leaves.
func edited(stamped string) string {
	return strings.Replace(stamped, "type For", "type Edited", 1)
}

// recorded returns the record in the root's state directory.
func recorded(t *testing.T, root string) manifest.Manifest {
	t.Helper()

	l, err := ledger.OpenDir(root, fixtureBrand)
	assert.NoError(t, err, "the ledger opens")
	return recordIn(t, l)
}

// recordIn returns the record a ledger contains: its manifest's
// documents, joined.
func recordIn(t *testing.T, l ledger.Ledger) manifest.Manifest {
	t.Helper()

	m, _, err := state.ReadManifest(t.Context(), l)
	assert.NoError(t, err, "the record reads")
	return m
}

// paths returns the paths a record lists, in its order.
func paths(m manifest.Manifest) []string {
	out := make([]string, 0, len(m.Files))
	for _, e := range m.Files {
		out = append(out, e.Path)
	}
	return out
}

// findings returns the findings of one code in a sink.
func findings(sink *diag.Sink, code diag.Code) []diag.Diag {
	var out []diag.Diag
	for d := range sink.All() {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

// faultyOutput returns an output opening a fresh faulty sink shaped by
// script for every plan.
func faultyOutput(script func(*faulty)) func() (output.Sink, error) {
	return func() (output.Sink, error) {
		f := &faulty{Mem: output.NewMem()}
		script(f)
		return f, nil
	}
}

// recording returns a memory ledger with one previous record.
func recording(t *testing.T, entries ...manifest.Entry) *ledger.Mem {
	t.Helper()

	mem := ledger.NewMem()
	_, err := state.WriteManifest(t.Context(), mem, manifest.Manifest{Version: manifest.Version, Files: entries}, nil)
	assert.NoError(t, err, "the previous record commits")
	return mem
}

// stageEntry is a previous record's entry under a plan whose path a
// sink refuses to stage: it ends in the reserved staging suffix.
func stageEntry(plan string) manifest.Entry {
	return manifest.Entry{Path: "svc/old.txt.stage", Plan: plan, Hash: "sha256:" + strings.Repeat("ab", 32)}
}

// dependent returns a plan that depends on deps and runs the reader
// through a printer of its own. It routes no file.
func dependent(tb assert.TB, name string, r *exportReader, deps ...string) workspace.Plan {
	tb.Helper()

	return workspace.Plan{
		Name:       name,
		DependsOn:  deps,
		Generators: []plugin.Generator{r},
		Backend:    printerAs(tb, plugin.ID(name+"-printer"), "fixture", ""),
	}
}

// failing returns a plan whose one generator reports an Error at every
// struct it sees, through a printer of its own.
func failing(tb assert.TB, name string) workspace.Plan {
	tb.Helper()

	return workspace.Plan{
		Name: name,
		Generators: []plugin.Generator{generator(plugin.ID(name+"-refuser"),
			func(m *eidos.StructMatch, _ *eidos.Emitter) error {
				m.Errorf(runCode, "%s is refused", m.Struct.Name)
				return nil
			})},
		Backend: printerAs(tb, plugin.ID(name+"-printer"), "fixture", ""),
	}
}

// broken returns a plan whose one generator returns errBroken and
// reports nothing, through a printer of its own.
func broken(tb assert.TB, name string) workspace.Plan {
	tb.Helper()

	return workspace.Plan{
		Name: name,
		Generators: []plugin.Generator{generator(plugin.ID(name+"-breaker"),
			func(*eidos.StructMatch, *eidos.Emitter) error { return errBroken })},
		Backend: printerAs(tb, plugin.ID(name+"-printer"), "fixture", ""),
	}
}
