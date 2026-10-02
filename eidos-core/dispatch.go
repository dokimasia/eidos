// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// invocation is one handler call: the subject, its position, the
// gating instance that caused the call, and the trigger's value.
// The constructors turn one into their typed match.
type invocation struct {
	rs      *runState
	fr      *flatRule
	seq     int
	subject symbol.Identity
	pos     position.Pos
	gate    *directive.Directive
	value   symbol.Symbol
}

// scratch returns the rule's reusable match on the invocation's lane,
// nil on the lane's first invocation of the rule. A match is valid for
// the duration of its handler call and is reused afterwards, so an
// invocation allocates nothing in steady state. Retaining one past the
// call is a defect. The scratch is on the lane, never the shared rule,
// so concurrent plans and concurrent lanes cannot meet.
func (inv invocation) scratch() any { return inv.rs.scratch[inv.fr.ordinal] }

// keep stores the rule's reusable match for the lane's next
// invocation.
func (inv invocation) keep(m any) { inv.rs.scratch[inv.fr.ordinal] = m }

// chunk is how many matches a call on more than one worker collects
// before it runs them: 96 KiB of collected matches on a 64-bit
// platform, so a rule of any size runs in one buffer of that size, and
// a chunk of trivial handlers is still tens of microseconds of work for
// a lane.
const chunk = 4096

// pending is one collected match: the trigger's value and the gating
// instance, nil where no directive gates the rule. The subject and its
// position are resolved again from the value when the match runs,
// which keeps a collected match at two words and a pointer.
type pending struct {
	value symbol.Symbol
	gate  *directive.Directive
}

// phaseCall is one plugin's phase call: the routing surface and the
// rank fields every lane reads, a rule's collected matches, and the
// accumulators the lanes' effects apply into when every rule has run.
// It runs on the calling goroutine, and its lanes write none of its
// fields.
type phaseCall struct {
	b       *built
	index   *plugin.Index
	facts   *meta.Facts
	sink    *diag.Sink
	emit    *plugin.Emit
	plugin  plugin.ID
	bucket  int
	rules   *rules.Registry
	kernel  meta.KernelKeys
	workers int
	// collects reports that the call runs on more than one worker, so
	// it collects each rule's matches before it runs them.
	collects bool
	// seq is the sequence the next match takes: the numbers continue
	// across the phase call's rules, so they are the canonical match
	// order.
	seq int
	// pending is the running rule's collected matches, at most a chunk,
	// where the call runs on more than one worker.
	pending []pending
	// stopped is the error of the sequential invocation that stopped
	// the running rule, which stops the phase call.
	stopped error
	lanes   []*runState
	// first is the first lane, which every call runs, and one is the
	// lane list of a call that runs one lane. Both are in the call's
	// own allocation, so a sequential call allocates no lane.
	first runState
	one   [1]*runState
	accs  map[accKey]*accumulator
	// reported records the languages whose absent rules the phase call
	// already warned about, so the warning comes once per plugin and
	// language.
	reported map[symbol.Lang]bool
}

// newPhaseCall binds one phase call. A worker count below two runs
// every rule sequentially, on one lane.
func newPhaseCall(
	b *built, ix *plugin.Index, facts *meta.Facts, sink *diag.Sink, em *plugin.Emit,
	id plugin.ID, bucket int, rs *rules.Registry, kernel meta.KernelKeys, workers int,
) *phaseCall {
	c := &phaseCall{
		b:        b,
		index:    ix,
		facts:    facts,
		sink:     sink,
		emit:     em,
		plugin:   id,
		bucket:   bucket,
		rules:    rs,
		kernel:   kernel,
		workers:  workers,
		collects: workers > 1,
		accs:     map[accKey]*accumulator{},
	}
	c.first = runState{phaseCall: c, scratch: make([]any, len(b.rules)), fx: effects{lastHost: -1}}
	c.one[0] = &c.first
	c.lanes = c.one[:]
	return c
}

// runState is one lane of a phase call: what invocations on one
// goroutine run with. A lane has reusable matches, a handle pool, the
// languages it already warned about and an effect buffer of its own,
// and reads the phase call's shared state through the embedded
// pointer. Invocations on one lane run one after another, in
// increasing sequence.
type runState struct {
	*phaseCall
	// scratch is one reusable match per rule, indexed by ordinal.
	scratch []any
	// handles is the pool the emitter's accessors are served from,
	// reset per invocation: every call mints a distinct entry, so two
	// live handles in one invocation never alias, and a handler
	// touching a few families allocates no Out at all. The pool is on
	// the lane, not on the emitter, so the match copy an invocation
	// makes remains small.
	handles [4]Out
	minted  int
	// warned records the languages this lane buffered the absent-rules
	// warning for, so the lane buffers one per language.
	warned map[symbol.Lang]bool
	fx     effects
	// failed is the index, into the rule's matches, of the invocation
	// that returned failure: the error that stopped the lane. Each
	// rule resets both.
	failed  int
	failure error
	// panicked is the index of the invocation that panicked, and
	// recovered its value, which the join raises again on the calling
	// goroutine.
	panicked  int
	recovered any
}

// lane returns the phase call's i-th lane, created on first use.
func (c *phaseCall) lane(i int) *runState {
	for len(c.lanes) <= i {
		c.lanes = append(c.lanes, &runState{
			phaseCall: c,
			scratch:   make([]any, len(c.b.rules)),
			fx:        effects{lastHost: -1},
		})
	}
	return c.lanes[i]
}

// emitterFor returns the effect handle bound to one invocation,
// wired into the match's own allocation.
func (rs *runState) emitterFor(m *match) *Emitter {
	m.em = Emitter{rs: rs, m: m}
	rs.minted = 0
	return &m.em
}

// run dispatches the rules of the given phases, in declaration order,
// and wraps a handler's error with the plugin and the rule it failed
// in. A failed rule stops the phase call, and the findings of the
// invocations up to the failed one still arrive in the sink.
func (c *phaseCall) run(phases ...plugin.Phase) error {
	for i := range c.b.rules {
		fr := &c.b.rules[i]
		if !slices.Contains(phases, fr.phase) {
			continue
		}
		if err := c.dispatch(fr); err != nil {
			return fmt.Errorf("eidos: %s rule %d: %w", c.plugin, fr.ordinal, err)
		}
	}
	return nil
}

// dispatch runs one rule's matches. A call on fewer than two workers
// runs each on the first lane as the rule's index enumerates it. A
// call on more collects them a chunk at a time and runs each chunk on
// up to its worker count.
func (c *phaseCall) dispatch(fr *flatRule) error {
	c.enumerate(fr)
	if c.collects {
		c.runPending(fr)
	}
	return c.stopped
}

// take runs one enumerated match on the first lane, under the next
// sequence number, where the call runs sequentially, and collects it
// where the call runs on more than one worker, running the collected
// chunk once it is full. It reports false where an invocation failed,
// which stops the rule: the findings up to the failed invocation
// arrive in the sink, and dispatch returns its error.
func (c *phaseCall) take(fr *flatRule, inv *invocation) bool {
	if c.collects {
		c.pending = append(c.pending, pending{value: inv.value, gate: inv.gate})
		return len(c.pending) < chunk || c.runPending(fr)
	}
	seq := c.seq
	c.seq++
	if err := c.lanes[0].invoke(fr, inv, seq); err != nil {
		c.reportThrough(seq)
		c.stopped = err
		return false
	}
	return true
}

// runPending runs the collected matches on up to the call's worker
// count, under the next sequence numbers, and empties the collection.
// It reports false where an invocation failed.
func (c *phaseCall) runPending(fr *flatRule) bool {
	n := len(c.pending)
	if n == 0 {
		return true
	}
	base := c.seq
	c.seq += n
	err := c.parallel(fr, base, min(c.workers, n))
	c.pending = c.pending[:0]
	if err != nil {
		c.stopped = err
		return false
	}
	return true
}

// parallel runs one rule's collected matches on n lanes, each taking
// the next match in sequence order, the calling goroutine running the
// first lane. A lane whose invocation fails stops every lane from
// taking another match. Every match below the failed one was taken
// already, so the first failure by sequence is the one the call
// returns, as it is when the rule runs sequentially. A panic in a
// handler is raised again on the calling goroutine, the first by
// sequence.
func (c *phaseCall) parallel(fr *flatRule, base, n int) error {
	c.lane(n - 1)
	var next atomic.Int64
	var stop atomic.Bool
	for _, ln := range c.lanes[:n] {
		ln.failed, ln.failure = -1, nil
		ln.panicked, ln.recovered = -1, nil
	}
	var wg sync.WaitGroup
	for _, ln := range c.lanes[1:n] {
		wg.Go(func() { ln.drain(fr, base, &next, &stop) })
	}
	c.lanes[0].drain(fr, base, &next, &stop)
	wg.Wait()

	first, firstPanic := -1, -1
	var failure error
	var recovered any
	for _, ln := range c.lanes[:n] {
		if ln.failure != nil && (first < 0 || ln.failed < first) {
			first, failure = ln.failed, ln.failure
		}
		if ln.panicked >= 0 && (firstPanic < 0 || ln.panicked < firstPanic) {
			firstPanic, recovered = ln.panicked, ln.recovered
		}
	}
	if firstPanic >= 0 && (first < 0 || firstPanic < first) {
		panic(recovered)
	}
	if failure != nil {
		c.reportThrough(base + first)
		return failure
	}
	return nil
}

// drain runs the rule's matches the lane takes, until none is left or
// a lane stopped the rule.
func (rs *runState) drain(fr *flatRule, base int, next *atomic.Int64, stop *atomic.Bool) {
	at := -1
	defer func() {
		if r := recover(); r != nil {
			rs.panicked, rs.recovered = at, r
			stop.Store(true)
		}
	}()
	for !stop.Load() {
		at = int(next.Add(1)) - 1
		if at >= len(rs.pending) {
			return
		}
		inv := rs.invocationOf(fr, rs.pending[at])
		if err := rs.invoke(fr, &inv, base+at); err != nil {
			rs.failed, rs.failure = at, err
			stop.Store(true)
			return
		}
	}
}

// invoke runs one match on the lane with its sequence in canonical
// match order, binding the lane, the rule and the sequence into inv.
func (rs *runState) invoke(fr *flatRule, inv *invocation, seq int) error {
	inv.rs, inv.fr, inv.seq = rs, fr, seq
	return fr.invoke(*inv)
}

// invocationOf resolves a collected match's subject and position
// again, the way its rule's enumeration resolved them.
func (c *phaseCall) invocationOf(fr *flatRule, p pending) invocation {
	inv := invocation{gate: p.gate, value: p.value}
	switch {
	case fr.graph:
	case fr.phase == plugin.PhaseEmit:
		inv.subject, _ = emit.OriginOf(p.value)
		inv.pos = c.positionOf(inv.subject)
	default:
		decl, _ := p.value.(node.Declaration)
		inv.subject, inv.pos = decl.Identity(), decl.Position()
	}
	return inv
}

// enumerate visits one rule's matches through the narrowest index its
// gates admit, in canonical order: the order the index enumerates the
// subjects in, then each subject's gating instances in source order.
// It hands each to take, and stops where take reports false. A
// fact-gated or bare rule has no gating directive, so its visits hand
// their one match to take directly.
func (c *phaseCall) enumerate(fr *flatRule) {
	switch {
	case fr.graph:
		c.take(fr, &invocation{})
	case fr.phase == plugin.PhaseEmit:
		c.enumerateEmit(fr)
	case fr.gate != "":
		c.enumerateDirective(fr)
	case len(fr.preds) > 0:
		c.enumerateFacts(fr)
	default:
		c.enumerateBare(fr)
	}
}

// enumerateEmit visits the plan's emit values of the rule's kind.
// Every emit-trigger mechanism resolves through the origin: the gate
// views, the predicates, skip, and the report position.
func (c *phaseCall) enumerateEmit(fr *flatRule) {
	for value := range c.emit.ByKind(fr.kind) {
		origin, _ := emit.OriginOf(value)
		if !c.admits(fr, origin) {
			continue
		}
		if !c.takeEach(fr, &invocation{subject: origin, pos: c.positionOf(origin), value: value}) {
			return
		}
	}
}

// enumerateDirective visits the carriers of the rule's gating
// directive, under every spelling it recognises.
func (c *phaseCall) enumerateDirective(fr *flatRule) {
	seen := map[symbol.Identity]struct{}{}
	for _, spelled := range spellingsOf(fr) {
		for s := range c.index.ByDirective(spelled) {
			decl, names := s.(node.Declaration)
			if !names || decl.Kind() != fr.kind {
				continue
			}
			id := decl.Identity()
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			if !c.admits(fr, id) {
				continue
			}
			if !c.takeEach(fr, &invocation{subject: id, pos: decl.Position(), value: s}) {
				return
			}
		}
	}
}

// enumerateFacts visits the subjects stamped with the rule's first
// gate key, resolving each to its declaration and checking it against
// the trigger's kind and the remaining predicates.
func (c *phaseCall) enumerateFacts(fr *flatRule) {
	for id := range c.index.ByFactKey(fr.preds[0].id) {
		s, held := c.index.Lookup(id)
		if !held || s.Kind() != fr.kind {
			continue
		}
		if _, names := s.(node.Declaration); !names || !c.admits(fr, id) {
			continue
		}
		if !c.take(fr, &invocation{subject: id, pos: s.Position(), value: s}) {
			return
		}
	}
}

// enumerateBare visits every declaration of the rule's kind in scope:
// the one path whose cost grows with the whole graph.
func (c *phaseCall) enumerateBare(fr *flatRule) {
	for s := range c.index.ByKind(fr.kind) {
		decl, names := s.(node.Declaration)
		if !names {
			continue
		}
		id := decl.Identity()
		if !c.admits(fr, id) {
			continue
		}
		if !c.take(fr, &invocation{subject: id, pos: decl.Position(), value: s}) {
			return
		}
	}
}

// takeEach hands one subject's matches to take: one, or where a
// directive gates the rule, one per instance of the gating directive on
// the subject, in source order, which is how a repeatable directive
// runs its handler per instance. A negated instance gates nothing. A
// gated match points into the validated table, so it allocates nothing
// of its own. It reports false where take stopped the rule.
func (c *phaseCall) takeEach(fr *flatRule, inv *invocation) bool {
	if fr.gate == "" {
		return c.take(fr, inv)
	}
	ds := c.index.DirectivesOf(inv.subject)
	for i := range ds {
		if ds[i].Name != fr.gate || ds[i].Negated {
			continue
		}
		inv.gate = &ds[i]
		if !c.take(fr, inv) {
			return false
		}
	}
	return true
}

// admits evaluates skip, a negated directive's opt-out and the
// predicates for one subject, untracked, because gate evaluation is
// routing. A directive-gated rule is exempt from both opt-outs: its
// subject opted in explicitly and withdraws by deleting the
// directive.
func (c *phaseCall) admits(fr *flatRule, subject symbol.Identity) bool {
	if fr.gate == "" && c.index.Skipped(subject, c.plugin) {
		return false
	}
	for _, p := range fr.preds {
		if !p.test(c.facts, subject) {
			return false
		}
	}
	return true
}

// positionOf resolves a subject's position for reporting, zero when
// the scope does not contain it.
func (c *phaseCall) positionOf(id symbol.Identity) position.Pos {
	s, held := c.index.Lookup(id)
	if !held {
		return position.Pos{}
	}
	return s.Position()
}

// spellingsOf returns the spellings a rule's gating directive may
// be indexed under: the canonical one, and the bare one where a
// schema's differ. A kernel gate has one spelling.
func spellingsOf(fr *flatRule) []directive.Name {
	if fr.schema == nil || fr.gate == fr.schema.Name {
		return []directive.Name{fr.gate}
	}
	return []directive.Name{fr.gate, fr.schema.Name}
}
