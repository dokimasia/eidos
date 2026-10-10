// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"iter"
	"math"
	"slices"
	"sync"
	"sync/atomic"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// kindSlots spans every value of a [symbol.Kind]: the kind is
// a uint8, so an array this long indexes by kind without depending
// on the generated kind count.
const kindSlots = math.MaxUint8 + 1

// Graph is the run's node declarations.
//
// A graph comes from one of two constructors. [New] returns an empty
// graph that a load fills through [Graph.AddPackage] and the attach
// methods and seals with [Graph.Freeze]. [Sealed] returns a frozen graph
// over a [Source]'s regions, which decode on first use.
//
// # Concurrency
//
// [Graph.AddPackage] is safe to call concurrently: frontends shard per
// unit and load in parallel, so the graph serializes writes itself.
// Serialization is per package, so two frontends adding two packages do
// not contend. [Graph.Freeze] is the one exclusive operation, and it
// excludes every write: a package cannot arrive after the indexes are
// built and go missing from them.
//
// Reads are safe to make concurrently with each other once the graph is
// frozen, which is the only state annotators and generators see it in.
// A sealed graph builds each lazy index under a [sync.Once], so two
// readers that need one region decode it once.
//
// # Reading
//
// [Graph.ByKind], [Graph.Lookup] and [Graph.PackageOf] return
// untracked, and are the kernel's own path. A plugin reads through a
// [Reader], which it is handed in place of the graph, so the single
// door is a property of the types.
//
// Both indexes of a graph [New] returned build at [Graph.Freeze]:
// nothing may add a declaration afterwards, so neither can go stale. An
// untracked read before the seal returns nothing, never a partial
// result.
//
// # Allocation contract
//
// A read of a built index allocates nothing, and neither does an
// enumeration ranged over directly. A sealed graph allocates while it
// decodes a region and builds an index over it, once for each region
// and each index.
type Graph struct {
	// seal guards the phase, not the data: a write takes it for reading
	// so two frontends do not exclude each other, and Freeze takes it
	// for writing so no write is in flight when the indexes build.
	seal sync.RWMutex
	// packages maps each loaded package's identity to its entry. It is
	// a sync.Map because the frontends writing it write disjoint keys,
	// so two packages never contend on one lock.
	packages sync.Map
	frozen   bool

	// declCounts tallies identity-bearing declarations per kind as
	// packages arrive, on the loading goroutine where the package is
	// cache-warm, so Freeze sizes every index once and never rehashes.
	declCounts [kindSlots]atomic.Int64

	// The indexes, built once at Freeze and read-only after it. They
	// contain declarations, not plain symbols, so a reader filtering
	// and recording by identity needs no assertion of its own.
	// Ownership is per package: a declaration's identity names its
	// package, so byPkg has one entry per package.
	byID   map[symbol.Identity]node.Declaration
	byKind map[symbol.Kind][]node.Declaration
	byPkg  map[symbol.Identity]*node.Package
	// pkgOrder lists the same packages in identity order, so an
	// enumeration is deterministic without sorting per call.
	pkgOrder []*node.Package

	// directives contains the raw directive instances per subject.
	// The seal sorts them and builds the directive index beside the
	// kind index.
	directives  attachSet[directive.Raw]
	byDirective map[directive.Name][]node.Declaration

	// stamps contains the raw classification stamps per subject. The
	// seal sorts them the way it sorts directives.
	stamps attachSet[meta.RawStamp]

	// lazy is a sealed graph's regions and the indexes it builds over
	// them on first use, and nil for a graph [New] returned. Every read
	// consults it in place of the indexes above where it is set.
	lazy *sealedIndex
}

// New returns an unfrozen graph that contains nothing. It allocates the
// graph, one allocation.
func New() *Graph { return &Graph{} }

// Sealed returns a frozen graph over the regions of src.
//
// [Graph.Lookup], [Graph.Holds] and [Graph.PackageOf] decode the
// regions of the package an identity names, and [Graph.ByKind] and
// [Graph.ByDirective] decode the regions whose summary lists the kind
// or the spelling, each on first use. [Graph.Packages],
// [Graph.Directives] and [Graph.Stamps] decode every region. Every read
// returns what a graph that loaded the same regions through [New]
// returns, in the same order: a package that more than one region
// contributes files to reads as one package, merged in region order the
// way the load's splice merges units.
//
// A region that fails to decode reads as absent, and [Graph.Damaged]
// then returns the failure. Sealed calls [Source.Regions] once and
// decodes no region itself.
//
// # Allocation contract
//
// Sealed allocates the graph and an index of the summaries 15 times
// over the canonical workspace of 1,000 packages. Only the tables of
// the index's two maps grow in number with the packages and spellings.
func Sealed(src Source) *Graph {
	return &Graph{frozen: true, lazy: newSealedIndex(src)}
}

// loadedPackage is one added package and the declarations one walk
// over it collected. The walk runs at [Graph.AddPackage] on the
// loading goroutine, so Freeze indexes flat slices instead of
// traversing every package again serially.
type loadedPackage struct {
	pkg *node.Package
	// decls lists the identity-bearing declarations, in identity
	// order. Freeze releases it once the indexes contain them.
	decls []node.Declaration
}

// AddPackage adds a parsed package.
//
// Error modes: a [RefusedError] under [FrozenWrite] after
// [Graph.Freeze] and over a graph [Sealed] returned, and under
// [DuplicatePackage] for a package identity the graph already contains.
// A nil package and a package that names no identity return a plain
// error, because only a defect produces either.
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

	// The walk runs here, on the frontend's goroutine, so the one
	// traversal every declaration needs parallelizes with loading, and
	// Freeze ranges flat slices. The entry is filled after it is
	// stored, which no reader can observe: a duplicate add never reads
	// it, and Freeze orders after every add through the seal.
	entry.decls = g.collect(p)
	return nil
}

// Freeze seals the graph and builds its indexes. It is idempotent, and
// a graph [Sealed] returned is frozen from the start.
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
	// in sorted package order, so both are deterministic.
	loaded := g.loaded()
	g.pkgOrder = make([]*node.Package, 0, len(loaded))
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
			g.pkgOrder = append(g.pkgOrder, entry.pkg)
			for _, decl := range entry.decls {
				g.byKind[decl.Kind()] = append(g.byKind[decl.Kind()], decl)
			}
		}
	})
	fill.Wait()

	g.freezeDirectives()
	g.stamps.seal(compareStamp)

	// The collected slices are spent: the indexes contain everything.
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
// a declaration that may not survive the phase. It allocates the
// handle, one allocation.
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

// ByKind enumerates the declarations of one kind, untracked, in
// identity order, the same on every run: packages sort by identity, and
// so does each package's declarations. Declarations under one identity
// come back in the order the walk of their package visits them. A sealed
// graph decodes the regions whose summary lists the kind, and enumerates
// them in the same order.
func (g *Graph) ByKind(k symbol.Kind) iter.Seq[symbol.Symbol] {
	return func(yield func(symbol.Symbol) bool) {
		for _, decl := range g.kind(k) {
			if !yield(decl) {
				return
			}
		}
	}
}

// Lookup returns one declaration by identity, untracked, and false
// where the graph contains none. A sealed graph decodes the regions of
// the package the identity names.
func (g *Graph) Lookup(id symbol.Identity) (symbol.Symbol, bool) {
	decl, held := g.declaration(id)
	if !held {
		return nil, false
	}
	return decl, true
}

// Holds reports whether the graph contains a subject under this
// identity, untracked. The identity index contains every package
// beside its declarations, so one lookup checks a package subject
// and a declaration subject alike.
func (g *Graph) Holds(id symbol.Identity) bool {
	_, held := g.declaration(id)
	return held
}

// Packages enumerates the loaded packages in identity order,
// untracked: the roster the seal fixed, the same on every run. Over a
// sealed graph it decodes every region, and a package that more than
// one region contributes files to comes back once, merged.
func (g *Graph) Packages() iter.Seq[*node.Package] {
	return func(yield func(*node.Package) bool) {
		if g.lazy != nil {
			g.lazy.eachPackage(yield)
			return
		}
		for _, pkg := range g.pkgOrder {
			if !yield(pkg) {
				return
			}
		}
	}
}

// PackageOf returns the package that contains a declaration, untracked:
// the kernel's own path, beside the tracked [Reader.PackageOf].
//
// It returns false for a declaration the graph does not contain, and
// nothing before [Graph.Freeze], because both indexes it reads are
// built there.
func (g *Graph) PackageOf(id symbol.Identity) (*node.Package, bool) {
	return g.packageOf(id)
}

// Damaged returns the first failure of a [Source] to decode a region of
// a graph [Sealed] returned, and nil where every region the graph
// decoded read whole. A graph [New] returned decodes nothing and
// returns nil. A run that finds Damaged set after a phase discards what
// it derived from the graph, because a region that failed read as
// absent.
func (g *Graph) Damaged() error {
	if g.lazy == nil {
		return nil
	}
	return g.lazy.damaged()
}

// collect returns one package's identity-bearing declarations in
// identity order, and adds each to the graph's count of its kind. The
// sort is stable, so declarations under one identity keep the order of
// the walk. It runs once per package, on the loading goroutine, so
// every enumeration of a kind is in identity order without a sort per
// call.
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
	slices.SortStableFunc(out, func(a, b node.Declaration) int { return a.Identity().Compare(b.Identity()) })
	return out
}

// packageOf returns the package that contains a declaration.
//
// The package is derived from the identity, not looked up per
// declaration: a declaration's Lang and Package name the package
// that declared it, which is the same derivation scope filtering
// uses. It returns false for a declaration the graph does not contain,
// and for one whose identity names a package that was never added.
func (g *Graph) packageOf(id symbol.Identity) (*node.Package, bool) {
	if g.lazy != nil {
		return g.lazy.packageOf(id)
	}
	if _, held := g.byID[id]; !held {
		return nil, false
	}
	pkg, held := g.byPkg[owningPackage(id)]
	return pkg, held
}

// kind returns the declarations of one kind in enumeration order: the
// graph's index, or the sealed graph's index over the regions whose
// summary lists the kind. The slice is the graph's own storage.
func (g *Graph) kind(k symbol.Kind) []node.Declaration {
	if g.lazy != nil {
		return g.lazy.kind(k)
	}
	return g.byKind[k]
}

// declaration returns the declaration under an identity, and false
// where the graph contains none. A sealed graph decodes the regions of
// the package the identity names.
func (g *Graph) declaration(id symbol.Identity) (node.Declaration, bool) {
	if g.lazy != nil {
		return g.lazy.declaration(id)
	}
	decl, held := g.byID[id]
	return decl, held
}

// loaded returns every added package, in identity order.
//
// The sort is what makes the indexes deterministic: the packages
// arrive in whatever order the frontends finished in, and a run that
// scheduled them differently must still return one order.
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
