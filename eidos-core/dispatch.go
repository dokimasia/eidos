// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos

import (
	"fmt"
	"slices"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// invocation is one handler firing: the subject, its position, the
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

// scratch answers the rule's reusable match, nil on its first
// invocation. A match is valid for the duration of its handler
// call and reused afterwards, which is what prices an invocation
// at zero steady-state allocations; retaining one past the call is
// a defect the conformance suite races. The scratch lives on the
// phase call, never the shared rule, so concurrent plans cannot
// meet.
func (inv invocation) scratch() any { return inv.rs.scratch[inv.fr.ordinal] }

// keep stores the rule's reusable match for the next invocation.
func (inv invocation) keep(m any) { inv.rs.scratch[inv.fr.ordinal] = m }

// runState is one phase call's dispatch: the routing surface, the
// rank fields, the running sequence, and the accumulators the
// emitter fills.
type runState struct {
	b      *built
	index  *plugin.Index
	facts  *meta.Facts
	sink   *diag.Sink
	emit   *plugin.Emit
	plugin plugin.ID
	bucket int
	seq    int
	accs   map[accKey]*accumulator
	// scratch holds one reusable match per rule, keyed by ordinal.
	scratch []any
}

// newRunState binds one phase call.
func newRunState(
	b *built, ix *plugin.Index, facts *meta.Facts, sink *diag.Sink,
	em *plugin.Emit, id plugin.ID, bucket int,
) *runState {
	return &runState{
		b:       b,
		index:   ix,
		facts:   facts,
		sink:    sink,
		emit:    em,
		plugin:  id,
		bucket:  bucket,
		accs:    map[accKey]*accumulator{},
		scratch: make([]any, len(b.rules)),
	}
}

// emitterFor answers the effect handle bound to one invocation,
// wired into the match's own allocation.
func (rs *runState) emitterFor(m *match) *Emitter {
	m.em = Emitter{rs: rs, m: m}
	return &m.em
}

// run dispatches the rules of the given phases, in declaration
// order, and wraps a handler's error with the plugin and the rule
// it failed in.
func (rs *runState) run(phases ...plugin.Phase) error {
	for i := range rs.b.rules {
		fr := &rs.b.rules[i]
		if !slices.Contains(phases, fr.phase) {
			continue
		}
		if err := rs.dispatch(fr); err != nil {
			return fmt.Errorf("eidos: %s rule %d: %w", rs.plugin, fr.ordinal, err)
		}
	}
	return nil
}

// dispatch runs one rule through the narrowest index its gates
// admit.
func (rs *runState) dispatch(fr *flatRule) error {
	switch {
	case fr.graph:
		return rs.invoke(fr, invocation{})
	case fr.phase == plugin.PhaseEmit:
		return rs.dispatchEmit(fr)
	case fr.schema != nil:
		return rs.dispatchDirective(fr)
	case len(fr.preds) > 0:
		return rs.dispatchFacts(fr)
	default:
		return rs.dispatchBare(fr)
	}
}

// dispatchEmit visits the plan's emit values of the rule's kind.
// Every emit-trigger mechanism resolves through the origin: the
// gate views, the predicates, skip, and the report position.
func (rs *runState) dispatchEmit(fr *flatRule) error {
	for value := range rs.emit.ByKind(fr.kind) {
		origin, _ := emit.OriginOf(value)
		if !rs.admits(fr, origin) {
			continue
		}
		pos := rs.positionOf(origin)
		if fr.schema == nil {
			err := rs.invoke(fr, invocation{subject: origin, pos: pos, value: value})
			if err != nil {
				return err
			}
			continue
		}
		for _, gate := range gateViews(rs.index.DirectivesOf(origin), fr.schema) {
			err := rs.invoke(fr, invocation{
				subject: origin, pos: pos, gate: gate, value: value,
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// dispatchDirective visits the carriers of the rule's schema, under
// every spelling the schema answers to, one invocation per gating
// instance in source order.
func (rs *runState) dispatchDirective(fr *flatRule) error {
	seen := map[symbol.Identity]struct{}{}
	for _, spelled := range spellingsOf(fr.schema) {
		for s := range rs.index.ByDirective(spelled) {
			decl, names := s.(node.Declaration)
			if !names || decl.Kind() != fr.kind {
				continue
			}
			id := decl.Identity()
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			if !rs.admits(fr, id) {
				continue
			}
			for _, gate := range gateViews(rs.index.DirectivesOf(id), fr.schema) {
				err := rs.invoke(fr, invocation{
					subject: id, pos: decl.Position(), gate: gate, value: s,
				})
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// dispatchFacts visits the stamped carriers of the rule's first
// gate key, resolving each to its declaration and holding it to the
// trigger's kind and the remaining predicates.
func (rs *runState) dispatchFacts(fr *flatRule) error {
	for id := range rs.index.ByFactKey(fr.preds[0].id) {
		s, held := rs.index.Lookup(id)
		if !held || s.Kind() != fr.kind {
			continue
		}
		if !rs.admits(fr, id) {
			continue
		}
		err := rs.invoke(fr, invocation{subject: id, pos: s.Position(), value: s})
		if err != nil {
			return err
		}
	}
	return nil
}

// dispatchBare visits every declaration of the rule's kind in
// scope: the one honest full-price path.
func (rs *runState) dispatchBare(fr *flatRule) error {
	for s := range rs.index.ByKind(fr.kind) {
		decl, names := s.(node.Declaration)
		if !names {
			continue
		}
		id := decl.Identity()
		if !rs.admits(fr, id) {
			continue
		}
		err := rs.invoke(fr, invocation{subject: id, pos: decl.Position(), value: s})
		if err != nil {
			return err
		}
	}
	return nil
}

// admits evaluates skip and the predicates for one subject,
// untracked, because gate evaluation is routing. A directive-gated
// rule is exempt from skip: its subject opted in explicitly and
// withdraws by deleting the directive.
func (rs *runState) admits(fr *flatRule, subject symbol.Identity) bool {
	if fr.schema == nil && rs.index.Skipped(subject, rs.plugin) {
		return false
	}
	for _, p := range fr.preds {
		if !p.test(rs.facts, subject) {
			return false
		}
	}
	return true
}

// invoke fires one handler with the invocation's sequence assigned
// in canonical match order.
func (rs *runState) invoke(fr *flatRule, inv invocation) error {
	inv.rs = rs
	inv.fr = fr
	inv.seq = rs.seq
	rs.seq++
	return fr.invoke(inv)
}

// positionOf resolves a subject's position for reporting, zero when
// the scope does not hold it.
func (rs *runState) positionOf(id symbol.Identity) position.Pos {
	s, held := rs.index.Lookup(id)
	if !held {
		return position.Pos{}
	}
	return s.Position()
}

// gateViews answers the instances of one schema on a subject, in
// source order: one invocation each, which is how a repeatable
// directive runs its handler per instance.
func gateViews(ds []directive.Directive, s *directive.Schema) []*directive.Directive {
	canonical := s.Canonical()
	var out []*directive.Directive
	for i := range ds {
		if ds[i].Name == canonical {
			out = append(out, &ds[i])
		}
	}
	return out
}

// spellingsOf answers the spellings a schema's carriers may be
// indexed under: the canonical one, and the bare one where they
// differ.
func spellingsOf(s *directive.Schema) []directive.Name {
	canonical := s.Canonical()
	if canonical == s.Name {
		return []directive.Name{canonical}
	}
	return []directive.Name{canonical, s.Name}
}
