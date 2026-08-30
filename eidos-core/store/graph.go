// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package store

import (
	"errors"
	"iter"
	"math"
	"slices"
	"sync"
	"sync/atomic"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// kindSlots spans every value a [symbol.Kind] can hold: the kind is
// a uint8, so an array this long indexes by kind without depending
// on the generated kind count.
const kindSlots = math.MaxUint8 + 1

// Graph is the run's node declarations.
//
// # Concurrency
//
// [Graph.AddPackage] is safe to call concurrently: frontends shard
// per unit and load in parallel, so the graph serializes writes
// itself rather than asking every frontend to. Serialization is per
// package, so two frontends adding two packages do not contend.
// [Graph.Freeze] is the one exclusive operation, and it excludes
// writes rather than racing them: a package cannot land after the
// indexes are built and go missing from them.
//
// Reads are safe to make concurrently with each other once the graph
// is frozen, which is the only state annotators and generators see
// it in.
//
// # Reading
//
// [Graph.ByKind] and [Graph.Lookup] answer untracked, and are the
// kernel's own path. Everything a plugin reaches goes through a
// [Reader], which a plugin is handed instead of the graph. That is
// what makes the single door structural rather than a review
// comment.
//
// Both indexes build at [Graph.Freeze], where they are free: nothing
// may add a declaration afterwards, so neither can go stale. An
// untracked read before the seal therefore answers nothing rather
// than a partial result.
type Graph struct {
	// seal guards the phase rather than the data: a write holds it
	// for reading so two frontends do not exclude each other, and
	// Freeze holds it for writing so no write is in flight when the
	// indexes build.
	seal sync.RWMutex
	// packages holds the loaded packages by identity. It is a
	// sync.Map because the frontends writing it own disjoint keys,
	// which is what keeps two packages from contending on one lock.
	packages sync.Map
	frozen   bool

	// declCounts tallies identity-bearing declarations per kind as
	// packages arrive, on the loading goroutine where the package is
	// cache-warm, so Freeze sizes every index once rather than
	// growing it through rehashes.
	declCounts [kindSlots]atomic.Int64

	// The indexes, built once at Freeze and read-only after it. They
	// hold declarations rather than plain symbols, so a reader
	// filtering and recording by identity needs no assertion of its
	// own. Ownership is per package rather than per declaration: a
	// declaration's identity names its package, so byPkg holds one
	// entry per package instead of one per declaration.
	byID   map[symbol.Identity]node.Declaration
	byKind map[symbol.Kind][]node.Declaration
	byPkg  map[symbol.Identity]*node.Package

	// attached holds raw directive instances per subject as they
	// arrive; the seal sorts them and builds the directive index
	// beside the kind index.
	attached       sync.Map
	directives     map[symbol.Identity][]directive.Raw
	directiveOrder []symbol.Identity
	byDirective    map[directive.Name][]node.Declaration
}

// New answers an unfrozen graph holding nothing.
func New() *Graph { return &Graph{} }

// loadedPackage is one added package and the declarations one walk
// over it collected. The walk runs at [Graph.AddPackage] on the
// loading goroutine, so Freeze indexes flat slices instead of
// traversing every package again serially.
type loadedPackage struct {
	pkg *node.Package
	// decls holds the identity-bearing declarations, in traversal
	// order. Freeze releases it once the indexes hold them.
	decls []node.Declaration
}

// AddPackage adds a parsed package.
//
// It is refused after [Graph.Freeze], and refused for a package
// identity the graph already holds. A package that names no identity
// cannot be indexed, so it is refused as the defect it is.
func (g *Graph) AddPackage(p *node.Package) error {
	if p == nil {
		return errors.New("store: no package to add")
	}
	if p.ID.IsZero() {
		return errors.New("store: the package names no identity, so nothing can index it")
	}

	g.seal.RLock()
	defer g.seal.RUnlock()

	if g.frozen {
		return &RefusedError{Code: FrozenWrite, Msg: p.ID.String() + " is added after Freeze"}
	}
	entry := &loadedPackage{pkg: p}
	if _, held := g.packages.LoadOrStore(p.ID, entry); held {
		return &RefusedError{Code: DuplicatePackage, Msg: p.ID.String() + " is added twice"}
	}

	// Collect and tally here rather than at Freeze: this walk runs
	// on the frontend's goroutine, so the one traversal every
	// declaration needs parallelizes with loading, and Freeze ranges
	// flat slices. The entry is filled after it is stored, which no
	// reader can observe: a duplicate add never reads it, and Freeze
	// orders after every add through the seal.
	entry.decls = g.collect(p)
	return nil
}

// Freeze seals the graph and builds its indexes. It is idempotent.
//
// A repeated identity keeps the declaration indexed last. Telling
// two declarations under one identity apart belongs to the step that
// assigns identities, which is where both spellings are still known.
func (g *Graph) Freeze() {
	g.seal.Lock()
	defer g.seal.Unlock()

	if g.frozen {
		return
	}

	// Size every index from the tallies before filling any, so no
	// map rehashes and no slice reallocates while 200k declarations
	// stream through.
	total, kinds := 0, 0
	for k := range g.declCounts {
		if n := int(g.declCounts[k].Load()); n > 0 {
			total += n
			kinds++
		}
	}
	g.byID = make(map[symbol.Identity]node.Declaration, total)
	g.byKind = make(map[symbol.Kind][]node.Declaration, kinds)
	for k := range g.declCounts {
		if n := int(g.declCounts[k].Load()); n > 0 {
			g.byKind[symbol.Kind(k)] = make([]node.Declaration, 0, n)
		}
	}
	g.byPkg = make(map[symbol.Identity]*node.Package,
		g.declCounts[symbol.KindPackage].Load())

	// The two heavy indexes fill concurrently: they write disjoint
	// maps and share only reads of the collected slices. Each fills
	// in sorted package order, so both stay deterministic.
	loaded := g.loaded()
	var fill sync.WaitGroup
	fill.Go(func() {
		for _, entry := range loaded {
			for _, decl := range entry.decls {
				g.byID[decl.Identity()] = decl
			}
		}
	})
	fill.Go(func() {
		for _, entry := range loaded {
			g.byPkg[entry.pkg.ID] = entry.pkg
			for _, decl := range entry.decls {
				g.byKind[decl.Kind()] = append(g.byKind[decl.Kind()], decl)
			}
		}
	})
	fill.Wait()

	g.freezeDirectives()

	// The collected slices are spent: the indexes hold everything.
	for _, entry := range loaded {
		entry.decls = nil
	}
	g.frozen = true
}

// Frozen reports whether the graph is sealed.
func (g *Graph) Frozen() bool {
	g.seal.RLock()
	defer g.seal.RUnlock()

	return g.frozen
}

// Reader hands out a tracked read handle recording into reads and
// seeing only what sc admits. A nil Scope admits everything.
//
// It is refused before [Graph.Freeze]: identities are not assigned
// and the graph is still moving, so an edge recorded then would name
// a declaration that may not survive the phase.
func (g *Graph) Reader(reads *ReadSet, sc Scope) (*Reader, error) {
	if reads == nil {
		return nil, errors.New("store: no read set to record into, " +
			"which would make every read through the reader an untracked one")
	}
	if !g.Frozen() {
		return nil, &RefusedError{
			Code: UnfrozenRead,
			Msg:  "a reader is asked for before Freeze",
		}
	}
	return &Reader{graph: g, reads: reads, scope: sc}, nil
}

// ByKind enumerates the declarations of one kind, untracked.
//
// The order is the graph's own and holds across runs: packages sort
// by identity, and a package's declarations answer in the order the
// traversal reaches them.
func (g *Graph) ByKind(k symbol.Kind) iter.Seq[symbol.Symbol] {
	return func(yield func(symbol.Symbol) bool) {
		for _, decl := range g.byKind[k] {
			if !yield(decl) {
				return
			}
		}
	}
}

// Lookup answers one declaration by identity, untracked.
func (g *Graph) Lookup(id symbol.Identity) (symbol.Symbol, bool) {
	decl, held := g.byID[id]
	if !held {
		return nil, false
	}
	return decl, true
}

// collect walks one package, answering its identity-bearing
// declarations in traversal order and tallying them per kind.
func (g *Graph) collect(p *node.Package) []node.Declaration {
	var out []node.Declaration
	node.Walk(p, func(s symbol.Symbol) bool {
		decl, names := s.(node.Declaration)
		if !names || decl.Identity().IsZero() {
			return true
		}
		g.declCounts[decl.Kind()].Add(1)
		out = append(out, decl)
		return true
	})
	return out
}

// packageOf answers the package holding a declaration.
//
// The holder is derived from the identity rather than looked up per
// declaration: a declaration's Lang and Package name the package
// that declared it, which is the same derivation scope filtering
// uses. It answers false for a declaration the graph does not hold,
// and for one whose identity names a package that was never added.
func (g *Graph) packageOf(id symbol.Identity) (*node.Package, bool) {
	if _, held := g.byID[id]; !held {
		return nil, false
	}
	pkg, held := g.byPkg[owningPackage(id)]
	return pkg, held
}

// loaded answers every added package, in identity order.
//
// The sort is what makes the indexes deterministic: the packages
// arrive in whatever order the frontends finished in, and a run that
// scheduled them differently must still answer one order.
func (g *Graph) loaded() []*loadedPackage {
	var out []*loadedPackage
	g.packages.Range(func(_, value any) bool {
		entry, stored := value.(*loadedPackage)
		if stored {
			out = append(out, entry)
		}
		return true
	})
	slices.SortFunc(out, func(a, b *loadedPackage) int { return a.pkg.ID.Compare(b.pkg.ID) })
	return out
}
