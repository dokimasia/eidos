// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"fmt"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// match is the base every Match embeds: the surface that positions
// a handler without confining it. One match is one invocation, so
// its read set and its sequence number are the invocation's own.
//
// A match is valid for the duration of its handler call. It is reused
// for the rule's next invocation on the same lane, and, unbound, for a
// rule of the same match type in the lane's next phase call, which
// keeps an invocation at zero steady-state allocations. Retaining a
// match, an effect handle, an [Out] or a [SlotView] past the call is a
// defect, the same rule that forbids state on the plugin struct.
type match struct {
	rs  *runState
	seq int
	// rule is the rule's place in its plugin's declaration order, the
	// first step of a claim's order.
	rule    int
	subject symbol.Identity
	pos     position.Pos
	gate    *directive.Directive
	// host is the emit value an [OnEmit] invocation matched, which a
	// slot append names its plugin's contribution to, and nil for
	// every other trigger.
	host   symbol.Symbol
	reads  *store.ReadSet
	reader *store.Reader
	// bound is the subject language's rules over the invocation's
	// view, minted on first use, so a handler that never projects
	// costs no memo. binds reports whether bound is minted.
	bound rules.Bound
	binds bool
	// resolve returns another language's rules to a binding's walks. It
	// reads the match's sequence and position when a walk calls it, so
	// the match makes it on its first binding and keeps it for every
	// invocation it is reused for.
	resolve func(symbol.Lang) rules.SourceRules
	// derivation is the snapshot derived last returned, current
	// while the read set has derivedAt edges. The set only grows
	// within an invocation, so an unchanged size means unchanged
	// reads. Every claim stamped between two reads shares the one
	// slice, and nothing writes to it.
	derivation []meta.Read
	derivedAt  int
	// em and st are the invocation's effect handles, stored in the
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
	if inv.fr != nil {
		m.rule = int(inv.fr.ordinal)
		if inv.fr.phase == plugin.PhaseEmit {
			m.host = inv.value
		}
	}
	// The lane's read set serves every invocation on the lane, one after
	// another. It resets here, so no read of the previous invocation
	// leaks into this one's derivation, and it keeps its storage. A lane
	// whose invocations never read costs nothing.
	if reads := inv.rs.reads; reads != nil && reads.Len() > 0 {
		reads.Reset()
	}
	return m
}

// bindMatch returns the rule's reusable match of type M with its base
// rebound to this invocation: the one sequence every trigger's invoke
// runs. On the rule's first invocation in the call it takes a match of
// type M that the lane's earlier call released, and allocates one only
// where the lane keeps none. The rebound base keeps the match's
// resolver of other languages.
func bindMatch[M any, P interface {
	*M
	Matcher
}](inv invocation) P {
	p, reused := inv.scratch().(P)
	if !reused {
		p = spareOf[M, P](inv.rs)
		inv.keep(p)
	}
	b := p.base()
	resolve := b.resolve
	*b = newMatch(inv)
	b.resolve = resolve
	return p
}

// spareOf returns a match of type M that an earlier phase call on the
// lane released, and a new one where the lane keeps none.
func spareOf[M any, P interface {
	*M
	Matcher
}](rs *runState) P {
	key := any((*M)(nil))
	list := rs.spare[key]
	n := len(list)
	if n == 0 {
		return P(new(M))
	}
	p, _ := list[n-1].(P)
	list[n-1] = nil
	rs.spare[key] = list[:n-1]
	return p
}

// Reader returns the invocation's tracked read handle: the lane's
// handle over the lane's read set, created on the call's first tracked
// read on the lane, so a handler that never reads costs no tracking. A
// handler that needs a sibling declaration looks it up like anyone
// else, instead of escaping to a graph-wide rule for an ordinary
// lookup. The lane allocates the handle once per phase call, and a
// later call allocates nothing.
func (m *match) Reader() *store.Reader {
	if m.reader == nil {
		m.reader = m.rs.readerOf(m.readset())
	}
	return m.reader
}

// Lang returns the subject's language, and the zero language on a
// graph match, which has no subject. It allocates nothing.
func (m *match) Lang() symbol.Lang { return m.subject.Lang }

// Rules returns the kernel's walks bound to the subject's language
// over the invocation's view, created on first use and reused for
// the invocation, so a reference folds once however many rules
// read it. Every read the walks make records into the invocation's
// read set like a handler's own. On a graph match it binds the
// absent rules.
//
// # Allocation contract
//
// The first call of an invocation allocates the binding's memo, one
// allocation, and the lane's tracked reader on the call's first
// tracked read. A later call of the invocation allocates nothing.
func (m *match) Rules() rules.Bound {
	if !m.binds {
		m.bound, m.binds = m.bind(m.subject.Lang), true
	}
	return m.bound
}

// RulesFor returns the walks bound to another language's rules
// over the same view: what a handler projecting a contributor or
// a target declared elsewhere asks for. Each call creates a binding of
// its own, and allocates its memo, one allocation.
func (m *match) RulesFor(lang symbol.Lang) rules.Bound { return m.bind(lang) }

// rulesFor returns the registered rules for a language, and the
// absent rules for one the composition registered none for,
// warning once per phase call and language under
// [rules.AbsentRules], from the first invocation in canonical match
// order that bound them. The zero language, a graph match's, binds
// the absent rules without a finding: nothing was declared in it.
func (rs *runState) rulesFor(lang symbol.Lang, seq int, at position.Pos) rules.SourceRules {
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
		rs.fx.report(seq, finding{
			d: diag.Diag{
				Code:     rules.AbsentRules,
				Severity: diag.SeverityWarning,
				Pos:      at,
				Msg:      fmt.Sprintf("no rules are registered for %s: its walks run under the absent rules", lang),
				Origin:   rs.plugin,
			},
			lang: lang,
		})
	}
	return rules.Absent(lang)
}

// reportf buffers one finding of the invocation at seq, under the
// plugin's origin.
func (rs *runState) reportf(
	seq int, c diag.Code, sev diag.Severity, at position.Pos, format string, a ...any,
) {
	rs.fx.report(seq, finding{d: diag.Diag{
		Code:     c,
		Severity: sev,
		Pos:      at,
		Msg:      fmt.Sprintf(format, a...),
		Origin:   rs.plugin,
	}})
}

// Directive returns the gating instance, nil for bare and
// fact-gated matches. Under a repeatable schema the handler runs
// once per instance and each match has its one instance, so the
// accessor remains singular. The instance is the validated table's
// own storage. Do not mutate it. Directive allocates nothing.
func (m *match) Directive() *directive.Directive { return m.gate }

// Errorf reports at Error severity, which fails the run, with the
// plugin's origin and the subject's position pre-bound. Every finding
// a match reports arrives in the sink when the phase call's rules have
// run, in canonical match order.
//
// # Allocation contract
//
// Each of the reporting methods allocates the finding's message, which
// the sink keeps, and what formatting its arguments allocates. The
// finding buffers in the phase call's pooled storage, and the sink's
// list of findings grows when the findings apply.
func (m *match) Errorf(c diag.Code, format string, a ...any) {
	m.rs.reportf(m.seq, c, diag.SeverityError, m.pos, format, a...)
}

// Warnf reports at Warning severity, origin and position pre-bound.
// A warning never fails a run. It allocates as [StructMatch.Errorf]
// states.
func (m *match) Warnf(c diag.Code, format string, a ...any) {
	m.rs.reportf(m.seq, c, diag.SeverityWarning, m.pos, format, a...)
}

// Infof reports at Info severity, origin and position pre-bound:
// provenance and progress, never a verdict. It allocates as
// [StructMatch.Errorf] states.
func (m *match) Infof(c diag.Code, format string, a ...any) {
	m.rs.reportf(m.seq, c, diag.SeverityInfo, m.pos, format, a...)
}

// ErrorfAt reports at Error severity at a position of the
// handler's own: a directive's carrier line, not the subject's, for
// a finding about what an author wrote there. The code comes first,
// as in every other reporting method, and the origin remains
// pre-bound. It allocates as [StructMatch.Errorf] states.
func (m *match) ErrorfAt(c diag.Code, at position.Pos, format string, a ...any) {
	m.rs.reportf(m.seq, c, diag.SeverityError, at, format, a...)
}

// Kernel returns the kernel's registered keys, for a handler that
// reads or stamps the kernel's own facts: the module identity, an
// authored sample, a witness. It returns the zero value where the
// phase call has none, and allocates nothing.
func (m *match) Kernel() meta.KernelKeys { return m.rs.kernel }

// Export returns the export of a plan the handler's plan depends on,
// and false for any other plan and in an annotator's phase call, which
// runs before any plan. Every dependent of the plan reads the same
// value, so a handler does not mutate it. The read records nothing in
// the invocation's read set. A call that journals notes the plan in the
// invocation's record, and a call that does not allocates nothing.
func (m *match) Export(plan string) (plugin.ExportDoc, bool) {
	doc, held := m.rs.exports[plan]
	if held && m.rs.journal != nil {
		m.rs.noteExport(plan)
	}
	return doc, held
}

// bind returns one binding over the invocation's view, with the
// match's resolver of other languages, which the match makes on its
// first binding.
func (m *match) bind(lang symbol.Lang) rules.Bound {
	view := rules.View{
		Decls: m.Reader(), Facts: m.rs.facts, Reads: m.readset(), Kernel: m.rs.kernel,
	}
	if m.resolve == nil {
		m.resolve = func(other symbol.Lang) rules.SourceRules { return m.rs.rulesFor(other, m.seq, m.pos) }
	}
	return rules.NewBound(m.rs.rulesFor(lang, m.seq, m.pos), view, m.resolve)
}

// readset returns the invocation's read set, the lane's own, and notes
// it on the lane on first use, so a call that journals logs what the
// invocation read when it returns.
func (m *match) readset() *store.ReadSet {
	if m.reads == nil {
		m.reads = m.rs.readSet()
		m.rs.reading = m.reads
	}
	return m.reads
}

// derived returns the invocation's point reads so far, in the order
// [store.ReadSet.AppendPointReads] gives: what a claim records as its
// derivation. It collects and sorts the set only after the set grew, so
// a handler stamping many facts after its reads sorts them once. Each
// derivation allocates one slice, sized to the set's point reads,
// because the claims stamped with it keep it.
func (m *match) derived() []meta.Read {
	if m.reads == nil || m.reads.Len() == m.derivedAt {
		return m.derivation
	}
	m.derivation, m.derivedAt = m.reads.AppendPointReads(nil), m.reads.Len()
	return m.derivation
}

// base returns the embedded surface. Its being unexported closes
// [Matcher].
func (m *match) base() *match { return m }

// Matcher is the closed set of match types: only this package's
// matches satisfy it, because the base surface is unexported.
type Matcher interface {
	base() *match
	// unbind zeroes the match for its lane's next phase call, so a
	// released match retains nothing of the run, and returns the key
	// the lane keeps released matches of its type under.
	unbind() any
}

// Fact returns the value of k on the subject whose claim ranks first,
// recording the read at (subject, key) into the invocation's read set,
// a miss included. On an emit match the subject is the origin. A graph
// match has no subject, so Fact returns false and records nothing. The
// lane's read set keeps its storage across invocations, so a read
// allocates nothing once the set has grown.
func Fact[T meta.FactValue](m Matcher, k meta.Key[T]) (T, bool) {
	b := m.base()
	if b.subject.IsZero() {
		var zero T
		return zero, false
	}
	return meta.Fact(b.rs.facts, b.readset(), b.subject, k)
}

// FactOf returns the value of k on another declaration whose claim
// ranks first, recorded the same way: reading a sibling's stamped
// facts is the sanctioned channel between plugins. A zero identity
// returns false and records nothing. It allocates as [Fact] does.
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
