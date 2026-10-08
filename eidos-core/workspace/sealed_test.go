// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io/fs"
	"math"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
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

// The annotator and the generator of the sealed-state composition, whose
// phase calls the invocation counts name.
const (
	noterID  plugin.ID = "noter"
	mirrorID plugin.ID = "mirror"
)

// The bytes of the template file a generator's tree contains, before and
// after an edit.
const (
	firstTemplate  = "{{.Name}}\n"
	editedTemplate = "{{.Name}} edited\n"
)

// anotherBuild is the path of the executable a forged generation states
// it was written by.
const anotherBuild = "/opt/another/build"

// The recording compositions name these plugins and this check. The
// flagger stamps a flag. The marker runs on each subject that carries the
// mark, and reads the flag. The lookup check looks the row up, and the
// moduler and the direct generator implement their roles directly.
const (
	flaggerID plugin.ID = "flagger"
	markerID  plugin.ID = "marker"
	lookupID  plugin.ID = "lookup"
	modulerID plugin.ID = "moduler"
	directID  plugin.ID = "direct"
)

// The module the moduler states the store package is in, the key whose
// contract nothing meets, and the mark's spelling in the tree.
const (
	storeModule              = "example.test/svc"
	contractKey meta.KeyName = "shape.audited"
	markLine                 = "+marker:mark"
)

// directFinding is the code the moduler, which implements its role
// directly, reports its finding under.
var directFinding = diag.Code{Prefix: "tst", Number: 8}

// The declarations of the sealed tree the recording cases name: the row
// struct and its package, which the scripted frontend loads.
var (
	rowID = symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: "svc/store", Name: "Row", Kind: symbol.KindStruct,
	}
	storePackage = symbol.Identity{Lang: frontendtest.ScriptedLang, Package: "svc/store", Kind: symbol.KindPackage}
)

// markSchema is the directive whose carriers the marker's rule runs on.
var markSchema = directive.Schema{Plugin: string(markerID), Name: "mark", Doc: "marks a row the marker mirrors"}

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

// moduler is an annotator that implements its role directly and journals
// nothing: it looks the row up, stamps the store package's two module
// facts, and reports one warning.
type moduler struct{}

// Name returns the moduler's name.
func (moduler) Name() plugin.ID { return modulerID }

// Annotate looks the row up, stamps the module facts and reports the
// warning.
func (moduler) Annotate(ctx *plugin.AnnotatorContext) error {
	ctx.Reader.Lookup(rowID)
	claim := meta.Claim{Subject: storePackage, Bucket: ctx.Bucket, Plugin: modulerID}
	if err := meta.Stamp(ctx.Facts, ctx.Kernel.Module, storeModule, claim); err != nil {
		return err
	}
	if err := meta.Stamp(ctx.Facts, ctx.Kernel.ModuleRoot, ".", claim); err != nil {
		return err
	}
	ctx.Sink.Warnf(directFinding, position.Pos{File: sealedSource, Line: 2}, modulerID,
		"the moduler states the store package's module")
	return nil
}

// direct is a generator that implements its role directly and journals
// nothing: it declares one family, looks the row up and flushes one unit
// of one struct into the family.
type direct struct{}

// Name returns the generator's name.
func (direct) Name() plugin.ID { return directID }

// Outputs declares the family the generator flushes into.
func (direct) Outputs() []plugin.Output {
	return []plugin.Output{{Per: plugin.PerPackage, Word: "direct"}}
}

// Generate looks the row up and flushes the unit.
func (direct) Generate(ctx *plugin.GeneratorContext) error {
	ctx.Reader.Lookup(rowID)
	return ctx.Emit.Add(plugin.Unit{
		Plugin: directID, Per: plugin.PerPackage, Word: "direct", Key: storePackage.Package, Pkg: storePackage,
		Decls: []symbol.Symbol{&emit.Struct{Origin: rowID, Name: "DirectRow"}},
	})
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
			assert.Empty(t, findings(report.Sink, workspace.ColdState), "and reports no ColdState")
		})

		t.Run("writes no blob for a second run over an unchanged tree", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := sealing(t, mem, "plan")
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			var report *workspace.Report
			assert.Pure(t, mem.Writes, func() { report = sealedRun(t, w, workspace.Input{Tree: sealedTree()}) },
				"the run changes no record")
			assert.False(t, report.Stats.Generation, "and makes no generation live")
		})

		t.Run("writes a cold run's generation for Input.Cold", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := sealing(t, mem, "plan")
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			report := sealedRun(t, w, workspace.Input{Tree: sealedTree(), Cold: true})
			assert.True(t, report.Stats.Cold, "the run ignores the generation")
			assert.Empty(t, findings(report.Sink, workspace.ColdState), "and reports nothing for it")
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

			dir := files.Workspace(t, files.Tree{templateFile: files.Text(firstTemplate)})
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

			dir := files.Workspace(t, files.Tree{templateFile: files.Text(firstTemplate)})
			w := built(t, sealingOf(t, ledger.NewMem(), "plan", templated("mirror", os.DirFS(dir))))
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			report := sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			assert.False(t, report.Stats.Cold, "the second run is warm")
			assert.Empty(t, findings(report.Sink, workspace.ColdState), "and reports no ColdState")
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

		t.Run("runs again cold over a damaged run of a phase table that the run reads", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := sealing(t, mem, "plan")
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			flipLastByte(t, mem)
			widened := editedAfter(sealedTree(), fstest.MapFS{
				sealedSource: {Data: []byte("package svc/store\n" + widerRow)},
			})
			report := sealedRun(t, w, workspace.Input{Tree: widened})
			cold := findings(report.Sink, workspace.ColdState)
			assert.Length(t, cold, 1, "one ColdState states the damage")
			assert.Contains(t, cold[0].Msg, "started again cold", "a read of the phases finds the damage")
			assert.True(t, report.Stats.Cold, "the report is the cold run's")
		})

		t.Run("keeps a damaged run of a phase table that no read of the run needs", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := sealing(t, mem, "plan")
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			flipLastByte(t, mem)
			report := sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			assert.Empty(t, findings(report.Sink, workspace.ColdState), "the run reads no row of the damaged run")
			assert.False(t, report.Stats.Cold, "the run reads the sealed state")
		})

		t.Run("runs again cold over a damaged region that the run reads", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := built(t, planMirroring(t, mem))
			sealedRun(t, w, workspace.Input{Tree: statsTree()})
			truncate(t, mem, func(live int) bool { return live > 0 })
			report := sealedRun(t, w, workspace.Input{Tree: editedStatsTree()})
			cold := findings(report.Sink, workspace.ColdState)
			assert.Length(t, cold, 1, "one ColdState states the damage")
			assert.Contains(t, cold[0].Msg, "started again cold", "and the cold run")
			g, err := state.Open(t.Context(), mem)
			assert.NoError(t, err, "the cold run's generation opens")
			assert.Equal(t, g.Header.Parent, "", "and has no parent")
		})

		t.Run("keeps a damaged region that no read of the run needs", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := sealing(t, mem, "plan")
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			truncate(t, mem, func(live int) bool { return live > 0 })
			report := sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			assert.Empty(t, findings(report.Sink, workspace.ColdState), "the run decodes no region")
			assert.False(t, report.Stats.Cold, "the run reads the sealed state")
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

		t.Run("records a row for every validated subject, fact, invocation and edge", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			flag := &meta.Key[bool]{}
			marked := fstest.MapFS{
				sealedSource: {Data: []byte("package svc/store\ntype Row int string\n" + markLine + "\n")},
			}
			report := sealedRun(t, built(t, phaseRecording(t, mem, flag)), workspace.Input{Tree: marked})
			s := phasesIn(t, mem)

			validation, held, err := s.Validation(rowID)
			assert.NoError(t, err, "the validations table reads")
			assert.True(t, held, "the marked subject's validation is recorded")
			assert.Length(t, validation.Directives, 1, "with the mark that passed")
			stamp := plugin.MatchKey{Plugin: flaggerID, Subject: rowID}
			stamped, held, err := s.Invocation("", stamp)
			assert.NoError(t, err, "the invocations table reads")
			assert.True(t, held, "the annotator's invocation is recorded")
			assert.Equal(t, stamped.Claimed, []meta.FactRef{{Subject: rowID, Key: flag.Name()}},
				"with the fact it claimed")
			read := plugin.MatchKey{Plugin: markerID, Subject: rowID}
			readFact, held, err := s.Invocation("plan", read)
			assert.NoError(t, err, "the invocations table reads")
			assert.True(t, held, "the generator's invocation is recorded")
			checked, held, err := s.Check(lookupID)
			assert.NoError(t, err, "the checks table reads")
			assert.True(t, held, "the check's call is recorded")

			records := []struct {
				ref   state.RecordRef
				reads []state.EdgeHash
			}{
				{ref: state.ValidationRef(rowID), reads: validation.Reads},
				{ref: state.InvocationRef("", stamp), reads: stamped.Reads},
				{ref: state.InvocationRef("plan", read), reads: readFact.Reads},
				{ref: state.CheckRef(lookupID), reads: checked.Reads},
			}
			for _, r := range records {
				assert.NotEmpty(t, r.reads, "each record read an edge")
				for _, h := range r.reads {
					readers, readErr := s.Readers(h)
					assert.NoError(t, readErr, "the readers table reads")
					expect.Contains(t, readers, r.ref, "every edge lists every record that read it")
				}
			}
			assert.Contains(t, readFact.Reads, state.FactEdge(rowID, flag.Name()), "the generator read the flag")

			for id, claims := range report.Facts.Bags() {
				got, claimsErr := s.Claims(id)
				assert.NoError(t, claimsErr, "the claims table reads")
				assert.Equal(t, got, claims, "every bag is recorded whole")
			}
			present, err := s.Present(flag.Name())
			assert.NoError(t, err, "the present table reads")
			assert.Equal(t, present, []symbol.Identity{rowID}, "and the fact that reads present")
			invoked := 0
			for _, c := range report.Stats.Invoked {
				invoked += c.Count
			}
			assert.Equal(t, rowsIn(t, mem, state.TableInvocations), invoked, "one row for each invocation")
		})

		t.Run("records a phase call that journals nothing as one whole call", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			report := sealedRun(t, built(t, directly(t, mem)), workspace.Input{Tree: sealedTree()})
			s := phasesIn(t, mem)

			annotated, held, err := s.Invocation("", plugin.MatchKey{Plugin: modulerID, Rule: plugin.WholeCall})
			assert.NoError(t, err, "the invocations table reads")
			assert.True(t, held, "the annotator's call is one whole call")
			assert.Permutation(t, annotated.Reads, []state.EdgeHash{state.DeclarationEdge(rowID), state.FindingsEdge},
				"the record lists the reads of its reader and the findings edge")
			assert.Equal(t, annotated.Claimed, []meta.FactRef{
				{Subject: storePackage, Key: meta.ModuleKey}, {Subject: storePackage, Key: meta.ModuleRootKey},
			}, "and claimed the module facts")
			assert.Equal(t, annotated.Findings, findings(report.Sink, directFinding), "and reported its finding")
			generated, held, err := s.Invocation("plan", plugin.MatchKey{Plugin: directID, Rule: plugin.WholeCall})
			assert.NoError(t, err, "the invocations table reads")
			assert.True(t, held, "the generator's call is one whole call")
			assert.Length(t, generated.Units, 1, "which flushed one unit")
			modules, err := s.Modules()
			assert.NoError(t, err, "the modules table reads")
			assert.Equal(t, modules, map[plugin.Module]int{
				{Lang: frontendtest.ScriptedLang, Path: storeModule, Root: "."}: 1,
			}, "the package names its module")
		})

		t.Run("records the finding of an unmet contract", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			report := sealedRun(t, built(t, sealingBuilder(t, mem, "plan").Keys(contracted)), workspace.Input{
				Tree: sealedTree(),
			})
			got, err := phasesIn(t, mem).Audits()
			assert.NoError(t, err, "the audit table reads")
			unmet := findings(report.Sink, workspace.UnmetContract)
			assert.Length(t, unmet, 1, "the run reports the unmet contract")
			assert.Equal(t, got, []state.Audit{{Key: contractKey, Subject: rowID, Finding: unmet[0]}},
				"and records it under its key and its subject")
		})
	})
}

// sealedTree returns a tree of one package the scripted frontend parses.
func sealedTree() fstest.MapFS {
	return fstest.MapFS{sealedSource: {Data: []byte("package svc/store\ntype Row int string\n")}}
}

// phaseRecording returns the builder of a composition over the scripted
// frontend that records into a ledger: an annotator that stamps a flag on
// every struct, a plan whose generator mirrors each marked struct after
// it reads the flag, and a check that looks the row up. The flag's handle
// arrives in flag when Build registers it.
func phaseRecording(tb assert.TB, l ledger.Ledger, flag *meta.Key[bool]) *workspace.Builder {
	tb.Helper()

	flagger, held := eidos.NewPlugin(flaggerID).
		Keys(func(r *meta.Registry) error {
			if err := r.ClaimNamespace("shape"); err != nil {
				return err
			}
			k, err := meta.Register[bool](r, meta.KeySpec{Name: "shape.flag", Doc: "marks a recorded row"})
			*flag = k
			return err
		}).
		Handle(eidos.OnStruct(func(_ *eidos.StructMatch, st *eidos.Stamper) error {
			eidos.Stamp(st, *flag, true)
			return nil
		})).Build().(plugin.Annotator)
	assert.True(tb, held, "the flagger lowers to the annotator role")
	marker, held := eidos.NewPlugin(markerID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.Directive(markSchema, eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			eidos.Fact(m, *flag)
			return mirrored(m, e)
		}))).Build().(plugin.Generator)
	assert.True(tb, held, "the marker lowers to the generator role")
	lookup := &recordingCheck{name: lookupID, reads: []string{"plan"}, script: func(ctx *plugin.CheckContext) {
		ctx.Reader.Lookup(rowID)
	}}
	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Annotators(flagger).
		Targets("fixture").
		Plans(workspace.Plan{Name: "plan", Generators: []plugin.Generator{marker}, Backend: printer(tb, "fixture")}).
		Checks(lookup).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Ledger(func() (ledger.Ledger, error) { return l, nil })
}

// directly returns the builder of a composition over the scripted
// frontend that records into a ledger, whose annotator and generator
// implement their roles directly.
func directly(tb assert.TB, l ledger.Ledger) *workspace.Builder {
	tb.Helper()

	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Annotators(moduler{}).
		Targets("fixture").
		Plans(workspace.Plan{Name: "plan", Generators: []plugin.Generator{direct{}}, Backend: printer(tb, "fixture")}).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Ledger(func() (ledger.Ledger, error) { return l, nil })
}

// contracted registers a key whose completeness contract promises it on
// every struct, at Warning, which nothing stamps.
func contracted(r *meta.Registry) error {
	if err := r.ClaimNamespace("shape"); err != nil {
		return err
	}
	_, err := meta.Register[bool](r, meta.KeySpec{
		Name: contractKey,
		Contract: &meta.Completeness{
			On: []symbol.Kind{symbol.KindStruct}, By: diag.PhaseAnnotate, Severity: diag.SeverityWarning,
		},
		Doc: "a promise the composition never keeps",
	})
	return err
}

// phasesIn returns the record of the phases of a ledger's live
// generation.
func phasesIn(t *testing.T, l ledger.Ledger) *state.PhaseState {
	t.Helper()

	g, err := state.Open(t.Context(), l)
	assert.NoError(t, err, "the live generation opens")
	return g.Phases(t.Context())
}

// rowsIn returns how many entries one table of a ledger's live generation
// keeps: each row's count of entries, summed.
func rowsIn(t *testing.T, l ledger.Ledger, table state.Table) int {
	t.Helper()

	g, err := state.Open(t.Context(), l)
	assert.NoError(t, err, "the live generation opens")
	rows, err := g.All(t.Context(), table)
	assert.NoError(t, err, "the table reads")
	n := 0
	for _, r := range rows {
		count, read := binary.Uvarint(r.Value)
		assert.InRange(t, read, 1, math.Inf(1), "a row opens with its count of entries")
		n += int(count)
	}
	return n
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

	return sealingOf(tb, l, plan, mirror(mirrorID))
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
		Annotators(stamper(noterID, quiet)).
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

	damageSegment(t, mem, pick, func(body []byte) []byte { return body[:1] })
}

// flipLastByte inverts the last byte of the live generation's run
// segment, which ends with the footer of the last table's run: a phase
// table's, whose rows follow the load's in table order. The load does not
// read a phase table, so a read of the phases finds the damage.
func flipLastByte(t *testing.T, mem *ledger.Mem) {
	t.Helper()

	damageSegment(t, mem, func(live int) bool { return live == 0 }, func(body []byte) []byte {
		body = bytes.Clone(body)
		body[len(body)-1] ^= 0xff
		return body
	})
}

// damageSegment rewrites the one segment of the live generation whose
// count of live regions picks with the bytes damage makes of it.
func damageSegment(t *testing.T, mem *ledger.Mem, pick func(live int) bool, damage func(body []byte) []byte) {
	t.Helper()

	g, err := state.Open(t.Context(), mem)
	assert.NoError(t, err, "the live generation opens")
	blobs, err := mem.List(t.Context(), segmentDir)
	assert.NoError(t, err, "the segments list")
	damaged := 0
	for _, b := range blobs {
		if !pick(g.Live(b.Name)) {
			continue
		}
		body, err := mem.Read(t.Context(), b.Name)
		assert.NoError(t, err, "the segment reads")
		assert.NoError(t, mem.Write(t.Context(), b.Name, damage(body)), "the damaged segment writes")
		damaged++
	}
	assert.Equal(t, damaged, 1, "one segment is damaged")
}
