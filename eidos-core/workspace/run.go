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
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Run takes one loaded graph through the frame: seal, directive
// validation, the kernel meta drops, annotate buckets, per-plan
// generation, and — where the composition declares output — the
// render, the stamp and the commit.
//
// The caller loads packages and attaches raw directives before
// handing the graph over. Run seals the graph itself, and takes
// one the load already sealed as it stands: sealing is idempotent,
// and a write after the seal refuses at the store under its own
// code, which is where that fault belongs.
//
// A handler's returned error stops the frame, wrapped with its
// role, and the report holds whatever ran before it. Findings
// never stop the frame: they arrive in the report's sink, and any
// Error among them returns [ErrRunFailed] beside the report. A
// pure refusal, a missing or pre-frozen graph, returns a nil
// report, because nothing ran.
func (w *Workspace) Run(ctx context.Context, g *store.Graph) (*Report, error) {
	if g == nil {
		return nil, errors.New("workspace: Run needs a loaded graph")
	}
	g.Freeze()

	sink := diag.NewSink()
	facts := meta.NewFacts(w.keys)
	report := &Report{Sink: sink, Facts: facts, Emits: map[string]*plugin.Emit{}}

	table := w.validated(g, sink)
	applyStamps(g, facts, sink)
	if err := w.applyDrops(table, facts); err != nil {
		return report, errors.Join(err, failure(sink))
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if err := w.annotateAll(ctx, g, facts, table, sink); err != nil {
		return report, errors.Join(err, failure(sink))
	}
	files, errs := w.generateAll(ctx, g, facts, table, sink, report.Emits)
	if err := failure(sink); err != nil {
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		// A run that reported nothing writes; one that failed keeps
		// its staging unwritten, because half a tree is worse than
		// none and the findings say why.
		written, err := w.commit(files)
		if err != nil {
			errs = append(errs, err)
		}
		report.Written = written
	} else if w.sink != nil {
		if err := w.sink.Discard(); err != nil {
			errs = append(errs, fmt.Errorf("workspace: discard the staging: %w", err))
		}
	}
	return report, errors.Join(errs...)
}

// failure classifies the sink: [ErrRunFailed] when any Error
// arrived, nil otherwise.
func failure(sink *diag.Sink) error {
	if sink.Failed() {
		return ErrRunFailed
	}
	return nil
}

// validated runs directive validation over every attached subject,
// in parallel, because one subject's validation is independent of
// every other. A subject the graph does not hold reports the
// dangling code and contributes nothing; the survivors form the
// table the routing indexes carry, so a rejected instance never
// gates a rule.
func (w *Workspace) validated(
	g *store.Graph, sink *diag.Sink,
) map[symbol.Identity][]directive.Directive {
	type attached struct {
		subject symbol.Identity
		raws    []directive.Raw
	}
	var subjects []attached
	for id, raws := range g.Directives() {
		subjects = append(subjects, attached{subject: id, raws: raws})
	}
	results := make([][]directive.Directive, len(subjects))
	var wg sync.WaitGroup
	for i := range subjects {
		wg.Go(func() {
			s := subjects[i]
			if !g.Holds(s.subject) {
				sink.Errorf(directive.DanglingSubject, s.raws[0].Pos, diag.PhaseFreeze,
					"directives on %s name a subject the graph does not hold", s.subject)
				return
			}
			results[i] = directive.Validate(s.subject, s.raws, w.directives, w.keys, sink)
		})
	}
	wg.Wait()
	table := make(map[symbol.Identity][]directive.Directive, len(subjects))
	for i, ds := range results {
		if len(ds) > 0 {
			table[subjects[i].subject] = ds
		}
	}
	return table
}

// applyStamps replays the load's classification stamps into the
// fact store at plugin authority, before any drop or annotator
// runs. Rank decides every winner, so a directive-authority drop
// still beats a stamp whichever applied first; the order here
// exists for the findings, not the outcome. A refusal reports
// under the fact store's own code at the stamp's position, and the
// frame continues. A stamp whose subject the graph does not hold
// dangles the way a directive's does, reported rather than filed:
// a ghost fact would enumerate under a subject no reader can
// reach.
func applyStamps(g *store.Graph, facts *meta.Facts, sink *diag.Sink) {
	for id, stamps := range g.Stamps() {
		if !g.Holds(id) {
			for _, s := range stamps {
				sink.Errorf(directive.DanglingSubject, s.Pos, s.Origin,
					"a %s stamp names %s, which the graph does not hold", s.Key, id)
			}
			continue
		}
		for i, s := range stamps {
			claim := meta.Claim{
				Subject:   id,
				Authority: meta.AuthorityPlugin,
				Plugin:    s.Origin,
				Seq:       i,
				Pos:       s.Pos,
			}
			if err := facts.StampRaw(s, claim); err != nil {
				sink.Errorf(meta.RefusedStamp, s.Pos, s.Origin, "%v", err)
			}
		}
	}
}

// applyDrops applies every validated meta instance's drop before
// any annotator runs: a key drop through the fact store's key
// tombstone, a group name through the group tombstone, each at
// directive authority with the instance's position as the claim's.
// The out and diag instances are carried, not consumed. A refused
// drop is a defect, because validation resolved the reference.
func (w *Workspace) applyDrops(
	table map[symbol.Identity][]directive.Directive, facts *meta.Facts,
) error {
	var errs []error
	for _, id := range slices.SortedFunc(maps.Keys(table), symbol.Identity.Compare) {
		for _, d := range table[id] {
			if d.Name != directive.KernelMeta {
				continue
			}
			v, held := d.Param(directive.MetaDrop)
			if !held {
				continue
			}
			claim := meta.Claim{
				Subject:   id,
				Authority: meta.AuthorityDirective,
				Seq:       d.Instance,
				Pos:       d.Pos,
			}
			if key, registered := w.keys.Resolve(meta.KeyName(v.Ref)); registered {
				if err := facts.DropKey(key, claim); err != nil {
					errs = append(errs, err)
				}
				continue
			}
			if err := facts.DropGroup(meta.GroupName(v.Ref), claim); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// annotateAll runs the annotate schedule in bucket order over one
// whole-graph index, each role handed its own tracked reader and
// its rank fields. A returned error stops the frame, wrapped with
// the role that returned it.
func (w *Workspace) annotateAll(
	ctx context.Context, g *store.Graph, facts *meta.Facts,
	table map[symbol.Identity][]directive.Directive, sink *diag.Sink,
) error {
	if len(w.annotate) == 0 {
		return nil
	}
	ix, err := plugin.NewIndex(g, facts, table, nil)
	if err != nil {
		return err
	}
	for _, s := range w.annotate {
		if err := ctx.Err(); err != nil {
			return err
		}
		reader, err := ix.Reader(store.NewReadSet())
		if err != nil {
			return err
		}
		call := &plugin.AnnotatorContext{
			Index:  ix,
			Reader: reader,
			Facts:  facts,
			Sink:   sink,
			Plugin: s.name,
			Bucket: s.bucket,
		}
		if err := s.run.Annotate(call); err != nil {
			return fmt.Errorf(
				"workspace: annotator %s in bucket %d: %w", s.name, s.bucket, err,
			)
		}
	}
	return nil
}

// generateAll runs the plans in parallel, each over its own emit
// store, its own scoped index and its own readers. A plan's
// failure does not stop its siblings; every plan's store arrives in
// emits either way, so the report shows what each plan produced.
func (w *Workspace) generateAll(
	ctx context.Context, g *store.Graph, facts *meta.Facts,
	table map[symbol.Identity][]directive.Directive, sink *diag.Sink,
	emits map[string]*plugin.Emit,
) ([][]staged, []error) {
	stores := make([]*plugin.Emit, len(w.plans))
	files := make([][]staged, len(w.plans))
	failures := make([]error, len(w.plans))
	var wg sync.WaitGroup
	for i := range w.plans {
		wg.Go(func() {
			stores[i] = plugin.NewEmit()
			files[i], failures[i] = runPlan(ctx, g, facts, table, sink, w.plans[i], stores[i])
		})
	}
	wg.Wait()
	var errs []error
	for i, pl := range w.plans {
		emits[pl.name] = stores[i]
		if failures[i] != nil {
			errs = append(errs, fmt.Errorf("workspace: plan %q: %w", pl.name, failures[i]))
		}
	}
	return files, errs
}

// runPlan runs one plan's roles in bucket order, which is what an
// emit-triggered rule's visibility is defined against: the store
// holds earlier buckets' units when a later role runs.
func runPlan(
	ctx context.Context, g *store.Graph, facts *meta.Facts,
	table map[symbol.Identity][]directive.Directive, sink *diag.Sink,
	pl compiledPlan, into *plugin.Emit,
) ([]staged, error) {
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
			Index:  ix,
			Reader: reader,
			Facts:  facts,
			Emit:   into,
			Sink:   sink,
			Plugin: s.name,
			Bucket: s.bucket,
		}
		if err := s.run.Generate(call); err != nil {
			return nil, fmt.Errorf("generator %s in bucket %d: %w", s.name, s.bucket, err)
		}
	}
	if err := plugin.Settle(into, pl.backend, sink); err != nil {
		return nil, fmt.Errorf("settle: %w", err)
	}
	return render(pl, into, sink)
}
