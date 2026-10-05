// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"crypto/sha256"
	"encoding/binary"
	"slices"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/wire"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The tag an edge's spelling opens with, one for each grain, so two
// edges of different grains never spell alike.
const (
	edgeDeclaration = 'd'
	edgePackage     = 'p'
	edgeKind        = 'k'
	edgeDirective   = 'r'
	edgeFact        = 'f'
)

// spellingCap is the capacity of the stack buffer an edge or a record's
// key is spelled in, so a spelling allocates only where it is longer.
const spellingCap = 256

// EdgeHash is the 64-bit hash of one edge a validation, an invocation
// or a check read: the first eight bytes, big-endian, of the SHA-256 of
// the edge's spelling, which is a tag naming the edge's grain followed by
// the record encoding of the edge's fields. A read record lists its edges
// by hash, and the readers table is keyed by it. Two edges that share a
// hash read as one, so a run executes more than an edit requires and
// never less.
type EdgeHash uint64

// DeclarationEdge returns the hash of the declaration edge on id: what a
// targeted read of a declaration records, and what every validation and
// invocation records for its subject. It allocates nothing for an
// identity whose spelling fits in 255 bytes.
func DeclarationEdge(id symbol.Identity) EdgeHash {
	var spelling [spellingCap]byte
	return hashOf(appendIdentity(append(spelling[:0], edgeDeclaration), id))
}

// PackageEdge returns the hash of the package edge on the package id
// names: what a reader that took the package whole records. It allocates
// nothing for an identity whose spelling fits in 255 bytes.
func PackageEdge(id symbol.Identity) EdgeHash {
	var spelling [spellingCap]byte
	return hashOf(appendIdentity(append(spelling[:0], edgePackage), id))
}

// KindEdge returns the hash of the membership edge on a declaration
// kind: what an enumeration by kind records. It allocates nothing.
func KindEdge(k symbol.Kind) EdgeHash {
	var spelling [spellingCap]byte
	return hashOf(binary.AppendUvarint(append(spelling[:0], edgeKind), uint64(k)))
}

// DirectiveEdge returns the hash of the membership edge on a directive's
// spelling: what an enumeration by directive records. It allocates
// nothing for a spelling that fits in 250 bytes.
func DirectiveEdge(n directive.Name) EdgeHash {
	var spelling [spellingCap]byte
	return hashOf(wire.AppendText(append(spelling[:0], edgeDirective), string(n)))
}

// FactEdge returns the hash of the fact edge at (subject, key): what a
// fact read records, whether the fact read present or absent. It
// allocates nothing for a spelling that fits in 255 bytes.
func FactEdge(subject symbol.Identity, key meta.KeyName) EdgeHash {
	var spelling [spellingCap]byte
	b := appendIdentity(append(spelling[:0], edgeFact), subject)
	return hashOf(wire.AppendText(b, string(key)))
}

// appendIdentity appends an identity's six fields to dst as the record
// format spells them, where no string table numbers the strings: the
// language, the package, the owner and the name, each behind its length,
// the kind as an unsigned varint, and the discriminator behind its
// length. It keeps nothing of dst, so a spelling built over a buffer on
// the stack does not move to the heap.
func appendIdentity(dst []byte, id symbol.Identity) []byte {
	dst = wire.AppendText(dst, string(id.Lang))
	dst = wire.AppendText(dst, id.Package)
	dst = wire.AppendText(dst, id.Owner)
	dst = wire.AppendText(dst, id.Name)
	dst = binary.AppendUvarint(dst, uint64(id.Kind))
	return wire.AppendText(dst, id.Disc)
}

// hashOf returns the hash of a spelling: the first eight bytes of its
// SHA-256, big-endian.
func hashOf(spelling []byte) EdgeHash {
	sum := sha256.Sum256(spelling)
	return EdgeHash(binary.BigEndian.Uint64(sum[:8]))
}

// appendEdges appends the hash of every edge s records to dst, grain by
// grain, and returns the extended slice. A nil set records nothing. It
// allocates only to grow dst, and for the sorted list of a grain the set
// keeps in maps.
func appendEdges(dst []EdgeHash, s *store.ReadSet) []EdgeHash {
	if s == nil {
		return dst
	}
	for id := range s.Identities() {
		dst = append(dst, DeclarationEdge(id))
	}
	for id := range s.Packages() {
		dst = append(dst, PackageEdge(id))
	}
	for k := range s.Kinds() {
		dst = append(dst, KindEdge(k))
	}
	for n := range s.Directives() {
		dst = append(dst, DirectiveEdge(n))
	}
	for subject, key := range s.Facts() {
		dst = append(dst, FactEdge(subject, key))
	}
	return dst
}

// readRecord sorts a record's edge hashes and drops the repeats, in
// place, and returns the record: what a validation, an invocation or a
// check lists as the edges it read.
func readRecord(edges []EdgeHash) []EdgeHash {
	slices.Sort(edges)
	return slices.Compact(edges)
}
