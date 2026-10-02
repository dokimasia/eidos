// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
)

// errRecord is the refusal the crashing ledger returns from its commit.
var errRecord = errors.New("the state directory is gone")

// aged is the mtime a case sets on a file, so a write moves it.
var aged = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

// crashing is a ledger whose commit fails: the window between the last
// plan's commit and the record.
type crashing struct{}

// BeginRun returns the empty manifest.
func (crashing) BeginRun(context.Context) (manifest.Manifest, error) {
	return manifest.Manifest{Version: manifest.Version}, nil
}

// CommitRun returns errRecord.
func (crashing) CommitRun(context.Context, manifest.Manifest) error { return errRecord }

// cancelling is a sink that cancels the run's context when it commits:
// a cancellation that arrives between two plans' commits.
type cancelling struct {
	output.Sink
	cancel context.CancelFunc
}

// Commit cancels the run, then commits.
func (c cancelling) Commit() ([]output.Written, error) {
	c.cancel()
	return c.Sink.Commit()
}

// meddling is a sink that edits a file of the root before it commits:
// a person's edit between the preparation and the commit.
type meddling struct {
	output.Sink
	at, content string
}

// Commit writes the edit, then commits.
func (m meddling) Commit() ([]output.Written, error) {
	if err := os.WriteFile(m.at, []byte(m.content), 0o644); err != nil {
		return nil, err
	}
	return m.Sink.Commit()
}

// digestOf returns a file's digest the way a record spells it.
func digestOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// mtimeOf returns the mtime of a path of the root.
func mtimeOf(t *testing.T, root, path string) time.Time {
	t.Helper()

	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
	assert.NoError(t, err, "the file stats")
	return info.ModTime()
}

// age sets the mtime of a path of the root to aged.
func age(t *testing.T, root, path string) {
	t.Helper()

	assert.NoError(t, os.Chtimes(filepath.Join(root, filepath.FromSlash(path)), aged, aged), "the file ages")
}

// The commit is two-phase: every plan stages, the clean plans commit in
// composition order, and the ledger records the merged manifest
// strictly after the last of them.
func TestCommit(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("records each file with its plan, digest, plugins and sources", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			mem := ledger.NewMem()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})).
				Ledger(func() (ledger.Ledger, error) { return mem, nil }))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			got, err := mem.BeginRun(t.Context())
			assert.NoError(t, err, "the record reads")
			assert.Equal(t, got.Files, []manifest.Entry{{
				Path:    storeGen,
				Plan:    "plan",
				Hash:    digestOf(read(t, root, storeGen)),
				Plugins: []plugin.ID{"plan-mirror"},
				Sources: []string{coretest.Struct(coretest.StorePath, "Alpha").ID.String()},
			}}, "the record of the one file")
		})

		t.Run("records the workspace's name in its manifest", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})).Workspace("platform"))
			report := cleanRun(t, w, routedIn(t, coretest.StorePath))
			assert.Equal(t, report.Manifest.Workspace, "platform", "the report's record")
			assert.Equal(t, recorded(t, root).Workspace, "platform", "and the ledger's")
		})

		t.Run("records nothing for a dry run", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			mem := ledger.NewMem()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})).
				Ledger(func() (ledger.Ledger, error) { return mem, nil }))
			report, err := w.Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.StorePath), Dry: true})
			assert.NoError(t, err, "the dry run is clean")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanPrepared, "the plan prepares")
			assert.Equal(t, report.Plans[0].Changes[0].Action, output.ActionCreated, "the change it would make")
			assert.Equal(t, paths(report.Manifest), []string{storeGen}, "the record it would commit")
			assert.True(t, absent(root, storeGen), "nothing is written")
			assert.Equal(t, mem.Writes(), 0, "and nothing is recorded")
		})

		t.Run("records nothing for a run that commits no plan", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			mem := ledger.NewMem()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})).
				Ledger(func() (ledger.Ledger, error) { return mem, nil }))
			g := routedIn(t, coretest.StorePath)
			assert.NoError(t, g.AttachDirectives(coretest.Struct(coretest.StorePath, "Alpha").ID,
				[]directive.Raw{{Name: unclaimedName}}), "an unclaimed directive attaches before the seal")
			_, err := runOver(t, w, g)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails")
			assert.Equal(t, mem.Writes(), 0, "nothing is recorded")
		})

		t.Run("returns an error for a sink that fails to commit", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Output(faultyOutput(func(f *faulty) { f.commitErr = errDiskFull })))
			report, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, errDiskFull, "the commit's error fails the run")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "the plan fails")
		})

		t.Run("returns an error for a dry run's sink that fails to discard", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Output(faultyOutput(func(f *faulty) { f.discardErr = errReadOnly })))
			report, err := w.Run(t.Context(), workspace.Input{Graph: routedIn(t, coretest.StorePath), Dry: true})
			assert.ErrorIs(t, err, errReadOnly, "the discard's error is returned")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanPrepared, "the plan prepared")
		})

		t.Run("keeps a removed plan's entries when the run commits nothing", func(t *testing.T) {
			t.Parallel()

			gone := manifest.Entry{Path: "b/" + storeGen, Plan: "gone", Hash: "sha256:" + strings.Repeat("ab", 32)}
			mem := recording(t, gone)
			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Ledger(func() (ledger.Ledger, error) { return mem, nil }))
			g := routedIn(t, coretest.StorePath)
			assert.NoError(t, g.AttachDirectives(coretest.Struct(coretest.StorePath, "Alpha").ID,
				[]directive.Raw{{Name: unclaimedName}}), "an unclaimed directive attaches before the seal")
			report, err := runOver(t, w, g)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails")
			assert.Equal(t, report.Manifest.Files, []manifest.Entry{gone}, "the removed plan's entry remains")
		})

		t.Run("keeps the previous entries of a plan that fails", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			place(t, root, storeGen, edited(read(t, root, storeGen)))

			report, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the drift fails the run")
			assert.Equal(t, paths(report.Manifest), []string{storeGen}, "the failed plan's entry remains")
		})

		t.Run("skips the commit of every plan after a cancellation", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			mem := ledger.NewMem()
			ctx, cancel := context.WithCancel(t.Context())
			first, second := diskPlan(t, "first", centralised("a")), diskPlan(t, "second", centralised("b"))
			w := built(t, onDisk(t, root, first, second).
				Ledger(func() (ledger.Ledger, error) { return mem, nil }).
				Output(func() (output.Sink, error) {
					d, err := output.NewDisk(root, fixtureBrand)
					return cancelling{Sink: d, cancel: cancel}, err
				}))
			report, err := w.Run(ctx, workspace.Input{Graph: routedIn(t, coretest.StorePath)})
			assert.ErrorIs(t, err, context.Canceled, "the cancellation is returned")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanCommitted, "the first plan committed")
			assert.Equal(t, report.Plans[1].Status, workspace.PlanCancelled, "the second is cancelled")
			assert.True(t, absent(root, "b/"+storeGen), "the cancelled plan writes nothing")
			assert.Equal(t, mem.Writes(), 1, "the record matches the destination")
			got, _ := mem.BeginRun(t.Context())
			assert.Equal(t, paths(got), []string{"a/" + storeGen}, "it lists the committed plan's file")
		})

		t.Run("derives the same bytes after a crash between the last commit and the record", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			crashed := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})).
				Ledger(func() (ledger.Ledger, error) { return crashing{}, nil }))
			_, err := runOver(t, crashed, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, errRecord, "the record fails")
			age(t, root, storeGen)

			report := cleanRun(t, built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{}))),
				routedIn(t, coretest.StorePath))
			change := report.Plans[0].Changes[0]
			assert.Equal(t, change.Found, output.FoundSame, "the derived bytes are on disk")
			assert.Equal(t, change.Action, output.ActionUnchanged, "so nothing is written")
			assert.True(t, mtimeOf(t, root, storeGen).Equal(aged), "and no mtime moves")
			assert.Equal(t, paths(recorded(t, root)), []string{storeGen}, "the record is written")
		})

		t.Run("writes nothing on a second run over an unchanged tree", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			age(t, root, storeGen)
			age(t, root, ledger.ManifestPath(fixtureBrand))

			report := cleanRun(t, w, routedIn(t, coretest.StorePath))
			assert.Equal(t, report.Plans[0].Changes[0].Action, output.ActionUnchanged, "nothing changes")
			assert.True(t, mtimeOf(t, root, storeGen).Equal(aged), "the file's mtime does not move")
			assert.True(t, mtimeOf(t, root, ledger.ManifestPath(fixtureBrand)).Equal(aged),
				"and neither does the record's")
		})

		t.Run("keeps the entry of a stale output edited between the preparation and the commit", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			w := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})))
			cleanRun(t, w, routedIn(t, coretest.StorePath))
			changed := edited(read(t, root, storeGen))
			meddled := built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{})).
				Output(func() (output.Sink, error) {
					d, err := output.NewDisk(root, fixtureBrand)
					at := filepath.Join(root, filepath.FromSlash(storeGen))
					return meddling{Sink: d, at: at, content: changed}, err
				}))

			report := cleanRun(t, meddled, routedIn(t, coretest.CachePath))
			assert.Equal(t, read(t, root, storeGen), changed, "the commit leaves the edited file")
			assert.Contains(t, report.Plans[0].Changes, output.Change{
				Path: storeGen, Action: output.ActionUnchanged, Found: output.FoundIntact,
			}, "the report records no removal")
			assert.Equal(t, paths(recorded(t, root)), []string{cacheGen, storeGen}, "and the record keeps its entry")
		})
	})
}
