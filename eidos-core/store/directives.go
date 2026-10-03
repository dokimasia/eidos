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
// It is safe to call concurrently. The instances sort at the seal by
// position, then name, then arguments, so two concurrent attachments
// produce one order.
//
// Error modes: a [RefusedError] under [FrozenWrite] after
// [Graph.Freeze] and over a graph [Sealed] returned. A zero subject and
// an empty attachment return a plain error, because only a defect
// produces either.
func (g *Graph) AttachDirectives(subject symbol.Identity, ds []directive.Raw) error {
	return attach(g, &g.directives, subject, ds, "directives")
}

// ByDirective enumerates the declarations carrying a spelling,
// untracked, in identity order. Every indexed subject is a declaration
// the graph contains: a subject the graph does not contain fails
// validation as dangling and indexes nowhere. The index builds at
// [Graph.Freeze] in the same pass as the kind index, and a sealed graph
// builds it over the regions whose summary lists the spelling. It is
// keyed by the name as written: the store has no registry, so the
// dispatcher, which has one, queries each spelling a schema recognises.
func (g *Graph) ByDirective(n directive.Name) iter.Seq[symbol.Symbol] {
	return func(yield func(symbol.Symbol) bool) {
		for _, decl := range g.carriers(n) {
			if !yield(decl) {
				return
			}
		}
	}
}

// DirectivesOf returns a subject's raw instances, untracked, in the
// seal's order, and nothing before [Graph.Freeze]: the order is fixed
// at the seal, so an earlier read would return a partial, unordered
// result. A sealed graph decodes the regions of the subject's package.
// It is the validator's read. No tracked equivalent exists: a plugin
// never reads another plugin's annotations, so the reader cannot return
// them.
//
// The returned slice is the graph's own storage and must not be
// mutated.
func (g *Graph) DirectivesOf(id symbol.Identity) []directive.Raw {
	if g.lazy != nil {
		return g.lazy.directivesOf(id)
	}
	return g.directives.of(id)
}

// Directives enumerates every subject that has instances, with its
// instances, in identity order: the validator's walk, dangling subjects
// included. Over a sealed graph it decodes every region.
func (g *Graph) Directives() iter.Seq2[symbol.Identity, []directive.Raw] {
	return func(yield func(symbol.Identity, []directive.Raw) bool) {
		if g.lazy != nil {
			g.lazy.eachDirected(yield)
			return
		}
		g.directives.each(yield)
	}
}

// carriers returns the declarations carrying a spelling, in identity
// order. The slice is the graph's own storage.
func (g *Graph) carriers(n directive.Name) []node.Declaration {
	if g.lazy != nil {
		return g.lazy.carriers(n)
	}
	return g.byDirective[n]
}

// freezeDirectives seals the attachments and builds the directive
// index over the declarations the graph contains. The caller runs it
// under the seal.
func (g *Graph) freezeDirectives() {
	g.directives.seal(compareRaw)
	g.byDirective = map[directive.Name][]node.Declaration{}
	for _, id := range g.directives.order {
		raws := g.directives.sealed[id]
		decl, held := g.byID[id]
		if !held {
			// A dangling subject remains walkable for the validator
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
