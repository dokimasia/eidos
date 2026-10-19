// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"fmt"
	"slices"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// match is the base of every Match type. It gives a handler its place in
// the run without confining it. One match is one invocation, so its read
// set and its sequence number belong to the invocation.
//
// A match is valid for the duration of its handler call. The lane reuses
// it for the next invocation of the rule. After an unbind, the lane
// reuses it for a rule of the same match type in its next phase call, so
// an invocation allocates nothing in steady state. Retaining a match, an
// effect handle, an [Out] or a [SlotView] past the call is a defect, as
// state on the plugin struct is.
type match struct {
	rs  *runState
	seq int
	// rule is the index of the rule in the declaration order of its
	// plugin. It is the first key of the order of a claim.
	rule int
	// fr and value are the rule and the matched value, and nil for a
	// graph match. The buffered touch of an accessor call derives the
	// subject from them again.
	fr      *flatRule
	value   symbol.Symbol
	subject symbol.Identity
	pos     position.Pos
	gate    *directive.Directive
	// host is the emit value that an [OnEmit] invocation matched, and nil
	// for every other trigger. A slot append adds the plugin to the
	// contributors of the unit of host.
	host   symbol.Symbol
	reads  *store.ReadSet
	reader *store.Reader
	// bound is the binding of the rules of the language of the subject
	// over the view of the invocation. The match creates it on first use,
	// so a handler that never projects allocates no memo. binds reports
	// whether bound exists.
	bound rules.Bound
	binds bool
	// resolve returns the rules of another language to the walks of a
	// binding. It reads the sequence and the position of the match when
	// a walk calls it, so the match makes it on its first binding and
	// keeps it across the invocations that reuse the match.
	resolve func(symbol.Lang) rules.SourceRules
	// override reads the policy overrides of the translation that runs on
	// the match. translating is the owner of that translation, and zero
	// outside a translation. The match makes override on its first
	// translation and keeps it, as it keeps resolve.
	override    func(plugin.PolicyKey) (plugin.Choice, bool)
	translating symbol.Identity
	// derivation is the snapshot that derived returned last. It is
	// current while the read set has derivedAt edges. The set only grows
	// within an invocation, so an unchanged size means unchanged reads.
	// Every claim stamped between two reads shares the one slice, and
	// nothing writes to it.
	derivation []meta.Read
	derivedAt  int
	// em and st are the effect handles of the invocation. They are
	// stored in the allocation of the match, so an invocation costs one
	// heap object and not two.
	em Emitter
	st Stamper
}

// newMatch returns the match of one invocation.
func newMatch(inv invocation) match {
	m := match{
		rs:      inv.rs,
		seq:     inv.seq,
		fr:      inv.fr,
		value:   inv.value,
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
	// The read set of the lane serves every invocation on the lane in
	// turn. It resets here and keeps its storage, so the derivation of
	// this invocation contains no read of the previous one. A lane whose
	// invocations never read allocates nothing.
	if reads := inv.rs.reads; reads != nil && reads.Len() > 0 {
		reads.Reset()
	}
	return m
}

// bindMatch returns the reusable match of type M of the rule, with its
// base bound to this invocation. The invoke of every trigger runs this
// sequence. On the first invocation of the rule in the call it takes a
// match of type M that an earlier call of the lane released, and it
// allocates one only where the lane keeps none. The bound base keeps the
// resolver of other languages and the reader of policy overrides of the
// match.
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
	resolve, override := b.resolve, b.override
	*b = newMatch(inv)
	b.resolve, b.override = resolve, override
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

// Reader returns the tracked reader of the invocation. It is the reader of
// the lane over the read set of the lane. The lane creates it on the first
// tracked read of the call, so a handler that never reads allocates no
// tracking. A handler that needs a sibling declaration looks it up through
// the reader, and does not need a graph-wide rule for an ordinary lookup.
// The lane allocates the reader once per phase call, and a later call
// allocates nothing.
func (m *match) Reader() *store.Reader {
	if m.reader == nil {
		m.reader = m.rs.readerOf(m.readset())
	}
	return m.reader
}

// Lang returns the language of the subject. A graph match has no subject
// and returns the zero language. It allocates nothing.
func (m *match) Lang() symbol.Lang { return m.subject.Lang }

// Rules returns the walks of the kernel bound to the language of the
// subject over the view of the invocation. It creates the binding on
// first use and reuses it for the invocation, so a reference folds once
// however many rules read it. Every read of the walks records into the
// read set of the invocation, as a read of the handler does. On a graph
// match it binds the absent rules.
//
// # Allocation contract
//
// The first call of an invocation allocates the memo of the binding, one
// allocation, and the tracked reader of the lane on the first tracked
// read of the call. A later call of the invocation allocates nothing.
func (m *match) Rules() rules.Bound {
	if !m.binds {
		m.bound, m.binds = m.bind(m.subject.Lang), true
	}
	return m.bound
}

// RulesFor returns the walks bound to the rules of another language over
// the same view. A handler asks for it to project a contributor or a
// target that another language declares. Each call creates a binding of
// its own, and allocates its memo, one allocation.
func (m *match) RulesFor(lang symbol.Lang) rules.Bound { return m.bind(lang) }

// rulesFor returns the registered rules for a language, and the absent
// rules for a language without registered rules. It then warns once per
// phase call and language under [rules.AbsentRules], from the first
// invocation in canonical match order that bound them. The zero
// language of a graph match binds the absent rules without a finding,
// because nothing was declared in it.
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
// origin of the plugin.
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

// Directive returns the gating instance, and nil for bare and
// fact-gated matches. Under a repeatable schema the handler runs once
// per instance and each match has its one instance, so the accessor is
// singular. The instance is the storage of the validated table, so a
// handler does not mutate it. Directive allocates nothing.
func (m *match) Directive() *directive.Directive { return m.gate }

// ErrorfAt reports at Error severity at the position at. A handler
// passes a position of its own, such as the carrier line of a directive,
// for a finding about what an author wrote there. A graph match has no
// subject, so it reports through the positioned methods alone. The
// origin of the plugin is bound. It allocates as [StructMatch.Errorf]
// states.
func (m *match) ErrorfAt(c diag.Code, at position.Pos, format string, a ...any) {
	m.rs.reportf(m.seq, c, diag.SeverityError, at, format, a...)
}

// WarnfAt reports at Warning severity at the position at. A warning
// never fails a run. It allocates as [StructMatch.Errorf] states.
func (m *match) WarnfAt(c diag.Code, at position.Pos, format string, a ...any) {
	m.rs.reportf(m.seq, c, diag.SeverityWarning, at, format, a...)
}

// InfofAt reports at Info severity at the position at. It allocates as
// [StructMatch.Errorf] states.
func (m *match) InfofAt(c diag.Code, at position.Pos, format string, a ...any) {
	m.rs.reportf(m.seq, c, diag.SeverityInfo, at, format, a...)
}

// Kernel returns the registered keys of the kernel, for a handler that
// reads or stamps the facts of the kernel, such as the module identity,
// an authored sample or a witness. It returns the zero value where the
// phase call has none, and allocates nothing.
func (m *match) Kernel() meta.KernelKeys { return m.rs.kernel }

// Export returns the export of a plan that the plan of the handler
// depends on. It returns false for any other plan and in the phase call
// of an annotator, which runs before any plan. Every dependent of the
// plan reads the same value, so a handler does not mutate it. The read
// records nothing in the read set of the invocation. A call that
// journals notes the plan in the record of the invocation, and a call
// that does not journal allocates nothing.
func (m *match) Export(plan string) (plugin.ExportDoc, bool) {
	doc, held := m.rs.exports[plan]
	if held && m.rs.journal != nil {
		m.rs.noteExport(plan)
	}
	return doc, held
}

// bind returns one binding over the view of the invocation, with the
// resolver of other languages of the match. The view contains the
// resolver too, so a binding that a rules implementation makes through
// the view resolves the same rules. The match makes the resolver on its
// first binding.
func (m *match) bind(lang symbol.Lang) rules.Bound {
	if m.resolve == nil {
		m.resolve = func(other symbol.Lang) rules.SourceRules { return m.rs.rulesFor(other, m.seq, m.pos) }
	}
	view := rules.NewView(m.Reader(), m.rs.facts, m.readset(), m.rs.kernel).WithRules(m.resolve)
	return rules.NewBound(m.rs.rulesFor(lang, m.seq, m.pos), view, m.resolve)
}

// overrides returns the reader of policy overrides of the match, which
// the match makes on its first translation. For a key, the reader reads
// the value of the key at directive authority on the owner of the
// running translation, then on the subject of the match, and then on the
// package of the owner. It returns the first value that it finds as the
// choice. It records each read into the read set of the invocation, a
// miss included. It reads an identity that it read before, or the zero
// identity, once at most.
func (m *match) overrides() func(plugin.PolicyKey) (plugin.Choice, bool) {
	if m.override != nil {
		return m.override
	}
	m.override = func(k plugin.PolicyKey) (plugin.Choice, bool) {
		name := meta.KeyName(k)
		key := meta.Named[string](name)
		reads := m.readset()
		on := [3]symbol.Identity{m.translating, m.subject}
		if !m.translating.IsZero() {
			on[2] = m.translating.PackageIdentity()
		}
		for i, id := range on {
			if id.IsZero() || slices.Contains(on[:i], id) {
				continue
			}
			reads.RecordFact(id, name)
			if v, held := meta.GetAtLeast(m.rs.facts, id, key, meta.AuthorityDirective); held {
				return plugin.Choice(v), true
			}
		}
		return "", false
	}
	return m.override
}

// readset returns the read set of the invocation, which is the read set
// of the lane. It notes the set on the lane on first use, so a call that
// journals logs what the invocation read when it returns.
func (m *match) readset() *store.ReadSet {
	if m.reads == nil {
		m.reads = m.rs.readSet()
		m.rs.reading = m.reads
	}
	return m.reads
}

// derived returns the point reads of the invocation so far, in the order
// of [store.ReadSet.AppendPointReads]. A claim records them as its
// derivation. It collects and sorts the set only after the set grew, so
// a handler that stamps many facts after its reads sorts them once. Each
// derivation allocates one slice, sized to the point reads of the set,
// because the claims stamped with it keep it.
func (m *match) derived() []meta.Read {
	if m.reads == nil || m.reads.Len() == m.derivedAt {
		return m.derivation
	}
	m.derivation, m.derivedAt = m.reads.AppendPointReads(nil), m.reads.Len()
	return m.derivation
}

// base returns the embedded surface. The method is unexported, so it
// closes [Matcher].
func (m *match) base() *match { return m }

// subjectMatch is the base of every match with a subject. It adds the
// reports at the position of the subject. [GraphMatch] embeds the plain
// match, because it has no subject.
type subjectMatch struct{ match }

// Errorf reports at Error severity, which fails the run. The origin of
// the plugin and the position of the subject are bound. The sink
// receives every finding of a match in canonical match order when the
// rules of the phase call have run.
//
// # Allocation contract
//
// Each reporting method allocates the message of the finding and what
// the formatting of its arguments allocates. The sink keeps the message.
// The finding buffers in the pooled storage of the phase call, and the
// list of findings of the sink grows when the findings apply.
func (m *subjectMatch) Errorf(c diag.Code, format string, a ...any) {
	m.rs.reportf(m.seq, c, diag.SeverityError, m.pos, format, a...)
}

// Warnf reports at Warning severity, with the origin and the position
// bound as [StructMatch.Errorf] binds them. A warning never fails a run.
// It allocates as [StructMatch.Errorf] states.
func (m *subjectMatch) Warnf(c diag.Code, format string, a ...any) {
	m.rs.reportf(m.seq, c, diag.SeverityWarning, m.pos, format, a...)
}

// Infof reports at Info severity, with the origin and the position
// bound. An info finding states provenance or progress, and never a
// verdict. It allocates as [StructMatch.Errorf] states.
func (m *subjectMatch) Infof(c diag.Code, format string, a ...any) {
	m.rs.reportf(m.seq, c, diag.SeverityInfo, m.pos, format, a...)
}

// Matcher is the closed set of match types. Only the matches of this
// package satisfy it, because the base surface is unexported.
type Matcher interface {
	base() *match
	// unbind zeroes the match for the next phase call of its lane, so a
	// released match retains nothing of the run. It returns the key under
	// which the lane keeps released matches of its type.
	unbind() any
}

// Reporter is the part of a match with a subject that reports findings:
// at the position of the subject, and at a position that the handler
// passes. Every match of a declaration satisfies it, and so do
// [EmitMatch] and [PackageMatch]. A [GraphMatch] has no subject, so it
// reports at a position alone and does not satisfy it.
type Reporter interface {
	Errorf(c diag.Code, format string, a ...any)
	Warnf(c diag.Code, format string, a ...any)
	Infof(c diag.Code, format string, a ...any)
	ErrorfAt(c diag.Code, at position.Pos, format string, a ...any)
	WarnfAt(c diag.Code, at position.Pos, format string, a ...any)
	InfofAt(c diag.Code, at position.Pos, format string, a ...any)
}

// SubjectMatch is the part that every match with a subject has: the
// language and the rules of the subject, the reader, the gating
// directive, the keys of the kernel, the exports of the plans that the
// plan depends on, and the reports of [Reporter]. A function that two
// handlers of different kinds share takes a SubjectMatch, such as one
// that writes the double of an interface and of a struct. Every match of
// a declaration satisfies it, and so do [EmitMatch] and [PackageMatch].
// The interface is closed, because [Matcher] is.
type SubjectMatch interface {
	Matcher
	Reporter
	Lang() symbol.Lang
	Rules() rules.Bound
	RulesFor(lang symbol.Lang) rules.Bound
	Reader() *store.Reader
	Directive() *directive.Directive
	Kernel() meta.KernelKeys
	Export(plan string) (plugin.ExportDoc, bool)
}

// Fact returns the value of the first-ranked claim of k on the subject,
// and records the read at (subject, key) into the read set of the
// invocation, a miss included. On an emit match the subject is the
// origin. A graph match has no subject, so Fact returns false and
// records nothing. The read set of the lane keeps its storage across
// invocations, so a read allocates nothing once the set has grown.
func Fact[T meta.FactValue](m Matcher, k meta.Key[T]) (T, bool) {
	b := m.base()
	if b.subject.IsZero() {
		var zero T
		return zero, false
	}
	return meta.Fact(b.rs.facts, b.readset(), b.subject, k)
}

// FactOf returns the value of the first-ranked claim of k on another
// declaration, and records the read as [Fact] does. A read of the stamped
// facts of a sibling is the sanctioned channel between plugins. A zero
// identity returns false and records nothing. It allocates as [Fact]
// does.
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
