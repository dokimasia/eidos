// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspacetest

import (
	"context"
	"io/fs"
	"os"
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/workspace"
)

// The names of the plans, plugins and checks the suite adds to a
// fixture's composition. Each opens with the suite's name, so none of
// them meets a name of the fixture's own.
const (
	copyPlan               = "workspacetest-copy"
	failingPlan            = "workspacetest-failing"
	probePlan              = "workspacetest-probe"
	cycleA                 = "workspacetest-cycle-a"
	cycleB                 = "workspacetest-cycle-b"
	failureID    plugin.ID = "workspacetest-failure"
	probeID      plugin.ID = "workspacetest-probe"
	blockedCheck plugin.ID = "workspacetest-blocked"
	readingCheck plugin.ID = "workspacetest-reading"
)

// failureCode is the code the suite's seeded failure reports under.
var failureCode = diag.MustRegister(diag.Prefix("WSTEST"), diag.CodeSpec{
	Number:  1,
	Meaning: "the workspace suite seeds a failure into a plan",
})

// Fixture is a multi-plan case: a source tree, the stores its load
// reads, the composition without its plans, the plans, every file the
// plans generate, and an edit of the tree.
type Fixture struct {
	// Tree is the source tree. A check copies it into each directory it
	// runs the fixture in.
	Tree fs.FS
	// Stores are the read-only trees the load's dependency units read,
	// keyed by store name.
	Stores map[string]fs.FS
	// Compose returns a builder over root, the directory a check runs
	// the fixture in, with every part of the composition except its
	// plans: the brand, the frontends, the annotators, the rules, the
	// targets, a disk sink at root and the state directory's ledger
	// under root. A check adds plans, checks and keys before it builds.
	// The suite runs its checks in parallel, so Compose returns fresh
	// plugin instances on every call and is safe for concurrent use.
	Compose func(root string) *workspace.Builder
	// Plans returns the fixture's plans, with fresh generator and
	// backend instances on every call: at least two, each routing at
	// least one file, no two routing a file to one path, the first
	// routing a file that derives from a source declaration, and a last
	// one that no other plan depends on and whose generators register no
	// directive the tree writes, so the composition without it loads the
	// tree clean.
	Plans func() []workspace.Plan
	// Want is every file the plans generate, frame included, keyed by
	// its workspace-relative, slash-separated path.
	Want map[string][]byte
	// Edit changes one declaration of the tree at root that the first
	// plan generates from, the way a person edits a source file: the
	// first plan's output changes, and its export does not. A warm check
	// applies it between a cold run and a warm one.
	Edit func(root string) error
}

// RunWorkspaceSuite checks the fixture against the workspace frame:
// [AssertGenerated], [AssertCollision], [AssertIsolated],
// [AssertExported], [AssertCycleRefused], [AssertSwept],
// [AssertAudited] and [AssertChecked], each in a parallel subtest over
// a temporary directory of its own.
func RunWorkspaceSuite(t *testing.T, f Fixture) {
	t.Helper()

	checks := []struct {
		name  string
		check func(assert.TB, Fixture, string)
	}{
		{name: "generates the wanted files", check: AssertGenerated},
		{name: "refuses two plans that route a file to one path", check: AssertCollision},
		{name: "commits every plan beside a failed one", check: AssertIsolated},
		{name: "hands a dependent the export of each plan", check: AssertExported},
		{name: "refuses a cycle of dependencies", check: AssertCycleRefused},
		{name: "removes the files of a removed plan", check: AssertSwept},
		{name: "reports an unmet contract at its severity", check: AssertAudited},
		{name: "runs a check over the records it reads", check: AssertChecked},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.check(t, f, t.TempDir())
		})
	}
}

// failure is a generator that reports one Error at a file of the
// fixture's tree and emits nothing: the failure the suite seeds into a
// plan.
type failure struct {
	at position.Pos
}

// Name returns the seeded failure's name.
func (failure) Name() plugin.ID { return failureID }

// Generate reports the Error.
func (g failure) Generate(ctx *plugin.GeneratorContext) error {
	ctx.Sink.Errorf(failureCode, g.at, ctx.Plugin, "the workspace suite seeds a failure into this plan")
	return nil
}

// probe is a generator that records the exports its context hands over
// and emits nothing: the dependent the export checks compose.
type probe struct {
	got map[string]plugin.ExportDoc
}

// withProbe returns the plans followed by a probe plan that depends on
// each of them and renders through the first plan's backend, and the
// probe that records the exports the probe plan reads.
func withProbe(plans []workspace.Plan) ([]workspace.Plan, *probe) {
	p := &probe{}
	probing := workspace.Plan{
		Name: probePlan, DependsOn: planNames(plans), Generators: []plugin.Generator{p}, Backend: plans[0].Backend,
	}
	return append(slices.Clip(plans), probing), p
}

// Name returns the probe's name.
func (*probe) Name() plugin.ID { return probeID }

// Generate records the exports.
func (p *probe) Generate(ctx *plugin.GeneratorContext) error {
	p.got = ctx.Exports
	return nil
}

// probeRun is one run of the fixture's plans beside a probe plan that
// depends on each of them: the run's report, the exports the probe read,
// and the brand the composition declares.
type probeRun struct {
	report  *workspace.Report
	exports map[string]plugin.ExportDoc
	brand   output.Brand
}

// probed runs the fixture's plans beside a probe plan that depends on
// each of them over the tree at root, and ignores the sealed state where
// cold is set. It stops the check where the composition does not build,
// and fails it where the run returns an error.
func probed(tb assert.TB, f Fixture, root string, cold bool) probeRun {
	tb.Helper()

	plans, p := withProbe(plansOf(tb, f))
	w := built(tb, f, root, plans, nil)
	report, err := w.Run(context.Background(), workspace.Input{Tree: os.DirFS(root), Stores: f.Stores, Cold: cold})
	assert.NoError(tb, err, "the fixture's plans and the probe run clean")
	return probeRun{report: report, exports: p.got, brand: w.Brand()}
}

// check is a workspace check that reads the plans it names and records
// the contexts it is called with.
type check struct {
	name   plugin.ID
	reads  []string
	called []*plugin.CheckContext
}

// Name returns the check's name.
func (c *check) Name() plugin.ID { return c.name }

// Reads returns the plans the check names.
func (c *check) Reads() []string { return c.reads }

// Check records the call.
func (c *check) Check(ctx *plugin.CheckContext) error {
	c.called = append(c.called, ctx)
	return nil
}

// copied copies the fixture's tree into root, an empty directory. It
// stops the check where the fixture states no tree or the tree does not
// copy.
func copied(tb assert.TB, f Fixture, root string) {
	tb.Helper()

	if f.Tree == nil {
		tb.Fatalf("the fixture states no tree")
	}
	assert.NoError(tb, os.CopyFS(root, f.Tree), "the fixture's tree copies into the run's directory")
}

// plansOf returns the fixture's plans. It stops the check where the
// fixture states no plans, or fewer than two.
func plansOf(tb assert.TB, f Fixture) []workspace.Plan {
	tb.Helper()

	if f.Plans == nil {
		tb.Fatalf("the fixture states no plans")
	}
	plans := f.Plans()
	if len(plans) < 2 {
		tb.Fatalf("the suite checks at least two plans, and the fixture states %d", len(plans))
	}
	return plans
}

// planNames returns the names of the plans, in their order.
func planNames(plans []workspace.Plan) []string {
	names := make([]string, 0, len(plans))
	for _, p := range plans {
		names = append(names, p.Name)
	}
	return names
}

// composed returns the fixture's composition over root with the plans,
// edited before it builds, and Build's error. It stops the check where
// the fixture states no composition.
func composed(
	tb assert.TB, f Fixture, root string, plans []workspace.Plan, edit func(*workspace.Builder),
) (*workspace.Workspace, error) {
	tb.Helper()

	if f.Compose == nil {
		tb.Fatalf("the fixture states no composition")
	}
	b := f.Compose(root).Plans(plans...)
	if edit != nil {
		edit(b)
	}
	return b.Build()
}

// built returns the fixture's composition over root with the plans,
// edited before it builds. It stops the check where the composition
// does not build.
func built(
	tb assert.TB, f Fixture, root string, plans []workspace.Plan, edit func(*workspace.Builder),
) *workspace.Workspace {
	tb.Helper()

	w, err := composed(tb, f, root, plans, edit)
	assert.NoError(tb, err, "the fixture's composition builds")
	return w
}

// ran runs a workspace over the tree at root, reading the fixture's
// stores, and returns the report and the run's error.
func ran(f Fixture, w *workspace.Workspace, root string) (*workspace.Report, error) {
	return w.Run(context.Background(), workspace.Input{Tree: os.DirFS(root), Stores: f.Stores})
}

// clean copies the fixture's tree into root, an empty directory, runs
// the fixture's plans over it, and returns the plans, the workspace and
// the report. It stops the check where the run returns an error.
func clean(tb assert.TB, f Fixture, root string) ([]workspace.Plan, *workspace.Workspace, *workspace.Report) {
	tb.Helper()

	plans := plansOf(tb, f)
	copied(tb, f, root)
	w := built(tb, f, root, plans, nil)
	report, err := ran(f, w, root)
	assert.NoError(tb, err, "the fixture's plans run clean")
	return plans, w, report
}

// treeFile returns the position of the first file of the fixture's
// tree, where the seeded failure reports. It stops the check where the
// tree does not walk or contains no file.
func treeFile(tb assert.TB, f Fixture) position.Pos {
	tb.Helper()

	var first string
	err := fs.WalkDir(f.Tree, ".", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && first == "" {
			first = path
		}
		return err
	})
	assert.NoError(tb, err, "the fixture's tree walks")
	if first == "" {
		tb.Fatalf("the fixture's tree contains no file")
	}
	return position.Pos{File: first, Line: 1, Col: 1}
}
