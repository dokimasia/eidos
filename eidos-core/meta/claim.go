// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"cmp"
	"strconv"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Authority orders who a write speaks for. A human override at the
// source declaration outranks any inference, whatever order the
// plugins ran in. Manual is reserved for consumer tooling. Nothing
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

// String returns the name of the authority in a report: plugin, directive
// or manual. For a value outside the three authorities, it returns the
// number in the form Authority(n). It allocates nothing for a declared
// authority.
func (a Authority) String() string {
	switch a {
	case AuthorityPlugin:
		return "plugin"
	case AuthorityDirective:
		return "directive"
	case AuthorityManual:
		return "manual"
	default:
		return "Authority(" + strconv.Itoa(int(a)) + ")"
	}
}

// Order is a claim's place among the claims of one plugin in one
// bucket: the rule that made it, the subject of the invocation that made
// it, and the gating instance. A frontend's stamp has rule zero and the
// stamp's index among its subject's stamps as its instance. A drop has
// rule zero and the directive's instance.
//
// Every run computes the same Order for the same claim, whatever else
// the run executes, so a claim a warm run keeps ranks against one it
// makes. The zero Order is rule zero on no subject at instance zero.
type Order struct {
	// Rule is the rule's place in its plugin's declaration order.
	Rule int
	// Subject is the identity of the invocation's subject, which for a
	// claim on a declaration the subject declares is the subject, not
	// the claim's own.
	Subject symbol.Identity
	// Instance is the gating directive instance's place among the
	// subject's instances of its directive, zero for a rule without one.
	Instance int
}

// Compare orders two places in canonical match order: the rule, then the
// subject identity, then the instance. It returns a negative number,
// zero or a positive one as o sorts before, with or after other, and
// allocates nothing.
func (o Order) Compare(other Order) int {
	return cmp.Or(
		cmp.Compare(o.Rule, other.Rule),
		o.Subject.Compare(other.Subject),
		cmp.Compare(o.Instance, other.Instance),
	)
}

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
	// Order is the claim's place in canonical match order, the last
	// step of the rank.
	Order Order

	// Pos locates the authoring carrier: a directive's position, or
	// zero for a plugin's inference.
	Pos position.Pos
	// Derived lists the point reads the writer made before the
	// claim: each declaration read by identity, and each fact read
	// at a subject and a key, in the read set's order. A
	// set-membership read, by kind or by directive, is not in it.
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
// rank the same claim first, which is what lets a parallel run
// report what a serial one does. Two claims from one rank source
// compare equal, and the store refuses those unless they state one
// value.
func rank(c, other Claim) int {
	return cmp.Or(
		// Higher authority ranks first, so the comparison flips.
		cmp.Compare(other.Authority, c.Authority),
		// The earlier bucket ranks first.
		cmp.Compare(c.Bucket, other.Bucket),
		// The alphabetically first plugin ranks first.
		cmp.Compare(c.Plugin, other.Plugin),
		// The first claim ranks first, in canonical match order.
		c.Order.Compare(other.Order),
	)
}

// outranks reports whether c ranks before other.
func (c Claim) outranks(other Claim) bool { return rank(c, other) < 0 }

// sameRankSource reports whether two claims come from one source at
// one rank, which is what an idempotent re-stamp repeats and what a
// withdrawal names.
func (c Claim) sameRankSource(other Claim) bool {
	return c.Authority == other.Authority && c.Bucket == other.Bucket &&
		c.Plugin == other.Plugin && c.Order == other.Order
}
