// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"cmp"
	"strconv"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Authority is the rank of the source of a write. A human override at
// the source declaration outranks any inference, in any order of the
// plugins. Manual is reserved for consumer tooling, and no write of a
// normal run has it.
//
// The zero value is [AuthorityPlugin], so a claim without an authority
// has the lowest one.
type Authority uint8

const (
	// AuthorityPlugin is the authority of a fact that a plugin stamped.
	AuthorityPlugin Authority = iota
	// AuthorityDirective is the authority of a write by a person at the
	// source declaration, a drop included.
	AuthorityDirective
	// AuthorityManual is the authority of consumer tooling, such as a
	// migration or a script that fixes directives. It outranks the
	// directives that the tooling rewrites.
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

// Order is the place of a claim among the claims of one plugin in one
// turn. It consists of the rule that made the claim, the subject of
// the invocation that made it, and the gating instance. The stamp of a
// frontend has rule zero, and its instance is its index among the stamps
// of its subject. A drop has rule zero and the instance of its
// directive.
//
// Every run computes the same Order for the same claim, whatever else
// the run executes. A claim that a warm run keeps therefore ranks
// correctly against a claim that the run makes. The zero Order is rule
// zero on no subject at instance zero.
type Order struct {
	// Rule is the place of the rule in the declaration order of its
	// plugin.
	Rule int
	// Subject is the identity of the subject of the invocation. For a
	// claim on a declaration that the subject declares, it is the
	// identity of the subject and not the identity of the declaration.
	Subject symbol.Identity
	// Instance is the place of the gating directive instance among the
	// instances of its directive on the subject. It is zero for a rule
	// without a gating directive.
	Instance int
}

// Compare orders two places in canonical match order, by the rule, then
// the identity of the subject, then the instance. It returns a negative
// number, zero or a positive number as o sorts before, with or after
// other, and allocates nothing.
func (o Order) Compare(other Order) int {
	return cmp.Or(
		cmp.Compare(o.Rule, other.Rule),
		o.Subject.Compare(other.Subject),
		cmp.Compare(o.Instance, other.Instance),
	)
}

// Claim is the envelope of one write. It contains the rank of the write
// and the provenance of the write.
type Claim struct {
	// Subject is the declaration that the fact is about. The kind
	// restriction of the key checks its Kind field.
	Subject symbol.Identity

	// These fields are the rank, highest first: the authority, then the
	// earlier turn in the schedule, then the name of the plugin, then the
	// first claim in canonical match order.
	Authority Authority
	Turn      int
	Plugin    diag.Origin
	// Order is the place of the claim in canonical match order, which is
	// the last step of the rank.
	Order Order

	// Pos is the position of the authoring carrier, such as the position
	// of a directive. It is zero for the inference of a plugin.
	Pos position.Pos
	// Derived lists the point reads that the writer made before the
	// claim, in the order of the read set. A point read is a read of a
	// declaration by identity or a read of a fact at a subject and a key.
	// A read of set membership, by kind or by directive, is not in the
	// list.
	Derived []Read
}

// LoadClaim returns the claim under which a run files the classification
// stamp s of subject. The index i is the place of s among the stamps of
// subject in load order. The claim has plugin authority, and its plugin
// is the origin of s. Its order has rule zero and i as its instance, and
// its position is the position of s. It allocates nothing.
func LoadClaim(subject symbol.Identity, i int, s RawStamp) Claim {
	return Claim{
		Subject:   subject,
		Authority: AuthorityPlugin,
		Plugin:    s.Origin,
		Order:     Order{Subject: subject, Instance: i},
		Pos:       s.Pos,
	}
}

// Read is one recorded read. It is a read of a declaration where Key is
// empty, and a read of the fact at the subject and the key otherwise.
type Read struct {
	Subject symbol.Identity
	Key     KeyName
}

// rank orders two claims under the four steps of the rank, best first.
//
// The order of arrival is not part of the rank. Two schedulings of the
// same claims rank the same claim first, so a parallel run reports the
// same result as a serial run. Two claims from one rank source compare
// equal, and the store refuses them unless they have the same value.
func rank(c, other Claim) int {
	return cmp.Or(
		// Higher authority ranks first, so the comparison flips.
		cmp.Compare(other.Authority, c.Authority),
		// The earlier turn ranks first.
		cmp.Compare(c.Turn, other.Turn),
		// The alphabetically first plugin ranks first.
		cmp.Compare(c.Plugin, other.Plugin),
		// The first claim ranks first, in canonical match order.
		c.Order.Compare(other.Order),
	)
}

// outranks reports whether c ranks before other.
func (c Claim) outranks(other Claim) bool { return rank(c, other) < 0 }

// sameRankSource reports whether two claims come from one source at
// one rank. An idempotent stamp repeats the claim of such a source, and
// a withdrawal identifies its claim by such a source.
func (c Claim) sameRankSource(other Claim) bool {
	return c.Authority == other.Authority && c.Turn == other.Turn &&
		c.Plugin == other.Plugin && c.Order == other.Order
}
