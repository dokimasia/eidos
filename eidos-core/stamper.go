// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// Stamper is the annotator effect: a write handle bound to its
// match's subject. A stamper writes only to its subject's bag and
// the bags of the declarations the subject declares, its parameters,
// returns and type parameters: a fact "about" a sibling is a fact
// on the subject whose value names the sibling, so every write's
// target is statically known from the trigger, which is what keeps
// invalidation edges and audit output analyzable.
type Stamper struct {
	m *match
}

// stamperInto wires the invocation's stamper handle into the
// match's own allocation.
func stamperInto(m *match) *Stamper {
	m.st = Stamper{m: m}
	return &m.st
}

// Stamp records v under k with the envelope pre-bound: plugin
// authority, the phase call's bucket and plugin, the invocation's
// place in canonical match order, meaning the rule, the subject and the
// gating instance, and the invocation's point reads so far as the
// claim's derivation. A write the fact store refuses reports an Error
// at the subject's position under [RefusedStamp], and the phase
// continues.
//
// # Allocation contract
//
// Stamp allocates what the fact store keeps for the claim, and the
// invocation's derivation where the read set grew since the last
// stamp. A second key on a subject with a claim allocates three times:
// the subject's map of keys with its first group, and the claim's
// state. A first claim on a subject allocates its bag, its boxed
// identity and an entry of the store's map of subjects, whose trie
// grows by nodes the subjects' hashes decide. The key's index grows as
// it gains members.
func Stamp[T meta.FactValue](st *Stamper, k meta.Key[T], v T) {
	stamp(st.m, st.m.subject, k, v)
}

// StampOn records v under k on a declaration the subject declares: one
// of its parameters, returns or type parameters, which no trigger
// matches on their own where a directive on the subject states
// something about them. The envelope is the subject's. An identity
// the subject does not declare is refused under [RefusedStamp] at the
// subject's position, and the phase continues. StampOn allocates as
// [Stamp] does, and a refusal allocates the finding's message.
func StampOn[T meta.FactValue](st *Stamper, owned symbol.Identity, k meta.Key[T], v T) {
	m := st.m
	if !owns(m.subject, owned) {
		m.rs.reportf(m.seq, RefusedStamp, diag.SeverityError, m.pos,
			"%s does not declare %s: a stamper writes to its subject and what the subject declares",
			m.subject, owned)
		return
	}
	stamp(m, owned, k, v)
}

// stamp records v under k on target with the invocation's
// envelope, and reports a refusal under [RefusedStamp] at the
// subject's position. A call that journals notes each accepted claim
// in the invocation's record.
func stamp[T meta.FactValue](m *match, target symbol.Identity, k meta.Key[T], v T) {
	err := meta.Stamp(m.rs.facts, k, v, meta.Claim{
		Subject:   target,
		Authority: meta.AuthorityPlugin,
		Bucket:    m.rs.bucket,
		Plugin:    m.rs.plugin,
		Order:     m.order(),
		Derived:   m.derived(),
	})
	if err != nil {
		m.rs.reportf(m.seq, RefusedStamp, diag.SeverityError, m.pos, "%v", err)
		return
	}
	if m.rs.journal != nil {
		m.rs.noteClaim(meta.FactRef{Subject: target, Key: k.Name()})
	}
}

// order returns the invocation's place in canonical match order: the
// rule, the subject, and the gating instance, zero without one.
func (m *match) order() meta.Order {
	o := meta.Order{Rule: m.rule, Subject: m.subject}
	if m.gate != nil {
		o.Instance = m.gate.Instance
	}
	return o
}

// owns reports whether owned is a parameter, a return or a type
// parameter the subject declares: the same package, the subject's
// own chain as owner, and the subject's discriminator.
func owns(subject, owned symbol.Identity) bool {
	switch owned.Kind {
	case symbol.KindParam, symbol.KindReturn, symbol.KindTypeParam:
	default:
		return false
	}
	chain := subject.Name
	if subject.Owner != "" {
		chain = subject.Owner + "." + subject.Name
	}
	return owned.Lang == subject.Lang && owned.Package == subject.Package &&
		owned.Owner == chain && owned.Disc == subject.Disc
}
