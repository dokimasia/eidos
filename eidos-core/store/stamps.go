// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"iter"
	"slices"
	"sync"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// stampAttachments is one subject's raw stamps, appended under its
// own lock so two units stamping one subject do not race.
type stampAttachments struct {
	mu    sync.Mutex
	items []meta.RawStamp
}

// AttachStamps records raw classification stamps on a subject: the
// second raw attachment class, carried beside directives from the
// load to the phase holding the registry.
//
// It is safe to call concurrently, refused after [Graph.Freeze]
// under the frozen write code, and refuses a zero subject and an
// empty attachment as the defects they are. The stamps sort by
// position at the seal, so two concurrent attachments produce one
// order.
func (g *Graph) AttachStamps(subject symbol.Identity, ss []meta.RawStamp) error {
	if subject.IsZero() {
		return errors.New("store: stamps on a zero subject index nowhere")
	}
	if len(ss) == 0 {
		return errors.New("store: no stamps to attach")
	}

	g.seal.RLock()
	defer g.seal.RUnlock()

	if g.frozen {
		return &RefusedError{
			Code: FrozenWrite,
			Msg:  "stamps on " + subject.String() + " are attached after Freeze",
		}
	}
	held, ok := g.stamped.Load(subject)
	if !ok {
		held, _ = g.stamped.LoadOrStore(subject, &stampAttachments{})
	}
	entry, _ := held.(*stampAttachments)
	entry.mu.Lock()
	entry.items = append(entry.items, ss...)
	entry.mu.Unlock()
	return nil
}

// StampsOf returns a subject's raw stamps, untracked, in position
// order, and nothing before [Graph.Freeze]: the order is fixed at
// the seal, so an earlier read would return a partial, unordered
// result. It is the apply step's read.
//
// The returned slice is the graph's own storage; do not mutate it.
func (g *Graph) StampsOf(id symbol.Identity) []meta.RawStamp {
	return g.stamps[id]
}

// Stamps enumerates every subject holding stamps, with its stamps,
// in identity order: what the run replays into the fact store at
// plugin authority.
func (g *Graph) Stamps() iter.Seq2[symbol.Identity, []meta.RawStamp] {
	return func(yield func(symbol.Identity, []meta.RawStamp) bool) {
		for _, id := range g.stampOrder {
			if !yield(id, g.stamps[id]) {
				return
			}
		}
	}
}

// freezeStamps sorts every attachment by position and fixes the
// subject order. The caller holds the seal.
func (g *Graph) freezeStamps() {
	g.stamps = map[symbol.Identity][]meta.RawStamp{}

	g.stamped.Range(func(key, value any) bool {
		id, _ := key.(symbol.Identity)
		entry, _ := value.(*stampAttachments)
		items := slices.Clone(entry.items)
		slices.SortFunc(items, func(a, b meta.RawStamp) int {
			return comparePos(a.Pos, b.Pos)
		})
		g.stamps[id] = items
		return true
	})

	g.stampOrder = make([]symbol.Identity, 0, len(g.stamps))
	for id := range g.stamps {
		g.stampOrder = append(g.stampOrder, id)
	}
	slices.SortFunc(g.stampOrder, symbol.Identity.Compare)
}
