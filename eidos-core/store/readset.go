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
// Each grain allocates its map on its first edge, and the map grows as
// edges arrive. [ReadSet.Reset] keeps every map, so a set reused across
// invocations allocates only to grow. A range over an enumeration sorts
// the grain's edges into one new list when the range starts, and
// allocates nothing for a grain without an edge.
type ReadSet struct {
	identities map[symbol.Identity]struct{}
	packages   map[symbol.Identity]struct{}
	kinds      map[symbol.Kind]struct{}
	facts      map[factRead]struct{}
	directives map[directive.Name]struct{}
}

// factRead is one (subject, key) edge.
type factRead struct {
	subject symbol.Identity
	key     meta.KeyName
}

// NewReadSet returns a read set with no edges. It allocates the set,
// one allocation.
func NewReadSet() *ReadSet { return &ReadSet{} }

// Reset drops every recorded edge and keeps the storage, so a
// dispatcher reuses one set across a rule's invocations instead of
// building five maps per subject. The set records again immediately.
func (s *ReadSet) Reset() {
	clear(s.identities)
	clear(s.packages)
	clear(s.kinds)
	clear(s.facts)
	clear(s.directives)
}

// Identities returns every declaration edge, in identity order.
//
// The order is the set's own and not the order the reads arrived, so
// two runs reading the same declarations in different orders agree.
func (s *ReadSet) Identities() iter.Seq[symbol.Identity] {
	return func(yield func(symbol.Identity) bool) {
		each(sortedKeys(s.identities, symbol.Identity.Compare), yield)
	}
}

// Packages returns every package edge, in identity order: each package
// a reader took whole through [Reader.PackageOf], or through
// [Reader.Lookup] of the package's own identity. A reader that took a
// package whole can walk any of its declarations without another
// tracked read, so the edge is dirty when any member changes.
func (s *ReadSet) Packages() iter.Seq[symbol.Identity] {
	return func(yield func(symbol.Identity) bool) {
		each(sortedKeys(s.packages, symbol.Identity.Compare), yield)
	}
}

// Kinds returns every membership edge by kind, in kind order.
func (s *ReadSet) Kinds() iter.Seq[symbol.Kind] {
	return func(yield func(symbol.Kind) bool) { each(sortedKeys(s.kinds, cmp.Compare[symbol.Kind]), yield) }
}

// RecordFact records a fact read at (subject, key), never as a bare
// declaration edge: an edge per subject would run every reader of a
// bag again on any stamp.
func (s *ReadSet) RecordFact(subject symbol.Identity, key meta.KeyName) {
	if s.facts == nil {
		s.facts = map[factRead]struct{}{}
	}
	s.facts[factRead{subject: subject, key: key}] = struct{}{}
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
	dst = slices.Grow(dst, len(s.identities)+len(s.packages)+len(s.facts))
	start := len(dst)
	for id := range s.identities {
		dst = append(dst, meta.Read{Subject: id})
	}
	for id := range s.packages {
		dst = append(dst, meta.Read{Subject: id})
	}
	decls := dst[start:]
	slices.SortFunc(decls, func(a, b meta.Read) int { return a.Subject.Compare(b.Subject) })
	dst = dst[:start+len(slices.CompactFunc(decls, func(a, b meta.Read) bool { return a.Subject == b.Subject }))]
	facts := len(dst)
	for edge := range s.facts {
		dst = append(dst, meta.Read{Subject: edge.subject, Key: edge.key})
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
	return func(yield func(directive.Name) bool) {
		each(sortedKeys(s.directives, cmp.Compare[directive.Name]), yield)
	}
}

// Len returns how many edges the set contains, all five grains
// counted.
func (s *ReadSet) Len() int {
	return len(s.identities) + len(s.packages) + len(s.kinds) + len(s.facts) + len(s.directives)
}

// eachFact calls yield with every recorded fact read, in subject then
// key order, until yield returns false.
func (s *ReadSet) eachFact(yield func(symbol.Identity, meta.KeyName) bool) {
	edges := sortedKeys(s.facts, func(a, b factRead) int {
		return cmp.Or(a.subject.Compare(b.subject), cmp.Compare(a.key, b.key))
	})
	for _, edge := range edges {
		if !yield(edge.subject, edge.key) {
			return
		}
	}
}

// recordIdentity records a declaration edge.
//
// A read that returned nothing records too: the artifact asked for a
// declaration, so it has to run again when one appears under that
// identity.
func (s *ReadSet) recordIdentity(id symbol.Identity) {
	if s.identities == nil {
		s.identities = map[symbol.Identity]struct{}{}
	}
	s.identities[id] = struct{}{}
}

// recordPackage records a package edge.
func (s *ReadSet) recordPackage(id symbol.Identity) {
	if s.packages == nil {
		s.packages = map[symbol.Identity]struct{}{}
	}
	s.packages[id] = struct{}{}
}

// reserve sizes the declaration grain for an enumeration about to
// record n edges, so a large read allocates its map once and does not
// grow it entry by entry. It sizes only a grain that has no map yet,
// and a grain that already has edges keeps its map.
func (s *ReadSet) reserve(n int) {
	if s.identities == nil && n > 0 {
		s.identities = make(map[symbol.Identity]struct{}, n)
	}
}

// recordDirective records a membership edge by directive.
func (s *ReadSet) recordDirective(n directive.Name) {
	if s.directives == nil {
		s.directives = map[directive.Name]struct{}{}
	}
	s.directives[n] = struct{}{}
}

// recordKind records a membership edge by kind.
func (s *ReadSet) recordKind(k symbol.Kind) {
	if s.kinds == nil {
		s.kinds = map[symbol.Kind]struct{}{}
	}
	s.kinds[k] = struct{}{}
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
