// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Input is what one run reads: a tree the composition's frontends
// load, or a graph the caller loaded or built. Exactly one of Tree and
// Graph is set.
type Input struct {
	// Tree is the workspace tree. Every path in a run is relative to
	// its root.
	Tree fs.FS
	// Stores are the read-only trees dependency units read, keyed by
	// store name: a Go module cache, a JDK's ct.sym.
	Stores map[string]fs.FS
	// Graph is a sealed graph, or one the run seals. The run starts at
	// directive validation and audits every declaration in it.
	Graph *store.Graph
	// Dry runs every phase and commits nothing: each plan stages,
	// prepares and discards, and the ledger records nothing.
	Dry bool
	// Cold runs the tree without reading the sealed state, and reports
	// nothing for it. The run writes the next generation as any run does.
	Cold bool
}

// Run takes the input through the frame and returns what happened:
// Begin, Load, directive validation and Annotate, then each plan's
// Generate, Settle, Layout, Render, Stamp and staging in parallel, a
// plan that depends on others after each of them has rendered, then
// Close with its workspace checks, the commits in dependency order and
// the record.
//
// A plan that reports an Error commits nothing, and neither does a
// plan that depends on it. The other plans commit. An Error in a phase
// every plan shares, Load, validation, Annotate or Close, commits
// nothing at all, and a workspace check's Error is a Close Error.
// Every finding is in the report's sink, and any Error returns
// [ErrRunFailed] beside the report. A handler's returned error is
// joined into Run's error, wrapped with its role: an annotator's stops
// the frame, a generator's fails its plan, and a check's fails Close.
// A cancelled context stops the run between units of work and returns
// the context's error beside the report, which states each plan's
// outcome. An input that sets neither or both of a tree and a graph is
// the one refusal that returns a nil report.
//
// The ledger records the run strictly after the last commit: the next
// generation of the sealed state, with the merged manifest's documents
// that differ from the ledger's. A dry run records nothing. A run over a
// caller's graph, a run whose previous record does not read, and a run
// that cannot read its executable write no generation, and record the
// manifest only where a plan or the sweep committed. The ledger records
// under a context without the run's cancellation, because the record
// has to match the destination once a commit wrote to it.
//
// A run over a tree compares the tree with the sealed state's live
// generation. A run that finds the generation damaged, at the load, at
// the record of the load, or at a region its graph decodes for a later
// phase, discards what it derived before any plan commits, reports
// [ColdState], and runs again cold over the same input.
func (w *Workspace) Run(ctx context.Context, in Input) (*Report, error) {
	if (in.Tree == nil) == (in.Graph == nil) {
		return nil, errors.New("workspace: Run needs exactly one of a tree to load and a graph")
	}
	report, err := w.run(ctx, in, nil)
	if d, found := errors.AsType[*damage](err); found && !in.Cold {
		in.Cold = true
		return w.run(ctx, in, d)
	}
	return report, err
}

// run is one attempt at a run: the frame over the input, after which
// [Workspace.Run] runs again cold where the attempt met damage. A cold
// attempt after damage reports the damage first.
func (w *Workspace) run(ctx context.Context, in Input, after *damage) (*Report, error) {
	sink := diag.NewSink()
	if after != nil {
		w.coldState(sink, "the sealed state is damaged, and the run started again cold: %v", after.err)
	}
	facts := meta.NewFacts(w.keys)
	report := &Report{Sink: sink, Facts: facts, Emits: map[string]*plugin.Emit{}}

	rec, err := w.begin(ctx, sink)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return w.stopped(ctx, report, err)
	}
	report.Manifest = rec.previous
	sealed, err := w.openSealed(ctx, rec, in, sink)
	if err == nil {
		err = w.openMemo(ctx, rec, sealed)
	}
	if err != nil {
		return w.stopped(ctx, report, err)
	}
	g, loaded, err := w.load(ctx, in, sealed, sink)
	report.Load = loaded
	report.Stats.count(loaded, sealed)
	if err == nil {
		err = sealed.record(ctx, loaded)
	}
	if err != nil {
		return w.stopped(ctx, report, err)
	}
	table, err := w.annotateRun(ctx, g, facts, sink)
	if err != nil {
		return w.stopped(ctx, report, err)
	}
	shared := sink.Failed()

	var src tree
	if w.open != nil {
		src = tree{residents: layout.Residents(g), modules: layout.Modules(g, facts, w.kernel)}
	}
	runs := w.generateAll(ctx, g, facts, table, src)
	if !shared {
		w.stageAll(ctx, runs, rec)
	}
	for _, p := range runs {
		report.Emits[p.plan.name] = p.emit
		for d := range p.sink.All() {
			sink.Report(d)
		}
	}

	collided := collide(runs, sink)
	sw, err := w.sweep(rec, runs, sink, shared || collided)
	errs := []error{err}
	unmet := w.audit(g, facts, loaded, sink)
	checked := false
	if !shared {
		checked, err = w.check(g, facts, table, runs, sink)
		errs = append(errs, err)
	}
	broken := damaged(g.Damaged())
	blocked := shared || collided || unmet || checked || broken != nil

	errs = append(errs, commitAll(ctx, runs, w.order, sw, blocked, in.Dry, report))
	if broken != nil {
		return report, errors.Join(append(errs, broken)...)
	}
	report.Manifest = merged(rec, runs, sw)
	if sealed.commit != nil {
		errs = append(errs, sealed.write(ctx, report.Manifest, &report.Stats))
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

// tree is the source tree every plan of one run routes against, read
// once from the frozen graph and the fact store: the files each
// directory contains, and the toolchain modules the load resolved.
type tree struct {
	residents map[string][]plugin.Resident
	modules   []plugin.Module
}

// record is the previous run's record as a run reads it: the ledger it
// came from, nil where the composition keeps none, the manifest, the
// digest of each of its documents, nil where the record does not read,
// its entries by plan and by path, and the workspace name the run
// records under.
type record struct {
	ledger   ledger.Ledger
	previous manifest.Manifest
	digests  state.Digests
	byPlan   map[string][]manifest.Entry
	byPath   map[string]manifest.Entry
	// workspace names the workspace in the manifest the run records: the
	// composition's name, or the ledger's where the composition states
	// none.
	workspace string
}

// named is a ledger that names the workspace it records, such as the
// disk ledger, which names it after the workspace root.
type named interface {
	Workspace() string
}

// generator returns the plan the record lists for a path, and fallback
// where the record lists no file there.
func (r *record) generator(path, fallback string) string {
	if e, held := r.byPath[path]; held {
		return e.Plan
	}
	return fallback
}

// begin opens the composition's ledger and reads the previous record:
// the documents of its manifest, joined. A composition that declares no
// output, or no ledger, reads the empty record. A record that does not
// read is reported and read as empty, so the run removes nothing. A
// ledger that fails to open is a returned error, because nothing in
// the source causes it.
func (w *Workspace) begin(ctx context.Context, sink *diag.Sink) (*record, error) {
	rec := &record{
		previous:  manifest.Manifest{Version: manifest.Version},
		byPlan:    map[string][]manifest.Entry{},
		byPath:    map[string]manifest.Entry{},
		workspace: w.id,
	}
	if w.ledger == nil || w.open == nil {
		return rec, nil
	}
	l, err := w.ledger()
	if err != nil {
		return nil, fmt.Errorf("workspace: open the ledger: %w", err)
	}
	if l == nil {
		return nil, errors.New("workspace: the ledger's open function returned (nil, nil)")
	}
	rec.ledger = l
	if n, names := l.(named); names && rec.workspace == "" {
		rec.workspace = n.Workspace()
	}
	previous, digests, err := state.ReadManifest(ctx, l)
	if err != nil {
		sink.Infof(UnreadableRecord, position.Pos{File: ledger.ManifestPath(w.brand)}, diag.PhaseLoad,
			"the previous record does not read, so the run removes nothing: %v", err)
		return rec, nil
	}
	rec.previous, rec.digests = previous, digests
	for _, e := range previous.Files {
		rec.byPlan[e.Plan] = append(rec.byPlan[e.Plan], e)
		rec.byPath[e.Path] = e
	}
	return rec, nil
}

// load returns the graph the run works on, sealed: the caller's, or
// the one the composition's frontends load from the input's tree under
// the composition's brand, against the sealed state's record of the
// last load where the run is warm. A load that meets a damaged record
// returns [damage]. A load that fails otherwise is a returned error,
// and so is a tree without a frontend to load it.
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
		Prior:     s.loadPrior(),
		Memo:      s.loadMemo(in.Cold),
	})
	if err != nil {
		return nil, loaded, damaged(fmt.Errorf("workspace: %w", err))
	}
	return g, loaded, ctx.Err()
}

// annotateRun runs the shared phases after the load: directive
// validation, the stamp replay, the kernel meta drops and the annotate
// schedule. An annotator's returned error stops the frame.
func (w *Workspace) annotateRun(
	ctx context.Context, g *store.Graph, facts *meta.Facts, sink *diag.Sink,
) (map[symbol.Identity][]directive.Directive, error) {
	table := w.validated(g, facts, sink)
	applyStamps(g, facts, sink)
	if err := w.applyDrops(table, facts); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return table, w.annotateAll(ctx, g, facts, table, sink)
}

// stopped returns the report of a frame that stopped before the plans
// committed: every plan is cancelled where the context ended, and
// failed otherwise.
func (w *Workspace) stopped(ctx context.Context, report *Report, err error) (*Report, error) {
	status := PlanFailed
	if ctx.Err() != nil {
		status = PlanCancelled
	}
	for i := range w.plans {
		report.Plans = append(report.Plans, PlanReport{Name: w.plans[i].name, Status: status})
	}
	return report, errors.Join(err, failure(report.Sink))
}

// failure classifies the sink: [ErrRunFailed] when any Error
// arrived, nil otherwise.
func failure(sink *diag.Sink) error {
	if sink.Failed() {
		return ErrRunFailed
	}
	return nil
}

// validated runs directive validation over every attached subject
// on up to GOMAXPROCS workers, because one subject's validation is
// independent of every other. Each worker reports into a sink of
// its own, and the findings merge into the run's sink in the
// canonical finding order, so the report is the same whatever order
// the workers finished in. A subject the graph does not contain
// reports the dangling code and contributes nothing. The survivors
// form the table the routing indexes read, so a rejected instance
// never gates a rule.
func (w *Workspace) validated(
	g *store.Graph, facts *meta.Facts, sink *diag.Sink,
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
	locals := make([]*diag.Sink, min(runtime.GOMAXPROCS(0), len(subjects)))
	var next atomic.Int64
	var wg sync.WaitGroup
	for k := range locals {
		local := diag.NewSink()
		locals[k] = local
		wg.Go(func() {
			for {
				i := int(next.Add(1)) - 1
				if i >= len(subjects) {
					return
				}
				s := subjects[i]
				if !g.Holds(s.subject) {
					local.Errorf(directive.DanglingSubject, s.raws[0].Pos, diag.PhaseFreeze,
						"directives on %s name a subject the graph does not contain", s.subject)
					continue
				}
				results[i] = directive.Validate(
					s.subject, s.raws, w.directives, w.keys, w.resolver(g, facts), local,
				)
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
	return table
}

// resolver binds a subject's reference params through the rules
// registered for the subject's language, each call over a view of
// its own whose reads the run discards, because validation runs
// whole on every run and one subject validates per goroutine. A
// language none registered for resolves nothing, naming itself.
func (w *Workspace) resolver(g *store.Graph, facts *meta.Facts) directive.Resolver {
	return func(subject symbol.Identity, name string, kind directive.ResolutionKind) (symbol.Identity, error) {
		src, held := w.rules.For(subject.Lang)
		if !held {
			return symbol.Identity{}, fmt.Errorf("workspace: no rules are registered for %s", subject.Lang)
		}
		reader, err := g.Reader(store.NewReadSet(), nil)
		if err != nil {
			return symbol.Identity{}, err
		}
		view := rules.View{Decls: reader, Facts: facts, Kernel: w.kernel}
		scope := rules.Scope{Subject: subject, File: fileOf(reader, subject)}
		sym, err := src.Resolve(scope, name, kind, view)
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

// fileOf returns the file a declaration is declared in, matched by
// the declaration's position against its package's files, and nil
// for one the reader does not contain.
func fileOf(reader *store.Reader, subject symbol.Identity) *node.File {
	decl, held := reader.Lookup(subject)
	if !held {
		return nil
	}
	pkg, held := reader.PackageOf(subject)
	if !held {
		return nil
	}
	at := decl.Position().File
	for _, f := range pkg.Files {
		if f != nil && f.Pos.File == at {
			return f
		}
	}
	return nil
}

// applyStamps replays the load's classification stamps into the
// fact store at plugin authority, before any drop or annotator
// runs. Rank decides every winner, so a directive-authority drop
// outranks a stamp whichever applied first. The order here exists
// for the findings, not the outcome. A refusal reports under the
// fact store's own code at the stamp's position, and the frame
// continues. A stamp whose subject the graph does not contain is
// reported and not filed, the way a dangling directive is: a fact
// filed there would enumerate under a subject no reader can look up.
func applyStamps(g *store.Graph, facts *meta.Facts, sink *diag.Sink) {
	for id, stamps := range g.Stamps() {
		if !g.Holds(id) {
			for _, s := range stamps {
				sink.Errorf(directive.DanglingSubject, s.Pos, s.Origin,
					"a %s stamp names %s, which the graph does not contain", s.Key, id)
			}
			continue
		}
		for i, s := range stamps {
			claim := meta.Claim{
				Subject:   id,
				Authority: meta.AuthorityPlugin,
				Plugin:    s.Origin,
				Order:     meta.Order{Subject: id, Instance: i},
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
// The out and diag instances are not applied here. A refused drop
// is a defect, because validation resolved the reference.
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
				Order:     meta.Order{Subject: id, Instance: d.Instance},
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
			Index:   ix,
			Reader:  reader,
			Facts:   facts,
			Sink:    sink,
			Rules:   w.rules,
			Kernel:  w.kernel,
			Plugin:  s.name,
			Bucket:  s.bucket,
			Workers: w.workers,
		}
		if err := s.run.Annotate(call); err != nil {
			return fmt.Errorf(
				"workspace: annotator %s in bucket %d: %w", s.name, s.bucket, err,
			)
		}
	}
	return nil
}
