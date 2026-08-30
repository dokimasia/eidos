// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos

import (
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// match is the base every Match embeds: the surface that positions
// a handler without confining it. One match is one invocation, so
// its read set and its sequence number are the invocation's own.
//
// A match is valid for the duration of its handler call and reused
// for the rule's next invocation, which is what prices an
// invocation at zero steady-state allocations. Retaining a match,
// an effect handle or an [Out] past the call is a defect, the same
// law that forbids state on the plugin struct.
type match struct {
	rs      *runState
	seq     int
	subject symbol.Identity
	pos     position.Pos
	gate    *directive.Directive
	reads   *store.ReadSet
	reader  *store.Reader
	// em and st are the invocation's effect handles, held in the
	// match's own allocation so an invocation costs one heap
	// object, not two.
	em Emitter
	st Stamper
}

// newMatch binds one invocation's surface.
func newMatch(inv invocation) match {
	return match{
		rs:      inv.rs,
		seq:     inv.seq,
		subject: inv.subject,
		pos:     inv.pos,
		gate:    inv.gate,
	}
}

// Reader answers the invocation's tracked read handle, minted on
// first use, so a handler that never reads costs no tracking. A
// handler that needs a sibling declaration looks it up like anyone
// else, instead of escaping to a graph-wide rule for an ordinary
// lookup.
func (m *match) Reader() *store.Reader {
	if m.reader == nil {
		reader, err := m.rs.index.Reader(m.readset())
		if err != nil {
			// The index refused an unfrozen graph at its own
			// construction, so a failed mint is a defect in the
			// dispatch plumbing rather than a run condition.
			panic("eidos: minting a reader over the routing surface failed: " + err.Error())
		}
		m.reader = reader
	}
	return m.reader
}

// Directive answers the gating instance, nil for bare and
// fact-gated matches. Under a repeatable schema the handler runs
// once per instance and each match carries its one instance, so
// the accessor stays singular. The instance is the validated
// table's own storage; do not mutate it.
func (m *match) Directive() *directive.Directive { return m.gate }

// Errorf reports at Error severity, which fails the run, with the
// plugin's origin and the subject's position pre-bound.
func (m *match) Errorf(c diag.Code, format string, a ...any) {
	m.rs.sink.Errorf(c, m.pos, m.rs.plugin, format, a...)
}

// Warnf reports at Warning severity, origin and position pre-bound.
// A warning never fails a run.
func (m *match) Warnf(c diag.Code, format string, a ...any) {
	m.rs.sink.Warnf(c, m.pos, m.rs.plugin, format, a...)
}

// Infof reports at Info severity, origin and position pre-bound:
// provenance and progress, never a verdict.
func (m *match) Infof(c diag.Code, format string, a ...any) {
	m.rs.sink.Infof(c, m.pos, m.rs.plugin, format, a...)
}

// readset answers the invocation's read set, created on first use.
func (m *match) readset() *store.ReadSet {
	if m.reads == nil {
		m.reads = store.NewReadSet()
	}
	return m.reads
}

// derived answers the invocation's point reads so far, in the read
// set's own order: what a claim carries as its derivation.
func (m *match) derived() []meta.Read {
	if m.reads == nil {
		return nil
	}
	var out []meta.Read
	for id := range m.reads.Identities() {
		out = append(out, meta.Read{Subject: id})
	}
	for id, key := range m.reads.Facts() {
		out = append(out, meta.Read{Subject: id, Key: key})
	}
	return out
}

// base answers the embedded surface; it is what closes [Matcher].
func (m *match) base() *match { return m }

// Matcher is the closed set of match types: only this package's
// matches satisfy it, because the base surface is unexported.
type Matcher interface {
	base() *match
}

// Fact answers the subject's winning value for k, recording the
// read at (subject, key) into the invocation's read set; a miss
// records too. On an emit match the subject is the origin; on a
// graph match there is no subject, so Fact answers false and
// records nothing.
func Fact[T meta.FactValue](m Matcher, k meta.Key[T]) (T, bool) {
	b := m.base()
	if b.subject.IsZero() {
		var zero T
		return zero, false
	}
	return meta.Fact(b.rs.facts, b.readset(), b.subject, k)
}

// FactOf answers another declaration's winning value, recorded the
// same way: reading a sibling's stamped facts is the sanctioned
// channel between plugins. A zero identity answers false and
// records nothing.
func FactOf[T meta.FactValue](
	m Matcher, id symbol.Identity, k meta.Key[T],
) (T, bool) {
	if id.IsZero() {
		var zero T
		return zero, false
	}
	b := m.base()
	return meta.Fact(b.rs.facts, b.readset(), id, k)
}
