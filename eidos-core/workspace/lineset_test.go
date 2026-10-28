// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/workspace"
)

// The line-set family of the lister: the name and the template of its
// file, the path of the file under the directory of the plan, and the
// template, which writes the name of each record on a line of its own.
const (
	lockName     = "fixture.lock"
	lockTemplate = "lock.tmpl"
	lockPath     = planDir + "/" + lockName
	lockText     = "{{range .Records}}{{.Decl.Name}}\n{{end}}"
	lockPlan     = "locks"
)

// keptName is the struct whose line the lister lets go, and keptLine
// declares it in the warm source.
const (
	keptName = "Kept"
	keptLine = "type " + keptName + " int\n"
)

// lostCheck is the code of the finding that the lister returns for a
// removed line.
var lostCheck = diag.Code{Prefix: "tst", Number: 12}

func TestLineSet(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the finding of the generator for a line that the committed file lost", func(t *testing.T) {
			t.Parallel()

			root := lockRoot(t, rowLine, colLine)
			lines := strings.Split(files.Read(t, filepath.Join(root, filepath.FromSlash(lockPath))), "\n")
			edit(t, root, rowLine)
			report, err := built(t, lockBuilder(root, lockPlan, planDir, true)).
				Run(t.Context(), workspace.Input{Tree: os.DirFS(root), Check: true})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the check fails the run")
			lost := findings(report.Sink, lostCheck)
			assert.Length(t, lost, 1, "one line is lost")
			expect.Equal(t, lost[0].Msg, "Col is lost", "the finding is the one of the generator")
			expect.Equal(t, lost[0].Pos, position.Pos{File: lockPath, Line: slices.Index(lines, "Col") + 1},
				"the finding is at the line of the committed file")
			expect.Equal(t, lost[0].Origin, listerID, "the finding has the origin of the generator")
		})

		t.Run("reports OutOfDate for a line-set file that only lost lines", func(t *testing.T) {
			t.Parallel()

			root := lockRoot(t, rowLine, colLine)
			edit(t, root, rowLine)
			report, _ := built(t, lockBuilder(root, lockPlan, planDir, true)).
				Run(t.Context(), workspace.Input{Tree: os.DirFS(root), Check: true})
			stale := findings(report.Sink, workspace.OutOfDate)
			assert.Length(t, stale, 1, "the file is out of date")
			assert.Equal(t, stale[0].Pos, position.Pos{File: lockPath}, "at the path of the file")
		})

		t.Run("reports nothing for a line that the fresh file adds", func(t *testing.T) {
			t.Parallel()

			root := lockRoot(t, rowLine)
			edit(t, root, rowLine, colLine)
			report, _ := built(t, lockBuilder(root, lockPlan, planDir, true)).
				Run(t.Context(), workspace.Input{Tree: os.DirFS(root), Check: true})
			expect.Empty(t, findings(report.Sink, lostCheck), "no line is lost")
			expect.Length(t, findings(report.Sink, workspace.OutOfDate), 1, "the file is out of date")
		})

		t.Run("reports nothing for a line-set file that the tree does not contain", func(t *testing.T) {
			t.Parallel()

			root := lockRoot(t, rowLine, colLine)
			assert.NoError(t, os.Remove(filepath.Join(root, filepath.FromSlash(lockPath))), "the file is removed")
			edit(t, root, rowLine)
			report, _ := built(t, lockBuilder(root, lockPlan, planDir, true)).
				Run(t.Context(), workspace.Input{Tree: os.DirFS(root), Check: true})
			expect.Empty(t, findings(report.Sink, lostCheck), "no committed line is read")
		})

		t.Run("reports nothing for a line whose removal the generator accepts", func(t *testing.T) {
			t.Parallel()

			root := lockRoot(t, rowLine, keptLine)
			edit(t, root, rowLine)
			report, _ := built(t, lockBuilder(root, lockPlan, planDir, true)).
				Run(t.Context(), workspace.Input{Tree: os.DirFS(root), Check: true})
			expect.Empty(t, findings(report.Sink, lostCheck), "the generator reports no finding for Kept")
			expect.Length(t, findings(report.Sink, workspace.OutOfDate), 1, "the file is out of date")
		})

		t.Run("reports nothing for a removed line of a data file that is no line set", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			assert.NoError(t, os.CopyFS(root, warmTree(rowLine, colLine)), "the tree copies into the run's directory")
			w := built(t, lockBuilder(root, lockPlan, planDir, false))
			_, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
			assert.NoError(t, err, "the first run is clean")
			edit(t, root, rowLine)
			report, _ := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root), Check: true})
			expect.Empty(t, findings(report.Sink, lostCheck), "the file is no line set")
			expect.Length(t, findings(report.Sink, workspace.OutOfDate), 1, "the file is out of date")
		})
	})
}

// lockBuilder returns a composition over the scripted frontend whose one
// plan, named plan, writes the name of each struct of the tree into the
// data file of the lister under dir, a line set where lineSet is true. It
// writes into a disk sink at root, with the ledger of the state directory
// under root. The lister reports each removed line as an Error under
// lostCheck, except the line of Kept, whose removal it accepts.
func lockBuilder(root, plan, dir string, lineSet bool) *workspace.Builder {
	lister := eidos.NewPlugin(listerID).
		Output(plugin.Output{Per: plugin.PerPlan, Word: "lock", Name: lockName, Template: lockTemplate, LineSet: lineSet}).
		Templates(fstest.MapFS{lockTemplate: {Data: []byte(lockText)}}).
		Removed(func(r plugin.RemovedLine) (diag.Diag, bool) {
			if r.Text == keptName {
				return diag.Diag{}, false
			}
			return diag.Diag{Code: lostCheck, Severity: diag.SeverityError, Msg: r.Text + " is lost"}, true
		}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			e.PlanFile().Append(&emit.Struct{Name: m.Struct.Name})
			return nil
		})).Generator()
	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Targets("fixture").
		Plans(workspace.Plan{
			Name: plan, Generators: []plugin.Generator{lister}, Backend: fixturePrinter(),
			Layout: layout.Config{Dir: dir},
		}).
		Output(func() (output.Sink, error) { return output.NewDisk(root, fixtureBrand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, fixtureBrand) })
}

// lockRoot returns a new root with the warm source of lines, after one run
// of the lister over it.
func lockRoot(t *testing.T, lines ...string) string {
	t.Helper()

	root := t.TempDir()
	assert.NoError(t, os.CopyFS(root, warmTree(lines...)), "the tree copies into the run's directory")
	w := built(t, lockBuilder(root, lockPlan, planDir, true))
	_, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
	assert.NoError(t, err, "the first run is clean")
	return root
}

// edit writes the warm source of lines into root.
func edit(t *testing.T, root string, lines ...string) {
	t.Helper()

	files.Write(t, root, files.Tree{warmSource: files.Text(string(warmTree(lines...)[warmSource].Data))})
}
