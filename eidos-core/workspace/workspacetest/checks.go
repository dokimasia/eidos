// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspacetest

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/internal/rundir"
)

// The key the audit check promises, in a namespace of the suite's own,
// and the documentation it registers with.
const (
	auditNamespace              = "workspacetest"
	auditKey       meta.KeyName = "workspacetest.audited"
	auditDoc                    = "a key the workspace suite promises and no annotator stamps"
)

// lacks separates the subject of an UnmetContract message from the key
// it lacks.
const lacks = " lacks "

// AssertGenerated runs the fixture's plans in root, an empty directory,
// and checks that the files under the brand's frame are the wanted
// files, byte for byte, and that the record lists each file under the
// plan whose commit wrote it.
func AssertGenerated(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	_, w, report := clean(tb, f, root)
	expect.Equal(tb, rundir.Framed(tb, root, w.Brand()), rundir.Texts(f.Want),
		"the files under the brand's frame are the fixture's wanted files, byte for byte")
	committed := map[string]string{}
	for _, p := range report.Plans {
		for _, c := range p.Changes {
			committed[c.Path] = p.Name
		}
	}
	recorded := map[string]string{}
	for _, e := range rundir.Record(tb, root, w.Brand()).Files {
		recorded[e.Path] = e.Plan
	}
	expect.Equal(tb, recorded, committed, "the record lists each file under the plan whose commit wrote it")
}

// AssertCollision runs the fixture's plans in root, an empty directory,
// with a copy of the first plan under another name, and checks that the
// two collide: the run reports PlanCollision naming both plans, no plan
// commits, and the run records nothing.
func AssertCollision(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	plans := plansOf(tb, f)
	copied(tb, f, root)
	twin := plans[0]
	twin.Name = copyPlan
	w := built(tb, f, root, append(plans, twin), nil)
	report, err := ran(f, w, root)
	collisions := codes(report, workspace.PlanCollision)
	assert.NotEmpty(tb, collisions, "the run reports a PlanCollision for a copy of plan "+plans[0].Name)
	for _, d := range collisions {
		expect.That(tb, d.Msg).
			Contains(strconv.Quote(plans[0].Name), "the collision names the copied plan").
			Contains(strconv.Quote(copyPlan), "the collision names the copy")
	}
	expect.ErrorIs(tb, err, workspace.ErrRunFailed, "the collision fails the run")
	for _, p := range report.Plans {
		expect.Equal(tb, p.Status, workspace.PlanFailed, "plan "+p.Name+" commits nothing")
	}
	files.Absent(tb, rundir.Path(root, ledger.ManifestPath(w.Brand())), "the run records nothing")
}

// AssertIsolated runs the fixture's plans in root, an empty directory,
// then runs them again with a generator that reports an Error added to
// the last plan. It checks that the last plan commits nothing and keeps
// its previous files and entries, and that every other plan commits.
func AssertIsolated(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	_, w, _ := clean(tb, f, root)
	at := treeFile(tb, f)
	before := rundir.Record(tb, root, w.Brand())
	files := rundir.Framed(tb, root, w.Brand())
	plans := plansOf(tb, f)
	last := &plans[len(plans)-1]
	last.Generators = append(slices.Clone(last.Generators), failure{at: at})
	again := built(tb, f, root, plans, nil)
	report, err := ran(f, again, root)
	expect.ErrorIs(tb, err, workspace.ErrRunFailed, "the seeded failure fails the run")
	for _, p := range report.Plans {
		if p.Name == last.Name {
			expect.Equal(tb, p.Status, workspace.PlanFailed, "the plan seeded to fail commits nothing")
			continue
		}
		expect.Equal(tb, p.Status, workspace.PlanCommitted, "plan "+p.Name+" commits beside it")
	}
	kept := entriesOf(before, last.Name)
	expect.Equal(tb, entriesOf(rundir.Record(tb, root, again.Brand()), last.Name), kept,
		"the failed plan's entries remain", assert.EquateEmpty())
	after := rundir.Framed(tb, root, again.Brand())
	for _, e := range kept {
		expect.Equal(tb, after[e.Path], files[e.Path], "the failed plan's file "+e.Path+" remains")
	}
}

// AssertExported runs the fixture's plans in root, an empty directory,
// with a probe plan that depends on each of them, and checks that the
// probe reads each plan's export, and that each export lists the files
// the plan's record lists: every exported declaration is in a recorded
// file, under one of the file's plugins and, at file level, from one of
// its sources, and every recorded file exports a declaration.
func AssertExported(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	names := planNames(plansOf(tb, f))
	copied(tb, f, root)
	run := probed(tb, f, root, false)
	expect.Permutation(tb, slices.Collect(maps.Keys(run.exports)), names,
		"the probe reads the export of each plan it depends on")
	record := rundir.Record(tb, root, run.brand)
	for _, name := range names {
		exportedIn(tb, run.exports[name], entriesOf(record, name))
	}
}

// AssertCycleRefused builds the fixture over root with copies of the
// first plan under two names that depend on each other, and checks that
// Build returns an error naming both. It builds the fixture's own plans
// first, so a composition that does not build fails the check under its
// own error.
func AssertCycleRefused(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	built(tb, f, root, plansOf(tb, f), nil)
	plans := plansOf(tb, f)
	a, b := plans[0], plans[0]
	a.Name, a.DependsOn = cycleA, []string{cycleB}
	b.Name, b.DependsOn = cycleB, []string{cycleA}
	_, err := composed(tb, f, root, append(plans, a, b), nil)
	assert.HasError(tb, err, "Build refuses two plans that depend on each other")
	expect.That(tb, err.Error()).
		Contains(strconv.Quote(cycleA), "the error names the first plan of the cycle").
		Contains(strconv.Quote(cycleB), "the error names the second plan of the cycle")
}

// AssertSwept runs the fixture's plans in root, an empty directory,
// then runs them without the last plan, and checks that the run removes
// every file of the last plan and changes no other file. It then runs
// every plan again, edits one file of the last plan, and runs without
// the last plan once more, and checks that the run keeps the edited
// file under a KeptOutput warning.
func AssertSwept(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	plans, w, _ := clean(tb, f, root)
	last := plans[len(plans)-1].Name
	gone := entriesOf(rundir.Record(tb, root, w.Brand()), last)
	assert.NotEmpty(tb, gone, "the fixture's last plan "+last+" routes a file")
	want := rundir.Framed(tb, root, w.Brand())
	for _, e := range gone {
		delete(want, e.Path)
	}
	_, after := withoutLast(tb, f, root)
	expect.Equal(tb, after, want, "the run without the last plan removes its files and changes no other")
	_, err := ran(f, built(tb, f, root, plansOf(tb, f), nil), root)
	assert.NoError(tb, err, "the fixture's plans run clean again")
	kept := gone[0].Path
	edited := drifted(tb, root, kept, w.Brand())
	report, after := withoutLast(tb, f, root)
	expect.Equal(tb, after[kept], edited, "the removed plan's edited file remains")
	var warned []string
	for _, d := range codes(report, workspace.KeptOutput) {
		warned = append(warned, d.Pos.File)
	}
	expect.Contains(tb, warned, kept, "a KeptOutput warning names the edited file")
}

// AssertAudited runs the fixture's plans in root, an empty directory,
// then runs them again with a key whose completeness contract promises
// it at Warning severity on the kind of the first plan's first origin,
// which no annotator stamps. It checks that the run reports an
// UnmetContract Warning naming that origin and commits every plan.
func AssertAudited(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	plans, _, report := clean(tb, f, root)
	id, found := firstOrigin(report.Emits[plans[0].Name])
	assert.True(tb, found, "the fixture's first plan emits a unit that derives from a source declaration")
	promise := func(r *meta.Registry) error {
		if err := r.ClaimNamespace(auditNamespace); err != nil {
			return err
		}
		_, err := meta.Register[bool](r, meta.KeySpec{
			Name: auditKey,
			Contract: &meta.Completeness{
				On: []symbol.Kind{id.Kind}, By: diag.PhaseAnnotate, Severity: diag.SeverityWarning,
			},
			Doc: auditDoc,
		})
		return err
	}
	audited := built(tb, f, root, plansOf(tb, f), func(b *workspace.Builder) { b.Keys(promise) })
	promised, err := ran(f, audited, root)
	expect.NoError(tb, err, "a Warning fails no run")
	var unmet []string
	for _, d := range codes(promised, workspace.UnmetContract) {
		if subject, _, cut := strings.Cut(d.Msg, lacks); cut && d.Severity == diag.SeverityWarning {
			unmet = append(unmet, subject)
		}
	}
	expect.Contains(tb, unmet, id.Kind.String()+" "+id.Name, "an UnmetContract Warning names the origin "+id.String())
	for _, p := range promised.Plans {
		expect.Equal(tb, p.Status, workspace.PlanCommitted, "plan "+p.Name+" commits")
	}
}

// AssertChecked runs the fixture's plans in root, an empty directory,
// with a plan seeded to fail and two checks: one reads the seeded plan,
// and one reads the first plan. It checks that the run does not call
// the first check and reports one FailedDependency for it at the seeded
// failure, and that the second check reads the first plan's files as
// its commit recorded them, and its export.
func AssertChecked(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	plans := plansOf(tb, f)
	copied(tb, f, root)
	at := treeFile(tb, f)
	seeded := workspace.Plan{
		Name: failingPlan, Generators: []plugin.Generator{failure{at: at}}, Backend: plans[0].Backend,
	}
	blocked := &check{name: blockedCheck, reads: []string{failingPlan}}
	reading := &check{name: readingCheck, reads: []string{plans[0].Name}}
	w := built(tb, f, root, append(plans, seeded), func(b *workspace.Builder) {
		b.Checks(blocked, reading)
	})
	report, err := ran(f, w, root)
	expect.ErrorIs(tb, err, workspace.ErrRunFailed, "the seeded failure fails the run")
	expect.Empty(tb, blocked.called, "the check that reads the failed plan does not run")
	var reported []diag.Diag
	for _, d := range codes(report, workspace.FailedDependency) {
		if strings.Contains(d.Msg, "check "+string(blockedCheck)) {
			reported = append(reported, d)
		}
	}
	assert.Length(tb, reported, 1, "one FailedDependency for the check that does not run")
	expect.Equal(tb, reported[0].Pos, at, "at the seeded failure")
	assert.Length(tb, reading.called, 1, "the check that reads plan "+plans[0].Name+" runs once")
	assert.Length(tb, reading.called[0].Plans, 1, "the check reads the one plan it names")
	read := reading.called[0].Plans[0]
	expect.Equal(tb, read.Name, plans[0].Name, "the check reads the plan it names")
	expect.Equal(tb, read.Files, entriesOf(rundir.Record(tb, root, w.Brand()), plans[0].Name),
		"the check reads the plan's files as its commit recorded them", assert.EquateEmpty())
	expect.Equal(tb, read.Export.Plan, plans[0].Name, "the check reads the plan's export")
}

// AssertWarmEdited checks that a warm run after the fixture's edit
// produces the same results as a cold run over the edited tree.
//
// It copies the fixture's tree into the directory warm, runs the plans,
// applies the edit, and runs the plans again. The second run reads the
// sealed state that the first run wrote, so it is a warm run. It then
// copies the tree into the directory cold, applies the edit, and runs the
// plans once. Both directories must be empty. Every run also composes a
// probe plan that depends on each of the fixture's plans and returns no
// error, so every plan commits.
//
// The check fails when the edit leaves the files of the first plan
// unchanged, or when the run after the edit does not read the sealed
// state. It then compares these results of the warm run with the results
// of the cold run:
//
//   - the files outside the state directory
//   - the entries of the record
//   - the findings, which it sorts by position, code, message and origin
//     before it compares them
//   - the exports that the probe reads
//
// The rest of the state directory differs between the two runs. The
// sealed state lists what each run read, and a disk ledger names the
// workspace of each manifest document after its directory.
func AssertWarmEdited(tb assert.TB, f Fixture, warm, cold string) {
	tb.Helper()

	assert.NotNil(tb, f.Edit, "the fixture states an edit")
	first := plansOf(tb, f)[0].Name
	copied(tb, f, warm)
	before := probed(tb, f, warm, false)
	planned := entriesOf(rundir.Record(tb, warm, before.brand), first)
	assert.NoError(tb, f.Edit(warm), "the fixture's edit applies in the warm directory")
	after := probed(tb, f, warm, false)
	record := rundir.Record(tb, warm, after.brand)
	assert.NotEqual(tb, entriesOf(record, first), planned, "the fixture's edit changes the files of plan "+first,
		assert.EquateEmpty())
	assert.False(tb, after.report.Stats.Cold, "the run after the edit reads the sealed state")

	copied(tb, f, cold)
	assert.NoError(tb, f.Edit(cold), "the fixture's edit applies in the cold directory")
	fresh := probed(tb, f, cold, true)
	expect.Equal(tb, outside(tb, warm, after.brand), outside(tb, cold, fresh.brand),
		"the warm run writes the same files as the cold run outside the state directory")
	expect.Equal(tb, record.Files, rundir.Record(tb, cold, fresh.brand).Files,
		"the warm run records the same entries as the cold run")
	expect.Equal(tb, slices.SortedStableFunc(after.report.Sink.All(), diag.Diag.Compare),
		slices.SortedStableFunc(fresh.report.Sink.All(), diag.Diag.Compare),
		"the warm run reports the same findings as the cold run")
	expect.Equal(tb, after.exports, fresh.exports,
		"the probe reads the same exports in the warm run as in the cold run")
}

// exportedIn checks one plan's export against the plan's record entries:
// every exported declaration is in a recorded file, under one of the
// file's plugins and, at file level, from one of its sources, and every
// recorded file exports a declaration. Each declaration and each file
// that breaks the contract fails the check on a record of its own.
func exportedIn(tb assert.TB, doc plugin.ExportDoc, entries []manifest.Entry) {
	tb.Helper()

	byPath := make(map[string]manifest.Entry, len(entries))
	for _, e := range entries {
		byPath[e.Path] = e
	}
	recorded := slices.Sorted(maps.Keys(byPath))
	var exported []string
	for _, s := range doc.Symbols {
		exported = append(exported, s.File)
		expect.Contains(tb, recorded, s.File,
			"plan "+doc.Plan+" records the file it exports "+s.Name+" in")
		e, held := byPath[s.File]
		if !held {
			continue
		}
		expect.Contains(tb, e.Plugins, s.Plugin, "the record of "+s.File+" names the plugin that exports "+s.Name)
		if s.Host == "" && !s.Origin.IsZero() {
			expect.Contains(tb, e.Sources, s.Origin.String(),
				"the record of "+s.File+" lists the source "+s.Name+" derives from")
		}
	}
	for _, path := range recorded {
		expect.Contains(tb, exported, path, "plan "+doc.Plan+" exports a declaration in "+path+", which it records")
	}
}

// withoutLast runs the fixture's plans without the last one over the
// tree at root, and returns the report and the files under the brand's
// frame after the run. It stops the check where the composition does not
// build or the run returns an error.
func withoutLast(tb assert.TB, f Fixture, root string) (*workspace.Report, map[string]string) {
	tb.Helper()

	plans := plansOf(tb, f)
	w := built(tb, f, root, plans[:len(plans)-1], nil)
	report, err := ran(f, w, root)
	assert.NoError(tb, err, "the run without the last plan is clean")
	return report, rundir.Framed(tb, root, w.Brand())
}

// drifted inserts an empty line before the final line of a generated
// file under root, which is the file's trailer. The file then differs
// from its stamp and still has the brand's frame. It returns the edited
// text, and stops the check where the file does not read, the edit
// loses the frame, or the file does not write.
func drifted(tb assert.TB, root, path string, brand output.Brand) string {
	tb.Helper()

	text := files.Read(tb, rundir.Path(root, path))
	cut := strings.LastIndexByte(strings.TrimSuffix(text, "\n"), '\n') + 1
	edited := text[:cut] + "\n" + text[cut:]
	p, framed := output.Read([]byte(edited))
	assert.True(tb, framed, "an empty line before the trailer of "+path+" keeps a frame")
	assert.Equal(tb, p.Brand, brand, "the frame of "+path+" names the brand")
	files.Write(tb, root, files.Tree{path: files.Text(edited)})
	return edited
}

// codes returns the findings of one code in a run's report.
func codes(report *workspace.Report, code diag.Code) []diag.Diag {
	var out []diag.Diag
	for d := range report.Sink.All() {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

// entriesOf returns the entries a record lists under one plan, in path
// order.
func entriesOf(m manifest.Manifest, plan string) []manifest.Entry {
	var out []manifest.Entry
	for _, e := range m.Files {
		if e.Plan == plan {
			out = append(out, e)
		}
	}
	return out
}

// outside returns the text of every file under root outside the brand's
// state directory, keyed by its slash-separated path relative to root:
// the tree's sources and the files the plans generate.
func outside(tb assert.TB, root string, brand output.Brand) map[string]string {
	tb.Helper()

	state := ledger.StateDir(brand) + "/"
	out := map[string]string{}
	for _, path := range rundir.Files(tb, root) {
		if strings.HasPrefix(path, state) {
			continue
		}
		out[path] = files.Read(tb, rundir.Path(root, path))
	}
	return out
}

// firstOrigin returns the first origin of a plan's emit store, in the
// store's unit order, and reports false for a store whose units derive
// from no source declaration.
func firstOrigin(e *plugin.Emit) (symbol.Identity, bool) {
	for u := range e.Units() {
		if len(u.Origins) > 0 {
			return u.Origins[0], true
		}
	}
	return symbol.Identity{}, false
}
