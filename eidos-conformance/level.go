// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance

import (
	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// projection is what the level check measured over one feature's
// declarations.
type projection struct {
	// refs counts the references reachable from the declarations,
	// and opaque how many folded to a form no projection holds.
	refs, opaque int
	// gaps counts the member-set gaps across the feature's types.
	gaps int
	// declaredOpaque reports that a declared alias's own target
	// folded to Opaque, and aliases how many aliases were declared.
	declaredOpaque, aliases int
	// unprojected names a callable the bound refused.
	unprojected []symbol.Identity
}

// AssertLevel holds one feature to the projection level its
// language declared, evaluated through the rules over the corpus
// fixture. A feature that projects worse than declared fails, and
// so does one that projects better: a capability nobody declared
// is a capability nobody tested.
func AssertLevel(tb assert.TB, c Corpus, fx *rulestest.Fixture, f Feature, verdict Verdict) {
	tb.Helper()

	if c.Rules == nil {
		tb.Errorf("%s states %s under a corpus without rules", f.ID, verdict)
		return
	}
	reads := store.NewReadSet()
	reader, err := fx.Graph.Reader(reads, nil)
	assert.NoError(tb, err, "the corpus graph hands out a reader")
	view := rules.View{Decls: reader, Facts: fx.Facts, Reads: reads, Kernel: fx.Keys}
	b := rules.NewBound(c.Rules, view, nil)
	p := measure(tb, c, fx.Graph, f, b)
	remainder := c.Remainder[f.ID]
	for _, r := range remainder {
		id := identityOf(c, f, r.Decl)
		if !stamped(fx.Graph, id, r.Key) {
			tb.Errorf("%s names %s as a remainder on %s, which carries no such stamp", f.ID, r.Key, id)
		}
	}
	whole := p.opaque == 0 && p.gaps == 0 && len(p.unprojected) == 0
	switch verdict {
	case Projects:
		if !whole {
			tb.Errorf("%s is declared to project and does not: %d of %d references fold to Opaque or "+
				"Inline, %d member gaps, %d callables refused", f.ID, p.opaque, p.refs, p.gaps, len(p.unprojected))
		}
		if len(remainder) > 0 {
			tb.Errorf("%s is declared to project and names a remainder: a whole projection leaves none", f.ID)
		}
	case ProjectsPartly:
		if whole {
			tb.Errorf("%s is declared to project partly and projects whole: "+
				"a capability nobody declared is a capability nobody tested", f.ID)
		}
	case Opaque:
		if p.aliases == 0 || p.declaredOpaque == 0 {
			tb.Errorf("%s is declared opaque and declares no alias whose own target folds to Opaque or Inline", f.ID)
		}
	default:
		tb.Errorf("%s states %s, which is no projection level", f.ID, verdict)
	}
}

// measure projects every declaration the feature declares and
// every declaration in the feature's own package, nested ones
// included, each once by identity, through the bound rules.
func measure(tb assert.TB, c Corpus, g *store.Graph, f Feature, b rules.Bound) projection {
	tb.Helper()

	var p projection
	seen := map[*node.TypeRef]bool{}
	measured := map[symbol.Identity]bool{}
	visit := func(decl node.Declaration) {
		id := decl.Identity()
		if id.IsZero() || id.Kind == symbol.KindPackage || id.Kind == symbol.KindFile || measured[id] {
			return
		}
		measured[id] = true
		node.Walk(decl, func(x symbol.Symbol) bool {
			t, is := x.(*node.TypeRef)
			if !is {
				return true
			}
			if t == nil || seen[t] {
				return false
			}
			seen[t] = true
			p.refs++
			if unprojected(b.TypeOf(t).Form) {
				p.opaque++
			}
			return true
		})
		if alias, is := decl.(*node.Alias); is && alias.Target != nil {
			p.aliases++
			if unprojected(b.TypeOf(alias.Target).Form) {
				p.declaredOpaque++
			}
		}
		if set, is := b.MembersOf(decl); is {
			p.gaps += len(set.Gaps)
		}
		switch id.Kind {
		case symbol.KindFunction, symbol.KindMethod:
			if _, ok := b.CallableOf(decl); !ok {
				p.unprojected = append(p.unprojected, id)
			}
		}
	}
	for _, d := range f.Declares {
		if sym, held := g.Lookup(identityOf(c, f, d)); held {
			for decl := range node.Declarations(sym) {
				visit(decl)
			}
		}
	}
	for pkg := range g.Packages() {
		if pkg.ID.Package != c.pkg(f.ID, "") {
			continue
		}
		for decl := range node.Declarations(pkg) {
			visit(decl)
		}
	}
	return p
}

// unprojected reports a shape no projection holds: Opaque, and
// Inline, an inline body the language has no structure for.
func unprojected(form symbol.TypeForm) bool {
	return form == symbol.FormOpaque || form == symbol.FormInline
}

// identityOf derives a declared declaration's identity through the
// corpus convention.
func identityOf(c Corpus, f Feature, d Decl) symbol.Identity {
	return symbol.Identity{
		Lang: c.Frontend.Lang(), Package: c.pkg(f.ID, d.Sub),
		Owner: d.Owner, Name: d.Name, Kind: d.Kind, Disc: d.Disc,
	}
}

// stamped reports whether the load stamped a key on a subject.
func stamped(g *store.Graph, id symbol.Identity, key meta.KeyName) bool {
	for subject, stamps := range g.Stamps() {
		if subject != id {
			continue
		}
		for _, s := range stamps {
			if s.Key == key {
				return true
			}
		}
	}
	return false
}
