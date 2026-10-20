// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"fmt"
	"maps"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// errNoRegion is the damage a [Source] causes when it decodes a region
// to nothing without an error.
var errNoRegion = errors.New("store: the source decoded the region to nothing")

// sealedIndex is a sealed graph's regions and the indexes it builds
// over them on first use. A read decodes the regions it needs and no
// other: a lookup the regions of one package, an enumeration by kind or
// by spelling the regions whose summary lists it.
//
// # Concurrency
//
// Every lazily built part is behind a [sync.Once], so concurrent
// readers decode a region and build an index once, and read the result
// without a lock afterwards. The maps and slots a read consults are
// complete when [Sealed] returns. An enumeration of every package builds
// the index of each package on up to GOMAXPROCS goroutines before its
// first yield.
//
// # Allocation contract
//
// The index of the summaries allocates each of its lists once: each
// list of region numbers is a window of one shared array, sized by a
// first pass over the summaries. Only the tables of its two maps grow
// in number with the packages and spellings.
type sealedIndex struct {
	src     Source
	infos   []RegionInfo
	regions []regionSlot
	// pkgSlots are the slots of every package a summary lists, and
	// byPkg maps each package to its slot's index. order lists the same
	// packages in identity order.
	pkgSlots []packageSlot
	byPkg    map[symbol.Identity]int
	order    []symbol.Identity
	// kinds and spellings are the enumerations by kind and by spelling,
	// each over the regions whose summary lists it. byName maps each
	// spelling to its slot's index.
	kinds     [kindSlots]declSlot
	spellings []declSlot
	byName    map[directive.Name]int

	// damage is the first decoding failure, which [Graph.Damaged]
	// returns.
	damageMu sync.Mutex
	damage   error

	// built builds the index of every package once, on a pool.
	built sync.Once
}

// regionSlot is one region, decoded once.
type regionSlot struct {
	once sync.Once
	// region is nil until the decode, and nil after a decode that
	// failed.
	region *Region
	// parts are the region's package nodes, each with the
	// identity-bearing declarations a walk of its files visits, in
	// traversal order.
	parts []regionPart
}

// regionPart is one package node of a region and the identity-bearing
// declarations its files contain, in traversal order. The package node
// itself is not among them, because a sealed graph merges the parts of
// one package into one node.
type regionPart struct {
	pkg   *node.Package
	decls []node.Declaration
}

// packageSlot is one package of a sealed graph: the regions that
// contribute to it, in region order, and its view, built once.
type packageSlot struct {
	regions []int
	// dependency reports that a dependency round loaded every region of
	// the package. [newSealedIndex] sets it from the summaries, so a read
	// of it decodes nothing.
	dependency bool
	once       sync.Once
	// pkg is the merged package node, nil where no decoded region
	// declares the package.
	pkg *node.Package
	// byID lists each identity-bearing declaration of the package, the
	// package node included, in identity order. Declarations under one
	// identity keep their region order.
	byID []node.Declaration
	// directives and stamps map each subject of the package to its
	// raw attachments in the seal's order. directed and stamped list
	// the same subjects in identity order.
	directives map[symbol.Identity][]directive.Raw
	directed   []symbol.Identity
	stamps     map[symbol.Identity][]meta.RawStamp
	stamped    []symbol.Identity
}

// lookup returns the declaration under an identity, by a binary search
// of the sorted declarations. Where two regions of the package declare
// one identity, it returns the declaration of the later region. It does
// not allocate.
func (slot *packageSlot) lookup(id symbol.Identity) (node.Declaration, bool) {
	i, found := slices.BinarySearchFunc(slot.byID, id, func(d node.Declaration, id symbol.Identity) int {
		return d.Identity().Compare(id)
	})
	if !found {
		return nil, false
	}
	for i+1 < len(slot.byID) && slot.byID[i+1].Identity() == id {
		i++
	}
	return slot.byID[i], true
}

// declSlot is one enumeration of a sealed graph: the regions whose
// summary lists its kind or spelling, and the declarations, built once.
type declSlot struct {
	regions []int
	once    sync.Once
	decls   []node.Declaration
}

// newSealedIndex indexes the summaries of src. It decodes no region.
//
// A first pass assigns every package and spelling its slot and counts
// the regions of each slot, kind included. A second pass fills each
// slot's region list, a window of one shared array, in region order.
func newSealedIndex(src Source) *sealedIndex {
	infos := src.Regions()
	var packageRefs, kindRefs, nameRefs int
	for _, info := range infos {
		packageRefs += len(info.Packages)
		kindRefs += len(info.Kinds)
		nameRefs += len(info.Directives)
	}
	s := &sealedIndex{
		src:       src,
		infos:     infos,
		regions:   make([]regionSlot, len(infos)),
		pkgSlots:  make([]packageSlot, 0, packageRefs),
		byPkg:     make(map[symbol.Identity]int, packageRefs),
		order:     make([]symbol.Identity, 0, packageRefs),
		spellings: make([]declSlot, 0, nameRefs),
		byName:    make(map[directive.Name]int, nameRefs),
	}

	packageCounts := make([]int, 0, packageRefs)
	nameCounts := make([]int, 0, nameRefs)
	var kindCounts [kindSlots]int
	for _, info := range infos {
		for _, pkg := range info.Packages {
			at, met := s.byPkg[pkg]
			if !met {
				at = len(s.pkgSlots)
				s.byPkg[pkg] = at
				s.pkgSlots = append(s.pkgSlots, packageSlot{dependency: true})
				s.order = append(s.order, pkg)
				packageCounts = append(packageCounts, 0)
			}
			packageCounts[at]++
			s.pkgSlots[at].dependency = s.pkgSlots[at].dependency && info.Dependency
		}
		for _, k := range info.Kinds {
			kindCounts[k]++
		}
		for _, n := range info.Directives {
			at, met := s.byName[n]
			if !met {
				at = len(s.spellings)
				s.byName[n] = at
				s.spellings = append(s.spellings, declSlot{})
				nameCounts = append(nameCounts, 0)
			}
			nameCounts[at]++
		}
	}

	shared := make([]int, packageRefs+kindRefs+nameRefs)
	for at := range s.pkgSlots {
		s.pkgSlots[at].regions, shared = window(shared, packageCounts[at])
	}
	for k := range s.kinds {
		s.kinds[k].regions, shared = window(shared, kindCounts[k])
	}
	for at := range s.spellings {
		s.spellings[at].regions, shared = window(shared, nameCounts[at])
	}
	for i, info := range infos {
		for _, pkg := range info.Packages {
			slot := &s.pkgSlots[s.byPkg[pkg]]
			slot.regions = append(slot.regions, i)
		}
		for _, k := range info.Kinds {
			s.kinds[k].regions = append(s.kinds[k].regions, i)
		}
		for _, n := range info.Directives {
			slot := &s.spellings[s.byName[n]]
			slot.regions = append(slot.regions, i)
		}
	}
	slices.SortFunc(s.order, symbol.Identity.Compare)
	return s
}

// window splits n elements off the front of shared: an empty slice of
// capacity n, which appends fill without allocating, and the rest of
// shared.
func window(shared []int, n int) (head, rest []int) {
	return shared[:0:n], shared[n:]
}

// declaration returns the declaration under an identity, decoding the
// regions of the package it names.
func (s *sealedIndex) declaration(id symbol.Identity) (node.Declaration, bool) {
	slot := s.pkg(owningPackage(id))
	if slot == nil {
		return nil, false
	}
	return slot.lookup(id)
}

// packageOf returns the merged package that contains a declaration the
// graph contains.
func (s *sealedIndex) packageOf(id symbol.Identity) (*node.Package, bool) {
	slot := s.pkg(owningPackage(id))
	if slot == nil || slot.pkg == nil {
		return nil, false
	}
	if _, held := slot.lookup(id); !held {
		return nil, false
	}
	return slot.pkg, true
}

// dependency reports whether a dependency round loaded every region of a
// package, and false for a package that no summary lists.
func (s *sealedIndex) dependency(pkg symbol.Identity) bool {
	at, met := s.byPkg[pkg]
	return met && s.pkgSlots[at].dependency
}

// kind returns the declarations of one kind in the graph's order:
// packages in identity order, and each package's declarations in
// identity order, those under one identity in region order. It decodes
// the regions whose summary lists the kind, and for packages, every
// region of each package listed.
func (s *sealedIndex) kind(k symbol.Kind) []node.Declaration {
	slot := &s.kinds[k]
	slot.once.Do(func() {
		for _, id := range s.packagesOf(slot.regions) {
			if k == symbol.KindPackage {
				if pkg := s.pkg(id); pkg.pkg != nil {
					slot.decls = append(slot.decls, pkg.pkg)
				}
				continue
			}
			start := len(slot.decls)
			for _, i := range s.pkgSlots[s.byPkg[id]].regions {
				if !slices.Contains(s.infos[i].Kinds, k) {
					continue
				}
				for _, part := range s.region(i).parts {
					if part.pkg.ID != id {
						continue
					}
					for _, decl := range part.decls {
						if decl.Kind() == k {
							slot.decls = append(slot.decls, decl)
						}
					}
				}
			}
			slices.SortStableFunc(slot.decls[start:], func(a, b node.Declaration) int {
				return a.Identity().Compare(b.Identity())
			})
		}
	})
	return slot.decls
}

// carriers returns the declarations with a spelling, in identity
// order. It decodes every region of each package a region that lists
// the spelling names, because a subject's directives come from every
// region of its package.
func (s *sealedIndex) carriers(n directive.Name) []node.Declaration {
	at, met := s.byName[n]
	if !met {
		return nil
	}
	slot := &s.spellings[at]
	slot.once.Do(func() {
		for _, id := range s.packagesOf(slot.regions) {
			pkg := s.pkg(id)
			for _, subject := range pkg.directed {
				decl, held := pkg.lookup(subject)
				if held && slices.ContainsFunc(pkg.directives[subject], func(raw directive.Raw) bool {
					return raw.Name == n
				}) {

					slot.decls = append(slot.decls, decl)
				}
			}
		}
	})
	return slot.decls
}

// directivesOf returns a subject's raw directives in the seal's order.
func (s *sealedIndex) directivesOf(id symbol.Identity) []directive.Raw {
	slot := s.pkg(owningPackage(id))
	if slot == nil {
		return nil
	}
	return slot.directives[id]
}

// stampsOf returns a subject's raw stamps in the seal's order.
func (s *sealedIndex) stampsOf(id symbol.Identity) []meta.RawStamp {
	slot := s.pkg(owningPackage(id))
	if slot == nil {
		return nil
	}
	return slot.stamps[id]
}

// eachPackage calls yield with every merged package in identity order,
// decoding every region, until yield returns false. It builds the index of
// every package through [sealedIndex.buildAll] first.
func (s *sealedIndex) eachPackage(yield func(*node.Package) bool) {
	s.buildAll()
	for _, id := range s.order {
		if pkg := s.pkg(id).pkg; pkg != nil && !yield(pkg) {
			return
		}
	}
}

// eachDirected calls yield with every subject that has directives,
// with its directives, in identity order, decoding every region, until
// yield returns false. It builds the index of every package through
// [sealedIndex.buildAll] first.
func (s *sealedIndex) eachDirected(yield func(symbol.Identity, []directive.Raw) bool) {
	s.buildAll()
	for _, id := range s.order {
		pkg := s.pkg(id)
		for _, subject := range pkg.directed {
			if !yield(subject, pkg.directives[subject]) {
				return
			}
		}
	}
}

// eachStamped calls yield with every subject that has stamps, with its
// stamps, in identity order, decoding every region, until yield
// returns false. It builds the index of every package through
// [sealedIndex.buildAll] first.
func (s *sealedIndex) eachStamped(yield func(symbol.Identity, []meta.RawStamp) bool) {
	s.buildAll()
	for _, id := range s.order {
		pkg := s.pkg(id)
		for _, subject := range pkg.stamped {
			if !yield(subject, pkg.stamps[subject]) {
				return
			}
		}
	}
}

// buildAll builds the index of every package once, on up to GOMAXPROCS
// goroutines, and returns when every index is built. An enumeration of
// every package would build each index anyway, one after another. A
// package that a reader built before keeps its index, through the
// package's own [sync.Once].
func (s *sealedIndex) buildAll() {
	s.built.Do(func() {
		var (
			next atomic.Int64
			wg   sync.WaitGroup
		)
		for range min(runtime.GOMAXPROCS(0), len(s.order)) {
			wg.Go(func() {
				for i := int(next.Add(1)) - 1; i < len(s.order); i = int(next.Add(1)) - 1 {
					s.pkg(s.order[i])
				}
			})
		}
		wg.Wait()
	})
}

// damaged returns the first decoding failure.
func (s *sealedIndex) damaged() error {
	s.damageMu.Lock()
	defer s.damageMu.Unlock()
	return s.damage
}

// pkg returns a package's slot with its view built, and nil for a
// package no summary lists.
func (s *sealedIndex) pkg(id symbol.Identity) *packageSlot {
	at, met := s.byPkg[id]
	if !met {
		return nil
	}
	slot := &s.pkgSlots[at]
	slot.once.Do(func() { s.build(id, slot) })
	return slot
}

// build decodes every region of one package and builds its view: the
// parts merged in region order the way the load's splice merges units,
// the identity index over the merged traversal, and each subject's
// attachments in the seal's order.
func (s *sealedIndex) build(id symbol.Identity, slot *packageSlot) {
	var (
		nodes []*node.Package
		decls []node.Declaration
	)
	slot.directives = map[symbol.Identity][]directive.Raw{}
	slot.stamps = map[symbol.Identity][]meta.RawStamp{}
	for _, i := range slot.regions {
		region := s.region(i)
		for _, part := range region.parts {
			if part.pkg.ID == id {
				nodes = append(nodes, part.pkg)
				decls = append(decls, part.decls...)
			}
		}
		if region.region == nil {
			continue
		}
		for subject, raws := range region.region.Directives {
			if owningPackage(subject) == id {
				slot.directives[subject] = append(slot.directives[subject], raws...)
			}
		}
		for subject, stamps := range region.region.Stamps {
			if owningPackage(subject) == id {
				slot.stamps[subject] = append(slot.stamps[subject], stamps...)
			}
		}
	}

	slot.pkg = merge(nodes)
	slot.byID = make([]node.Declaration, 0, len(decls)+1)
	if slot.pkg != nil {
		slot.byID = append(slot.byID, slot.pkg)
	}
	slot.byID = append(slot.byID, decls...)
	slices.SortStableFunc(slot.byID, func(a, b node.Declaration) int {
		return a.Identity().Compare(b.Identity())
	})
	for subject, raws := range slot.directives {
		slices.SortFunc(raws, compareRaw)
		slot.directives[subject] = raws
	}
	for subject, stamps := range slot.stamps {
		slices.SortFunc(stamps, compareStamp)
		slot.stamps[subject] = stamps
	}
	slot.directed = slices.SortedFunc(maps.Keys(slot.directives), symbol.Identity.Compare)
	slot.stamped = slices.SortedFunc(maps.Keys(slot.stamps), symbol.Identity.Compare)
}

// region returns a region's slot, decoded. A region whose decode
// failed has no parts, and the failure is recorded as the graph's
// damage.
func (s *sealedIndex) region(i int) *regionSlot {
	slot := &s.regions[i]
	slot.once.Do(func() {
		region, err := s.src.Region(i)
		if err == nil && region == nil {
			err = errNoRegion
		}
		if err != nil {
			s.fail(fmt.Errorf("store: region %d: %w", i, err))
			return
		}
		slot.region = region
		slot.parts = make([]regionPart, 0, len(region.Packages))
		for _, p := range region.Packages {
			slot.parts = append(slot.parts, regionPart{pkg: p, decls: members(p)})
		}
	})
	return slot
}

// packagesOf returns the packages the given regions' summaries list,
// in identity order and without repeats.
func (s *sealedIndex) packagesOf(regions []int) []symbol.Identity {
	var out []symbol.Identity
	for _, i := range regions {
		out = append(out, s.infos[i].Packages...)
	}
	slices.SortFunc(out, symbol.Identity.Compare)
	return slices.Compact(out)
}

// fail records the first decoding failure.
func (s *sealedIndex) fail(err error) {
	s.damageMu.Lock()
	defer s.damageMu.Unlock()
	if s.damage == nil {
		s.damage = err
	}
}

// members returns the identity-bearing declarations a package node's
// files contain, in traversal order, without the package node.
func members(p *node.Package) []node.Declaration {
	var out []node.Declaration
	node.Walk(p, func(sym symbol.Symbol) bool {
		decl, names := sym.(node.Declaration)
		if names && sym != symbol.Symbol(p) && !decl.Identity().IsZero() {
			out = append(out, decl)
		}
		return true
	})
	return out
}

// merge returns one package node over the parts of a package, in
// region order, the way the load's splice merges units: the first
// part's identity, position and path, the first non-empty name and
// documentation, and every part's files in order. A package of one
// part is that part's node, and a package of none is nil. The parts are
// not mutated.
func merge(parts []*node.Package) *node.Package {
	switch len(parts) {
	case 0:
		return nil
	case 1:
		return parts[0]
	}
	merged := *parts[0]
	merged.Files = nil
	for _, p := range parts {
		if merged.Name == "" {
			merged.Name = p.Name
		}
		if len(merged.Doc) == 0 {
			merged.Doc = p.Doc
		}
		merged.Files = append(merged.Files, p.Files...)
	}
	return &merged
}
