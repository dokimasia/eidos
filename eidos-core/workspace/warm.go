// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// warmRun is the state of the shared phases of one warm run. loaded is
// the load's report, and changes its changes. facts is the run's fact
// store, which restores each bag from the generation on first use.
// recorded is a second store over the same generation. No write changes
// recorded, so it returns the winners of the generation. dirty contains
// the dirty edges and the records that read them.
type warmRun struct {
	w        *Workspace
	g        *store.Graph
	loaded   *load.Report
	changes  *load.Changes
	phases   *state.PhaseState
	facts    *meta.Facts
	recorded *meta.Facts
	rec      *state.Recorder
	sink     *diag.Sink
	stats    *Stats
	dirty    *dirtySet
	table    *validations
	// frontends lists the composition's frontends by name. The claim of a
	// classification stamp names one of them as its plugin.
	frontends map[plugin.ID]struct{}
	// candidates lists the subjects whose matches may have changed. These
	// are the declarations that appeared or changed, the subjects whose
	// validated directives changed, and the subject of each fact whose
	// winner changed.
	candidates map[symbol.Identity]struct{}
	// revalidated lists the subjects that the run validated again, in
	// subject order.
	revalidated []revalidation
	// dropped lists the annotator invocations that the run executed again
	// or removed, by match.
	dropped map[plugin.MatchKey]struct{}
	// changed lists the facts whose winner differs from the winner in the
	// generation.
	changed map[meta.FactRef]struct{}
	// refused reports whether the stamp replay refused a stamp or met a
	// dangling one.
	refused bool
}

// revalidation is one subject that the warm run validated again. before
// lists the directives that the generation recorded for the subject, and
// after lists the directives that the run validated.
type revalidation struct {
	subject       symbol.Identity
	before, after []directive.Directive
}

// claimJournal is the journal of a warm phase call. It collects the facts
// that each invocation claimed, and passes each record on to next. After
// the call, the run compares the winners of the collected facts with the
// winners in the generation.
//
// # Concurrency
//
// A claimJournal is not safe for concurrent use, because a phase call
// journals on the goroutine that made the call.
type claimJournal struct {
	facts []meta.FactRef
	next  plugin.Journal
}

var _ plugin.Journal = (*claimJournal)(nil)

// Invoked collects the facts that the invocation claimed, and passes the
// record on.
func (c *claimJournal) Invoked(inv plugin.Invocation) {
	c.facts = append(c.facts, inv.Claimed...)
	c.next.Invoked(inv)
}

// Evaluated passes the matches of a candidate on.
func (c *claimJournal) Evaluated(subject symbol.Identity, matches []plugin.MatchKey) {
	c.next.Evaluated(subject, matches)
}

// warmShared runs the shared phases of a warm run. Each phase executes
// again only the records that read a dirty edge, and it reads only what
// the earlier phases settled. The phases run in this order:
//
//  1. The load's changes make dirty the edges of each declaration,
//     package, kind and directive spelling that they changed.
//  2. Directive validation runs again over each subject whose raw
//     directives changed, and over each subject whose recorded validation
//     read a dirty edge. It reads an empty fact store, as the validation
//     of a cold run does. A subject whose validated directives changed
//     makes its declaration edge dirty.
//  3. The run withdraws every claim on each declaration that
//     disappeared. For each subject whose stamps changed, it withdraws
//     the recorded stamps and files the subject's stamps again. For each
//     subject that it validated again, it withdraws the recorded drops
//     and files the subject's drops again.
//  4. Each annotator call runs again the recorded invocations of its
//     plugin that read a dirty edge or whose subject is a candidate, and
//     it evaluates every candidate. The run first withdraws the claims of
//     each invocation that the call runs again. A plugin that implements
//     its role directly runs whole when the run found any change. The run
//     first withdraws the claims of that plugin in its bucket, on the
//     facts that the plugin claimed in the generation.
//  5. After the stamps and after each annotator call, each fact whose
//     winner differs from the winner in the generation makes its fact
//     edge dirty and its subject a candidate. A fact whose winner did not
//     change stops there, so no record that read the fact runs again.
//  6. The run reports the findings of each validation and annotator
//     invocation that it keeps.
//  7. The run probes every candidate, so the dirty set lists each plan's
//     recorded invocations of the candidate, which the plan executes
//     again.
//  8. The run counts the packages that name each module: the
//     generation's count, with the count of each package whose module
//     facts changed and of each package that the load removed taken again.
//     Where the modules change, their edge is dirty.
//
// warmShared returns the dirty set, the candidates in identity order and
// the count of the modules, which the plans read. It records into rec
// each validation and invocation that it executes, and each record and
// claim that it drops. rec is nil when the run does not record its
// phases.
//
// Error modes:
//   - [damage] for a record of the generation that does not read whole.
//     [Workspace.Run] then runs again cold.
//   - the fact store's error for a drop or a withdrawal that it refuses
//   - an annotator's returned error, wrapped with the annotator's role
func (w *Workspace) warmShared(
	ctx context.Context, g *store.Graph, loaded *load.Report, phases *state.PhaseState,
	facts *meta.Facts, sink *diag.Sink, stats *Stats, rec *state.Recorder,
) (shared, error) {
	r := &warmRun{
		w:          w,
		g:          g,
		loaded:     loaded,
		changes:    loaded.Changes,
		phases:     phases,
		facts:      facts,
		recorded:   meta.Restore(w.keys, phases),
		rec:        rec,
		sink:       sink,
		stats:      stats,
		dirty:      newDirtySet(phases),
		table:      &validations{fresh: map[symbol.Identity][]directive.Directive{}, prior: phases, graph: g},
		frontends:  map[plugin.ID]struct{}{},
		candidates: map[symbol.Identity]struct{}{},
		dropped:    map[plugin.MatchKey]struct{}{},
		changed:    map[meta.FactRef]struct{}{},
	}
	for _, f := range w.frontends {
		r.frontends[f.Name()] = struct{}{}
	}
	rec.Keep()
	out := shared{table: r.table, lazy: r.table}
	err := r.seed()
	if err == nil {
		err = r.validate()
	}
	if err == nil {
		err = r.restamp()
	}
	out.refused = r.refused
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = r.annotate(ctx)
	}
	if err == nil {
		err = r.replay()
	}
	out.dirty = r.dirty
	out.candidates = slices.SortedFunc(maps.Keys(r.candidates), symbol.Identity.Compare)
	for _, id := range out.candidates {
		if err == nil {
			err = r.dirty.probe(id)
		}
	}
	if err == nil {
		out.modules, err = r.modules()
	}
	if err == nil {
		err = cmp.Or(facts.Damaged(), r.recorded.Damaged(), r.table.Damaged())
	}
	if errors.Is(err, state.ErrDamaged) {
		err = fmt.Errorf("workspace: read the record of the phases: %w", err)
	}
	return out, damaged(err)
}

// seed makes dirty the edges that the load changed. These are the
// declaration edge of each declaration that appeared, disappeared or
// changed, the kind edge of each declaration that appeared or
// disappeared, the edge of each changed package, the directive edge of
// each spelling that a subject gained or lost, and the edges of where the
// layout places files, which [warmRun.placements] lists. The declarations
// that appeared or changed become the first candidates.
//
// Error modes: an error wrapping [state.ErrDamaged] for a row of the
// generation that does not read whole.
func (r *warmRun) seed() error {
	for _, e := range r.placements() {
		if err := r.dirty.add(e); err != nil {
			return err
		}
	}
	c := r.changes
	for _, id := range slices.Concat(c.Appeared, c.Disappeared) {
		if err := cmp.Or(r.dirty.declaration(id), r.dirty.add(state.KindEdge(id.Kind))); err != nil {
			return err
		}
	}
	for _, id := range c.Changed {
		if err := r.dirty.declaration(id); err != nil {
			return err
		}
	}
	for _, id := range slices.Concat(c.Appeared, c.Changed) {
		r.candidates[id] = struct{}{}
	}
	for _, p := range c.Packages {
		if err := r.dirty.add(state.PackageEdge(p)); err != nil {
			return err
		}
	}
	for _, n := range c.Spellings {
		if err := r.dirty.add(state.DirectiveEdge(n)); err != nil {
			return err
		}
	}
	return nil
}

// validate runs directive validation again, in subject order, over each
// subject whose raw directives changed and each subject whose recorded
// validation read a dirty edge. Validation reads an empty fact store,
// because a cold run validates before it files any fact. Each validation
// replaces the subject's record, and a subject without a raw instance
// loses its record. The findings merge into the run's sink in the
// canonical finding order. A subject whose validated directives changed
// becomes a candidate and makes its declaration edge dirty.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record of the
// generation that does not read whole.
func (r *warmRun) validate() error {
	subjects := maps.Clone(r.dirty.validations)
	for _, id := range r.changes.Directed {
		subjects[id] = struct{}{}
	}
	var lane *state.Lane
	if r.rec != nil {
		lane = r.rec.Lane("")
	}
	empty := meta.NewFacts(r.w.keys)
	reads := store.NewReadSet()
	var found []diag.Diag
	for _, id := range slices.SortedFunc(maps.Keys(subjects), symbol.Identity.Compare) {
		prior, _, err := r.phases.Validation(id)
		if err != nil {
			return err
		}
		r.rec.DropValidation(id)
		var ds []directive.Directive
		switch raws := r.g.DirectivesOf(id); {
		case len(raws) == 0:
		case !r.g.Holds(id):
			d := danglingDirectives(id, raws[0].Pos)
			lane.Validation(id, nil, nil, []diag.Diag{d})
			found = append(found, d)
		default:
			reads.Reset()
			local := diag.NewSink()
			ds = directive.Validate(id, raws, r.w.directives, r.w.keys, r.w.resolver(r.g, empty, reads), local)
			reported := slices.Collect(local.All())
			lane.Validation(id, ds, reads, reported)
			found = append(found, reported...)
			r.stats.Validated++
		}
		r.table.fresh[id] = ds
		r.revalidated = append(r.revalidated, revalidation{subject: id, before: prior.Directives, after: ds})
		if (len(prior.Directives) == 0 && len(ds) == 0) || reflect.DeepEqual(prior.Directives, ds) {
			continue
		}
		r.candidates[id] = struct{}{}
		if err := r.dirty.declaration(id); err != nil {
			return err
		}
	}
	slices.SortStableFunc(found, diag.Diag.Compare)
	for _, d := range found {
		r.sink.Report(d)
	}
	return nil
}

// restamp brings the stamps and drops of the fact store up to date with
// the load. It withdraws every claim on each declaration that
// disappeared. For each subject whose stamps changed, it withdraws the
// recorded stamp claims and files the subject's stamps again. For each
// subject that the run validated again, it withdraws the recorded drops
// and files the subject's drops again. A stamp reports its findings as it
// does on a cold run, and refused reports whether one of them is an
// Error. Each touched fact whose winner changed then makes its fact edge
// dirty.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record of the
// generation that does not read whole, and the fact store's error for a
// drop or a withdrawal that it refuses.
func (r *warmRun) restamp() error {
	var touched []meta.FactRef
	for _, id := range r.changes.Disappeared {
		var err error
		if touched, err = r.withdrawAll(id, touched); err != nil {
			return err
		}
	}
	for _, id := range r.changes.Restamped {
		var err error
		if touched, err = r.withdrawStamps(id, touched); err != nil {
			return err
		}
		stamps := r.g.StampsOf(id)
		r.refused = stampSubject(r.g, r.facts, r.sink, id, stamps) || r.refused
		for _, s := range stamps {
			touched = append(touched, meta.FactRef{Subject: id, Key: s.Key})
		}
	}
	for _, v := range r.revalidated {
		var withdrawn, filed error
		before := len(touched)
		touched, withdrawn = r.w.drops(r.facts, v.subject, v.before, true, touched)
		if len(touched) > before {
			r.rec.Withdrew(v.subject)
		}
		touched, filed = r.w.drops(r.facts, v.subject, v.after, false, touched)
		if err := cmp.Or(withdrawn, filed); err != nil {
			return err
		}
	}
	return r.settle(touched)
}

// withdrawAll withdraws every claim that the generation recorded on a
// subject, including its drops and group drops. It appends the facts that
// the claims covered to touched. A declaration that disappeared keeps no
// claim.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that
// does not read whole, and the fact store's error for a withdrawal that
// it refuses.
func (r *warmRun) withdrawAll(id symbol.Identity, touched []meta.FactRef) ([]meta.FactRef, error) {
	claims, err := r.phases.Claims(id)
	if err != nil || len(claims) == 0 {
		return touched, err
	}
	r.rec.Withdrew(id)
	for _, sc := range claims {
		if sc.Group != "" {
			for member := range r.w.keys.Group(sc.Group) {
				spec, _ := r.w.keys.Spec(member)
				touched = append(touched, meta.FactRef{Subject: id, Key: spec.Name})
			}
			if err := r.facts.WithdrawGroup(sc.Group, sc.Claim); err != nil {
				return touched, err
			}
			continue
		}
		key, registered := r.w.keys.Resolve(sc.Key)
		if !registered {
			continue
		}
		touched = append(touched, meta.FactRef{Subject: id, Key: sc.Key})
		if err := r.facts.Withdraw(key, sc.Claim); err != nil {
			return touched, err
		}
	}
	return touched, nil
}

// withdrawStamps withdraws the classification stamps that the generation
// recorded on a subject, and appends the facts that they covered to
// touched. A frontend makes the claim of a stamp at plugin authority, in
// bucket zero and under rule zero, and ranks it by the subject itself.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record that
// does not read whole, and the fact store's error for a withdrawal that
// it refuses.
func (r *warmRun) withdrawStamps(id symbol.Identity, touched []meta.FactRef) ([]meta.FactRef, error) {
	claims, err := r.phases.Claims(id)
	if err != nil {
		return touched, err
	}
	for _, sc := range claims {
		c := sc.Claim
		_, stamp := r.frontends[c.Plugin]
		if !stamp || sc.Group != "" || sc.Drop || c.Authority != meta.AuthorityPlugin || c.Bucket != 0 ||
			c.Order.Rule != 0 || c.Order.Subject != id {
			continue
		}
		key, registered := r.w.keys.Resolve(sc.Key)
		if !registered {
			continue
		}
		r.rec.Withdrew(id)
		touched = append(touched, meta.FactRef{Subject: id, Key: sc.Key})
		if err := r.facts.Withdraw(key, c); err != nil {
			return touched, err
		}
	}
	return touched, nil
}

// annotate runs the annotate schedule in bucket order over one
// whole-graph index. Each call runs over the records that are dirty when
// the call starts. annotate counts the invocations of each call into the
// run's stats, and counts zero for a call that did not run.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record of the
// generation that does not read whole, the context's error, and an
// annotator's returned error, wrapped with the annotator's role.
func (r *warmRun) annotate(ctx context.Context) error {
	if len(r.w.annotate) == 0 {
		return nil
	}
	ix, err := plugin.NewIndex(r.g, r.facts, r.table, nil)
	if err != nil {
		return err
	}
	counted := &tally{}
	if r.rec != nil {
		counted.next = r.rec.Lane("")
	}
	for _, s := range r.w.annotate {
		if err := ctx.Err(); err != nil {
			return err
		}
		whole, direct, err := r.phases.Invocation("", plugin.MatchKey{Plugin: s.name, Rule: plugin.WholeCall})
		if err != nil {
			return err
		}
		var (
			touched []meta.FactRef
			ran     bool
		)
		if direct {
			touched, ran, err = r.annotateWhole(ix, s, whole, counted)
		} else {
			touched, ran, err = r.annotateSelected(ix, s, counted)
		}
		if err != nil {
			return err
		}
		count := 0
		if ran {
			count = counted.count
		}
		r.stats.Invoked = append(r.stats.Invoked, Invoked{Plugin: s.name, Phase: plugin.PhaseAnnotate, Count: count})
		if err := r.settle(touched); err != nil {
			return err
		}
	}
	return nil
}

// annotateWhole runs the call of a plugin that implements its role
// directly. Such a plugin journals no invocation, so its call runs whole
// when the run found any change. annotateWhole first withdraws the claims
// of the plugin in its bucket, on the facts that the call claimed in the
// generation. It records the call under [plugin.WholeCall]. It returns the
// facts that the call touched, and reports whether the call ran.
//
// Error modes: the fact store's error for a withdrawal that it refuses,
// and the call's error, wrapped with the annotator's role.
func (r *warmRun) annotateWhole(
	ix *plugin.Index, s annEntry, whole state.Invocation, counted *tally,
) ([]meta.FactRef, bool, error) {
	if len(r.dirty.edges) == 0 && len(r.candidates) == 0 {
		return nil, false, nil
	}
	var touched []meta.FactRef
	for _, f := range whole.Claimed {
		key, registered := r.w.keys.Resolve(f.Key)
		if !registered {
			continue
		}
		touched = append(touched, f)
		for v := range r.facts.Claims(f.Subject, key) {
			if v.Claim.Plugin != s.name || v.Claim.Bucket != s.bucket {
				continue
			}
			r.rec.Withdrew(f.Subject)
			if err := r.facts.Withdraw(key, v.Claim); err != nil {
				return touched, true, err
			}
		}
	}
	r.drop(whole.Match)
	err := r.w.annotateCall(ix, s, r.facts, r.sink, counted, counted, nil, true)
	return append(touched, r.facts.ClaimedBy(s.name)...), true, err
}

// annotateSelected runs the call of a plugin on the authoring surface
// over a selection. The selection lists the recorded invocations of the
// plugin that read a dirty edge or whose subject is a candidate. The call
// runs each listed invocation again where its gates still admit it, and
// it evaluates every match of each candidate. Before the call,
// annotateSelected withdraws the claims of each listed invocation and
// drops its record. The call's journal records the invocation again when
// it runs and is not pure. A plugin without a listed invocation or a
// candidate does not run. annotateSelected returns the facts that the
// call touched, and reports whether the call ran.
//
// Error modes: an error wrapping [state.ErrDamaged] for a record of the
// generation that does not read whole, the fact store's error for a
// withdrawal that it refuses, and the call's error, wrapped with the
// annotator's role.
func (r *warmRun) annotateSelected(ix *plugin.Index, s annEntry, counted *tally) ([]meta.FactRef, bool, error) {
	candidates := slices.SortedFunc(maps.Keys(r.candidates), symbol.Identity.Compare)
	for _, id := range candidates {
		if err := r.dirty.probe(id); err != nil {
			return nil, false, err
		}
	}
	listed := r.dirty.take(s.name)
	if len(listed) == 0 && len(candidates) == 0 {
		return nil, false, nil
	}
	var touched []meta.FactRef
	keys := make([]plugin.MatchKey, 0, len(listed))
	for _, inv := range listed {
		keys = append(keys, inv.Match)
		r.drop(inv.Match)
		for _, f := range inv.Claimed {
			key, registered := r.w.keys.Resolve(f.Key)
			if !registered {
				continue
			}
			touched = append(touched, f)
			r.rec.Withdrew(f.Subject)
			err := r.facts.Withdraw(key, meta.Claim{
				Subject:   f.Subject,
				Authority: meta.AuthorityPlugin,
				Bucket:    s.bucket,
				Plugin:    s.name,
				Order: meta.Order{
					Rule: int(inv.Match.Rule), Subject: inv.Match.Subject, Instance: inv.Match.Instance,
				},
			})
			if err != nil {
				return touched, true, err
			}
		}
	}
	journal := &claimJournal{next: counted}
	sel := &plugin.Selection{Matches: keys, Candidates: candidates}
	err := r.w.annotateCall(ix, s, r.facts, r.sink, counted, journal, sel, false)
	return append(touched, journal.facts...), true, err
}

// drop drops the generation's record of an annotator invocation that the
// run executes again or removes.
func (r *warmRun) drop(m plugin.MatchKey) {
	r.dropped[m] = struct{}{}
	r.rec.DropInvocation("", m)
}

// settle compares the winner of each fact that the last step touched with
// the winner in the generation. When the winner or the presence of the
// fact differs, settle makes the fact edge dirty and the subject a
// candidate. It handles each fact once.
//
// Error modes: an error wrapping [state.ErrDamaged] for a row of the
// generation that does not read whole.
func (r *warmRun) settle(touched []meta.FactRef) error {
	for _, f := range touched {
		if _, done := r.changed[f]; done {
			continue
		}
		key, registered := r.w.keys.Resolve(f.Key)
		if !registered {
			continue
		}
		now, present := winner(r.facts, f.Subject, key)
		was, held := winner(r.recorded, f.Subject, key)
		if present == held && (!present || reflect.DeepEqual(now, was)) {
			continue
		}
		r.changed[f] = struct{}{}
		r.candidates[f.Subject] = struct{}{}
		if err := r.dirty.add(state.FactEdge(f.Subject, f.Key)); err != nil {
			return err
		}
	}
	return nil
}

// replay reports the findings of each validation and annotator
// invocation that the run keeps. The readers table lists every record
// that reported a finding under [state.FindingsEdge]. replay skips the
// records that the run executed again or removed.
//
// Error modes: an error wrapping [state.ErrDamaged] for a row of the
// generation that does not read whole.
func (r *warmRun) replay() error {
	refs, err := r.phases.Readers(state.FindingsEdge)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		switch ref.Kind {
		case state.RecordValidation:
			vs, err := r.phases.Validations(ref)
			if err != nil {
				return err
			}
			for _, v := range vs {
				if _, again := r.table.fresh[v.Subject]; !again {
					r.report(v.Findings)
				}
			}
		case state.RecordInvocation:
			invs, err := r.phases.Invocations(ref)
			if err != nil {
				return err
			}
			for _, inv := range invs {
				if _, again := r.dropped[inv.Match]; inv.Plan == "" && !again {
					r.report(inv.Findings)
				}
			}
		}
	}
	return nil
}

// report reports the findings of a kept record into the run's sink.
func (r *warmRun) report(found []diag.Diag) {
	for _, d := range found {
		r.sink.Report(d)
	}
}

// winner returns the value of the claim that ranks first on a fact. It
// returns false when that claim is a drop, or when nothing claimed the
// fact.
func winner(f *meta.Facts, id symbol.Identity, k meta.KeyID) (any, bool) {
	for v := range f.Claims(id, k) {
		return v.Value, v.Value != nil
	}
	return nil, false
}

// danglingDirectives returns the finding for directives on a subject that
// the graph does not contain. The finding is at the position of the first
// instance.
func danglingDirectives(subject symbol.Identity, at position.Pos) diag.Diag {
	return diag.Diag{
		Code:     directive.DanglingSubject,
		Severity: diag.SeverityError,
		Pos:      at,
		Msg:      fmt.Sprintf("directives on %s name a subject the graph does not contain", subject),
		Origin:   diag.PhaseFreeze,
	}
}
