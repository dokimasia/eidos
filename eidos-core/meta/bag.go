// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package meta

import (
	"fmt"
	"sync"
)

// stored is one claim as a bag holds it: the envelope, the value,
// and nil for a drop.
type stored struct {
	claim Claim
	value any
	drop  bool
}

// factState is every claim on one (subject, key) or (subject,
// group), with the winner cached at write time so a read is one
// lookup.
type factState struct {
	claims []stored
	// winner indexes claims; -1 while nothing claimed.
	winner int
}

// bag is one subject's facts. Writes and arbitration serialize on
// the write lock; reads share it, so the write-free generate phase
// reads one bag in parallel.
type bag struct {
	mu sync.RWMutex
	// Most bags hold one key, so the first key's state lives
	// inline and perKey spills on the second: a single-key bag
	// costs no map and no separate state object. perGroup holds
	// group drops and is created on the first drop. A key's
	// presence considers both.
	firstID  KeyID
	hasFirst bool
	first    factState
	perKey   map[KeyID]*factState
	perGroup map[GroupName]*factState
}

// state returns the bag's state for k and false where the key was
// never written. The caller holds b.mu.
func (b *bag) state(k KeyID) (*factState, bool) {
	if b.hasFirst && b.firstID == k {
		return &b.first, true
	}
	state, held := b.perKey[k]
	return state, held
}

// presentLocked reports whether key k reads present on b, group
// tombstones considered. The caller holds b.mu.
func (b *bag) presentLocked(group GroupName, k KeyID) bool {
	state, held := b.state(k)
	if !held || state.winner < 0 {
		return false
	}
	best := state.claims[state.winner]

	if group != "" {
		if tombs, dropped := b.perGroup[group]; dropped && tombs.winner >= 0 {
			if tombs.claims[tombs.winner].claim.outranks(best.claim) {
				return false
			}
		}
	}
	return !best.drop
}

// key returns the bag's state for k, creating it on first touch:
// inline for the bag's first key, in the spill map for the rest,
// so a bag costs its writes alone. The caller holds b.mu.
func (b *bag) key(k KeyID) *factState {
	if state, held := b.state(k); held {
		return state
	}
	if !b.hasFirst {
		b.firstID, b.hasFirst = k, true
		b.first = factState{winner: -1}
		return &b.first
	}
	if b.perKey == nil {
		b.perKey = map[KeyID]*factState{}
	}
	state := &factState{winner: -1}
	b.perKey[k] = state
	return state
}

// group returns the bag's state for a group tombstone, creating the
// map and the state on first touch. The caller holds b.mu.
func (b *bag) group(g GroupName) *factState {
	if b.perGroup == nil {
		b.perGroup = map[GroupName]*factState{}
	}
	state, held := b.perGroup[g]
	if !held {
		state = &factState{winner: -1}
		b.perGroup[g] = state
	}
	return state
}

// admit appends one claim and re-ranks the winner. An identical
// claim, same rank source and equal value, changes nothing; a claim
// from the same source carrying a different value is an error,
// because rank could not order the two and arrival would.
//
// The dedupe scan is linear in the claims already held, which the
// channel's design bounds to a few claimants per fact: every write
// is a registered plugin, a directive or a manual override, not an
// open set.
func (s *factState) admit(entry stored) (bool, error) {
	for _, held := range s.claims {
		if !held.claim.sameRankSource(entry.claim) {
			continue
		}
		if held.drop == entry.drop && equalValue(held.value, entry.value) {
			return false, nil
		}
		return false, fmt.Errorf(
			"claims twice from one source with two values: rank cannot order them",
		)
	}
	s.claims = append(s.claims, entry)
	if s.winner < 0 || entry.claim.outranks(s.claims[s.winner].claim) {
		s.winner = len(s.claims) - 1
	}
	return true, nil
}
