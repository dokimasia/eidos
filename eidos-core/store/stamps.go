// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"iter"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// AttachStamps records raw classification stamps on a subject: the
// second raw attachment class, passed beside directives from the load
// to the phase that has the registry.
//
// It is safe to call concurrently. The stamps sort at the seal by
// position, then key, origin and value, so two concurrent attachments
// produce one order, and the run applies them in the same order on
// every run.
//
// Error modes: a [RefusedError] under [FrozenWrite] after
// [Graph.Freeze] and over a graph [Sealed] returned. A zero subject and
// an empty attachment return a plain error, because only a defect
// produces either.
func (g *Graph) AttachStamps(subject symbol.Identity, ss []meta.RawStamp) error {
	return attach(g, &g.stamps, subject, ss, "stamps")
}

// StampsOf returns a subject's raw stamps, untracked, in the seal's
// order, and nothing before [Graph.Freeze]: the order is fixed at the
// seal, so an earlier read would return a partial, unordered result. A
// sealed graph decodes the regions of the subject's package. It is the
// apply step's read.
//
// The returned slice is the graph's own storage and must not be
// mutated.
func (g *Graph) StampsOf(id symbol.Identity) []meta.RawStamp {
	if g.lazy != nil {
		return g.lazy.stampsOf(id)
	}
	return g.stamps.of(id)
}

// Stamps enumerates every subject that has stamps, with its stamps, in
// identity order: what the run applies to the fact store at plugin
// authority. Over a sealed graph it decodes every region.
func (g *Graph) Stamps() iter.Seq2[symbol.Identity, []meta.RawStamp] {
	return func(yield func(symbol.Identity, []meta.RawStamp) bool) {
		if g.lazy != nil {
			g.lazy.eachStamped(yield)
			return
		}
		g.stamps.each(yield)
	}
}
