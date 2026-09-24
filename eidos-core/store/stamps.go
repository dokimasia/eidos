// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"iter"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// AttachStamps records raw classification stamps on a subject: the
// second raw attachment class, carried beside directives from the
// load to the phase holding the registry.
//
// It is safe to call concurrently, refused after [Graph.Freeze]
// under the frozen write code, and refuses a zero subject and an
// empty attachment as the defects they are. The stamps sort at the
// seal by position, then key, origin and value, so two concurrent
// attachments produce one order, and the claim sequence the run
// replays them under is the same on every run.
func (g *Graph) AttachStamps(subject symbol.Identity, ss []meta.RawStamp) error {
	return attach(g, &g.stamps, subject, ss, "stamps")
}

// StampsOf returns a subject's raw stamps, untracked, in the seal's
// order, and nothing before [Graph.Freeze]: the order is fixed at
// the seal, so an earlier read would return a partial, unordered
// result. It is the apply step's read.
//
// The returned slice is the graph's own storage; do not mutate it.
func (g *Graph) StampsOf(id symbol.Identity) []meta.RawStamp {
	return g.stamps.of(id)
}

// Stamps enumerates every subject holding stamps, with its stamps,
// in identity order: what the run replays into the fact store at
// plugin authority.
func (g *Graph) Stamps() iter.Seq2[symbol.Identity, []meta.RawStamp] {
	return g.stamps.all()
}
