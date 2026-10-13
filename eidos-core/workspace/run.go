// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Input is the input of one run. It is either a tree that the frontends of
// the composition load, or a graph that the caller loaded or built.
// Exactly one of Tree and Graph is set.
type Input struct {
	// Tree is the workspace tree. Every path in a run is relative to
	// its root.
	Tree fs.FS
	// Stores maps the name of each store to a read-only tree that
	// dependency units read, such as a Go module cache or the ct.sym file
	// of a JDK.
	Stores map[string]fs.FS
	// Getenv reads the environment for the store locators of the
	// frontends. The load of the run calls the locator of a frontend only
	// when the tree has an input file of the frontend. It adds the stores
	// that the locator returns to Stores. A nil Getenv turns the location
	// of stores off.
	Getenv func(key string) string
	// Graph is a sealed graph, or a graph that the run seals. The run
	// starts at directive validation and audits every declaration in it.
	Graph *store.Graph
	// Plans restricts the run to the plans that it lists and to every plan
	// that they depend on, transitively. An empty list runs every plan. A
	// plan that the selection leaves out reports [PlanSkipped]. It does
	// not run, and its files and records remain. Run returns an error for
	// a name that the composition does not declare.
	Plans []string
	// Patterns restricts the commits of the run to the changes inside the
	// patterns. A change is inside the patterns where its file derives
	// from a declaration of a package that the patterns admit, in the file
	// that the run renders or in the entry of the previous record. The
	// patterns admit packages as [Sources.Packages] does. The removal of a
	// stale file whose plugins left its plan is inside the patterns too.
	// The run loads, validates, annotates and generates as a run without
	// patterns does, and reports every other change in
	// [PlanReport.Withheld]. Run returns an error for a pattern that
	// matches no directory of a workspace tree.
	Patterns []string
	// Dry runs every phase and commits nothing. Each plan stages, prepares
	// and discards its files, and the ledger records nothing.
	Dry bool
	// Cold runs the tree without a read of the sealed state, and reports
	// no finding about the state. The run writes the next generation as
	// any run does.
	Cold bool
	// OverwriteDrift lets a plan write over a generated file that was
	// edited since its stamp, where the run otherwise reports
	// [DriftedOutput].
	OverwriteDrift bool
	// Adopt lets a plan write over a file without the frame of the brand
	// at a path that the plan routes a file to. Without Adopt, the run
	// reports [ForeignFile].
	Adopt bool
	// Strict reports every Warning as an Error, so a Warning fails the
	// plan that reported it. A Warning of a phase that every plan shares
	// fails every plan. A diag directive removes a Warning that it
	// suppresses before Strict promotes it.
	Strict bool
	// Check runs as Dry runs. It reports each path that the run would
	// create, update, remove or refuse to write as an Error under
	// [OutOfDate], so a gate needs only the error of the run.
	Check bool
	// Prune runs every plan as a run without patterns does, and commits
	// only the removal of stale files. [PlanReport.Withheld] lists every
	// write. Run returns an error for Prune with Patterns or with Check.
	Prune bool
	// Caller describes the caller in the record of the holder of the
	// lock, such as the command line of the run. An empty Caller records
	// "workspace.Run".
	Caller string
}

// defaultCaller is the caller that a run records in the holder record of
// the lock where [Input.Caller] is empty.
const defaultCaller = "workspace.Run"

// overwritten returns the verdicts whose files the input lets the plans
// write over. It returns [output.FoundDrifted] under OverwriteDrift and
// [output.FoundForeign] under Adopt.
func (in Input) overwritten() []output.Found {
	var found []output.Found
	if in.OverwriteDrift {
		found = append(found, output.FoundDrifted)
	}
	if in.Adopt {
		found = append(found, output.FoundForeign)
	}
	return found
}

// Run runs the frame over the input and returns the report. The frame
// runs these steps:
//
//   - Begin, Load, directive validation and Annotate
//   - Generate, Settle, Layout, Render, Stamp and staging of each plan,
//     in parallel, where a plan that depends on other plans runs after
//     each of them has rendered
//   - Close with its workspace checks
//   - the commits in dependency order, and the record
//
// A plan that reports an Error commits nothing, and a plan that depends
// on it commits nothing either. The other plans commit. An Error in a
// phase that every plan shares commits nothing at all. Those phases are
// Load, validation, Annotate and Close, and an Error of a workspace check
// is an Error of Close. The sink of the report contains every finding,
// and any Error returns [ErrRunFailed] with the report. Run joins the
// error that a handler returns into its own error, and wraps it with the
// role of the handler:
//
//   - The error of an annotator stops the frame.
//   - The error of a generator fails its plan.
//   - The error of a check fails Close.
//
// A cancelled context stops the run between units of work. Run then
// returns the error of the context with the report, which contains the
// outcome of each plan. Run returns an error and a nil report, and runs
// nothing, for these inputs:
//
//   - neither or both of a tree and a graph
//   - a plan that the composition does not declare
//   - a pattern that matches no directory of a workspace tree
//   - Prune with Patterns or with Check
//
// A run under [Input.Patterns] or [Input.Prune] commits the changes that
// they admit. It prepares every other change in a sink of its own,
// discards the sink, and lists those changes in [PlanReport.Withheld]. A
// withheld change leaves the entry of its path in the record unchanged.
// The sweep of the plans that the composition no longer declares commits
// whatever the patterns admit. Under [Input.Check], each change that the
// run prepared and does not withhold is an Error under [OutOfDate] at its
// path.
//
// A diag directive removes the findings of its code at its declaration
// from the sink of the run and from the sink of every plan.
// [Input.Strict] then reports every Warning that remains as an Error.
//
// The ledger records the run strictly after the last commit. The record
// is the next generation of the sealed state, with each document of the
// merged manifest that differs from the document in the ledger. A dry run
// records nothing. These runs do not write a generation, and they record
// the manifest only where a plan or the sweep committed:
//
//   - a run over a graph of the caller
//   - a run that cannot read its previous record
//   - a run that cannot read its executable
//   - a run whose stamp replay refused a stamp or found a dangling one.
//     No record of the sealed state keeps that finding.
//   - a run with patterns, a prune, and a run that skipped a plan. The
//     next run then compares against the generation and finds every
//     change that such a run left.
//
// The ledger records under a context that the cancellation of the run
// does not end, because the record has to match the destination once a
// commit wrote to it.
//
// Run takes the lock of the ledger before it reads the previous record.
// It releases the lock after the record, and after a second cold attempt
// where one runs. Run reports a lock that another holder has under
// [StateLocked] at the lock file of the state directory. The run then
// marks every plan failed, writes nothing and returns [ErrRunFailed]. Run
// returns the error of a ledger that fails to open or to lock, and the
// error of a release that fails.
//
// A run over a tree compares the tree with the live generation of the
// sealed state. A warm run validates again only the subjects whose
// directives changed or whose validation read a declaration that changed.
// It runs again only the annotator invocations whose subject changed or
// that read a changed declaration, package or fact. An annotator that
// implements its role directly runs whole when the load found any change.
// The run reports the findings of every validation and annotator
// invocation that it keeps. A run can find the generation damaged at the
// load, at the record of the load, at a record of the phases, or at a
// region that its graph decodes for a later phase. Such a run discards
// what it derived before any plan commits, reports [ColdState], and runs
// again cold over the same input.
func (w *Workspace) Run(ctx context.Context, in Input) (*Report, error) {
	if (in.Tree == nil) == (in.Graph == nil) {
		return nil, errors.New("workspace: Run needs exactly one of a tree to load and a graph")
	}
	if in.Prune && (len(in.Patterns) > 0 || in.Check) {
		return nil, errors.New("workspace: a prune reconciles the whole workspace, and takes no patterns or check")
	}
	for _, p := range in.Patterns {
		if _, valid := parsePattern(p); !valid {
			return nil, fmt.Errorf("workspace: the pattern %q names no directory of a workspace tree", p)
		}
	}
	selected, err := w.selected(in.Plans)
	if err != nil {
		return nil, err
	}
	in.Dry = in.Dry || in.Check
	l, release, err := w.lock(ctx, in.Caller)
	if err != nil {
		report := &Report{Sink: diag.NewSink(), Facts: meta.NewFacts(w.keys), Emits: map[string]*plugin.Emit{}}
		if locked, is := errors.AsType[*ledger.LockedError](err); is {
			report.Sink.Errorf(StateLocked, position.Pos{File: ledger.LockPath(w.brand)}, diag.PhaseLoad,
				"the state directory is locked by %v, and the run writes nothing", locked.Holder)
			err = nil
		}
		return w.stopped(ctx, report, selected, err)
	}
	report, err := w.run(ctx, in, l, selected, nil)
	if d, found := errors.AsType[*damage](err); found && !in.Cold {
		in.Cold = true
		report, err = w.run(ctx, in, l, selected, d)
	}
	return report, errors.Join(err, release())
}

// selected reports for each plan of the composition whether a run that
// lists names runs the plan. Such a run runs each listed plan and every
// plan that a listed plan depends on, transitively. selected returns nil
// for no names, which selects every plan. One backward pass over the
// commit order selects the dependencies, because the backward order puts
// each plan before the plans that it depends on.
//
// Error modes: a name that the composition does not declare.
func (w *Workspace) selected(names []string) ([]bool, error) {
	if len(names) == 0 {
		return nil, nil
	}
	byName := make(map[string]int, len(w.plans))
	for i := range w.plans {
		byName[w.plans[i].name] = i
	}
	out := make([]bool, len(w.plans))
	for _, name := range names {
		i, declared := byName[name]
		if !declared {
			return nil, fmt.Errorf("workspace: Run selects the plan %q, which the composition does not declare", name)
		}
		out[i] = true
	}
	for _, i := range slices.Backward(w.order) {
		for _, d := range w.plans[i].deps {
			out[d] = out[d] || out[i]
		}
	}
	return out, nil
}

// lock opens the ledger of the composition and takes its lock for the
// run. It returns the ledger and the function that releases the lock. For
// a composition that declares no ledger, it returns a nil ledger and a
// release function that does nothing. Build refuses a ledger without an
// output, so a composition with a ledger has an output. The record of the
// holder contains the process, the host, the caller and the time at which
// the run took the lock.
//
// Error modes:
//   - the error of an open function that fails, or an error for an open
//     function that returns no ledger
//   - a *[ledger.LockedError] where another holder has the lock
//   - the errors of the lock itself
func (w *Workspace) lock(ctx context.Context, caller string) (ledger.Ledger, func() error, error) {
	if w.ledger == nil {
		return nil, func() error { return nil }, nil
	}
	l, err := w.ledger()
	if err != nil {
		return nil, nil, fmt.Errorf("workspace: open the ledger: %w", err)
	}
	if l == nil {
		return nil, nil, errors.New("workspace: the ledger's open function returned (nil, nil)")
	}
	// The record contains no host where the operating system reports none.
	host, _ := os.Hostname()
	release, err := l.Lock(ctx, ledger.Holder{
		PID:    os.Getpid(),
		Host:   host,
		Caller: cmp.Or(caller, defaultCaller),
		Since:  time.Now(),
	})
	if err != nil {
		return nil, nil, err
	}
	return l, release, nil
}

// run makes one attempt at a run, which runs the frame over the input. l
// is the ledger of the composition, and nil where the composition keeps
// none. [Workspace.Run] runs again cold where the attempt met damage.
// selected is the selection of [Workspace.selected]. A cold attempt after
// damage reports the damage first.
func (w *Workspace) run(
	ctx context.Context, in Input, l ledger.Ledger, selected []bool, after *damage,
) (*Report, error) {
	sink := diag.NewSink()
	if in.Strict {
		sink.Promote()
	}
	if after != nil {
		w.coldState(sink, "the sealed state is damaged, and the run started again cold: %v", after.err)
	}
	facts := meta.NewFacts(w.keys)
	report := &Report{Sink: sink, Facts: facts, Emits: map[string]*plugin.Emit{}}

	rec := w.begin(ctx, sink, l)
	if err := ctx.Err(); err != nil {
		return w.stopped(ctx, report, selected, err)
	}
	report.Manifest = rec.previous
	sealed, err := w.openSealed(ctx, rec, in, sink)
	if err == nil {
		err = w.openMemo(ctx, rec, sealed)
	}
	if err != nil {
		return w.stopped(ctx, report, selected, err)
	}
	g, loaded, err := w.load(ctx, in, sealed, sink)
	report.Load = loaded
	report.Stats.count(loaded, sealed)
	if err == nil {
		err = sealed.record(ctx, loaded)
	}
	if err != nil {
		return w.stopped(ctx, report, selected, err)
	}
	var phases *state.PhaseState
	if sealed.warm(loaded) {
		phases = sealed.gen.Phases(ctx)
		facts = meta.Restore(w.keys, phases)
		report.Facts = facts
	}
	out, err := w.annotateRun(ctx, g, loaded, phases, facts, sink, &report.Stats, sealed.recorder)
	if out.refused {
		sealed.commit, sealed.recorder = nil, nil
	}
	if err != nil {
		return w.stopped(ctx, report, selected, err)
	}
	pol := policyOf(g, out.table, in.Strict)
	sink.Suppress(pol.table)
	shared := sink.Failed()

	var src tree
	if w.open != nil {
		src = tree{residents: residentsOf(g, loaded), modules: layout.ModulesOf(out.modules), packages: g.PackageOf}
		if out.modules == nil {
			src.modules = layout.Modules(g, facts, w.kernel)
		}
	}
	var ws *warmState
	if phases != nil {
		ws = &warmState{
			phases: phases, dirty: out.dirty, candidates: out.candidates, prior: out.prior,
			moved: loaded.Moved, vanished: loaded.Vanished, tree: in.Tree, previous: rec.byPlan,
		}
	}
	runs := w.generateAll(ctx, g, facts, out.table, src, sealed.recorder, ws, selected, rec.byPlan, pol)
	if !shared {
		w.stageAll(ctx, runs, rec, in.overwritten(), w.narrowed(in, g, facts, rec))
	}
	// recordErr is the damage that the plans, the collisions, the audit and
	// the checks met in the records of the generation.
	var recordErr error
	for _, p := range runs {
		report.Emits[p.plan.name] = p.emit
		report.Stats.Invoked = append(report.Stats.Invoked, p.invoked...)
		report.Stats.Rendered += len(p.files)
		for d := range p.sink.All() {
			sink.Report(d)
		}
		if errors.Is(p.err, state.ErrDamaged) {
			recordErr = cmp.Or(recordErr, fmt.Errorf("workspace: plan %q: %w", p.plan.name, p.err))
		}
	}

	var (
		collided, unmet      bool
		collideErr, auditErr error
	)
	if phases != nil {
		collided, collideErr = collideWarm(runs, phases, sink)
		recordErr = cmp.Or(recordErr, collideErr)
	} else {
		collided = collide(runs, routedFiles(runs), sink)
	}
	sw, err := w.sweep(rec, runs, sink, shared || collided)
	errs := []error{err}
	if phases != nil {
		unmet, auditErr = w.auditWarm(g, facts, loaded, phases, out.candidates, sink, sealed.recorder)
		recordErr = cmp.Or(recordErr, auditErr)
	} else {
		unmet = w.audit(g, facts, loaded, sink, sealed.recorder)
	}
	checked := false
	if !shared {
		checked, err = w.check(g, facts, out.table, runs, sink, &report.Stats, sealed.recorder, ws)
		if errors.Is(err, state.ErrDamaged) {
			recordErr, err = cmp.Or(recordErr, err), nil
		}
		errs = append(errs, err)
	}
	broken := damaged(cmp.Or(g.Damaged(), facts.Damaged(), out.damaged(), recordErr))
	if broken == nil {
		broken = sealed.recordPhases(ctx, g, facts, w.kernel, out.modules)
	}
	blocked := shared || collided || unmet || checked || broken != nil

	errs = append(errs, commitAll(ctx, runs, w.order, sw, blocked, in.Dry, report))
	if in.Check {
		outOfDate(runs, sw, rec, sink)
	}
	report.Suppressions, report.Suppressed = pol.counted(sink, runs)
	if ctx.Err() == nil && !slices.Contains(selected, false) {
		unused(sink, report.Suppressions)
	}
	if broken != nil {
		return report, errors.Join(append(errs, broken)...)
	}
	report.Manifest = merged(rec, runs, sw)
	partial := in.Prune || len(in.Patterns) > 0 || slices.Contains(selected, false)
	if sealed.commit != nil && !partial {
		errs = append(errs, sealed.write(ctx, report.Manifest, runs, &report.Stats))
	} else {
		errs = append(errs, commitRecord(ctx, rec, runs, sw, in.Dry, report.Manifest))
	}
	if sealed.memo != nil && !in.Dry {
		errs = append(errs, sealed.writeMemo(ctx))
	}
	if loaded != nil {
		report.Stats.Decoded = loaded.Decoded()
	}
	for _, p := range runs {
		if p.err != nil && !p.cancelled {
			errs = append(errs, fmt.Errorf("workspace: plan %q: %w", p.plan.name, p.err))
		}
	}
	errs = append(errs, ctx.Err(), failure(sink))
	return report, errors.Join(errs...)
}

// record is the record of the previous run as a run reads it. It contains
// the ledger that it came from, the manifest, the digest of each document
// of the manifest, the entries by plan and by path, and the name of the
// workspace under which the run records. The ledger is nil where the
// composition keeps none, and the digests are nil where the record cannot
// be read.
type record struct {
	ledger   ledger.Ledger
	previous manifest.Manifest
	digests  state.Digests
	byPlan   map[string][]manifest.Entry
	byPath   map[string]manifest.Entry
	// workspace is the name of the workspace in the manifest that the run
	// records. It is the name of the composition, or the name that the
	// ledger returns where the composition has none.
	workspace string
}

// named is a ledger that returns the name of the workspace that it
// records. The disk ledger is a named ledger, and derives the name from
// the workspace root.
type named interface {
	Workspace() string
}

// generator returns the plan that the record lists for a path, and
// fallback where the record lists no file at the path.
func (r *record) generator(path, fallback string) string {
	if e, held := r.byPath[path]; held {
		return e.Plan
	}
	return fallback
}

// begin reads the previous record from the ledger l of the composition,
// and joins the documents of its manifest. A composition without a
// ledger, where l is nil, reads the empty record. begin reports a record
// that cannot be read and reads it as empty, so the run removes nothing.
func (w *Workspace) begin(ctx context.Context, sink *diag.Sink, l ledger.Ledger) *record {
	rec := &record{
		previous:  manifest.Manifest{Version: manifest.Version},
		byPlan:    map[string][]manifest.Entry{},
		byPath:    map[string]manifest.Entry{},
		workspace: w.id,
	}
	if l == nil {
		return rec
	}
	rec.ledger = l
	if n, names := l.(named); names && rec.workspace == "" {
		rec.workspace = n.Workspace()
	}
	previous, digests, err := state.ReadManifest(ctx, l)
	if err != nil {
		sink.Infof(UnreadableRecord, position.Pos{File: ledger.ManifestPath(w.brand)}, diag.PhaseLoad,
			"the previous record does not read, so the run removes nothing: %v", err)
		return rec
	}
	rec.previous, rec.digests = previous, digests
	for _, e := range previous.Files {
		rec.byPlan[e.Plan] = append(rec.byPlan[e.Plan], e)
		rec.byPath[e.Path] = e
	}
	return rec
}

// load returns the sealed graph of the run. It returns the graph of the
// caller, or the graph that the frontends of the composition load from
// the tree of the input under the brand of the composition. A warm load
// compares against the record of the last load in the sealed state. A
// load that meets a damaged record returns [damage]. load returns the
// error of a load that fails otherwise, and an error for a tree without
// a frontend.
func (w *Workspace) load(
	ctx context.Context, in Input, s *sealedState, sink *diag.Sink,
) (*store.Graph, *load.Report, error) {
	if in.Graph != nil {
		in.Graph.Freeze()
		return in.Graph, nil, nil
	}
	if len(w.frontends) == 0 {
		return nil, nil, errors.New("workspace: the composition registers no frontend to load a tree with")
	}
	g, loaded, err := load.Load(ctx, load.Config{
		FS:        in.Tree,
		Frontends: w.frontends,
		Sink:      sink,
		Brand:     w.brand,
		Stores:    in.Stores,
		Getenv:    in.Getenv,
		Prior:     s.loadPrior(),
		Memo:      s.loadMemo(in.Cold),
	})
	if err != nil {
		return nil, loaded, damaged(fmt.Errorf("workspace: %w", err))
	}
	return g, loaded, ctx.Err()
}

// shared is the result of the shared phases, which the plans and Close
// read. table is the validated directive table that every route reads.
type shared struct {
	table plugin.Validated
	// lazy is the table of a warm run, which reads each subject from the
	// generation on first use. It is nil on a cold run.
	lazy *validations
	// dirty is the dirty set of a warm run. It lists the invocations and
	// groups of the plans that the changes of the run make dirty.
	// candidates are the subjects whose matches may have changed, in
	// identity order. modules counts the packages of each module. It is
	// the count of the generation, updated with the changes of the run. A
	// cold run has none of them.
	dirty      *dirtySet
	candidates []symbol.Identity
	modules    map[plugin.Module]int
	// prior is the admission in the generation of each package whose
	// files or module fact changed, in identity order. A cold run has
	// none.
	prior []admission
	// refused reports whether the stamp replay refused a stamp or met a
	// dangling one. No record of the sealed state keeps such a finding.
	refused bool
}

// damaged returns the error of the first record that the table of a warm
// run could not read. It returns nil on a cold run.
func (s shared) damaged() error {
	if s.lazy == nil {
		return nil
	}
	return s.lazy.Damaged()
}

// annotateRun runs the shared phases after the load. They are directive
// validation, the stamp replay, the kernel meta drops and the annotate
// schedule. On a warm run, phases is the record of the phases in the
// generation, and the run executes again only the records that the
// changes of the load make dirty. A cold run passes nil phases and
// executes every phase whole. annotateRun counts the validated subjects
// into stats. It records each validation and invocation into rec, and rec
// is nil when the run does not record its phases. An error that an
// annotator returns stops the frame.
func (w *Workspace) annotateRun(
	ctx context.Context, g *store.Graph, loaded *load.Report, phases *state.PhaseState,
	facts *meta.Facts, sink *diag.Sink, stats *Stats, rec *state.Recorder,
) (shared, error) {
	if phases != nil {
		return w.warmShared(ctx, g, loaded, phases, facts, sink, stats, rec)
	}
	validated, count := w.validated(g, facts, sink, rec)
	stats.Validated = count
	out := shared{table: plugin.ValidatedMap(validated), refused: applyStamps(g, facts, sink)}
	if err := w.applyMeta(validated, facts); err != nil {
		return out, err
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	return out, w.annotateAll(ctx, g, facts, out.table, sink, stats, rec)
}

// stopped returns the report of a frame that stopped before the plans
// committed. A plan that selected leaves out is skipped. Every other plan
// is cancelled where the context ended, and failed otherwise.
func (w *Workspace) stopped(ctx context.Context, report *Report, selected []bool, err error) (*Report, error) {
	status := PlanFailed
	if ctx.Err() != nil {
		status = PlanCancelled
	}
	for i := range w.plans {
		pr := PlanReport{Name: w.plans[i].name, Status: status}
		if len(selected) > 0 && !selected[i] {
			pr.Status = PlanSkipped
		}
		report.Plans = append(report.Plans, pr)
	}
	return report, errors.Join(err, failure(report.Sink))
}

// failure returns [ErrRunFailed] when the sink received any Error, and nil
// otherwise.
func failure(sink *diag.Sink) error {
	if sink.Failed() {
		return ErrRunFailed
	}
	return nil
}

// validated runs directive validation over every attached subject on up
// to GOMAXPROCS workers, because the validation of one subject is
// independent of every other. Each worker reports into a sink of its own.
// The findings merge into the sink of the run in the canonical finding
// order, so the report does not depend on the order in which the workers
// finished. A subject that the graph does not contain reports the
// dangling code and contributes nothing. The surviving directives form
// the table that the routing indexes read, so no rule dispatches on a
// rejected instance. validated also returns the number of subjects that
// it validated. A dangling subject is not validated.
//
// Where rec is set, each worker records the validation of each subject
// into a lane of its own. The record contains the directives that passed,
// the edges through which the references of the subject resolved, and the
// findings. A sink of the subject collects the findings before they
// merge. The record of a dangling subject contains its one finding.
func (w *Workspace) validated(
	g *store.Graph, facts *meta.Facts, sink *diag.Sink, rec *state.Recorder,
) (map[symbol.Identity][]directive.Directive, int) {
	type attached struct {
		subject symbol.Identity
		raws    []directive.Raw
	}
	var subjects []attached
	for id, raws := range g.Directives() {
		subjects = append(subjects, attached{subject: id, raws: raws})
	}
	results := make([][]directive.Directive, len(subjects))
	locals := make([]*diag.Sink, min(runtime.GOMAXPROCS(0), len(subjects)))
	// counts is the number of validated subjects of each worker. A worker
	// writes its count once, when it finds no subject left.
	counts := make([]int, len(locals))
	var next atomic.Int64
	var wg sync.WaitGroup
	for k := range locals {
		local := diag.NewSink()
		locals[k] = local
		wg.Go(func() {
			var lane *state.Lane
			if rec != nil {
				lane = rec.Lane("")
			}
			reads := store.NewReadSet()
			count := 0
			for {
				i := int(next.Add(1)) - 1
				if i >= len(subjects) {
					counts[k] = count
					return
				}
				s := subjects[i]
				if !g.Holds(s.subject) {
					d := danglingDirectives(s.subject, s.raws[0].Pos)
					local.Report(d)
					lane.Validation(s.subject, nil, nil, []diag.Diag{d})
					continue
				}
				reads.Reset()
				target := local
				if lane != nil {
					target = diag.NewSink()
				}
				results[i] = directive.Validate(
					s.subject, s.raws, w.directives, w.keys, w.resolver(g, facts, reads), target,
				)
				if lane != nil {
					lane.Validation(s.subject, results[i], reads, reported(target, local))
				}
				count++
			}
		})
	}
	wg.Wait()
	var found []diag.Diag
	for _, local := range locals {
		found = slices.AppendSeq(found, local.All())
	}
	slices.SortStableFunc(found, diag.Diag.Compare)
	for _, d := range found {
		sink.Report(d)
	}
	table := make(map[symbol.Identity][]directive.Directive, len(subjects))
	for i, ds := range results {
		if len(ds) > 0 {
			table[subjects[i].subject] = ds
		}
	}
	validated := 0
	for _, count := range counts {
		validated += count
	}
	return table, validated
}

// resolver returns the resolver that binds the reference params of a
// subject through the rules of the language of the subject. Each call
// reads through a view of its own. The reader of the view records into
// reads, which is the read set of the subject, and one goroutine
// validates a subject at a time. For a language without registered
// rules, the resolver returns an error that contains the language.
func (w *Workspace) resolver(g *store.Graph, facts *meta.Facts, reads *store.ReadSet) directive.Resolver {
	return func(subject symbol.Identity, name string, kind directive.ResolutionKind) (symbol.Identity, error) {
		src, held := w.rules.For(subject.Lang)
		if !held {
			return symbol.Identity{}, fmt.Errorf("workspace: no rules are registered for %s", subject.Lang)
		}
		reader, err := g.Reader(reads, nil)
		if err != nil {
			return symbol.Identity{}, err
		}
		view := rules.NewView(reader, facts, nil, w.kernel).WithRules(w.rulesFor)
		sym, err := src.Resolve(view.ScopeOf(subject), name, kind, view)
		if err != nil {
			return symbol.Identity{}, err
		}
		decl, is := sym.(node.Declaration)
		if !is {
			return symbol.Identity{}, fmt.Errorf("workspace: %q resolves to a symbol without an identity", name)
		}
		return decl.Identity(), nil
	}
}

// rulesFor returns the rules that the workspace registers for lang, and
// [rules.Absent] for a language without registered rules. A binding that
// the rules of a directive's subject make resolves another language
// through it.
func (w *Workspace) rulesFor(lang symbol.Lang) rules.SourceRules {
	if src, held := w.rules.For(lang); held {
		return src
	}
	return rules.Absent(lang)
}

// applyStamps replays the classification stamps of the load into the fact
// store at plugin authority, before any drop or annotator runs. It
// reports whether it refused a stamp or found a dangling one. The rank
// selects every winner, so a drop at directive authority ranks above a
// stamp in any order of arrival at the store. The replay order affects
// only the order of the findings.
func applyStamps(g *store.Graph, facts *meta.Facts, sink *diag.Sink) bool {
	refused := false
	for id, stamps := range g.Stamps() {
		refused = stampSubject(g, facts, sink, id, stamps) || refused
	}
	return refused
}

// stampSubject files the classification stamps of one subject at plugin
// authority, and ranks each stamp by its index among the stamps of the
// subject. It reports whether it refused a stamp or found a dangling
// subject. It reports a refusal under the code of the fact store at the
// position of the stamp, and the frame continues. It reports a stamp on a
// subject that the graph does not contain and does not file it, as for a
// dangling directive, because no reader can look up the subject of such a
// fact.
func stampSubject(g *store.Graph, facts *meta.Facts, sink *diag.Sink, id symbol.Identity, stamps []meta.RawStamp) bool {
	if !g.Holds(id) {
		for _, s := range stamps {
			sink.Errorf(directive.DanglingSubject, s.Pos, s.Origin,
				"a %s stamp names %s, which the graph does not contain", s.Key, id)
		}
		return len(stamps) > 0
	}
	refused := false
	for i, s := range stamps {
		if err := facts.StampRaw(s, meta.LoadClaim(id, i, s)); err != nil {
			sink.Errorf(meta.RefusedStamp, s.Pos, s.Origin, "%v", err)
			refused = true
		}
	}
	return refused
}

// applyMeta files the drops and the sets of every validated meta instance
// before any annotator runs. A key drop applies through the key tombstone
// of the fact store, and a group drop through the group tombstone. A set
// writes its fact through [meta.Facts.StampRaw]. Each claim has directive
// authority and the position of its instance. applyMeta does not apply
// the out and diag instances. A refused drop or set is a defect, because
// validation resolved and typed each instance.
func (w *Workspace) applyMeta(
	table map[symbol.Identity][]directive.Directive, facts *meta.Facts,
) error {
	errs := make([]error, 0, len(table))
	for _, id := range slices.SortedFunc(maps.Keys(table), symbol.Identity.Compare) {
		_, err := w.metaClaims(facts, id, table[id], false, nil)
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// metaClaims files the drops and the sets of the validated meta instances
// of a subject into the fact store, or withdraws them when withdraw is
// set. Validation keeps a meta instance only with exactly one of drop and
// set, so an instance that does not set drops. A drop that references a
// registered key is a key drop, and any other drop is a group drop. A set
// writes the value that validation typed for its key. Each claim has
// directive authority and the position of its instance. metaClaims appends
// each fact that a drop or a set covers to touched, including every member
// of a dropped group. It returns the joined errors of the claims that the
// store refused. A refusal is a defect, because validation resolved and
// typed each instance.
func (w *Workspace) metaClaims(
	facts *meta.Facts, id symbol.Identity, ds []directive.Directive, withdraw bool, touched []meta.FactRef,
) ([]meta.FactRef, error) {
	var errs []error
	for _, d := range ds {
		if d.Name != directive.KernelMeta {
			continue
		}
		claim := meta.Claim{
			Subject:   id,
			Authority: meta.AuthorityDirective,
			Order:     meta.Order{Subject: id, Instance: d.Instance},
			Pos:       d.Pos,
		}
		if set, sets := d.Param(directive.MetaSet); sets {
			name := meta.KeyName(set.Ref)
			key, _ := w.keys.Resolve(name)
			touched = append(touched, meta.FactRef{Subject: id, Key: name})
			if withdraw {
				errs = append(errs, facts.Withdraw(key, claim))
				continue
			}
			if value, typed := setValue(d); typed {
				errs = append(errs, facts.StampRaw(meta.RawStamp{Key: name, Value: value, Pos: d.Pos}, claim))
			}
			continue
		}
		v, _ := d.Param(directive.MetaDrop)
		if key, registered := w.keys.Resolve(meta.KeyName(v.Ref)); registered {
			touched = append(touched, meta.FactRef{Subject: id, Key: meta.KeyName(v.Ref)})
			if withdraw {
				errs = append(errs, facts.Withdraw(key, claim))
				continue
			}
			errs = append(errs, facts.DropKey(key, claim))
			continue
		}
		group := meta.GroupName(v.Ref)
		for member := range w.keys.Group(group) {
			spec, _ := w.keys.Spec(member)
			touched = append(touched, meta.FactRef{Subject: id, Key: spec.Name})
		}
		if withdraw {
			errs = append(errs, facts.WithdrawGroup(group, claim))
			continue
		}
		errs = append(errs, facts.DropGroup(group, claim))
	}
	return touched, errors.Join(errs...)
}

// setValue returns the fact that a validated set instance writes: the
// list of its values param, or the typed value of its value param. It
// reports false for an identity without a target, which only a validation
// without a resolver leaves, and the workspace always validates with one.
func setValue(d directive.Directive) (any, bool) {
	if list, held := d.Param(directive.MetaValues); held {
		values := make([]string, 0, len(list.List))
		for _, e := range list.List {
			values = append(values, e.Str)
		}
		return values, true
	}
	v, _ := d.Param(directive.MetaValue)
	switch v.Kind {
	case directive.TypeBool:
		return v.Bool, true
	case directive.TypeInt:
		return v.Int, true
	case directive.TypeReference:
		return v.Target, !v.Target.IsZero()
	default:
		return v.Str, true
	}
}

// annotateAll runs the annotate schedule in turn order over one
// whole-graph index. Each role receives its own tracked reader, its rank
// fields and a journal that counts its invocations into stats. A returned
// error stops the frame, and annotateAll wraps it with the role that
// returned it.
//
// Where rec is set, these rules also apply:
//   - The journal passes each invocation to a lane of the shared phases,
//     which records every invocation that is not pure.
//   - Each call reports into a sink of its own, whose findings then merge
//     into the sink of the run.
//   - A call of an entry that does not lower, and that journals no
//     invocation of its own, is recorded as one invocation under
//     [plugin.WholeCall]. The record contains the reads of its reader,
//     the facts that its plugin claimed and its findings.
func (w *Workspace) annotateAll(
	ctx context.Context, g *store.Graph, facts *meta.Facts,
	table plugin.Validated, sink *diag.Sink, stats *Stats, rec *state.Recorder,
) error {
	if len(w.annotate) == 0 {
		return nil
	}
	ix, err := plugin.NewIndex(g, facts, table, nil)
	if err != nil {
		return err
	}
	counted := &tally{}
	if rec != nil {
		counted.next = rec.Lane("")
	}
	for _, s := range w.annotate {
		if err := ctx.Err(); err != nil {
			return err
		}
		// A lowering entry journals every invocation that it runs, so a
		// call that journals none ran none.
		_, lowers := s.run.(*lowering)
		if err := w.annotateCall(ix, s, facts, sink, counted, counted, nil, !lowers); err != nil {
			return err
		}
		stats.Invoked = append(stats.Invoked, Invoked{
			Plugin: s.name, Phase: plugin.PhaseAnnotate, Count: counted.count,
		})
	}
	return nil
}

// annotateCall runs the phase call of one annotator over a whole-graph
// index. The call receives a tracked reader of its own, its rank fields
// and journal, which receives each invocation in canonical match order.
// sel restricts the call to the matches that a warm run executes again,
// and a nil sel runs every match. counted counts the invocations of the
// call from zero, and journal passes each record on to counted. A
// returned error stops the frame, and annotateCall wraps it with the role
// that returned it.
//
// When counted passes its records to a lane, the call reports into a sink
// of its own, and its findings then merge into the run's sink. When whole
// is set and the call journals no invocation, annotateCall records the
// call as one invocation under [plugin.WholeCall], with the reads of its
// reader, the facts that its plugin claimed and its findings.
func (w *Workspace) annotateCall(
	ix *plugin.Index, s annEntry, facts *meta.Facts, sink *diag.Sink,
	counted *tally, journal plugin.Journal, sel *plugin.Selection, whole bool,
) error {
	reads := store.NewReadSet()
	reader, err := ix.Reader(reads)
	if err != nil {
		return err
	}
	counted.count = 0
	calls := sink
	if counted.next != nil {
		calls = diag.NewSink()
	}
	err = s.run.Annotate(&plugin.AnnotatorContext{
		Index:   ix,
		Reader:  reader,
		Facts:   facts,
		Sink:    calls,
		Rules:   w.rules,
		Kernel:  w.kernel,
		Plugin:  s.name,
		Turn:    s.turn,
		Workers: w.workers,
		Select:  sel,
		Journal: journal,
	})
	if counted.next != nil {
		found := reported(calls, sink)
		if whole && counted.count == 0 {
			counted.next.Invoked(plugin.Invocation{
				Match:    plugin.MatchKey{Plugin: s.name, Rule: plugin.WholeCall},
				Reads:    reads,
				Claimed:  facts.ClaimedBy(s.name),
				Findings: found,
			})
		}
	}
	if err != nil {
		return fmt.Errorf("workspace: annotator %s in turn %d: %w", s.name, s.turn, err)
	}
	return nil
}

// reported reports the findings of a phase call into sink, in the order
// of the sink of the call, and returns them.
func reported(calls, sink *diag.Sink) []diag.Diag {
	found := slices.Collect(calls.All())
	for _, d := range found {
		sink.Report(d)
	}
	return found
}
