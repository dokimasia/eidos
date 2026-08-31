// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package store

import (
	"cmp"
	"iter"
	"maps"
	"slices"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// ReadSet is what one derived artifact read.
//
// Edges deduplicate, so a loop reading one declaration a thousand
// times records one edge. Four grains are recorded: a per-identity
// edge for a targeted read, a set-membership edge for an
// enumeration by kind, a directive-membership edge for one by
// directive, and a (subject, key) edge for a fact read — ReadSet
// satisfies [meta.Recorder], so one artifact's declaration reads
// and fact reads arrive in one set.
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
	directives map[directive.Name]struct{}
}

// factRead is one (subject, key) edge.
type factRead struct {
	subject symbol.Identity
	key     meta.KeyName
}

// NewReadSet returns a read set holding no edges.
func NewReadSet() *ReadSet { return &ReadSet{} }

// Reset drops every recorded edge and keeps the storage, so a
// dispatcher reuses one set across a rule's invocations instead of
// building four maps per subject. The set records again
// immediately; only the edges are gone.
func (s *ReadSet) Reset() {
	clear(s.identities)
	clear(s.kinds)
	clear(s.facts)
	clear(s.directives)
}

// Identities returns every per-identity edge, in identity order.
//
// The order is the set's own rather than the order the reads
// arrived, so two runs reading the same declarations in different
// orders agree.
func (s *ReadSet) Identities() iter.Seq[symbol.Identity] {
	out := slices.Collect(maps.Keys(s.identities))
	slices.SortFunc(out, symbol.Identity.Compare)
	return slices.Values(out)
}

// Kinds returns every set-membership edge, in kind order.
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

// Facts returns every recorded fact read, in subject then key
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

// Directives returns every recorded directive-membership edge, in
// name order. Each edge means the artifact enumerated that
// spelling's carriers, so it runs again when a subject gains or
// loses the directive — and never when a carrier merely changes,
// which the per-identity edges recorded alongside cover.
func (s *ReadSet) Directives() iter.Seq[directive.Name] {
	out := slices.Sorted(maps.Keys(s.directives))
	return slices.Values(out)
}

// Len returns how many edges the set holds, all four grains
// counted.
func (s *ReadSet) Len() int {
	return len(s.identities) + len(s.kinds) + len(s.facts) + len(s.directives)
}

// recordIdentity records a per-identity edge.
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

// reserve sizes the identity grain for an enumeration about to
// record n edges, so a large read allocates its map once rather than
// growing it entry by entry. It only helps a set holding nothing
// yet; one that already holds edges keeps its map.
func (s *ReadSet) reserve(n int) {
	if s.identities == nil && n > 0 {
		s.identities = make(map[symbol.Identity]struct{}, n)
	}
}

// recordDirective records a directive-membership edge.
func (s *ReadSet) recordDirective(n directive.Name) {
	if s.directives == nil {
		s.directives = map[directive.Name]struct{}{}
	}
	s.directives[n] = struct{}{}
}

// recordKind records a set-membership edge.
func (s *ReadSet) recordKind(k symbol.Kind) {
	if s.kinds == nil {
		s.kinds = map[symbol.Kind]struct{}{}
	}
	s.kinds[k] = struct{}{}
}
