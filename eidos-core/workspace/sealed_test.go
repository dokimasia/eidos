// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
)

// The cases read and break these ledger paths: the sealed state's
// directory, its pointer to the live generation, the directory of its
// segments, and one manifest document.
const (
	sealedDir      = "state/"
	currentBlob    = "state/CURRENT"
	segmentDir     = "state/seg"
	brokenDocument = "manifest/ea.json"
)

// sealedSource is the source file of the tree the sealed-state cases
// load.
const sealedSource = "svc/store/row.zz"

// The bytes of the template file a generator's tree contains, before and
// after an edit.
const (
	firstTemplate  = "{{.Name}}\n"
	editedTemplate = "{{.Name}} edited\n"
)

// anotherBuild is the path of the executable a forged generation states
// it was written by.
const anotherBuild = "/opt/another/build"

// errStateRead is the failure the blind ledger returns for a read of
// the sealed state.
var errStateRead = errors.New("the state directory does not read")

// blind is a memory ledger whose reads of the sealed state fail: a
// ledger failing for a cause outside the source.
type blind struct {
	*ledger.Mem
}

// Read returns errStateRead for a name under the sealed state's
// directory, and reads the memory ledger otherwise.
func (b blind) Read(ctx context.Context, name string) ([]byte, error) {
	if strings.HasPrefix(name, sealedDir) {
		return nil, errStateRead
	}
	return b.Mem.Read(ctx, name)
}

// unreadable is a template tree that lists its template file and fails
// to read it: a tree the run cannot fold whole into the generation's
// header.
type unreadable struct{ fstest.MapFS }

// Open returns an error wrapping fs.ErrPermission for the template file,
// and opens every other name of the map.
func (u unreadable) Open(name string) (fs.File, error) {
	if name == templateFile {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return u.MapFS.Open(name)
}

// ReadFile returns an error wrapping fs.ErrPermission for the template
// file, and reads every other name of the map.
func (u unreadable) ReadFile(name string) ([]byte, error) {
	if name == templateFile {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrPermission}
	}
	return u.MapFS.ReadFile(name)
}

// counting is a template tree that counts the reads of its files, for a
// case that pins how often a run reads a tree. It is not safe for
// concurrent use.
type counting struct {
	fstest.MapFS

	reads int
}

// ReadFile adds one to the count of reads and returns the map's bytes
// for name.
func (c *counting) ReadFile(name string) ([]byte, error) {
	c.reads++
	return c.MapFS.ReadFile(name)
}

// The sealed state is what a run over a tree compares the tree with, so
// when a run reads it, when it ignores it and why, and when it writes
// the next generation are each pinned.
func TestSealed(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a generation for a run over a tree", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			report := sealedRun(t, sealing(t, mem, "plan"), workspace.Input{Tree: sealedTree()})
			assert.True(t, report.Stats.Generation, "the run reports the generation")
			g, err := state.Open(t.Context(), mem)
			assert.NoError(t, err, "the live generation opens")
			assert.Equal(t, g.Header.Parent, "", "and is a cold run's")
		})

		t.Run("writes no generation for a run over a graph", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			sealedRun(t, sealing(t, mem, "plan"), workspace.Input{Graph: routedIn(t, coretest.StorePath)})
			_, err := state.Open(t.Context(), mem)
			assert.ErrorIs(t, err, fs.ErrNotExist, "a graph has no tree to compare")
		})

		t.Run("writes nothing for a dry run over a tree", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			sealedRun(t, sealing(t, mem, "plan"), workspace.Input{Tree: sealedTree(), Dry: true})
			assert.Equal(t, mem.Writes(), 0, "a dry run records nothing")
		})

		t.Run("writes no generation for a run whose record does not read", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			assert.NoError(t, mem.Write(t.Context(), brokenDocument, []byte("not a record\n")),
				"a broken document writes")
			report := sealedRun(t, sealing(t, mem, "plan"), workspace.Input{Tree: sealedTree()})
			assert.Length(t, findings(report.Sink, workspace.UnreadableRecord), 1, "the record does not read")
			_, err := state.Open(t.Context(), mem)
			assert.ErrorIs(t, err, fs.ErrNotExist, "and the run writes no generation over it")
		})

		t.Run("reads the generation the last run wrote", func(t *testing.T) {
			t.Parallel()

			w := sealing(t, ledger.NewMem(), "plan")
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			report := sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			assert.False(t, report.Stats.Cold, "the second run is warm")
			assert.Length(t, findings(report.Sink, workspace.ColdState), 0, "and reports no ColdState")
		})

		t.Run("writes no blob for a second run over an unchanged tree", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := sealing(t, mem, "plan")
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			writes := mem.Writes()
			report := sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			assert.Equal(t, mem.Writes(), writes, "the run changes no record")
			assert.False(t, report.Stats.Generation, "and makes no generation live")
		})

		t.Run("writes a cold run's generation for Input.Cold", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := sealing(t, mem, "plan")
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			report := sealedRun(t, w, workspace.Input{Tree: sealedTree(), Cold: true})
			assert.True(t, report.Stats.Cold, "the run ignores the generation")
			assert.Length(t, findings(report.Sink, workspace.ColdState), 0, "and reports nothing for it")
			g, err := state.Open(t.Context(), mem)
			assert.NoError(t, err, "the live generation opens")
			assert.Equal(t, g.Header.Parent, "", "and has no parent")
		})

		t.Run("reports ColdState for a generation of another composition", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			sealedRun(t, sealing(t, mem, "plan"), workspace.Input{Tree: sealedTree()})
			report := sealedRun(t, sealing(t, mem, "renamed"), workspace.Input{Tree: sealedTree()})
			assert.True(t, report.Stats.Cold, "the run ignores the generation")
			cold := findings(report.Sink, workspace.ColdState)
			assert.Length(t, cold, 1, "one ColdState states why")
			assert.Contains(t, cold[0].Msg, "composition changed", "the composition is the cause")
		})

		t.Run("reports ColdState for a generation of another executable", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := sealing(t, mem, "plan")
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			g, err := state.Open(t.Context(), mem)
			assert.NoError(t, err, "the live generation opens")
			h := g.Header
			h.Executable.Path = anotherBuild
			h.Executable.Digest = sha256.Sum256([]byte(anotherBuild))
			_, err = state.NewCommit(nil, g.Manifest).Write(t.Context(), mem, h, recordIn(t, mem))
			assert.NoError(t, err, "another build's generation commits")

			report := sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			assert.True(t, report.Stats.Cold, "the run ignores the generation")
			cold := findings(report.Sink, workspace.ColdState)
			assert.Length(t, cold, 1, "one ColdState states why")
			assert.Contains(t, cold[0].Msg, "executable changed", "the executable is the cause")
		})

		t.Run("reports ColdState after an edit to a template a plan renders through", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			place(t, dir, templateFile, firstTemplate)
			w := built(t, sealingOf(t, ledger.NewMem(), "plan", templated("mirror", os.DirFS(dir))))
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			place(t, dir, templateFile, editedTemplate)
			report := sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			assert.True(t, report.Stats.Cold, "the run ignores the generation")
			cold := findings(report.Sink, workspace.ColdState)
			assert.Length(t, cold, 1, "one ColdState states why")
			assert.Contains(t, cold[0].Msg, "composition changed", "the composition is the cause")
		})

		t.Run("reads the generation over template trees the last run read", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			place(t, dir, templateFile, firstTemplate)
			w := built(t, sealingOf(t, ledger.NewMem(), "plan", templated("mirror", os.DirFS(dir))))
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			report := sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			assert.False(t, report.Stats.Cold, "the second run is warm")
			assert.Length(t, findings(report.Sink, workspace.ColdState), 0, "and reports no ColdState")
		})

		t.Run("reads the generation over a template file that does not read", func(t *testing.T) {
			t.Parallel()

			tree := unreadable{fstest.MapFS{templateFile: {Data: []byte(firstTemplate)}}}
			w := built(t, sealingOf(t, ledger.NewMem(), "plan", templated("mirror", tree)))
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			report := sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			assert.False(t, report.Stats.Cold, "the error folds alike in both runs")
		})

		t.Run("reads the generation of the same composition with its plans in another order", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			first := sealedPlan(t, "first",
				templated("first-mirror", fstest.MapFS{templateFile: {Data: []byte(firstTemplate)}}))
			second := sealedPlan(t, "second",
				templated("second-mirror", fstest.MapFS{templateFile: {Data: []byte(firstTemplate)}}))
			sealedRun(t, built(t, sealingPlans(mem, first, second)), workspace.Input{Tree: sealedTree()})
			report := sealedRun(t, built(t, sealingPlans(mem, second, first)), workspace.Input{Tree: sealedTree()})
			assert.False(t, report.Stats.Cold, "the trees fold in the order of their generators")
		})

		t.Run("reports ColdState after an edit to a template of a second plan", func(t *testing.T) {
			t.Parallel()

			edited := fstest.MapFS{templateFile: {Data: []byte(firstTemplate)}}
			first := sealedPlan(t, "first",
				templated("first-mirror", fstest.MapFS{templateFile: {Data: []byte(firstTemplate)}}))
			second := sealedPlan(t, "second", templated("second-mirror", edited))
			w := built(t, sealingPlans(ledger.NewMem(), first, second))
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			edited[templateFile] = &fstest.MapFile{Data: []byte(editedTemplate)}
			report := sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			cold := findings(report.Sink, workspace.ColdState)
			assert.Length(t, cold, 1, "one ColdState states why")
			assert.Contains(t, cold[0].Msg, "composition changed", "the composition is the cause")
		})

		t.Run("reads a template once for two plans that share its generator", func(t *testing.T) {
			t.Parallel()

			tree := &counting{MapFS: fstest.MapFS{templateFile: {Data: []byte(firstTemplate)}}}
			shared := templated("shared", tree)
			first, second := sealedPlan(t, "first", shared), sealedPlan(t, "second", shared)
			w := built(t, sealingPlans(ledger.NewMem(), first, second))
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			assert.Equal(t, tree.reads, 1, "the run folds the shared tree once")
		})

		t.Run("reports ColdState at CURRENT for a CURRENT that names no generation", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := sealing(t, mem, "plan")
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			assert.NoError(t, mem.Write(t.Context(), currentBlob, []byte("garbage\n")), "CURRENT breaks")
			report := sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			cold := findings(report.Sink, workspace.ColdState)
			assert.Length(t, cold, 1, "one ColdState states why")
			assert.Equal(t, cold[0].Pos.File, ledger.StateDir(fixtureBrand)+"/"+currentBlob,
				"at the state directory's CURRENT")
		})

		t.Run("runs again cold over a damaged run segment", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := sealing(t, mem, "plan")
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			truncate(t, mem, func(live int) bool { return live == 0 })
			report := sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			cold := findings(report.Sink, workspace.ColdState)
			assert.Length(t, cold, 1, "one ColdState states the damage")
			assert.Contains(t, cold[0].Msg, "started again cold", "and the cold run")
			assert.True(t, report.Stats.Cold, "the report is the cold run's")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanCommitted, "which commits")
		})

		t.Run("runs again cold over a damaged region", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := sealing(t, mem, "plan")
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			truncate(t, mem, func(live int) bool { return live > 0 })
			report := sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			cold := findings(report.Sink, workspace.ColdState)
			assert.Length(t, cold, 1, "one ColdState states the damage")
			assert.Contains(t, cold[0].Msg, "started again cold", "and the cold run")
			g, err := state.Open(t.Context(), mem)
			assert.NoError(t, err, "the cold run's generation opens")
			assert.Equal(t, g.Header.Parent, "", "and has no parent")
		})

		t.Run("returns the error of a ledger that fails to read the state", func(t *testing.T) {
			t.Parallel()

			w := sealing(t, blind{Mem: ledger.NewMem()}, "plan")
			_, err := w.Run(t.Context(), workspace.Input{Tree: sealedTree()})
			assert.ErrorIs(t, err, errStateRead, "the ledger's own error returns")
		})

		t.Run("returns the error of a ledger that fails to write the state", func(t *testing.T) {
			t.Parallel()

			w := sealing(t, crashing{Mem: ledger.NewMem()}, "plan")
			_, err := w.Run(t.Context(), workspace.Input{Tree: sealedTree()})
			assert.ErrorIs(t, err, errRecord, "the ledger's own error returns")
		})
	})
}

// sealedTree returns a tree of one package the scripted frontend parses.
func sealedTree() fstest.MapFS {
	return fstest.MapFS{sealedSource: {Data: []byte("package svc/store\ntype Row int string\n")}}
}

// sealing returns a composition over the scripted frontend that records
// into a ledger and renders one plan of a name into memory.
func sealing(tb assert.TB, l ledger.Ledger, plan string) *workspace.Workspace {
	tb.Helper()

	return built(tb, sealingBuilder(tb, l, plan))
}

// sealingBuilder returns the builder of the composition [sealing]
// returns, for a case that adds to it.
func sealingBuilder(tb assert.TB, l ledger.Ledger, plan string) *workspace.Builder {
	tb.Helper()

	return sealingOf(tb, l, plan, mirror("mirror"))
}

// sealingOf returns the builder of a composition over the scripted
// frontend that records into a ledger and renders one plan of a name,
// whose one generator is gen, into memory.
func sealingOf(tb assert.TB, l ledger.Ledger, plan string, gen plugin.Generator) *workspace.Builder {
	tb.Helper()

	return sealingPlans(l, workspace.Plan{
		Name:       plan,
		Generators: []plugin.Generator{gen},
		Backend:    printer(tb, "fixture"),
	})
}

// sealingPlans returns the builder of a composition over the scripted
// frontend that records into a ledger and renders the plans into memory.
func sealingPlans(l ledger.Ledger, plans ...workspace.Plan) *workspace.Builder {
	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Annotators(stamper("noter", quiet)).
		Targets("fixture").
		Plans(plans...).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Ledger(func() (ledger.Ledger, error) { return l, nil })
}

// sealedPlan returns a plan of a name whose one generator is gen. A
// printer named after the plan renders every unit into a file named after
// the plan, so two plans of one composition never write to one path.
func sealedPlan(tb assert.TB, name string, gen plugin.Generator) workspace.Plan {
	tb.Helper()

	return workspace.Plan{
		Name:       name,
		Generators: []plugin.Generator{gen},
		Backend:    printerAs(tb, plugin.ID(name)+"-printer", "fixture", name+".txt"),
	}
}

// sealedRun runs a composition and fails the test where the run is not
// clean.
func sealedRun(t *testing.T, w *workspace.Workspace, in workspace.Input) *workspace.Report {
	t.Helper()

	report, err := w.Run(t.Context(), in)
	assert.NoError(t, err, "the run is clean")
	return report
}

// truncate cuts every segment of the live generation whose count of live
// regions picks, the region segment where the count is above zero and
// the run segment where it is zero, to its first byte: the files table's
// run, which the gate reads first, begins the run segment.
func truncate(t *testing.T, mem *ledger.Mem, pick func(live int) bool) {
	t.Helper()

	g, err := state.Open(t.Context(), mem)
	assert.NoError(t, err, "the live generation opens")
	blobs, err := mem.List(t.Context(), segmentDir)
	assert.NoError(t, err, "the segments list")
	cut := 0
	for _, b := range blobs {
		if !pick(g.Live(b.Name)) {
			continue
		}
		body, err := mem.Read(t.Context(), b.Name)
		assert.NoError(t, err, "the segment reads")
		assert.NoError(t, mem.Write(t.Context(), b.Name, body[:1]), "the cut segment writes")
		cut++
	}
	assert.Equal(t, cut, 1, "one segment is cut")
}
