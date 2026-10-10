// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"bytes"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The annotators of the warm composition. The sizer stamps the width of
// every struct, and provides the capability that puts the watcher in the
// next bucket. The watcher copies the width of the row onto the reader.
// The warner reports a warning on the row, and the counter implements its
// role directly.
const (
	sizerID   plugin.ID         = "sizer"
	watcherID plugin.ID         = "watcher"
	warnerID  plugin.ID         = "warner"
	counterID plugin.ID         = "counter"
	sized     plugin.Capability = "sized"
)

// copierID is the generator that copies the width of the row onto each
// struct with its directive.
const copierID plugin.ID = "copier"

// labelerID is the annotator that stamps the width of each struct with its
// directive, at the directive's authority.
const labelerID plugin.ID = "labeler"

// labelLine attaches the labeler's directive to the struct on the line
// before it.
const labelLine = "+labeler:label\n"

// copyLine attaches the copier's directive to the struct on the line
// before it.
const copyLine = "+copier:copy\n"

// The file of the warm tree, and the lines that the cases edit. Each line
// declares a struct, or attaches a directive or a stamp to the line before
// it. An edit that must not move the declarations after it replaces one
// line with one line.
const (
	warmSource   = "svc/store/row.zz"
	rowLine      = "type Row int string\n"
	widerRow     = "type Row int string bool\n"
	swappedRow   = "type Row string int\n"
	colLine      = "type Col int\n"
	widerCol     = "type Col int bool\n"
	readerLine   = "type Reader int\n"
	markedLine   = markLine + "\n"
	bogusMark    = markLine + " bogus=1\n"
	dropLine     = "+meta drop=size.width\n"
	testStampYes = "stamp fake.testFile yes\n"
	testStampNo  = "stamp fake.testFile no\n"
	unknownStamp = "stamp fake.unknown yes\n"
)

// warmWarning is the code of the warning that the warner reports on the
// row.
var warmWarning = diag.Code{Prefix: "tst", Number: 9}

// warmEdit is the modification time of each file that an edit changes,
// as an editor's write moves it.
var warmEdit = time.Date(2026, time.October, 7, 12, 0, 0, 0, time.UTC)

// copySchema is the copier's directive. The copier's one rule runs on each
// struct with the directive.
var copySchema = directive.Schema{
	Plugin: string(copierID), Name: "copy", Doc: "copies the width of the row onto the struct",
}

// labelSchema is the labeler's directive. The labeler's one rule runs on
// each struct with the directive.
var labelSchema = directive.Schema{
	Plugin: string(labelerID), Name: "label", Doc: "stamps the width of the struct",
}

// The structs of the warm tree besides the row.
var (
	colID = symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: "svc/store", Name: "Col", Kind: symbol.KindStruct,
	}
	readerID = symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: "svc/store", Name: "Reader", Kind: symbol.KindStruct,
	}
)

// warmKeys are the handles of the keys of the warm composition. Each
// Build sets them, and the handlers read them through the pointer.
type warmKeys struct {
	width   meta.Key[int64]
	seen    meta.Key[int64]
	labeled meta.Key[int64]
}

// counter is an annotator that implements its role directly. It counts
// its calls in calls, and looks the row up on each call.
type counter struct {
	calls *atomic.Int64
}

// Name returns the counter's name.
func (counter) Name() plugin.ID { return counterID }

// Annotate counts the call and looks the row up.
func (c counter) Annotate(ctx *plugin.AnnotatorContext) error {
	c.calls.Add(1)
	ctx.Reader.Lookup(rowID)
	return nil
}

// A warm run executes again only the validations and annotator
// invocations that read what an edit changed. Its facts and its findings
// equal those of a cold run over the edited tree. A generator invocation
// that read a fact whose winner did not change does not run again.
func TestWarm(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		before := warmTree(rowLine, colLine, readerLine)

		t.Run("invokes the sizer again only on the struct that the edit changed", func(t *testing.T) {
			t.Parallel()

			report, err := warmAfter(t, built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{})), before,
				warmTree(rowLine, widerCol, readerLine))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, invoked(report, sizerID), 1, "the sizer runs on Col alone")
		})

		t.Run("runs again an invocation that read a fact whose winner changed", func(t *testing.T) {
			t.Parallel()

			keys := &warmKeys{}
			report, err := warmAfter(t, built(t, warmBuilder(t, ledger.NewMem(), keys)), before,
				warmTree(widerRow, colLine, readerLine))
			assert.NoError(t, err, "the run is clean")
			seen, held := meta.Get(report.Facts, readerID, keys.seen)
			assert.True(t, held, "the watcher stamps the reader")
			assert.Equal(t, seen, int64(3), "the reader's copy has the new width of the row")
		})

		t.Run("skips an invocation that read a fact whose winner is unchanged", func(t *testing.T) {
			t.Parallel()

			report, err := warmAfter(t, built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{})), before,
				warmTree(swappedRow, colLine, readerLine))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, invoked(report, watcherID), 1, "the watcher evaluates the changed row alone")
		})

		t.Run("runs no generator invocation that read a fact whose winner is unchanged", func(t *testing.T) {
			t.Parallel()

			keys := &warmKeys{}
			w := built(t, warmBuilder(t, ledger.NewMem(), keys).Plans(copying(t, keys)))
			report, err := warmAfter(t, w, warmTree(rowLine, colLine, readerLine, copyLine),
				warmTree(swappedRow, colLine, readerLine, copyLine))
			assert.NoError(t, err, "the run is clean")
			expect.Equal(t, generatedBy(report, copierID), 0, "the copier keeps its invocation on the reader")
			expect.Equal(t, report.Stats.Rendered, 0, "the run renders no file")
		})

		t.Run("runs again a generator invocation that read a fact whose winner changed", func(t *testing.T) {
			t.Parallel()

			keys := &warmKeys{}
			w := built(t, warmBuilder(t, ledger.NewMem(), keys).Plans(copying(t, keys)))
			report, err := warmAfter(t, w, warmTree(rowLine, colLine, readerLine, copyLine),
				warmTree(widerRow, colLine, readerLine, copyLine))
			assert.NoError(t, err, "the run is clean")
			expect.Equal(t, generatedBy(report, copierID), 1, "the copier runs on the reader again")
			expect.Equal(t, report.Stats.Rendered, 1, "the run renders the copier's file")
		})

		t.Run("withdraws the claim of an invocation that a directive gates and that runs again", func(t *testing.T) {
			t.Parallel()

			keys := &warmKeys{}
			w := built(t, warmBuilder(t, ledger.NewMem(), keys, labeler(t, keys)))
			report, err := warmAfter(t, w, warmTree(rowLine, labelLine, colLine, readerLine),
				warmTree(widerRow, labelLine, colLine, readerLine))
			assert.NoError(t, err, "the run is clean")
			labeled, held := meta.Get(report.Facts, rowID, keys.labeled)
			assert.True(t, held, "the labeler stamps the row again")
			assert.Equal(t, labeled, int64(3), "the stamp has the width of the edited row")
		})

		t.Run("withdraws the claims on a struct that the edit removed", func(t *testing.T) {
			t.Parallel()

			keys := &warmKeys{}
			report, err := warmAfter(t, built(t, warmBuilder(t, ledger.NewMem(), keys)), before,
				warmTree(rowLine, readerLine))
			assert.NoError(t, err, "the run is clean")
			_, held := meta.Get(report.Facts, colID, keys.width)
			assert.False(t, held, "the removed struct has no width")
		})

		t.Run("validates again only the subject whose directives changed", func(t *testing.T) {
			t.Parallel()

			w := built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{}))
			report, err := warmAfter(t, w, warmTree(rowLine, markedLine, readerLine, markedLine, colLine),
				warmTree(rowLine, markedLine, readerLine, markedLine, colLine, markedLine))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Stats.Validated, 1, "the run validates Col alone")
		})

		t.Run("reports the finding of a validation that the run keeps", func(t *testing.T) {
			t.Parallel()

			w := built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{}))
			report, err := warmAfter(t, w, warmTree(rowLine, bogusMark, colLine, readerLine),
				warmTree(rowLine, bogusMark, widerCol, readerLine))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the kept finding is an Error")
			assert.Length(t, findings(report.Sink, directive.UnknownKey), 1,
				"the run reports the finding on the row again")
		})

		t.Run("reports the finding of an annotator invocation that the run keeps", func(t *testing.T) {
			t.Parallel()

			w := built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{}, warner()))
			report, err := warmAfter(t, w, before, warmTree(rowLine, widerCol, readerLine))
			assert.NoError(t, err, "a warning fails no run")
			assert.Length(t, findings(report.Sink, warmWarning), 1, "the run reports the warning on the row again")
		})

		t.Run("withdraws the drop of a directive that the edit removed", func(t *testing.T) {
			t.Parallel()

			keys := &warmKeys{}
			w := built(t, warmBuilder(t, ledger.NewMem(), keys))
			report, err := warmAfter(t, w, warmTree(rowLine, dropLine, colLine, readerLine),
				warmTree(rowLine, colLine, readerLine))
			assert.NoError(t, err, "the run is clean")
			width, held := meta.Get(report.Facts, rowID, keys.width)
			assert.True(t, held, "the row has a width without the drop")
			assert.Equal(t, width, int64(2), "the width is the sizer's count")
		})

		t.Run("files the drop of a directive that the edit added", func(t *testing.T) {
			t.Parallel()

			keys := &warmKeys{}
			w := built(t, warmBuilder(t, ledger.NewMem(), keys))
			report, err := warmAfter(t, w, before, warmTree(rowLine, dropLine, colLine, readerLine))
			assert.NoError(t, err, "the run is clean")
			_, held := meta.Get(report.Facts, rowID, keys.width)
			assert.False(t, held, "the drop hides the width of the row")
		})

		t.Run("files the stamps of a file whose stamps changed", func(t *testing.T) {
			t.Parallel()

			w := built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{}).Keys(frontendtest.ScriptedKeys))
			report, err := warmAfter(t, w, warmTree(rowLine, colLine, readerLine, testStampYes),
				warmTree(rowLine, colLine, readerLine, testStampNo))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, testStamp(t, report.Facts), any("no"), "the file has the new stamp")
		})

		t.Run("does not write a generation after a stamp that the fact store refuses", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := built(t, warmBuilder(t, mem, &warmKeys{}))
			sealedRun(t, w, workspace.Input{Tree: before})
			refused := editedAfter(before, warmTree(rowLine, colLine, readerLine, unknownStamp))
			var err error
			assert.Pure(t, func() string { return liveGeneration(t, mem) }, func() {
				_, err = w.Run(t.Context(), workspace.Input{Tree: refused})
			}, "the generation of the first run is still live")
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the refused stamp fails the run")
		})

		t.Run("reports a refused stamp again on the next run", func(t *testing.T) {
			t.Parallel()

			w := built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{}))
			sealedRun(t, w, workspace.Input{Tree: before})
			refused := editedAfter(before, warmTree(rowLine, colLine, readerLine, unknownStamp))
			_, err := w.Run(t.Context(), workspace.Input{Tree: refused})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the refused stamp fails the run")
			report, err := w.Run(t.Context(), workspace.Input{Tree: refused})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the stamp fails the next run as well")
			assert.Length(t, findings(report.Sink, meta.RefusedStamp), 1, "the next run reports the refused stamp")
		})

		t.Run("writes nothing for a dry warm run", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := built(t, warmBuilder(t, mem, &warmKeys{}))
			sealedRun(t, w, workspace.Input{Tree: before})
			after := editedAfter(before, warmTree(rowLine, widerCol, readerLine))
			var report *workspace.Report
			assert.Pure(t, mem.Writes, func() { report = sealedRun(t, w, workspace.Input{Tree: after, Dry: true}) },
				"the dry run writes nothing to the ledger")
			assert.False(t, report.Stats.Cold, "the dry run reads the sealed state")
		})

		t.Run("skips an annotator that implements its role directly over an unchanged tree", func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int64
			w := built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{}, counter{calls: &calls}))
			_, err := warmAfter(t, w, before, warmTree(rowLine, colLine, readerLine))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, calls.Load(), int64(1), "only the first run calls the counter")
		})

		t.Run("calls an annotator that implements its role directly again after an edit", func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int64
			w := built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{}, counter{calls: &calls}))
			_, err := warmAfter(t, w, before, warmTree(rowLine, widerCol, readerLine))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, calls.Load(), int64(2), "both runs call the counter")
		})

		t.Run("leaves the facts that a cold run over the edited tree leaves", func(t *testing.T) {
			t.Parallel()

			edited := warmTree(widerRow, colLine, readerLine)
			keys, coldKeys := &warmKeys{}, &warmKeys{}
			warm, err := warmAfter(t, built(t, warmBuilder(t, ledger.NewMem(), keys)), before, edited)
			assert.NoError(t, err, "the warm run is clean")
			cold := sealedRun(t, built(t, warmBuilder(t, ledger.NewMem(), coldKeys)), workspace.Input{Tree: edited})
			for _, id := range []symbol.Identity{rowID, colID, readerID} {
				warmWidth, warmHeld := meta.Get(warm.Facts, id, keys.width)
				coldWidth, coldHeld := meta.Get(cold.Facts, id, coldKeys.width)
				expect.Equal(t, warmHeld, coldHeld, id.Name+" has a width in both runs or in neither")
				expect.Equal(t, warmWidth, coldWidth, id.Name+" has the same width in both runs")
				warmSeen, warmCopied := meta.Get(warm.Facts, id, keys.seen)
				coldSeen, coldCopied := meta.Get(cold.Facts, id, coldKeys.seen)
				expect.Equal(t, warmCopied, coldCopied, id.Name+" has a copy in both runs or in neither")
				expect.Equal(t, warmSeen, coldSeen, id.Name+" has the same copy in both runs")
			}
		})

		t.Run("reports the findings that a cold run over the edited tree reports", func(t *testing.T) {
			t.Parallel()

			first := warmTree(rowLine, bogusMark, colLine, readerLine)
			edited := warmTree(rowLine, bogusMark, widerCol, readerLine)
			warm, _ := warmAfter(t, built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{}, warner())), first, edited)
			cold, _ := built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{}, warner())).
				Run(t.Context(), workspace.Input{Tree: edited})
			assert.Equal(t, slices.SortedStableFunc(warm.Sink.All(), diag.Diag.Compare),
				slices.SortedStableFunc(cold.Sink.All(), diag.Diag.Compare),
				"the warm run reports the same findings as the cold run")
		})
	})
}

// warmBuilder returns the builder of the warm composition, which records
// into a ledger. The sizer stamps the width of every struct, and the
// watcher copies the width of the row onto the reader in the next
// bucket. The plan's generator mirrors each struct that has the mark.
// extra lists more annotators of the composition.
func warmBuilder(tb assert.TB, l ledger.Ledger, keys *warmKeys, extra ...plugin.Annotator) *workspace.Builder {
	tb.Helper()

	sizer, held := eidos.NewPlugin(sizerID).
		Provides(sized).
		Keys(func(r *meta.Registry) error {
			if err := r.ClaimNamespace("size"); err != nil {
				return err
			}
			k, err := meta.Register[int64](r, meta.KeySpec{Name: "size.width", Doc: "counts the fields of a struct"})
			keys.width = k
			return err
		}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
			eidos.Stamp(st, keys.width, int64(len(m.Struct.Fields)))
			return nil
		})).Build().(plugin.Annotator)
	assert.True(tb, held, "the sizer lowers to the annotator role")
	watcher, held := eidos.NewPlugin(watcherID).
		Requires(sized).
		Keys(func(r *meta.Registry) error {
			if err := r.ClaimNamespace("watch"); err != nil {
				return err
			}
			k, err := meta.Register[int64](r, meta.KeySpec{Name: "watch.seen", Doc: "copies the width of the row"})
			keys.seen = k
			return err
		}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
			if m.Struct.Name != readerID.Name {
				return nil
			}
			if width, measured := eidos.FactOf(m, rowID, keys.width); measured {
				eidos.Stamp(st, keys.seen, width)
			}
			return nil
		})).Build().(plugin.Annotator)
	assert.True(tb, held, "the watcher lowers to the annotator role")
	marker, held := eidos.NewPlugin(markerID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.Directive(markSchema, eidos.OnStruct(mirrored))).Build().(plugin.Generator)
	assert.True(tb, held, "the marker lowers to the generator role")
	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Annotators(append([]plugin.Annotator{sizer, watcher}, extra...)...).
		Targets("fixture").
		Plans(workspace.Plan{Name: "plan", Generators: []plugin.Generator{marker}, Backend: printer(tb, "fixture")}).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Ledger(func() (ledger.Ledger, error) { return l, nil })
}

// copying returns the plan of the copier. On each struct with the copy
// directive, the copier reads the width of the row. It mirrors the struct
// with the width as its documentation, into a file named after the plan.
func copying(tb assert.TB, keys *warmKeys) workspace.Plan {
	tb.Helper()

	copier, held := eidos.NewPlugin(copierID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "copy"}).
		Handle(eidos.Directive(copySchema, eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			width, _ := eidos.FactOf(m, rowID, keys.width)
			e.PackageFile().Append(&emit.Struct{
				Origin: m.Struct.Identity(), Name: "Copy" + m.Struct.Name, Doc: []string{strconv.FormatInt(width, 10)},
			})
			return nil
		}))).Build().(plugin.Generator)
	assert.True(tb, held, "the copier lowers to the generator role")
	return sealedPlan(tb, "copies", copier)
}

// labeler returns the annotator of the label directive. On each struct
// with the directive, it stamps the width of the struct under
// label.width, which has the directive's authority.
func labeler(tb assert.TB, keys *warmKeys) plugin.Annotator {
	tb.Helper()

	p, held := eidos.NewPlugin(labelerID).
		Keys(func(r *meta.Registry) error {
			if err := r.ClaimNamespace("label"); err != nil {
				return err
			}
			k, err := meta.Register[int64](r, meta.KeySpec{
				Name: "label.width", Doc: "counts the fields of a labeled struct",
			})
			keys.labeled = k
			return err
		}).
		Handle(eidos.Directive(labelSchema, eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
			eidos.Stamp(st, keys.labeled, int64(len(m.Struct.Fields)))
			return nil
		}))).Build().(plugin.Annotator)
	assert.True(tb, held, "the labeler lowers to the annotator role")
	return p
}

// warner returns an annotator that reports a warning on the row.
func warner() plugin.Annotator {
	return stamper(warnerID, func(m *eidos.StructMatch, _ *eidos.Stamper) error {
		if m.Struct.Name == rowID.Name {
			m.Warnf(warmWarning, "the warner watches the row")
		}
		return nil
	})
}

// warmTree returns a tree of the store package with one file that
// contains the lines.
func warmTree(lines ...string) fstest.MapFS {
	return fstest.MapFS{warmSource: {Data: []byte("package svc/store\n" + strings.Join(lines, ""))}}
}

// editedAfter returns a copy of after in which each file whose bytes
// differ from the same file of before has warmEdit as its modification
// time. It changes neither tree, so parallel tests can share both.
func editedAfter(before, after fstest.MapFS) fstest.MapFS {
	edited := make(fstest.MapFS, len(after))
	for path, f := range after {
		file := *f
		if was, found := before[path]; !found || !bytes.Equal(was.Data, file.Data) {
			file.ModTime = warmEdit
		}
		edited[path] = &file
	}
	return edited
}

// warmAfter runs w over the tree before, and then over the tree after,
// and returns the report and the error of the second run. The second run
// reads the sealed state that the first run wrote.
func warmAfter(t *testing.T, w *workspace.Workspace, before, after fstest.MapFS) (*workspace.Report, error) {
	t.Helper()

	_, _ = w.Run(t.Context(), workspace.Input{Tree: before})
	report, err := w.Run(t.Context(), workspace.Input{Tree: editedAfter(before, after)})
	assert.NotNil(t, report, "the second run returns a report")
	assert.False(t, report.Stats.Cold, "the second run reads the sealed state")
	return report, err
}

// invoked returns the number of invocations that a run's statistics list
// for an annotator, and zero for an annotator that they do not list.
func invoked(report *workspace.Report, p plugin.ID) int {
	for _, c := range report.Stats.Invoked {
		if c.Plugin == p && c.Phase == plugin.PhaseAnnotate {
			return c.Count
		}
	}
	return 0
}

// liveGeneration returns the name of the live generation of a ledger.
func liveGeneration(t *testing.T, l ledger.Ledger) string {
	t.Helper()

	g, err := state.Open(t.Context(), l)
	assert.NoError(t, err, "the live generation opens")
	return g.Name
}

// testStamp returns the value of the scripted test key on the one
// subject that has the key.
func testStamp(t *testing.T, facts *meta.Facts) any {
	t.Helper()

	key, registered := facts.Registry().Resolve(frontendtest.ScriptedTestKey)
	assert.True(t, registered, "the composition registers the scripted test key")
	subjects := slices.Collect(facts.ByKey(key))
	assert.Length(t, subjects, 1, "one file has the stamp")
	for v := range facts.Claims(subjects[0], key) {
		return v.Value
	}
	return nil
}
