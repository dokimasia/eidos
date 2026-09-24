// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"iter"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// AttachDirectives records raw instances on a subject.
//
// It is safe to call concurrently, refused after [Graph.Freeze]
// under the frozen write code, and refuses a zero subject and an
// empty attachment as the defects they are. The instances sort at
// the seal by position, then name, then arguments, so two concurrent
// attachments produce one order.
func (g *Graph) AttachDirectives(subject symbol.Identity, ds []directive.Raw) error {
	return attach(g, &g.directives, subject, ds, "directives")
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
// the seal's order, and nothing before [Graph.Freeze]: the order is
// fixed at the seal, so an earlier read would return a partial,
// unordered result. It is the validator's read. No tracked
// equivalent exists: a plugin never reads another plugin's
// annotations, so the reader deliberately cannot return them.
//
// The returned slice is the graph's own storage; do not mutate it.
func (g *Graph) DirectivesOf(id symbol.Identity) []directive.Raw {
	return g.directives.of(id)
}

// Directives enumerates every subject holding instances, with its
// instances, in identity order: the validator's walk, dangling
// subjects included.
func (g *Graph) Directives() iter.Seq2[symbol.Identity, []directive.Raw] {
	return g.directives.all()
}

// freezeDirectives seals the attachments and builds the directive
// index, held declarations alone. The caller holds the seal.
func (g *Graph) freezeDirectives() {
	g.directives.seal(compareRaw)
	g.byDirective = map[directive.Name][]node.Declaration{}
	for id, raws := range g.directives.all() {
		decl, held := g.byID[id]
		if !held {
			// A dangling subject stays walkable for the validator
			// and indexes nowhere: it is no dispatch match.
			continue
		}
		seen := map[directive.Name]struct{}{}
		for _, raw := range raws {
			if _, twice := seen[raw.Name]; twice {
				continue
			}
			seen[raw.Name] = struct{}{}
			g.byDirective[raw.Name] = append(g.byDirective[raw.Name], decl)
		}
	}
}
