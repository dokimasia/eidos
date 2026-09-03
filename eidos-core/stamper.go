// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos

import (
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// Stamper is the annotator effect: a write handle bound to its
// match's subject. A stamper writes only to its subject's bag and
// the bags of the declarations the subject owns, its parameters,
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
// sequence in canonical match order, and the invocation's point
// reads so far as the claim's derivation. A write the fact store
// refuses reports an Error at the subject's position under
// [RefusedStamp], and the phase continues.
func Stamp[T meta.FactValue](st *Stamper, k meta.Key[T], v T) {
	m := st.m
	err := meta.Stamp(m.rs.facts, k, v, meta.Claim{
		Subject:   m.subject,
		Authority: meta.AuthorityPlugin,
		Bucket:    m.rs.bucket,
		Plugin:    m.rs.plugin,
		Seq:       m.seq,
		Derived:   m.derived(),
	})
	if err != nil {
		m.rs.sink.Errorf(RefusedStamp, m.pos, m.rs.plugin, "%v", err)
	}
}

// StampOn records v under k on a declaration the subject owns: one
// of its parameters, returns or type parameters, which no trigger
// matches on their own where a directive on the subject states
// something about them. The envelope is the subject's. An identity
// the subject does not own is refused under [RefusedStamp] at the
// subject's position, and the phase continues.
func StampOn[T meta.FactValue](st *Stamper, owned symbol.Identity, k meta.Key[T], v T) {
	m := st.m
	if !owns(m.subject, owned) {
		m.rs.sink.Errorf(RefusedStamp, m.pos, m.rs.plugin,
			"%s does not own %s: a stamper writes to its subject and what the subject declares",
			m.subject, owned)
		return
	}
	err := meta.Stamp(m.rs.facts, k, v, meta.Claim{
		Subject:   owned,
		Authority: meta.AuthorityPlugin,
		Bucket:    m.rs.bucket,
		Plugin:    m.rs.plugin,
		Seq:       m.seq,
		Derived:   m.derived(),
	})
	if err != nil {
		m.rs.sink.Errorf(RefusedStamp, m.pos, m.rs.plugin, "%v", err)
	}
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
