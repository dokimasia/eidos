// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package store

import (
	"cmp"
	"iter"
	"maps"
	"slices"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// ReadSet is what one derived artifact read.
//
// Edges deduplicate, so a loop reading one declaration a thousand
// times records one edge. Three grains are recorded: a per-identity
// edge for a targeted read, a set-membership edge for an
// enumeration, and a (subject, key) edge for a fact read — ReadSet
// satisfies [meta.Recorder], so one artifact's declaration reads
// and fact reads land in one set.
//
// A ReadSet belongs to one derived artifact and is not shared, so it
// is not safe for concurrent use even though the graph beneath it
// is. That is the honest split: the graph is shared and the
// bookkeeping is not.
//
// The zero ReadSet is ready to record. [NewReadSet] is the spelling
// that says so.
type ReadSet struct {
	identities map[symbol.Identity]struct{}
	kinds      map[symbol.Kind]struct{}
	facts      map[factRead]struct{}
}

// factRead is one (subject, key) edge.
type factRead struct {
	subject symbol.Identity
	key     meta.KeyName
}

// NewReadSet answers a read set holding no edges.
func NewReadSet() *ReadSet { return &ReadSet{} }

// Identities answers every per-identity edge, in identity order.
//
// The order is the set's own rather than the order the reads
// arrived, so two runs reading the same declarations in different
// orders answer alike.
func (s *ReadSet) Identities() iter.Seq[symbol.Identity] {
	out := slices.Collect(maps.Keys(s.identities))
	slices.SortFunc(out, symbol.Identity.Compare)
	return slices.Values(out)
}

// Kinds answers every set-membership edge, in kind order.
func (s *ReadSet) Kinds() iter.Seq[symbol.Kind] {
	out := slices.Sorted(maps.Keys(s.kinds))
	return slices.Values(out)
}

// RecordFact records a fact read at (subject, key), never as a bare
// identity edge: per-subject recording would re-run every reader of
// a bag on any stamp.
func (s *ReadSet) RecordFact(subject symbol.Identity, key meta.KeyName) {
	if s.facts == nil {
		s.facts = map[factRead]struct{}{}
	}
	s.facts[factRead{subject: subject, key: key}] = struct{}{}
}

// Facts answers every recorded fact read, in subject then key
// order.
func (s *ReadSet) Facts() iter.Seq2[symbol.Identity, meta.KeyName] {
	edges := slices.SortedFunc(maps.Keys(s.facts), func(a, b factRead) int {
		if by := a.subject.Compare(b.subject); by != 0 {
			return by
		}
		return cmp.Compare(a.key, b.key)
	})
	return func(yield func(symbol.Identity, meta.KeyName) bool) {
		for _, edge := range edges {
			if !yield(edge.subject, edge.key) {
				return
			}
		}
	}
}

// Len answers how many edges the set holds, all three grains
// counted.
func (s *ReadSet) Len() int { return len(s.identities) + len(s.kinds) + len(s.facts) }

// recordIdentity records a per-identity edge.
//
// A read that answered nothing records too: the artifact asked for a
// declaration, so it has to run again when one appears under that
// identity.
func (s *ReadSet) recordIdentity(id symbol.Identity) {
	if s.identities == nil {
		s.identities = map[symbol.Identity]struct{}{}
	}
	s.identities[id] = struct{}{}
}

// reserve sizes the identity grain for an enumeration about to
// record n edges, so a large read allocates its map once rather than
// growing it entry by entry. It only helps a set holding nothing
// yet; one that already holds edges keeps its map.
func (s *ReadSet) reserve(n int) {
	if s.identities == nil && n > 0 {
		s.identities = make(map[symbol.Identity]struct{}, n)
	}
}

// recordKind records a set-membership edge.
func (s *ReadSet) recordKind(k symbol.Kind) {
	if s.kinds == nil {
		s.kinds = map[symbol.Kind]struct{}{}
	}
	s.kinds[k] = struct{}{}
}
