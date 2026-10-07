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
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
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
	// the report's sink in composition order.
	sink *diag.Sink
	// done is closed when the plan's render ends, whatever its outcome.
	done chan struct{}
	// err is a returned error: a generator's, the settle's, the
	// layout's, the sink's or the commit's.
	err error
	// cancelled reports a plan the run's cancellation stopped.
	cancelled bool
	// upstream is the first plan this one depends on that failed
	// before this one generated, nil where every one of them rendered.
	// A plan with an upstream generates nothing.
	upstream *planRun
	// invoked counts the invocations of each of the plan's generator
	// calls, in schedule order.
	invoked []Invoked
	// files are what the plan rendered, in path order.
	files []stagedFile
	// export is what the plan rendered, as its dependents and the
	// checks read it. The run builds it only for a plan marked exported.
	export plugin.ExportDoc
	// out is the plan's prepared sink, nil where it staged nothing or
	// discarded its staging.
	out output.Sink
	// changes are what the preparation found, in path order.
	changes []output.Change
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
// its own, where rec is set.
func (w *Workspace) generateAll(
	ctx context.Context, g *store.Graph, facts *meta.Facts,
	table map[symbol.Identity][]directive.Directive, src tree, rec *state.Recorder,
) []*planRun {
	runs := make([]*planRun, len(w.plans))
	for i := range w.plans {
		runs[i] = &planRun{
			plan: &w.plans[i], emit: plugin.NewEmit(), sink: diag.NewSink(), done: make(chan struct{}),
		}
	}
	var wg sync.WaitGroup
	for _, p := range runs {
		wg.Go(func() {
			defer close(p.done)
			exports, ready := p.await(runs)
			if !ready {
				return
			}
			var rendered []plugin.File
			p.files, rendered, p.err = w.runPlan(ctx, g, facts, table, src, p, exports, rec)
			p.cancelled = p.err != nil && ctx.Err() != nil && errors.Is(p.err, ctx.Err())
			if p.plan.exported && !p.failed() {
				p.export = plugin.NewExport(p.plan.name, rendered, p.emit)
			}
		})
	}
	wg.Wait()
	return runs
}

// await blocks until every plan this one depends on has rendered, and
// returns their exports, keyed by plan name. It reports false where
// one of them was cancelled, which cancels this plan, or failed, which
// it records as this plan's upstream. Either way the plan generates
// nothing. A plan without dependencies returns at once.
func (p *planRun) await(runs []*planRun) (map[string]plugin.ExportDoc, bool) {
	if len(p.plan.deps) == 0 {
		return nil, true
	}
	exports := make(map[string]plugin.ExportDoc, len(p.plan.deps))
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
		exports[dep.plan.name] = dep.export
	}
	return exports, true
}

// runPlan runs one plan's roles in bucket order, which is what an
// emit-triggered rule's visibility is defined against: the store
// contains earlier buckets' units when a later role runs. The plan's
// sources bind its scope over the run's graph and facts, every
// generator reads the exports of the plans it depends on, and a journal
// counts each generator's invocations into the plan's run. Where the
// composition writes output, the settled store routes to files against
// the run's source tree, and the files render and stamp. It returns the
// stamped files, and for a plan marked exported, the routed files that
// rendered, which the plan's export lists.
//
// Where rec is set, the journal also hands each invocation to the plan's
// lane, which records every one that is not pure, each call reports into
// a sink of its own whose findings then merge into the plan's, and a call
// that journals no invocation of its own is recorded as one under
// [plugin.WholeCall]: its reader's reads, the plans whose export it was
// handed, the units its plugin flushed or appended into, and its
// findings.
func (w *Workspace) runPlan(
	ctx context.Context, g *store.Graph, facts *meta.Facts,
	table map[symbol.Identity][]directive.Directive, src tree, p *planRun,
	exports map[string]plugin.ExportDoc, rec *state.Recorder,
) ([]stagedFile, []plugin.File, error) {
	pl := p.plan
	ix, err := plugin.NewIndex(g, facts, table, pl.sources.bind(g, facts, w.kernel))
	if err != nil {
		return nil, nil, err
	}
	counted := &tally{}
	if rec != nil {
		counted.next = rec.Lane(pl.name)
	}
	for _, s := range pl.entries {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		reads := store.NewReadSet()
		reader, err := ix.Reader(reads)
		if err != nil {
			return nil, nil, err
		}
		counted.count = 0
		calls := p.sink
		if counted.next != nil {
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
			Journal: counted,
		}
		err = s.run.Generate(call)
		if counted.next != nil {
			found := reported(calls, p.sink)
			if counted.count == 0 {
				counted.next.Invoked(plugin.Invocation{
					Match:    plugin.MatchKey{Plugin: s.name, Rule: plugin.WholeCall},
					Reads:    reads,
					Exports:  slices.Sorted(maps.Keys(exports)),
					Units:    unitsOf(p.emit, s.name),
					Findings: found,
				})
			}
		}
		if err != nil {
			return nil, nil, fmt.Errorf("generator %s in bucket %d: %w", s.name, s.bucket, err)
		}
		p.invoked = append(p.invoked, Invoked{
			Plan: pl.name, Plugin: s.name, Phase: plugin.PhaseGenerate, Count: counted.count,
		})
	}
	if err := plugin.Settle(p.emit, pl.backend, facts, p.sink); err != nil {
		return nil, nil, fmt.Errorf("settle: %w", err)
	}
	if pl.contract == nil {
		return nil, nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return w.write(pl, ix, src, p.emit, p.sink)
}

// stageAll stages, in parallel, every plan that can commit into a sink
// of its own and prepares it. A plan removes only the stale paths no
// plan routes a file to in this run, so a file that moved between
// plans is written by its new plan and never removed by its old one.
func (w *Workspace) stageAll(ctx context.Context, runs []*planRun, rec *record) {
	routed := map[string]struct{}{}
	for _, p := range runs {
		for _, f := range p.files {
			routed[f.path] = struct{}{}
		}
	}
	var wg sync.WaitGroup
	for _, p := range runs {
		if p.failed() || p.cancelled || p.plan.contract == nil {
			continue
		}
		wg.Go(func() { w.stage(ctx, p, rec, routed) })
	}
	wg.Wait()
}

// stage opens one plan's sink, writes every file the plan rendered,
// stages the removal of its stale entries, prepares, and reports what
// the preparation found. A plan whose sink refuses, a plan the
// preparation fails, and a plan the run cancels before its preparation
// discards its staging.
func (w *Workspace) stage(ctx context.Context, p *planRun, rec *record, routed map[string]struct{}) {
	out, err := w.openSink()
	if err != nil {
		p.err = err
		return
	}
	if err = p.stageInto(out, rec.byPlan[p.plan.name], routed); err != nil {
		p.err = discarding(out, err)
		return
	}
	if err = ctx.Err(); err != nil {
		p.cancelled, p.err = true, discarding(out, err)
		return
	}
	changes, err := out.Prepare()
	if err != nil {
		p.err = discarding(out, fmt.Errorf("prepare the output: %w", err))
		return
	}
	p.changes = changes
	w.verdicts(p, rec)
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
// file to in this run.
func (p *planRun) stageInto(out output.Sink, previous []manifest.Entry, routed map[string]struct{}) error {
	for _, f := range p.files {
		if err := out.Write(f.path, f.body); err != nil {
			return fmt.Errorf("stage %s: %w", f.path, err)
		}
	}
	for _, e := range previous {
		if _, still := routed[e.Path]; still {
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

// verdicts reports what the preparation found on the plan's paths. A
// drifted or foreign file where the plan writes fails the plan, and a
// drifted or foreign file where it removes a stale output remains
// under a warning. A drift finding names the plan the record lists for
// the file, and the plan itself where the record lists no file there.
func (w *Workspace) verdicts(p *planRun, rec *record) {
	written := make(map[string]*stagedFile, len(p.files))
	for i := range p.files {
		written[p.files[i].path] = &p.files[i]
	}
	for _, c := range p.changes {
		f, write := written[c.Path]
		switch {
		case write && c.Found == output.FoundDrifted:
			p.sink.Errorf(DriftedOutput, f.at, diag.PhaseClose,
				"%s was edited since plan %q generated it, and the run does not overwrite it: "+
					"move the edit into the source, or revert it", c.Path, rec.generator(c.Path, p.plan.name))
		case write && c.Found == output.FoundForeign:
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
