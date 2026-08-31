// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package store

import (
	"cmp"
	"errors"
	"iter"
	"slices"
	"sync"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// attachments is one subject's raw instances, appended under its
// own lock so two carriers attaching to one subject do not race.
type attachments struct {
	mu    sync.Mutex
	items []directive.Raw
}

// AttachDirectives records raw instances on a subject.
//
// It is safe to call concurrently, refused after [Graph.Freeze]
// under the frozen write code, and refuses a zero subject and an
// empty attachment as the defects they are. The instances sort by
// position at the seal, so two concurrent attachments produce one
// order.
func (g *Graph) AttachDirectives(subject symbol.Identity, ds []directive.Raw) error {
	if subject.IsZero() {
		return errors.New("store: directives on a zero subject index nowhere")
	}
	if len(ds) == 0 {
		return errors.New("store: no directives to attach")
	}

	g.seal.RLock()
	defer g.seal.RUnlock()

	if g.frozen {
		return &RefusedError{
			Code: FrozenWrite,
			Msg:  "directives on " + subject.String() + " are attached after Freeze",
		}
	}
	held, ok := g.attached.Load(subject)
	if !ok {
		held, _ = g.attached.LoadOrStore(subject, &attachments{})
	}
	entry, _ := held.(*attachments)
	entry.mu.Lock()
	entry.items = append(entry.items, ds...)
	entry.mu.Unlock()
	return nil
}

// ByDirective enumerates the declarations carrying a spelling,
// untracked, in identity order — the same yield [Graph.ByKind]
// returns, because every indexed subject is a held declaration:
// one that is not fails validation as dangling. The index builds
// at [Graph.Freeze] in the same pass as the kind index and is
// keyed by the name as written: the store holds no registry, so
// the dispatcher, which does, queries each spelling a schema
// recognises.
func (g *Graph) ByDirective(n directive.Name) iter.Seq[symbol.Symbol] {
	return func(yield func(symbol.Symbol) bool) {
		for _, decl := range g.byDirective[n] {
			if !yield(decl) {
				return
			}
		}
	}
}

// DirectivesOf returns a subject's raw instances, untracked, in
// position order, and nothing before [Graph.Freeze]: the order is
// fixed at the seal, so an earlier read would return a partial,
// unordered result. It is the validator's read. No tracked
// equivalent exists: a plugin never reads another plugin's
// annotations, so the reader deliberately cannot return them.
//
// The returned slice is the graph's own storage; do not mutate it.
func (g *Graph) DirectivesOf(id symbol.Identity) []directive.Raw {
	return g.directives[id]
}

// Directives enumerates every subject holding instances, with its
// instances, in identity order: the validator's walk, dangling
// subjects included.
func (g *Graph) Directives() iter.Seq2[symbol.Identity, []directive.Raw] {
	return func(yield func(symbol.Identity, []directive.Raw) bool) {
		for _, id := range g.directiveOrder {
			if !yield(id, g.directives[id]) {
				return
			}
		}
	}
}

// freezeDirectives sorts every attachment by position and builds
// the directive index, held declarations alone. The caller holds
// the seal.
func (g *Graph) freezeDirectives() {
	g.directives = map[symbol.Identity][]directive.Raw{}
	g.byDirective = map[directive.Name][]node.Declaration{}

	g.attached.Range(func(key, value any) bool {
		id, _ := key.(symbol.Identity)
		entry, _ := value.(*attachments)
		items := slices.Clone(entry.items)
		slices.SortFunc(items, func(a, b directive.Raw) int {
			return comparePos(a.Pos, b.Pos)
		})
		g.directives[id] = items
		return true
	})

	g.directiveOrder = make([]symbol.Identity, 0, len(g.directives))
	for id := range g.directives {
		g.directiveOrder = append(g.directiveOrder, id)
	}
	slices.SortFunc(g.directiveOrder, symbol.Identity.Compare)

	for _, id := range g.directiveOrder {
		decl, held := g.byID[id]
		if !held {
			// A dangling subject stays walkable for the validator
			// and indexes nowhere: it is no dispatch match.
			continue
		}
		seen := map[directive.Name]struct{}{}
		for _, raw := range g.directives[id] {
			if _, twice := seen[raw.Name]; twice {
				continue
			}
			seen[raw.Name] = struct{}{}
			g.byDirective[raw.Name] = append(g.byDirective[raw.Name], decl)
		}
	}
}

// comparePos orders positions by file, line then column.
func comparePos(a, b position.Pos) int {
	return cmp.Or(
		cmp.Compare(a.File, b.File),
		cmp.Compare(a.Line, b.Line),
		cmp.Compare(a.Col, b.Col),
	)
}
