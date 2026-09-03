// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// match is the base every Match embeds: the surface that positions
// a handler without confining it. One match is one invocation, so
// its read set and its sequence number are the invocation's own.
//
// A match is valid for the duration of its handler call and reused
// for the rule's next invocation, which is what holds an
// invocation at zero steady-state allocations. Retaining a match,
// an effect handle or an [Out] past the call is a defect, the same
// rule that forbids state on the plugin struct.
type match struct {
	rs      *runState
	seq     int
	subject symbol.Identity
	pos     position.Pos
	gate    *directive.Directive
	reads   *store.ReadSet
	reader  *store.Reader
	// bound is the subject language's rules over the invocation's
	// view, minted on first use, so a handler that never projects
	// costs no memo.
	bound *rules.Bound
	// em and st are the invocation's effect handles, held in the
	// match's own allocation so an invocation costs one heap
	// object, not two.
	em Emitter
	st Stamper
}

// newMatch binds one invocation's surface.
func newMatch(inv invocation) match {
	m := match{
		rs:      inv.rs,
		seq:     inv.seq,
		subject: inv.subject,
		pos:     inv.pos,
		gate:    inv.gate,
	}
	// The previous invocation's read set and reader recycle off the
	// rule's scratch: the set resets so no read leaks into this
	// invocation's derivation, the storage stays, and the reader
	// stays valid because it binds the same index and the same set.
	// A rule that never reads keeps costing nothing.
	if prev, held := inv.scratch().(Matcher); held {
		if b := prev.base(); b.reads != nil {
			b.reads.Reset()
			m.reads = b.reads
			m.reader = b.reader
		}
	}
	return m
}

// Reader returns the invocation's tracked read handle, minted on
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

// Lang returns the subject's language, and the zero language on a
// graph match, which has no subject.
func (m *match) Lang() symbol.Lang { return m.subject.Lang }

// Rules returns the kernel's walks bound to the subject's language
// over the invocation's view, minted on first use and reused for
// the invocation, so a reference folds once however many rules
// read it. Every read the walks make records into the invocation's
// read set like a handler's own. On a graph match it binds the
// absent rules.
func (m *match) Rules() rules.Bound {
	if m.bound == nil {
		b := m.bind(m.subject.Lang)
		m.bound = &b
	}
	return *m.bound
}

// RulesFor returns the walks bound to another language's rules
// over the same view: what a handler projecting a contributor or
// a target declared elsewhere asks for.
func (m *match) RulesFor(lang symbol.Lang) rules.Bound { return m.bind(lang) }

// rulesFor returns the registered rules for a language, and the
// absent rules for one the composition registered none for,
// warning once per phase call and language under
// [rules.AbsentRules]. The zero language, a graph match's, binds
// the absent rules without a finding: nothing was declared in it.
func (rs *runState) rulesFor(lang symbol.Lang, at position.Pos) rules.SourceRules {
	if rs.rules != nil {
		if src, held := rs.rules.For(lang); held {
			return src
		}
	}
	if lang != "" && !rs.warned[lang] {
		if rs.warned == nil {
			rs.warned = map[symbol.Lang]bool{}
		}
		rs.warned[lang] = true
		rs.sink.Warnf(rules.AbsentRules, at, rs.plugin,
			"no rules are registered for %s: its walks run under the absent rules", lang)
	}
	return rules.Absent(lang)
}

// Directive returns the gating instance, nil for bare and
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

// ErrorfAt reports at Error severity at a position of the
// handler's own: a directive's carrier line rather than the
// subject's, for a finding about what an author wrote there. The
// origin stays pre-bound.
func (m *match) ErrorfAt(at position.Pos, c diag.Code, format string, a ...any) {
	m.rs.sink.Errorf(c, at, m.rs.plugin, format, a...)
}

// Kernel returns the kernel's registered keys, for a handler that
// reads or stamps the kernel's own facts: the module identity, an
// authored sample, a witness. The zero value arrives where the
// phase call carried none.
func (m *match) Kernel() meta.KernelKeys { return m.rs.kernel }

// bind mints one binding over the invocation's view.
func (m *match) bind(lang symbol.Lang) rules.Bound {
	view := rules.View{
		Decls: m.Reader(), Facts: m.rs.facts, Reads: m.readset(), Kernel: m.rs.kernel,
	}
	return rules.NewBound(m.rs.rulesFor(lang, m.pos), view, func(other symbol.Lang) rules.SourceRules {
		return m.rs.rulesFor(other, m.pos)
	})
}

// readset returns the invocation's read set, created on first use.
func (m *match) readset() *store.ReadSet {
	if m.reads == nil {
		m.reads = store.NewReadSet()
	}
	return m.reads
}

// derived returns the invocation's point reads so far, in the read
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

// base returns the embedded surface; it is what closes [Matcher].
func (m *match) base() *match { return m }

// Matcher is the closed set of match types: only this package's
// matches satisfy it, because the base surface is unexported.
type Matcher interface {
	base() *match
}

// Fact returns the subject's winning value for k, recording the
// read at (subject, key) into the invocation's read set; a miss
// records too. On an emit match the subject is the origin; on a
// graph match there is no subject, so Fact returns false and
// records nothing.
func Fact[T meta.FactValue](m Matcher, k meta.Key[T]) (T, bool) {
	b := m.base()
	if b.subject.IsZero() {
		var zero T
		return zero, false
	}
	return meta.Fact(b.rs.facts, b.readset(), b.subject, k)
}

// FactOf returns another declaration's winning value, recorded the
// same way: reading a sibling's stamped facts is the sanctioned
// channel between plugins. A zero identity returns false and
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
