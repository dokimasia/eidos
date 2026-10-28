// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"bytes"
	"context"
	"io/fs"
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
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The annotators of the warm composition. The sizer stamps the width of
// every struct, and provides the capability that puts the watcher in the
// next turn. The watcher copies the width of the row onto the reader.
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

// subjectListerID is the generator that lists the structs on which the
// labeler stamped a width.
const subjectListerID plugin.ID = "subject-lister"

// labelLine is the directive of the labeler on the struct of the line
// before it.
const labelLine = "+labeler:label\n"

// copyLine is the directive of the copier on the struct of the line
// before it.
const copyLine = "+copier:copy\n"

// packageOutLine is the kernel out directive on the package of the warm
// tree, and packageOutFile the path of the copier's file under it. The
// plan of the copier names the file after the plan.
const (
	packageOutLine = "pkgnote out path=\"gen/\"\n"
	packageOutFile = "svc/store/gen/copies.txt"
)

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
	setLine      = "+meta set=size.width value=7\n"
	testStampYes = "stamp fake.testFile yes\n"
	testStampNo  = "stamp fake.testFile no\n"
	unknownStamp = "stamp fake.unknown yes\n"
)

// The cache package of the cases of the declared names: its source file,
// and the header line of a file that names the package clause of the file
// in the [headed] language.
const (
	cacheSource = "svc/cache/cache.zz"
	namePrefix  = "// name "
)

// warmWarning is the code of the warning that the warner reports on the
// row.
var warmWarning = diag.Code{Prefix: "tst", Number: 9}

// cachePackage is the package of [cacheSource].
var cachePackage = symbol.Identity{Lang: frontendtest.ScriptedLang, Package: "svc/cache", Kind: symbol.KindPackage}

// warmEdit is the modification time of each file that an edit changes,
// as the write of an editor sets it.
var warmEdit = time.Date(2026, time.October, 7, 12, 0, 0, 0, time.UTC)

// copySchema is the directive of the copier. The one rule of the copier
// runs on each struct with the directive.
var copySchema = directive.Schema{
	Plugin: string(copierID), Name: "copy", Doc: "copies the width of the row onto the struct",
}

// labelSchema is the directive of the labeler. The one rule of the
// labeler runs on each struct with the directive.
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

// headed is the scripted language with a header that names the package
// clause of a file. A first line "// name NAME" sets the name that the
// packages of the file's unit declare. The scripted language takes the
// last element of the package's path as the name otherwise.
type headed struct {
	*frontendtest.Scripted
}

// Parse parses the scripted unit, and then names its packages after the
// header of a member that has one.
func (f headed) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	if err := f.Scripted.Parse(ctx, u); err != nil {
		return err
	}
	for _, ref := range u.Files() {
		b, err := u.Read(ref.Path)
		if err != nil {
			return err
		}
		first, _, _ := strings.Cut(string(b), "\n")
		if name, named := strings.CutPrefix(first, namePrefix); named {
			for _, p := range u.Graph().Packages() {
				p.Name = name
			}
		}
	}
	return nil
}

// TestWarm tests a warm run. A warm run runs again only the validations
// and annotator invocations that read what an edit changed. Its facts
// and its findings equal those of a cold run over the edited tree. A
// generator invocation that read a fact whose winner did not change
// does not run again.
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

		t.Run("runs again a generator invocation that enumerated a key that a subject gained", func(t *testing.T) {
			t.Parallel()

			keys := &warmKeys{}
			w := built(t, warmBuilder(t, ledger.NewMem(), keys, labeler(t, keys)).Plans(subjectListing(t, keys)))
			report, err := warmAfter(t, w, warmTree(rowLine, labelLine, colLine, readerLine),
				warmTree(rowLine, labelLine, colLine, labelLine, readerLine))
			assert.NoError(t, err, "the run is clean")
			expect.Equal(t, generatedBy(report, subjectListerID), 1, "the lister runs again for the labeled column")
		})

		t.Run("runs no generator invocation that enumerated a key whose value alone changed", func(t *testing.T) {
			t.Parallel()

			keys := &warmKeys{}
			w := built(t, warmBuilder(t, ledger.NewMem(), keys, labeler(t, keys)).Plans(subjectListing(t, keys)))
			report, err := warmAfter(t, w, warmTree(rowLine, labelLine, colLine, readerLine),
				warmTree(widerRow, labelLine, colLine, readerLine))
			assert.NoError(t, err, "the run is clean")
			expect.Equal(t, generatedBy(report, subjectListerID), 0,
				"the label of the row changed its value and not its presence")
		})

		t.Run("routes a file again after an edit of the out directive on its package", func(t *testing.T) {
			t.Parallel()

			keys := &warmKeys{}
			w := built(t, warmBuilder(t, ledger.NewMem(), keys).Plans(copying(t, keys)))
			report, err := warmAfter(t, w, warmTree(rowLine, colLine, readerLine, copyLine),
				warmTree(packageOutLine, rowLine, colLine, readerLine, copyLine))
			assert.NoError(t, err, "the run is clean")
			paths := make([]string, 0, len(report.Manifest.Files))
			for _, f := range report.Manifest.Files {
				paths = append(paths, f.Path)
			}
			assert.Equal(t, paths, []string{packageOutFile},
				"the copier's file moves into the directory of the out directive on the package")
		})

		t.Run("renders a file again after an edit renames a package whose declared name the file read",
			func(t *testing.T) {
				t.Parallel()

				var o opener
				w := built(t, clauseBuilder(ledger.NewMem(), headed{frontendtest.NewScripted()}, o.open))
				_, err := warmAfter(t, w, clauseTree(cacheClause), clauseTree(renamedClause))
				assert.NoError(t, err, "the run is clean")
				sinks := o.opened()
				assert.Length(t, sinks, 2, "each run opens one sink")
				assert.Contains(t, string(sinks[1].Files()[mirroredFile]), "type ForRow "+renamedClause,
					"the file of the store package writes the new name of the cache package")
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

		t.Run("reports nothing for a validation of a dependency package", func(t *testing.T) {
			t.Parallel()

			marker := eidos.NewPlugin(markerID).
				Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
				Handle(eidos.Directive(markSchema, eidos.OnStruct(mirrored))).Generator()
			w := dependentOf(t, workspace.Sources{Lang: frontendtest.ScriptedLang}, marker)
			first := map[string]fs.FS{frontendtest.ScriptedStore: fstest.MapFS{
				libSource: {Data: []byte(libBody + bogusMark)},
			}}
			cold, err := w.Run(t.Context(), workspace.Input{Tree: dependentTree(), Stores: first})
			assert.NoError(t, err, "the cold run is clean")
			assert.Empty(t, slices.Collect(cold.Sink.All()), "the cold run reports no finding of the dependency")
			edited := map[string]fs.FS{frontendtest.ScriptedStore: fstest.MapFS{
				libSource: {Data: []byte(libBody + colLine + bogusMark), ModTime: warmEdit},
			}}
			report, err := w.Run(t.Context(), workspace.Input{Tree: dependentTree(), Stores: edited})
			assert.NoError(t, err, "the warm run is clean")
			assert.False(t, report.Stats.Cold, "the second run reads the sealed state")
			assert.Equal(t, report.Stats.Validated, 1, "the run validates Col alone")
			assert.Empty(t, slices.Collect(report.Sink.All()), "the warm run reports no finding of the dependency")
		})

		shared := []struct {
			name      string
			giveOwn   string
			giveLib   string
			wantFound int
		}{
			{
				name:    "reports nothing for a validation of a dependency in a package that the tree shares",
				giveOwn: ownBody,
				giveLib: libBody + bogusMark,
			},
			{
				name:      "reports the validation of a declaration of the tree in a package that a store shares",
				giveOwn:   ownBody + bogusMark,
				giveLib:   libBody,
				wantFound: 1,
			},
		}
		for _, tt := range shared {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				marker := eidos.NewPlugin(markerID).
					Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
					Handle(eidos.Directive(markSchema, eidos.OnStruct(mirrored))).Generator()
				w := dependentOf(t, workspace.Sources{Lang: frontendtest.ScriptedLang}, marker)
				tree := dependentTree()
				tree[ownSource] = &fstest.MapFile{Data: []byte(tt.giveOwn)}
				stores := func(body string, at time.Time) map[string]fs.FS {
					return map[string]fs.FS{frontendtest.ScriptedStore: fstest.MapFS{
						usedSource: {Data: []byte(body), ModTime: at},
					}}
				}
				cold, _ := w.Run(t.Context(), workspace.Input{Tree: tree, Stores: stores(tt.giveLib, time.Time{})})
				assert.Equal(t, cold.Stats.Parsed, 3, "the run parses the application and both halves of the library")
				assert.Length(t, findings(cold.Sink, directive.UnknownKey), tt.wantFound,
					"the cold run reports the validation of the tree alone")
				warm, _ := w.Run(t.Context(), workspace.Input{Tree: tree, Stores: stores(tt.giveLib+colLine, warmEdit)})
				assert.False(t, warm.Stats.Cold, "the second run reads the sealed state")
				assert.Length(t, findings(warm.Sink, directive.UnknownKey), tt.wantFound,
					"the warm run reports the validation of the tree alone")
			})
		}

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

		t.Run("withdraws the set of a directive that the edit removed", func(t *testing.T) {
			t.Parallel()

			keys := &warmKeys{}
			w := built(t, warmBuilder(t, ledger.NewMem(), keys))
			report, err := warmAfter(t, w, warmTree(rowLine, setLine, colLine, readerLine),
				warmTree(rowLine, colLine, readerLine))
			assert.NoError(t, err, "the run is clean")
			width, _ := meta.Get(report.Facts, rowID, keys.width)
			assert.Equal(t, width, int64(2), "the width is the sizer's count again")
		})

		t.Run("files the set of a directive that the edit added", func(t *testing.T) {
			t.Parallel()

			keys := &warmKeys{}
			w := built(t, warmBuilder(t, ledger.NewMem(), keys))
			report, err := warmAfter(t, w, before, warmTree(rowLine, setLine, colLine, readerLine))
			assert.NoError(t, err, "the run is clean")
			width, _ := meta.Get(report.Facts, rowID, keys.width)
			expect.Equal(t, width, int64(7), "the set outranks the stamp of the sizer")
			seen, _ := meta.Get(report.Facts, readerID, keys.seen)
			expect.Equal(t, seen, int64(7), "the watcher reads the set width again")
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
// turn. The generator of the plan mirrors each struct that has the
// mark. extra lists more annotators of the composition.
func warmBuilder(tb assert.TB, l ledger.Ledger, keys *warmKeys, extra ...plugin.Annotator) *workspace.Builder {
	tb.Helper()

	sizer := eidos.NewPlugin(sizerID).
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
		})).Annotator()
	watcher := eidos.NewPlugin(watcherID).
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
		})).Annotator()
	marker := eidos.NewPlugin(markerID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.Directive(markSchema, eidos.OnStruct(mirrored))).Generator()
	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Annotators(append([]plugin.Annotator{sizer, watcher}, extra...)...).
		Targets("fixture").
		Plans(workspace.Plan{Name: "plan", Generators: []plugin.Generator{marker}, Backend: fixturePrinter()}).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Ledger(func() (ledger.Ledger, error) { return l, nil })
}

// copying returns the plan of the copier. On each struct with the copy
// directive, the copier reads the width of the row. It mirrors the struct
// with the width as its documentation, into a file named after the plan.
func copying(tb assert.TB, keys *warmKeys) workspace.Plan {
	tb.Helper()

	copier := eidos.NewPlugin(copierID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "copy"}).
		Handle(eidos.Directive(copySchema, eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			width, _ := eidos.FactOf(m, rowID, keys.width)
			e.PackageFile().Append(&emit.Struct{
				Origin: m.Struct.Identity(), Name: "Copy" + m.Struct.Name, Doc: []string{strconv.FormatInt(width, 10)},
			})
			return nil
		}))).Generator()
	return sealedPlan("copies", copier)
}

// subjectListing returns the plan of the subject lister. Its graph rule
// lists the structs on which the labeler stamped a width, without a read of
// the widths, and writes one declaration for each into a file named after
// the plan.
func subjectListing(tb assert.TB, keys *warmKeys) workspace.Plan {
	tb.Helper()

	lister := eidos.NewPlugin(subjectListerID).
		Output(plugin.Output{Per: plugin.PerPlan, Word: "list"}).
		Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
			for id := range eidos.Subjects(m, keys.labeled) {
				e.PlanFile().Append(&emit.Struct{Name: "Listed" + id.Name})
			}
			return nil
		})).Generator()
	return workspace.Plan{
		Name:       "lists",
		Generators: []plugin.Generator{lister},
		Backend:    fixturePrinter(),
		Layout:     layout.Config{Dir: planDir},
	}
}

// labeler returns the annotator of the label directive. On each struct
// with the directive, it stamps the width of the struct under
// label.width at the authority of the directive.
func labeler(tb assert.TB, keys *warmKeys) plugin.Annotator {
	tb.Helper()

	return eidos.NewPlugin(labelerID).
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
		}))).Annotator()
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

// clauseBuilder returns the builder of a composition over the frontend f
// that records into a ledger and renders into the output that open
// returns. Its plan mirrors every struct through the clause printer of
// the scripted language, which writes the name that the cache package
// declares.
func clauseBuilder(l ledger.Ledger, f plugin.Frontend, open func() (output.Sink, error)) *workspace.Builder {
	target := plugin.Target(frontendtest.ScriptedLang)
	return workspace.New().
		Brand(fixtureBrand).
		Frontends(f).
		Annotators(stamper(noterID, quiet)).
		Targets(target).
		Plans(workspace.Plan{
			Name:       "plan",
			Generators: []plugin.Generator{mirror(mirrorID)},
			Backend:    clausePrinter(target, cachePackage.Package),
		}).
		Output(open).
		Ledger(func() (ledger.Ledger, error) { return l, nil })
}

// clauseTree returns a tree of the store package with its row, and of the
// cache package with its column, whose file names its package clause name
// in the header of the [headed] language.
func clauseTree(name string) fstest.MapFS {
	return fstest.MapFS{
		sealedSource: {Data: []byte("package svc/store\n" + rowLine)},
		cacheSource:  {Data: []byte(namePrefix + name + "\npackage svc/cache\n" + colLine)},
	}
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
