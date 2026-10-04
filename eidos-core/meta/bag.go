// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"fmt"
	"slices"
	"sync"
)

// stored is one claim as a bag keeps it: the envelope, the value, nil
// for a drop, and whether a recorded source restored it, where a write
// of the current run did not make it.
type stored struct {
	claim    Claim
	value    any
	drop     bool
	restored bool
}

// factState is every claim on one (subject, key) or (subject,
// group), with the winner cached at write time so a read is one
// lookup.
//
// The first claim is stored in one, which claims points into until a
// second claim arrives, so a fact with one claimant costs no slice of
// its own. A state is therefore never copied once it keeps a claim: a
// bag holds its first state in place and every other state through a
// pointer.
type factState struct {
	claims []stored
	// winner indexes claims; -1 while nothing claimed.
	winner int
	one    [1]stored
}

// bag is one subject's facts. Writes and arbitration serialize on the
// write lock. Reads share it, so the write-free generate phase reads one
// bag in parallel.
type bag struct {
	mu sync.RWMutex
	// restore runs once, before the first read, write or withdrawal of
	// a bag in a store that [Restore] returned, and fills the bag from
	// the recorded source.
	restore sync.Once
	// Most bags contain one key, so the first key's state is inline
	// and perKey spills on the second: a single-key bag costs no map
	// and no separate state object. perGroup contains the group drops
	// and is created on the first drop. A key's presence considers
	// both.
	firstID  KeyID
	hasFirst bool
	first    factState
	perKey   map[KeyID]*factState
	perGroup map[GroupName]*factState
}

// state returns the bag's state for k and false where the key was
// never written. The caller has locked b.mu.
func (b *bag) state(k KeyID) (*factState, bool) {
	if b.hasFirst && b.firstID == k {
		return &b.first, true
	}
	state, held := b.perKey[k]
	return state, held
}

// presentLocked reports whether key k reads present on b, group
// tombstones considered. The caller has locked b.mu.
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
// so a bag costs its writes alone. The caller has locked b.mu.
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
// map and the state on first touch. The caller has locked b.mu.
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

// admit appends one claim and re-ranks the winner, and reports whether
// it appended. An identical claim, from the same rank source with an
// equal value, changes nothing. A claim from the same source with
// another value is an error, because rank could not order the two and
// arrival would.
//
// The dedupe scan is linear in the claims already kept, which the
// channel's design bounds to a few claimants per fact: every write
// is a registered plugin, a directive or a manual override, not an
// open set.
func (s *factState) admit(entry stored) (bool, error) {
	if held, kept := s.from(entry.claim); kept {
		if held.drop == entry.drop && equalValue(held.value, entry.value) {
			return false, nil
		}
		return false, fmt.Errorf(
			"claims twice from one source with two values: rank cannot order them",
		)
	}
	s.add(entry)
	if s.winner < 0 || entry.claim.outranks(s.claims[s.winner].claim) {
		s.winner = len(s.claims) - 1
	}
	return true, nil
}

// from returns the kept claim with c's rank source, and false where
// the state keeps none. A source has one claim at most, because admit
// refuses a second.
func (s *factState) from(c Claim) (*stored, bool) {
	for i := range s.claims {
		if s.claims[i].claim.sameRankSource(c) {
			return &s.claims[i], true
		}
	}
	return nil, false
}

// add appends one claim without ranking it: into the state's own
// storage where it keeps no claim yet, and into a slice of its own
// from the second claim on.
func (s *factState) add(entry stored) {
	if s.claims == nil {
		s.claims = s.one[:0]
	}
	s.claims = append(s.claims, entry)
}

// withdraw removes the claim that has c's rank source and ranks the
// remaining claims again, and reports whether it removed one. A source
// has one claim at most, because admit refuses a second.
func (s *factState) withdraw(c Claim) bool {
	at := slices.IndexFunc(s.claims, func(held stored) bool { return held.claim.sameRankSource(c) })
	if at < 0 {
		return false
	}
	s.claims = slices.Delete(s.claims, at, at+1)
	s.rewin()
	return true
}

// rewin finds the claim that ranks first among the kept claims: -1
// where the state keeps none. It allocates nothing.
func (s *factState) rewin() {
	s.winner = -1
	for i := range s.claims {
		if s.winner < 0 || s.claims[i].claim.outranks(s.claims[s.winner].claim) {
			s.winner = i
		}
	}
}
