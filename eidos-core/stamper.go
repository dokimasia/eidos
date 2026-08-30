// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos

import "go.dokimi.dev/eidos/core/meta"

// Stamper is the annotator effect: a write handle bound to its
// match's subject. A stamper writes only to its subject's bag: a
// fact "about" a sibling is a fact on the subject whose value names
// the sibling, so every write's target is statically known from the
// trigger, which is what keeps invalidation edges and audit output
// analyzable.
type Stamper struct {
	m *match
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
