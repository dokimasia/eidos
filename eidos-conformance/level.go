// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance

import (
	"fmt"
	"slices"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

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
	// opaque counts the references reachable from the declarations
	// that folded to a form no projection resolves.
	opaque int
	// gaps counts the member-set gaps across the feature's types.
	gaps int
	// declaredOpaque counts the declared aliases whose own target folded
	// to Opaque or Inline.
	declaredOpaque int
	// callables are the feature's functions and methods in measure order,
	// each with whether the bound projected it.
	callables []callable
}

// callable is one function or method the level check measured, and
// whether the bound rules projected it.
type callable struct {
	id       symbol.Identity
	projects bool
}

// AssertLevel checks one feature against the projection level its
// language declared, evaluated through the rules over the corpus
// fixture. A feature that projects worse than declared fails, and
// so does one that projects better: a capability nobody declared
// is a capability nobody tested.
//
// A verdict that is no projection level, and a corpus without rules,
// stop the check, because nothing can evaluate the level. Every other
// contract it states reports on its own, so one run names each way the
// feature misses its level: a remainder the load did not stamp, and for
// a whole projection each reference, member gap and callable that falls
// short.
func AssertLevel(tb assert.TB, c Corpus, fx *rulestest.Fixture, f Feature, verdict Verdict) {
	tb.Helper()

	assert.True(tb, verdict.projected(), fmt.Sprintf("%s states projects, projects partly or opaque", f.ID))
	assert.NotNil(tb, c.Rules, fmt.Sprintf("%s states %s under a corpus whose rules evaluate it", f.ID, verdict))
	reads := store.NewReadSet()
	reader, err := fx.Graph.Reader(reads, nil)
	assert.NoError(tb, err, "the corpus graph hands out a reader")
	view := rules.View{Decls: reader, Facts: fx.Facts, Reads: reads, Kernel: fx.Keys}
	b := rules.NewBound(c.Rules, view, nil)
	p := measure(tb, c, fx.Graph, f, b)
	remainder := c.Remainder[f.ID]
	for _, r := range remainder {
		id := identityOf(c, f, r.Decl)
		expect.Contains(tb, stampKeys(fx.Graph, id), r.Key,
			fmt.Sprintf("the load stamps %s on %s, which %s names as a remainder", r.Key, id, f.ID))
	}
	switch verdict {
	case Projects:
		expect.Equal(tb, p.opaque, 0,
			fmt.Sprintf("%s projects, so no reference it reaches folds to Opaque or Inline", f.ID))
		expect.Equal(tb, p.gaps, 0, fmt.Sprintf("%s projects, so no member set it declares has a gap", f.ID))
		for _, call := range p.callables {
			expect.True(tb, call.projects, fmt.Sprintf("%s projects, as every callable of %s does", call.id, f.ID))
		}
		expect.Empty(tb, remainder, fmt.Sprintf("%s projects, and a whole projection leaves no remainder", f.ID))
	case ProjectsPartly:
		refused := slices.ContainsFunc(p.callables, func(call callable) bool { return !call.projects })
		expect.False(tb, p.opaque == 0 && p.gaps == 0 && !refused, fmt.Sprintf("%s projects partly, so a "+
			"reference, a member set or a callable falls short: a capability nobody declared is a capability "+
			"nobody tested", f.ID))
	case Opaque:
		expect.NotEqual(tb, p.declaredOpaque, 0,
			fmt.Sprintf("%s is opaque, so an alias it declares has a target that folds to Opaque or Inline", f.ID))
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
			if unprojected(b.TypeOf(t).Form) {
				p.opaque++
			}
			return true
		})
		if alias, is := decl.(*node.Alias); is && alias.Target != nil && unprojected(b.TypeOf(alias.Target).Form) {
			p.declaredOpaque++
		}
		if set, is := b.MembersOf(decl); is {
			p.gaps += len(set.Gaps)
		}
		switch id.Kind {
		case symbol.KindFunction, symbol.KindMethod:
			_, projects := b.CallableOf(decl)
			p.callables = append(p.callables, callable{id: id, projects: projects})
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

// unprojected reports a form no projection resolves: Opaque, and
// Inline, an inline body the language has no structure for.
func unprojected(form symbol.TypeForm) bool {
	return form == symbol.FormOpaque || form == symbol.FormInline
}

// identityOf derives a declared declaration's identity through the
// corpus convention, under the empty discriminator where the
// language's frontend reports that it cannot overload.
func identityOf(c Corpus, f Feature, d Decl) symbol.Identity {
	id := symbol.Identity{
		Lang: c.Frontend.Lang(), Package: c.pkg(f.ID, d.Sub),
		Owner: d.Owner, Name: d.Name, Kind: d.Kind,
	}
	if c.Frontend.Overloads() {
		id.Disc = d.Disc
	}
	return id
}

// stampKeys returns the keys of the stamps the load made on a subject,
// in the seal's order, and nil for a subject it stamped nothing on.
func stampKeys(g *store.Graph, id symbol.Identity) []meta.KeyName {
	var keys []meta.KeyName
	for _, s := range g.StampsOf(id) {
		keys = append(keys, s.Key)
	}
	return keys
}
