// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"cmp"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Authority orders who a write speaks for. A human override at the
// source declaration beats any inference, whatever order the
// plugins ran in. Manual is reserved for consumer tooling; nothing
// in a normal run writes at it.
//
// The zero value is [AuthorityPlugin]: a claim that returned no
// authority speaks with the least.
type Authority uint8

const (
	// AuthorityPlugin is an inference: a fact a plugin stamped.
	AuthorityPlugin Authority = iota
	// AuthorityDirective is a human's write at the source
	// declaration, a drop included.
	AuthorityDirective
	// AuthorityManual is consumer tooling: a migration or a fix-it
	// script that outranks even the directives it rewrites.
	AuthorityManual
)

// Claim is one write's envelope: the rank that arbitrates it and
// the provenance that explains it.
type Claim struct {
	// Subject is the declaration the fact is about. Its Kind field
	// is what the key's kind restriction checks.
	Subject symbol.Identity

	// The rank, highest first: authority, then the earlier
	// capability bucket, then the plugin name, then the first claim
	// in canonical match order.
	Authority Authority
	Bucket    int
	Plugin    diag.Origin
	Seq       int

	// Pos locates the authoring carrier: a directive's position, or
	// zero for a plugin's inference.
	Pos position.Pos
	// Derived is what produced the value: the reads the writer
	// made. It is the same edge attribution walks and invalidation
	// follows, so neither can drift from the other.
	Derived []Read
}

// Read is one recorded read: a declaration read when Key is empty,
// a fact read at (subject, key) otherwise.
type Read struct {
	Subject symbol.Identity
	Key     KeyName
}

// rank orders two claims under the four-step rank, best first.
//
// Arrival order appears nowhere: two schedulings of the same claims
// pick the same winner, which is what lets a parallel run report
// what a serial one does. Two claims from one rank source compare
// equal, and the store refuses those unless they carry one value.
func rank(c, other Claim) int {
	return cmp.Or(
		// Higher authority wins, so the comparison flips.
		cmp.Compare(other.Authority, c.Authority),
		// The earlier bucket wins.
		cmp.Compare(c.Bucket, other.Bucket),
		// The alphabetically first plugin wins.
		cmp.Compare(c.Plugin, other.Plugin),
		// The first claim wins, in canonical match order.
		cmp.Compare(c.Seq, other.Seq),
	)
}

// outranks reports whether c beats other under the rank.
func (c Claim) outranks(other Claim) bool { return rank(c, other) < 0 }

// sameRankSource reports whether two claims come from one source at
// one rank, which is what an idempotent re-stamp repeats.
func (c Claim) sameRankSource(other Claim) bool {
	return c.Authority == other.Authority && c.Bucket == other.Bucket &&
		c.Plugin == other.Plugin && c.Seq == other.Seq
}
