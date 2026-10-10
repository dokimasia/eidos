// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"iter"
	"slices"

	"go.dokimi.dev/eidos/core/symbol"
)

// ClaimView is one claim as the record shows it.
type ClaimView struct {
	Claim Claim
	// Value is the claimed value, nil for a drop.
	Value any
	// Won marks the claim rank selected.
	Won bool
}

// Claims enumerates every claim on (subject, key) when the range
// starts, in rank order, winner first, group drops covering the key
// included. This is the record attribution walks: a losing write stays
// visible instead of mysterious.
//
// A view contains copies of the claim's value and provenance, so a
// caller that edits either leaves the store unchanged.
//
// # Allocation contract
//
// A range over the result allocates the list of views, one allocation,
// and a copy of each list value and of each non-empty provenance. It
// allocates nothing for a fact never claimed. The returned function
// inlines into the range, so the range's body allocates nothing.
func (f *Facts) Claims(id symbol.Identity, k KeyID) iter.Seq[ClaimView] {
	return func(yield func(ClaimView) bool) { f.eachClaim(id, k, yield) }
}

// eachClaim yields a view of every claim on (subject, key) in rank
// order. It stops when yield returns false.
func (f *Facts) eachClaim(id symbol.Identity, k KeyID, yield func(ClaimView) bool) {
	for _, view := range f.views(id, k) {
		if !yield(view) {
			return
		}
	}
}

// views returns a view of every claim on (subject, key) in rank order,
// with the winner first and marked, and nil for a fact never claimed.
func (f *Facts) views(id symbol.Identity, k KeyID) []ClaimView {
	b, held := f.peek(id)
	if !held {
		return nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()

	var keyed, dropped []stored
	if state, held := b.state(k); held {
		keyed = state.claims
	}
	if group := f.group(k); group != "" {
		if state, held := b.perGroup[group]; held {
			dropped = state.claims
		}
	}
	if len(keyed)+len(dropped) == 0 {
		return nil
	}
	views := make([]ClaimView, 0, len(keyed)+len(dropped))
	views = appendViews(views, keyed)
	views = appendViews(views, dropped)
	slices.SortFunc(views, func(a, b ClaimView) int { return rank(a.Claim, b.Claim) })
	views[0].Won = true
	return views
}

// appendViews appends a view of each kept claim, with copies of its
// value and its provenance.
func appendViews(views []ClaimView, claims []stored) []ClaimView {
	for _, entry := range claims {
		claim := entry.claim
		claim.Derived = slices.Clone(claim.Derived)
		views = append(views, ClaimView{Claim: claim, Value: cloneValue(entry.value)})
	}
	return views
}
