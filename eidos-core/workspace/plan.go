// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
)

// planRun is one plan's way through a run: its store and findings,
// what it rendered and exported, and what its sink staged and found.
// Each plan's run is written by its own goroutine until it closes
// done, and read after that by the plans that depend on it and by the
// run's goroutine.
type planRun struct {
	plan *compiledPlan
	emit *plugin.Emit
	// sink takes the plan's own findings, which the run merges into
	// the report's sink in composition order. It is under the run's
	// policy, which is the policy of every sink the plan's run opens.
	sink   *diag.Sink
	policy policy
	// done is closed when the plan's render ends, whatever its outcome.
	done chan struct{}
	// err is a returned error: a generator's, the settle's, the
	// layout's, the sink's or the commit's.
	err error
	// cancelled reports a plan the run's cancellation stopped.
	cancelled bool
	// skipped reports a plan that [Input.Plans] left out, which does not
	// run. Its kept are its previous entries, which the run keeps.
	skipped bool
	// upstream is the first plan this one depends on that failed
	// before this one generated, nil where every one of them rendered.
	// A plan with an upstream generates nothing.
	upstream *planRun
	// invoked counts the invocations of each of the plan's generator
	// calls, in schedule order.
	invoked []Invoked
	// lane records the plan's invocations, groups and files where the
	// run records its phases, and is nil where it does not.
	lane *state.Lane
	// selective reports a plan that a warm run executed in part. kept are
	// the previous entries of the files that such a plan keeps, in path
	// order, and its files are those of the groups it executed again.
	// exportChanged reports that the plan's export may differ from the
	// export the generation records, which makes dirty every invocation
	// that read it.
	selective     bool
	kept          []manifest.Entry
	exportChanged bool
	// files are what the plan rendered, in path order.
	files []stagedFile
	// fresh are the export rows of the files that the plan rendered, in
	// the order an export lists them, for a plan marked exported.
	fresh []plugin.ExportedSymbol
	// export returns the plan's export, as its dependents and the checks
	// read it. The run sets it for a plan marked exported that did not
	// fail. The export of a plan that a warm run executed in part reads
	// the export rows of the kept files from the generation on its first
	// call, and its error wraps [state.ErrDamaged] for an artifact that
	// does not read whole.
	export func() (plugin.ExportDoc, error)
	// out is the plan's prepared sink, nil where it staged nothing or
	// discarded its staging.
	out output.Sink
	// changes are what the preparation found, in path order.
	changes []output.Change
	// withheld are the changes that the run's narrowing does not commit,
	// as a preparation of their own found them, in path order.
	withheld []output.Change
	// refused are the writes that the run refused because the path has a
	// drifted or a foreign file, in path order.
	refused []output.Change
	// stale are the previous entries the plan staged for removal, by
	// path.
	stale map[string]manifest.Entry
	// status is how the plan's run ended, set by the commit step.
	status PlanStatus
}

// failed reports whether the plan cannot commit: it returned an error,
// reported an Error, or generated nothing because a plan it depends on
// failed.
func (p *planRun) failed() bool { return p.err != nil || p.sink.Failed() || p.upstream != nil }

// cause returns the position of the first Error of the plan whose
// failure made this one fail: its own first Error, or, for a plan that
// failed only because a plan it depends on failed, that plan's cause.
// It reports false for a plan that failed on a returned error alone.
func (p *planRun) cause() (position.Pos, bool) {
	for d := range p.sink.All() {
		if d.Severity == diag.SeverityError {
			return d.Pos, true
		}
	}
	if p.upstream != nil {
		return p.upstream.cause()
	}
	return position.Pos{}, false
}

// generateAll runs the plans in parallel through Generate, Settle,
// Layout, Render and Stamp, each over its own emit store, scoped
// index, readers and findings. A plan that depends on others starts
// after each of them has rendered and reads their exports. A plan's
// failure does not stop its siblings, every plan that depends on it
// generates nothing, and every plan's store arrives in the report
// either way. Each plan records its invocations into a lane of rec of
// its own, where rec is set. On a warm run, ws is what the plans read of
// the run so far, and a plan that writes output executes the groups that
// the run's changes make dirty, as [Workspace.runWarm] states. A cold run
// passes a nil ws. A plan that selected leaves out is skipped and keeps
// its previous entries, and a nil selected runs every plan. Each plan's
// findings arrive in a sink under pol.
func (w *Workspace) generateAll(
	ctx context.Context, g *store.Graph, facts *meta.Facts, table plugin.Validated, src tree,
	rec *state.Recorder, ws *warmState, selected []bool, previous map[string][]manifest.Entry, pol policy,
) []*planRun {
	runs := make([]*planRun, len(w.plans))
	for i := range w.plans {
		runs[i] = &planRun{
			plan: &w.plans[i], emit: plugin.NewEmit(), sink: pol.sink(), policy: pol, done: make(chan struct{}),
		}
		if len(selected) > 0 && !selected[i] {
			runs[i].skipped, runs[i].kept = true, previous[w.plans[i].name]
			close(runs[i].done)
		}
	}
	var wg sync.WaitGroup
	for _, p := range runs {
		if p.skipped {
			continue
		}
		wg.Go(func() {
			defer close(p.done)
			deps, ready := p.await(runs)
			if !ready {
				return
			}
			var rendered []plugin.File
			if ws != nil && rec != nil && p.plan.contract != nil {
				p.files, rendered, p.err = w.runWarm(ctx, g, facts, table, src, p, deps, rec, ws)
			} else {
				p.files, rendered, p.err = w.runPlan(ctx, g, facts, table, src, p, deps, rec)
			}
			p.cancelled = p.err != nil && ctx.Err() != nil && errors.Is(p.err, ctx.Err())
			if p.plan.exported && !p.failed() && !p.selective {
				doc := plugin.NewExport(p.plan.name, rendered, p.emit, nil)
				p.fresh = doc.Symbols
				p.export = func() (plugin.ExportDoc, error) { return doc, nil }
			}
		})
	}
	wg.Wait()
	return runs
}

// await blocks until every plan this one depends on has rendered, and
// returns those plans in the order the plan lists them. It reports false
// where one of them was cancelled, which cancels this plan, or failed,
// which it records as this plan's upstream. Either way the plan generates
// nothing. A plan without dependencies returns at once.
func (p *planRun) await(runs []*planRun) ([]*planRun, bool) {
	if len(p.plan.deps) == 0 {
		return nil, true
	}
	deps := make([]*planRun, 0, len(p.plan.deps))
	for _, d := range p.plan.deps {
		dep := runs[d]
		<-dep.done
		switch {
		case dep.cancelled:
			p.cancelled, p.err = true, dep.err
			return nil, false
		case dep.failed():
			p.upstream = dep
			return nil, false
		}
		deps = append(deps, dep)
	}
	return deps, true
}

// exportsOf returns the exports of the plans that a plan depends on,
// keyed by plan name, and nil for a plan without dependencies.
//
// Error modes: an error wrapping [state.ErrDamaged] for an export that
// reads an artifact of the generation that does not read whole.
func exportsOf(deps []*planRun) (map[string]plugin.ExportDoc, error) {
	var out map[string]plugin.ExportDoc
	if len(deps) > 0 {
		out = make(map[string]plugin.ExportDoc, len(deps))
	}
	for _, d := range deps {
		doc, err := d.export()
		if err != nil {
			return nil, fmt.Errorf("read the export of plan %q: %w", d.plan.name, err)
		}
		out[d.plan.name] = doc
	}
	return out, nil
}

// runPlan runs one plan's roles in bucket order, which is what an
// emit-triggered rule's visibility is defined against: the store
// contains earlier buckets' units when a later role runs. The plan's
// sources bind its scope over the run's graph and facts, every
// generator reads the exports of the plans it depends on, and a journal
// counts each generator's invocations into the plan's run. Where the
// composition writes output, the settled store routes to files against
// the run's source tree, and the files render and stamp. It returns the
// stamped files, and the routed files that rendered, which the export of
// a plan marked exported lists.
//
// Where rec is set, the journal also hands each invocation to the plan's
// lane, which records every one that is not pure, each call reports into
// a sink of its own whose findings then merge into the plan's, and a call
// that journals no invocation of its own is recorded as one under
// [plugin.WholeCall]: its reader's reads, the plans whose export it was
// handed, the units its plugin flushed or appended into, and its
// findings. The lane also records the plan's groups once the files
// render, where the plan reported no Error.
//
// Error modes: an error wrapping [state.ErrDamaged] for an export of a
// plan in deps that does not read, the error of an index that does not
// build, the errors of [Workspace.generate], the settle's error, wrapped,
// the context's error, and the errors of the layout and the render.
func (w *Workspace) runPlan(
	ctx context.Context, g *store.Graph, facts *meta.Facts, table plugin.Validated, src tree, p *planRun,
	deps []*planRun, rec *state.Recorder,
) ([]stagedFile, []plugin.File, error) {
	pl := p.plan
	exports, err := exportsOf(deps)
	if err != nil {
		return nil, nil, err
	}
	ix, err := plugin.NewIndex(g, facts, table, pl.sources.bind(g, facts, w.kernel))
	if err != nil {
		return nil, nil, err
	}
	var touches *touchJournal
	var next plugin.Journal
	if rec != nil {
		p.lane = rec.Lane(pl.name)
		touches = &touchJournal{next: p.lane}
		next = touches
	}
	err = w.generate(ctx, ix, facts, p, exports, nil, next)
	if err != nil {
		return nil, nil, err
	}
	var settled plugin.Settled
	if touches != nil {
		settled, err = plugin.SettleWith(p.emit, pl.backend, facts, p.sink, nil)
	} else {
		err = plugin.Settle(p.emit, pl.backend, facts, p.sink)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("settle: %w", err)
	}
	if pl.contract == nil {
		return nil, nil, nil
	}
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	staged, rendered, err := w.write(pl, ix, src, p.emit, p.sink, nil)
	if err == nil && touches != nil && !p.sink.Failed() {
		_, placed := pl.backend.(plugin.Packager)
		pg := planGroups{
			plan: pl.name, emit: p.emit, files: rendered, staged: staged,
			touches: touches.touches, settled: settled, placed: placed,
		}
		pg.record(p.lane)
	}
	return staged, rendered, err
}

// generate runs the plan's generator calls in bucket order into the
// plan's store, each over a tracked reader of its own, with the exports
// of the plans it depends on, and counts each call's invocations into
// the plan's run. sel restricts each call to what a warm run executes
// again, and a nil sel runs every match.
//
// Where next is set, each call journals into it after the count, reports
// into a sink of its own whose findings then merge into the plan's, and a
// call that journals no invocation of its own is journaled as one under
// [plugin.WholeCall]: its reader's reads, the plans whose export it was
// handed, the units its plugin flushed or appended into, and its
// findings.
//
// Error modes: the context's error, a reader the index refuses, and a
// generator's returned error, wrapped with the generator's bucket.
func (w *Workspace) generate(
	ctx context.Context, ix *plugin.Index, facts *meta.Facts, p *planRun,
	exports map[string]plugin.ExportDoc, sel *plugin.Selection, next plugin.Journal,
) error {
	pl := p.plan
	counted := &tally{next: next}
	for _, s := range pl.entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		reads := store.NewReadSet()
		reader, err := ix.Reader(reads)
		if err != nil {
			return err
		}
		counted.count = 0
		calls := p.sink
		if next != nil {
			calls = diag.NewSink()
		}
		call := &plugin.GeneratorContext{
			Index:   ix,
			Reader:  reader,
			Facts:   facts,
			Emit:    p.emit,
			Sink:    calls,
			Rules:   w.rules,
			Kernel:  w.kernel,
			Plugin:  s.name,
			Bucket:  s.bucket,
			Workers: w.workers,
			Exports: exports,
			Select:  sel,
			Journal: counted,
			Target:  pl.backend.Target(),
			Types:   pl.types,
			Policy:  pl.policy,
		}
		err = s.run.Generate(call)
		if next != nil {
			found := reported(calls, p.sink)
			if counted.count == 0 {
				next.Invoked(plugin.Invocation{
					Match:    plugin.MatchKey{Plugin: s.name, Rule: plugin.WholeCall},
					Reads:    reads,
					Exports:  slices.Sorted(maps.Keys(exports)),
					Units:    unitsOf(p.emit, s.name),
					Findings: found,
				})
			}
		}
		if err != nil {
			return fmt.Errorf("generator %s in bucket %d: %w", s.name, s.bucket, err)
		}
		p.invoked = append(p.invoked, Invoked{
			Plan: pl.name, Plugin: s.name, Phase: plugin.PhaseGenerate, Count: counted.count,
		})
	}
	return nil
}

// stageAll stages, in parallel, every plan that can commit into a sink
// of its own and prepares it. A plan removes only the stale paths no
// plan routes a file to in this run and no plan keeps, so a file that
// moved between plans is written by its new plan and never removed by
// its old one. Each sink that implements [output.Overwriter] writes over
// the files of the allowed verdicts. A plan stages the changes that the
// narrowing withholds into a second sink, which it prepares and discards.
// A skipped plan stages nothing.
func (w *Workspace) stageAll(
	ctx context.Context, runs []*planRun, rec *record, allowed []output.Found, n narrowing,
) {
	routed := map[string]struct{}{}
	for _, p := range runs {
		for _, f := range p.files {
			routed[f.path] = struct{}{}
		}
		for _, e := range p.kept {
			routed[e.Path] = struct{}{}
		}
	}
	var wg sync.WaitGroup
	for _, p := range runs {
		if p.failed() || p.cancelled || p.skipped || p.plan.contract == nil {
			continue
		}
		wg.Go(func() { w.stage(ctx, p, rec, routed, allowed, n) })
	}
	wg.Wait()
}

// stage opens one plan's sink, lets it write over the files of the
// allowed verdicts where it implements [output.Overwriter], writes every
// file the plan rendered, stages the removal of its stale entries,
// prepares, and reports what the preparation found. Where the narrowing
// withholds a change, it stages that change into a second sink instead,
// which it prepares for the plan's withheld changes and discards. A plan
// whose sink refuses, a plan the preparation fails, and a plan the run
// cancels before its preparation discards its staging.
func (w *Workspace) stage(
	ctx context.Context, p *planRun, rec *record, routed map[string]struct{}, allowed []output.Found,
	n narrowing,
) {
	out, err := w.openSink()
	if err != nil {
		p.err = err
		return
	}
	var overwritten []output.Found
	if o, overwrites := out.(output.Overwriter); overwrites && len(allowed) > 0 {
		if err = o.Overwrite(allowed...); err != nil {
			p.err = discarding(out, fmt.Errorf("let the output write over drifted or foreign files: %w", err))
			return
		}
		overwritten = allowed
	}
	later, withholding := out, n.scope != nil || n.prune
	if withholding {
		if later, err = w.openSink(); err != nil {
			p.err = discarding(out, err)
			return
		}
	}
	err = p.stageInto(out, later, rec.byPlan[p.plan.name], routed, n)
	if err == nil {
		err = ctx.Err()
		p.cancelled = err != nil
	}
	switch {
	case withholding && err != nil:
		err = discarding(later, err)
	case withholding:
		err = p.withhold(later)
	}
	if err != nil {
		p.err = discarding(out, err)
		return
	}
	changes, err := out.Prepare()
	if err != nil {
		p.err = discarding(out, fmt.Errorf("prepare the output: %w", err))
		return
	}
	p.changes = changes
	w.verdicts(p, rec, overwritten)
	if p.sink.Failed() {
		if err := out.Discard(); err != nil {
			p.err = fmt.Errorf("discard the output: %w", err)
		}
		return
	}
	p.out = out
}

// stageInto writes the plan's files into its sink and stages the
// removal of every previous entry of the plan that no plan routes a
// file to in this run. A write and a removal that the narrowing
// withholds go to later instead, which is out itself where the narrowing
// withholds nothing.
func (p *planRun) stageInto(
	out, later output.Sink, previous []manifest.Entry, routed map[string]struct{}, n narrowing,
) error {
	for _, f := range p.files {
		to := out
		if !n.writes(f.path, f.sources) {
			to = later
		}
		if err := to.Write(f.path, f.body); err != nil {
			return fmt.Errorf("stage %s: %w", f.path, err)
		}
	}
	for _, e := range previous {
		if _, still := routed[e.Path]; still {
			continue
		}
		if !n.removes(e, p.plan) {
			if err := later.Delete(e.Path); err != nil {
				return fmt.Errorf("stage the removal of %s: %w", e.Path, err)
			}
			continue
		}
		if err := out.Delete(e.Path); err != nil {
			return fmt.Errorf("stage the removal of %s: %w", e.Path, err)
		}
		if p.stale == nil {
			p.stale = map[string]manifest.Entry{}
		}
		p.stale[e.Path] = e
	}
	return nil
}

// withhold prepares the sink of the changes that the narrowing withholds,
// keeps what the preparation found as the plan's withheld changes, and
// discards the sink.
//
// Error modes: the preparation's error and the discard's, wrapped.
func (p *planRun) withhold(later output.Sink) error {
	changes, err := later.Prepare()
	if err != nil {
		return discarding(later, fmt.Errorf("prepare the withheld changes: %w", err))
	}
	p.withheld = changes
	if err := later.Discard(); err != nil {
		return fmt.Errorf("discard the withheld changes: %w", err)
	}
	return nil
}

// verdicts reports what the preparation found on the plan's paths. A
// drifted or foreign file where the plan writes fails the plan, unless
// the sink writes over the files of that verdict, and a drifted or
// foreign file where it removes a stale output remains under a warning.
// The plan keeps each write that it refuses in its refused changes. A
// drift finding names the plan the record lists for the file, and the
// plan itself where the record lists no file there.
func (w *Workspace) verdicts(p *planRun, rec *record, overwritten []output.Found) {
	written := make(map[string]*stagedFile, len(p.files))
	for i := range p.files {
		written[p.files[i].path] = &p.files[i]
	}
	for _, c := range p.changes {
		f, write := written[c.Path]
		switch {
		case write && slices.Contains(overwritten, c.Found):
			// The sink writes over the file, which the run's input allowed.
		case write && c.Found == output.FoundDrifted:
			p.refused = append(p.refused, c)
			p.sink.Errorf(DriftedOutput, f.at, diag.PhaseClose,
				"%s was edited since plan %q generated it, and the run does not overwrite it: "+
					"move the edit into the source, or revert it", c.Path, rec.generator(c.Path, p.plan.name))
		case write && c.Found == output.FoundForeign:
			p.refused = append(p.refused, c)
			p.sink.Errorf(ForeignFile, f.at, diag.PhaseClose,
				"%s is a file the %s brand did not write, and plan %q routes %s there",
				c.Path, w.brand, p.plan.name, describe(f.first))
		case !write:
			kept(p.sink, c, w.brand)
		}
	}
}

// kept reports a stale output a removal leaves in place: one edited
// since its stamp, and one without the brand's intact frame.
func kept(sink *diag.Sink, c output.Change, brand output.Brand) {
	at := position.Pos{File: c.Path}
	switch c.Found {
	case output.FoundDrifted:
		sink.Warnf(KeptOutput, at, diag.PhaseClose,
			"%s is no longer generated, and it remains because it was edited since its stamp", c.Path)
	case output.FoundForeign:
		sink.Warnf(KeptOutput, at, diag.PhaseClose,
			"%s is no longer generated, and it remains because it is not the %s brand's output", c.Path, brand)
	}
}

// openSink opens the composition's output once, refusing an open
// function that returns neither a sink nor an error. Its errors name
// the step, and the run wraps them with the package and the plan.
func (w *Workspace) openSink() (output.Sink, error) {
	out, err := w.open()
	if err != nil {
		return nil, fmt.Errorf("open the output: %w", err)
	}
	if out == nil {
		return nil, fmt.Errorf("open the output: the open function returned (nil, nil)")
	}
	return out, nil
}

// unitsOf returns the references of the units of a plan's store that a
// plugin flushed or appended into, sorted: what the run can tell from
// the store that a phase call journaling no invocation of its own
// touched.
func unitsOf(e *plugin.Emit, p plugin.ID) []plugin.UnitRef {
	var out []plugin.UnitRef
	for u := range e.Units() {
		if u.Plugin == p || slices.Contains(u.Contributors, p) {
			out = append(out, u.Ref())
		}
	}
	slices.SortFunc(out, plugin.UnitRef.Compare)
	return out
}

// discarding drops a staging after a failure, joining whatever the
// discard itself reports so neither failure hides the other.
func discarding(sink output.Sink, err error) error {
	if derr := sink.Discard(); derr != nil {
		return fmt.Errorf("%w (the staging also failed to discard: %w)", err, derr)
	}
	return err
}
