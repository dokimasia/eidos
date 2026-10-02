// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// planRun is one plan's way through a run: its store and findings,
// what it rendered, and what its sink staged and found. Each plan's run
// is written by its own goroutine until the plans finish, and read on
// the run's goroutine after.
type planRun struct {
	plan *compiledPlan
	emit *plugin.Emit
	// sink takes the plan's own findings, which the run merges into
	// the report's sink in composition order.
	sink *diag.Sink
	// err is a returned error: a generator's, the settle's, the
	// layout's, the sink's or the commit's.
	err error
	// cancelled reports a plan the run's cancellation stopped.
	cancelled bool
	// files are what the plan rendered, in path order.
	files []stagedFile
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

// failed reports whether the plan cannot commit: it returned an error
// or reported an Error.
func (p *planRun) failed() bool { return p.err != nil || p.sink.Failed() }

// generateAll runs the plans in parallel through Generate, Settle,
// Layout, Render and Stamp, each over its own emit store, scoped
// index, readers and findings. A plan's failure does not stop its
// siblings, and every plan's store arrives in the report either way.
func (w *Workspace) generateAll(
	ctx context.Context, g *store.Graph, facts *meta.Facts,
	table map[symbol.Identity][]directive.Directive, src tree,
) []*planRun {
	runs := make([]*planRun, len(w.plans))
	var wg sync.WaitGroup
	for i := range w.plans {
		p := &planRun{plan: &w.plans[i], emit: plugin.NewEmit(), sink: diag.NewSink()}
		runs[i] = p
		wg.Go(func() {
			p.files, p.err = w.runPlan(ctx, g, facts, table, src, p)
			p.cancelled = p.err != nil && ctx.Err() != nil && errors.Is(p.err, ctx.Err())
		})
	}
	wg.Wait()
	return runs
}

// runPlan runs one plan's roles in bucket order, which is what an
// emit-triggered rule's visibility is defined against: the store
// contains earlier buckets' units when a later role runs. Where the
// composition writes output, the settled store routes to files
// against the run's source tree, and the files render and stamp.
func (w *Workspace) runPlan(
	ctx context.Context, g *store.Graph, facts *meta.Facts,
	table map[symbol.Identity][]directive.Directive, src tree, p *planRun,
) ([]stagedFile, error) {
	pl := p.plan
	ix, err := plugin.NewIndex(g, facts, table, pl.scope)
	if err != nil {
		return nil, err
	}
	for _, s := range pl.entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		reader, err := ix.Reader(store.NewReadSet())
		if err != nil {
			return nil, err
		}
		call := &plugin.GeneratorContext{
			Index:   ix,
			Reader:  reader,
			Facts:   facts,
			Emit:    p.emit,
			Sink:    p.sink,
			Rules:   w.rules,
			Kernel:  w.kernel,
			Plugin:  s.name,
			Bucket:  s.bucket,
			Workers: w.workers,
		}
		if err := s.run.Generate(call); err != nil {
			return nil, fmt.Errorf("generator %s in bucket %d: %w", s.name, s.bucket, err)
		}
	}
	if err := plugin.Settle(p.emit, pl.backend, facts, p.sink); err != nil {
		return nil, fmt.Errorf("settle: %w", err)
	}
	if pl.contract == nil {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
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

// discarding drops a staging after a failure, joining whatever the
// discard itself reports so neither failure hides the other.
func discarding(sink output.Sink, err error) error {
	if derr := sink.Discard(); derr != nil {
		return fmt.Errorf("%w (the staging also failed to discard: %w)", err, derr)
	}
	return err
}
