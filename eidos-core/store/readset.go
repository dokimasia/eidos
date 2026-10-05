// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"cmp"
	"iter"
	"slices"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// inlineEdges is how many edges a read set keeps in place before it
// moves them into maps: more than an invocation that reads a fact or
// looks up a declaration records.
const inlineEdges = 4

// grain is the kind of one edge, one of the five a set records.
type grain uint8

// The five grains, numbered from one, so the zero edge has none.
const (
	grainIdentity  grain = 1
	grainPackage   grain = 2
	grainKind      grain = 3
	grainFact      grain = 4
	grainDirective grain = 5
)

// edge is one recorded edge: its grain and the fields the grain uses. A
// declaration edge and a package edge use id, a membership edge by kind
// uses kind, a fact edge uses id for the subject and name for the key,
// and a membership edge by directive uses name for the spelling.
type edge struct {
	grain grain
	kind  symbol.Kind
	id    symbol.Identity
	name  string
}

// factRead is one (subject, key) edge.
type factRead struct {
	subject symbol.Identity
	key     meta.KeyName
}

// ReadSet is what one derived artifact read.
//
// Edges deduplicate, so a loop reading one declaration a thousand
// times records one edge. The set records five grains of the four edge
// kinds: a declaration edge for a targeted read, a package edge for a
// package taken whole, a membership edge for an enumeration by kind,
// a membership edge for an enumeration by directive, and a fact edge at
// (subject, key) for a fact read. ReadSet satisfies [meta.Recorder], so
// one artifact's declaration reads and fact reads arrive in one set.
//
// The zero ReadSet is ready to record, and [NewReadSet] returns one.
//
// # Concurrency
//
// A ReadSet belongs to one derived artifact and is not safe for
// concurrent use. The graph it records reads of is shared, and its
// bookkeeping is not.
//
// # Allocation contract
//
// A set keeps its first four edges in place and allocates nothing for
// them, which covers an invocation that reads a fact or looks up a few
// declarations. A fifth edge, or an enumeration that reserves room for
// more, moves every edge into a map of its grain, and each map then
// grows as edges arrive. [ReadSet.Reset] keeps the maps, so a set reused
// across invocations allocates only to grow. A range over an enumeration
// of the edges a set keeps in place, or of the edges a [ReadLog] loaded
// into it, allocates nothing, and one over edges in maps sorts the
// grain's edges into one new list when the range starts.
type ReadSet struct {
	// inline contains the set's edges in the order they arrived, each
	// once, while spilled is false and loaded is empty, and n counts them.
	inline [inlineEdges]edge
	n      int
	// loaded contains the edges a log entry of more than four edges
	// restored, sorted by [compareEdges], which the set enumerates in place
	// until it records an edge. Its storage is the set's own, reused from
	// load to load.
	loaded []edge
	// spilled reports that every edge is in the maps below.
	spilled    bool
	identities map[symbol.Identity]struct{}
	packages   map[symbol.Identity]struct{}
	kinds      map[symbol.Kind]struct{}
	facts      map[factRead]struct{}
	directives map[directive.Name]struct{}
}

// NewReadSet returns a read set with no edges. It allocates the set,
// one allocation.
func NewReadSet() *ReadSet { return &ReadSet{} }

// Reset drops every recorded edge and keeps the maps, so a dispatcher
// reuses one set across a rule's invocations instead of building five
// maps per subject. The set records again immediately, in place. It
// zeroes the edges it kept in place or loaded, so a set kept for reuse
// retains no string of the graph that recorded them.
func (s *ReadSet) Reset() {
	clear(s.inline[:s.n])
	s.n = 0
	clear(s.loaded)
	s.loaded = s.loaded[:0]
	if !s.spilled {
		return
	}
	clear(s.identities)
	clear(s.packages)
	clear(s.kinds)
	clear(s.facts)
	clear(s.directives)
	s.spilled = false
}

// Identities returns every declaration edge, in identity order.
//
// The order is the set's own and not the order the reads arrived, so
// two runs reading the same declarations in different orders agree.
func (s *ReadSet) Identities() iter.Seq[symbol.Identity] {
	return func(yield func(symbol.Identity) bool) { s.eachIdentity(grainIdentity, s.identities, yield) }
}

// Packages returns every package edge, in identity order: each package
// a reader took whole through [Reader.PackageOf], or through
// [Reader.Lookup] of the package's own identity. A reader that took a
// package whole can walk any of its declarations without another
// tracked read, so the edge is dirty when any member changes.
func (s *ReadSet) Packages() iter.Seq[symbol.Identity] {
	return func(yield func(symbol.Identity) bool) { s.eachIdentity(grainPackage, s.packages, yield) }
}

// Kinds returns every membership edge by kind, in kind order.
func (s *ReadSet) Kinds() iter.Seq[symbol.Kind] {
	return func(yield func(symbol.Kind) bool) { s.eachKind(yield) }
}

// RecordFact records a fact read at (subject, key), never as a bare
// declaration edge: an edge per subject would run every reader of a
// bag again on any stamp.
func (s *ReadSet) RecordFact(subject symbol.Identity, key meta.KeyName) {
	s.record(edge{grain: grainFact, id: subject, name: string(key)})
}

// Facts returns every recorded fact read, in subject then key
// order.
func (s *ReadSet) Facts() iter.Seq2[symbol.Identity, meta.KeyName] {
	return func(yield func(symbol.Identity, meta.KeyName) bool) { s.eachFact(yield) }
}

// AppendPointReads appends the set's point reads to dst and returns the
// extended slice: what a claim records as its derivation. The
// declaration and package edges come first, as reads of their subject,
// each identity once and in identity order. The fact edges follow, in
// subject then key order. A membership edge is no point read, and the
// slice leaves it out.
//
// # Allocation contract
//
// AppendPointReads grows dst at most once, to room for every
// declaration, package and fact edge, and allocates nothing where dst
// has that room. It sorts within dst.
func (s *ReadSet) AppendPointReads(dst []meta.Read) []meta.Read {
	listed := s.listed()
	points := len(s.identities) + len(s.packages) + len(s.facts)
	if !s.spilled {
		points = 0
		for i := range listed {
			if g := listed[i].grain; g != grainKind && g != grainDirective {
				points++
			}
		}
	}
	dst = slices.Grow(dst, points)
	start := len(dst)
	if s.spilled {
		for id := range s.identities {
			dst = append(dst, meta.Read{Subject: id})
		}
		for id := range s.packages {
			dst = append(dst, meta.Read{Subject: id})
		}
	} else {
		for i := range listed {
			if e := &listed[i]; e.grain == grainIdentity || e.grain == grainPackage {
				dst = append(dst, meta.Read{Subject: e.id})
			}
		}
	}
	decls := dst[start:]
	slices.SortFunc(decls, func(a, b meta.Read) int { return a.Subject.Compare(b.Subject) })
	dst = dst[:start+len(slices.CompactFunc(decls, func(a, b meta.Read) bool { return a.Subject == b.Subject }))]
	facts := len(dst)
	if s.spilled {
		for edge := range s.facts {
			dst = append(dst, meta.Read{Subject: edge.subject, Key: edge.key})
		}
	} else {
		for i := range listed {
			if e := &listed[i]; e.grain == grainFact {
				dst = append(dst, meta.Read{Subject: e.id, Key: meta.KeyName(e.name)})
			}
		}
	}
	slices.SortFunc(dst[facts:], func(a, b meta.Read) int {
		return cmp.Or(a.Subject.Compare(b.Subject), cmp.Compare(a.Key, b.Key))
	})
	return dst
}

// Directives returns every membership edge by directive, in name
// order. Each edge means the artifact enumerated that spelling's
// carriers, so it runs again when a subject gains or loses the
// directive. A carrier that only changes runs it again through the
// declaration edge recorded beside the membership edge.
func (s *ReadSet) Directives() iter.Seq[directive.Name] {
	return func(yield func(directive.Name) bool) { s.eachDirective(yield) }
}

// Len returns how many edges the set contains, all five grains
// counted.
func (s *ReadSet) Len() int {
	if !s.spilled {
		return len(s.listed())
	}
	return len(s.identities) + len(s.packages) + len(s.kinds) + len(s.facts) + len(s.directives)
}

// eachIdentity calls yield with every edge of g, the declaration grain
// or the package grain, in identity order, until yield returns false. It
// reads the edges the set lists, or m, the map of g, once the set has
// moved its edges into maps.
func (s *ReadSet) eachIdentity(g grain, m map[symbol.Identity]struct{}, yield func(symbol.Identity) bool) {
	if !s.spilled {
		eachListed(s, g, func(e *edge) symbol.Identity { return e.id }, symbol.Identity.Compare, yield)
		return
	}
	each(sortedKeys(m, symbol.Identity.Compare), yield)
}

// eachKind calls yield with every membership edge by kind, in kind
// order, until yield returns false.
func (s *ReadSet) eachKind(yield func(symbol.Kind) bool) {
	if !s.spilled {
		eachListed(s, grainKind, func(e *edge) symbol.Kind { return e.kind }, cmp.Compare[symbol.Kind], yield)
		return
	}
	each(sortedKeys(s.kinds, cmp.Compare[symbol.Kind]), yield)
}

// eachDirective calls yield with every membership edge by directive, in
// name order, until yield returns false.
func (s *ReadSet) eachDirective(yield func(directive.Name) bool) {
	if !s.spilled {
		eachListed(s, grainDirective, func(e *edge) directive.Name { return directive.Name(e.name) },
			cmp.Compare[directive.Name], yield)
		return
	}
	each(sortedKeys(s.directives, cmp.Compare[directive.Name]), yield)
}

// eachFact calls yield with every recorded fact read, in subject then
// key order, until yield returns false.
func (s *ReadSet) eachFact(yield func(symbol.Identity, meta.KeyName) bool) {
	compare := func(a, b factRead) int { return cmp.Or(a.subject.Compare(b.subject), cmp.Compare(a.key, b.key)) }
	pair := func(f factRead) bool { return yield(f.subject, f.key) }
	if !s.spilled {
		eachListed(s, grainFact, func(e *edge) factRead {
			return factRead{subject: e.id, key: meta.KeyName(e.name)}
		}, compare, pair)
		return
	}
	each(sortedKeys(s.facts, compare), pair)
}

// listed returns the edges the set lists rather than maps: the edges it
// loaded, or else the edges it keeps in place. A set that moved its
// edges into maps lists none.
func (s *ReadSet) listed() []edge {
	if len(s.loaded) > 0 {
		return s.loaded
	}
	return s.inline[:s.n]
}

// reserve sizes the declaration grain for an enumeration about to
// record n edges, so a large read allocates its map once and does not
// grow it entry by entry. An enumeration whose edges fit in place beside
// the set's own reserves nothing. Otherwise the set moves its edges into
// maps, and it sizes only a declaration grain that has no map yet: a
// grain that already has a map keeps it.
func (s *ReadSet) reserve(n int) {
	if !s.spilled && len(s.listed())+n <= inlineEdges {
		return
	}
	if s.identities == nil && n > 0 {
		s.identities = make(map[symbol.Identity]struct{}, n)
	}
	if !s.spilled {
		s.spill()
	}
}

// record records one edge, once. While the set keeps its edges in place
// and has room, the edge goes in place. Otherwise the set moves its
// edges, the ones it loaded included, into the maps, and the edge goes
// into the map of its grain.
func (s *ReadSet) record(e edge) {
	if !s.spilled && len(s.loaded) == 0 {
		for i := range s.n {
			if s.inline[i] == e {
				return
			}
		}
		if s.n < inlineEdges {
			s.inline[s.n] = e
			s.n++
			return
		}
	}
	if !s.spilled {
		s.spill()
	}
	s.insert(e)
}

// restore records edges an earlier set recorded, each once and sorted by
// [compareEdges], into a set that has none: in place where they fit, and
// into the set's list of loaded edges otherwise.
func (s *ReadSet) restore(edges []edge) {
	if len(edges) <= inlineEdges {
		s.n = copy(s.inline[:], edges)
		return
	}
	s.loaded = append(s.loaded[:0], edges...)
}

// spill moves the edges the set lists, in place or loaded, into the
// maps, which contain every edge from then on.
func (s *ReadSet) spill() {
	s.spilled = true
	for _, e := range s.listed() {
		s.insert(e)
	}
	clear(s.inline[:s.n])
	s.n = 0
	clear(s.loaded)
	s.loaded = s.loaded[:0]
}

// insert adds one edge to the map of its grain, which it creates on the
// grain's first edge.
func (s *ReadSet) insert(e edge) {
	switch e.grain {
	case grainIdentity:
		if s.identities == nil {
			s.identities = map[symbol.Identity]struct{}{}
		}
		s.identities[e.id] = struct{}{}
	case grainPackage:
		if s.packages == nil {
			s.packages = map[symbol.Identity]struct{}{}
		}
		s.packages[e.id] = struct{}{}
	case grainKind:
		if s.kinds == nil {
			s.kinds = map[symbol.Kind]struct{}{}
		}
		s.kinds[e.kind] = struct{}{}
	case grainFact:
		if s.facts == nil {
			s.facts = map[factRead]struct{}{}
		}
		s.facts[factRead{subject: e.id, key: meta.KeyName(e.name)}] = struct{}{}
	case grainDirective:
		if s.directives == nil {
			s.directives = map[directive.Name]struct{}{}
		}
		s.directives[directive.Name(e.name)] = struct{}{}
	}
}

// eachListed calls yield with what get takes from every edge of one
// grain the set lists, in compare's order, until yield returns false.
// The edges a set loaded are in that order already, and it yields them
// in place. Of the edges a set keeps in place it sorts a copy of at most
// four values on the stack. It allocates nothing.
func eachListed[T any](s *ReadSet, g grain, get func(*edge) T, compare func(a, b T) int, yield func(T) bool) {
	if len(s.loaded) > 0 {
		edges := grainOf(s.loaded, g)
		for i := range edges {
			if !yield(get(&edges[i])) {
				return
			}
		}
		return
	}
	var values [inlineEdges]T
	m := 0
	for i := range s.n {
		if s.inline[i].grain == g {
			values[m] = get(&s.inline[i])
			m++
		}
	}
	sorted := values[:m]
	slices.SortFunc(sorted, compare)
	each(sorted, yield)
}

// compareEdges orders two edges by grain, then in the order of the
// grain's enumeration: declaration and package edges by identity, fact
// edges by subject and then by key, membership edges by kind or by the
// directive's name.
func compareEdges(a, b edge) int {
	if a.grain != b.grain {
		return cmp.Compare(a.grain, b.grain)
	}
	if a.grain == grainKind {
		return cmp.Compare(a.kind, b.kind)
	}
	return cmp.Or(a.id.Compare(b.id), cmp.Compare(a.name, b.name))
}

// grainOf returns the edges of one grain of a list sorted by
// [compareEdges], a window of the list, which it finds by binary search.
func grainOf(edges []edge, g grain) []edge {
	byGrain := func(e edge, g grain) int { return cmp.Compare(e.grain, g) }
	start, _ := slices.BinarySearchFunc(edges, g, byGrain)
	end, _ := slices.BinarySearchFunc(edges, g+1, byGrain)
	return edges[start:end]
}

// each calls yield with every element of list in order, until yield
// returns false. It does not keep yield, so a range over an enumeration
// built on it allocates nothing beyond the list.
func each[T any](list []T, yield func(T) bool) {
	for _, v := range list {
		if !yield(v) {
			return
		}
	}
}

// sortedKeys returns a map's keys in compare's order, in one
// allocation, and nil for an empty map.
func sortedKeys[K comparable, V any](m map[K]V, compare func(a, b K) int) []K {
	if len(m) == 0 {
		return nil
	}
	out := make([]K, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.SortFunc(out, compare)
	return out
}
