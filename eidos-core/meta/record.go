// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

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

// Claims answers every claim on (subject, key) in rank order,
// winner first, group drops covering the key included. This is the
// record attribution walks: a losing write stays visible instead of
// mysterious.
func (f *Facts) Claims(id symbol.Identity, k KeyID) iter.Seq[ClaimView] {
	b, held := f.peek(id)
	if !held {
		return func(func(ClaimView) bool) {}
	}
	b.mu.RLock()
	var all []stored
	if state, held := b.state(k); held {
		all = slices.Clone(state.claims)
	}
	if group := f.group(k); group != "" {
		if state, held := b.perGroup[group]; held {
			all = append(all, state.claims...)
		}
	}
	b.mu.RUnlock()

	slices.SortFunc(all, func(a, b stored) int { return rank(a.claim, b.claim) })
	views := make([]ClaimView, 0, len(all))
	for i, entry := range all {
		views = append(views, ClaimView{Claim: entry.claim, Value: entry.value, Won: i == 0})
	}
	return slices.Values(views)
}
