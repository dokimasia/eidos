// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspacetest

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/assert"

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

// AssertGenerated runs the fixture's plans in root, an empty directory,
// and checks that the files under the brand's frame are the wanted
// files, byte for byte, and that the record lists each file under the
// plan whose commit wrote it.
func AssertGenerated(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	_, w, report := clean(tb, f, root)
	assert.Equal(tb, rundir.Framed(tb, root, w.Brand()), rundir.Texts(f.Want),
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
	assert.Equal(tb, recorded, committed, "the record lists each file under the plan whose commit wrote it")
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
	if len(collisions) == 0 {
		tb.Fatalf("the run reports no PlanCollision for a copy of plan %q", plans[0].Name)
	}
	for _, d := range collisions {
		assert.Contains(tb, d.Msg, strconv.Quote(plans[0].Name), "the collision names the copied plan")
		assert.Contains(tb, d.Msg, strconv.Quote(copyPlan), "the collision names the copy")
	}
	assert.ErrorIs(tb, err, workspace.ErrRunFailed, "the collision fails the run")
	for _, p := range report.Plans {
		assert.Equal(tb, p.Status, workspace.PlanFailed, "plan "+p.Name+" commits nothing")
	}
	_, err = os.Stat(rundir.Path(root, ledger.ManifestPath(w.Brand())))
	assert.True(tb, errors.Is(err, fs.ErrNotExist), "the run records nothing")
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
	assert.ErrorIs(tb, err, workspace.ErrRunFailed, "the seeded failure fails the run")
	for _, p := range report.Plans {
		if p.Name == last.Name {
			assert.Equal(tb, p.Status, workspace.PlanFailed, "the plan seeded to fail commits nothing")
			continue
		}
		assert.Equal(tb, p.Status, workspace.PlanCommitted, "plan "+p.Name+" commits beside it")
	}
	kept := entriesOf(before, last.Name)
	assert.True(tb, sameEntries(entriesOf(rundir.Record(tb, root, again.Brand()), last.Name), kept),
		"the failed plan's entries remain")
	after := rundir.Framed(tb, root, again.Brand())
	for _, e := range kept {
		assert.Equal(tb, after[e.Path], files[e.Path], "the failed plan's file "+e.Path+" remains")
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

	plans := plansOf(tb, f)
	copied(tb, f, root)
	names := make([]string, 0, len(plans))
	for _, p := range plans {
		names = append(names, p.Name)
	}
	p := &probe{}
	probing := workspace.Plan{
		Name: probePlan, DependsOn: names, Generators: []plugin.Generator{p}, Backend: plans[0].Backend,
	}
	w := built(tb, f, root, append(plans, probing), nil)
	_, err := ran(f, w, root)
	assert.NoError(tb, err, "the fixture's plans and the probe run clean")
	assert.Equal(tb, slices.Sorted(maps.Keys(p.got)), slices.Sorted(slices.Values(names)),
		"the probe reads the export of each plan it depends on")
	record := rundir.Record(tb, root, w.Brand())
	for _, name := range names {
		exportedIn(tb, p.got[name], entriesOf(record, name))
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
	assert.Contains(tb, err.Error(), strconv.Quote(cycleA), "the error names the first plan of the cycle")
	assert.Contains(tb, err.Error(), strconv.Quote(cycleB), "the error names the second plan of the cycle")
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
	if len(gone) == 0 {
		tb.Fatalf("the fixture's last plan %q routes no file", last)
	}
	want := rundir.Framed(tb, root, w.Brand())
	for _, e := range gone {
		delete(want, e.Path)
	}
	_, after := withoutLast(tb, f, root)
	assert.Equal(tb, after, want, "the run without the last plan removes its files and changes no other")
	_, err := ran(f, built(tb, f, root, plansOf(tb, f), nil), root)
	assert.NoError(tb, err, "the fixture's plans run clean again")
	kept := gone[0].Path
	edited := drifted(tb, root, kept, w.Brand())
	report, after := withoutLast(tb, f, root)
	assert.Equal(tb, after[kept], edited, "the removed plan's edited file remains")
	warned := slices.ContainsFunc(codes(report, workspace.KeptOutput), func(d diag.Diag) bool {
		return d.Pos.File == kept
	})
	assert.True(tb, warned, "a KeptOutput warning names the edited file")
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
	if !found {
		tb.Fatalf("the fixture's first plan emits nothing that derives from a source declaration")
	}
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
	assert.NoError(tb, err, "a Warning fails no run")
	unmet := slices.ContainsFunc(codes(promised, workspace.UnmetContract), func(d diag.Diag) bool {
		return d.Severity == diag.SeverityWarning && strings.Contains(d.Msg, id.Kind.String()+" "+id.Name+" lacks")
	})
	assert.True(tb, unmet, "an UnmetContract Warning names the origin "+id.String())
	for _, p := range promised.Plans {
		assert.Equal(tb, p.Status, workspace.PlanCommitted, "plan "+p.Name+" commits")
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
	assert.ErrorIs(tb, err, workspace.ErrRunFailed, "the seeded failure fails the run")
	assert.Length(tb, blocked.called, 0, "the check that reads the failed plan does not run")
	var reported []diag.Diag
	for _, d := range codes(report, workspace.FailedDependency) {
		if strings.Contains(d.Msg, "check "+string(blockedCheck)) {
			reported = append(reported, d)
		}
	}
	assert.Length(tb, reported, 1, "one FailedDependency for the check that does not run")
	assert.Equal(tb, reported[0].Pos, at, "at the seeded failure")
	assert.Length(tb, reading.called, 1, "the check that reads plan "+plans[0].Name+" runs once")
	assert.Length(tb, reading.called[0].Plans, 1, "the check reads the one plan it names")
	read := reading.called[0].Plans[0]
	assert.Equal(tb, read.Name, plans[0].Name, "the check reads the plan it names")
	assert.True(tb, sameEntries(read.Files, entriesOf(rundir.Record(tb, root, w.Brand()), plans[0].Name)),
		"the check reads the plan's files as its commit recorded them")
	assert.Equal(tb, read.Export.Plan, plans[0].Name, "the check reads the plan's export")
}

// exportedIn checks one plan's export against the plan's record entries:
// every exported declaration is in a recorded file, under one of the
// file's plugins and, at file level, from one of its sources, and every
// recorded file exports a declaration.
func exportedIn(tb assert.TB, doc plugin.ExportDoc, entries []manifest.Entry) {
	tb.Helper()

	byPath := make(map[string]manifest.Entry, len(entries))
	for _, e := range entries {
		byPath[e.Path] = e
	}
	files := map[string]bool{}
	for _, s := range doc.Symbols {
		files[s.File] = true
		e, recorded := byPath[s.File]
		switch {
		case !recorded:
			tb.Errorf("plan %q exports %s in %s, which its record does not list", doc.Plan, s.Name, s.File)
		case !slices.Contains(e.Plugins, s.Plugin):
			tb.Errorf("plan %q exports %s from plugin %s, which the record of %s does not name", doc.Plan,
				s.Name, s.Plugin, s.File)
		case s.Host == "" && !s.Origin.IsZero() && !slices.Contains(e.Sources, s.Origin.String()):
			tb.Errorf("plan %q exports %s derived from %s, which the record of %s does not list as a source",
				doc.Plan, s.Name, s.Origin, s.File)
		}
	}
	for _, e := range entries {
		if !files[e.Path] {
			tb.Errorf("plan %q records %s, and its export lists no declaration in it", doc.Plan, e.Path)
		}
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

	b, err := os.ReadFile(rundir.Path(root, path))
	assert.NoError(tb, err, "the generated file "+path+" reads")
	text := string(b)
	cut := strings.LastIndexByte(strings.TrimSuffix(text, "\n"), '\n') + 1
	edited := text[:cut] + "\n" + text[cut:]
	p, framed := output.Read([]byte(edited))
	assert.True(tb, framed && p.Brand == brand, "an empty line before the trailer of "+path+" keeps the brand's frame")
	assert.NoError(tb, os.WriteFile(rundir.Path(root, path), []byte(edited), 0o644), "the edit of "+path+" writes")
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

// sameEntries reports whether two lists record the same files alike,
// a nil list and an empty one alike, as a record compares them.
func sameEntries(a, b []manifest.Entry) bool {
	return manifest.Manifest{Version: manifest.Version, Files: a}.
		Equal(manifest.Manifest{Version: manifest.Version, Files: b})
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
