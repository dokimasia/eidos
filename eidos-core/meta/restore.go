// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"cmp"
	"fmt"
	"iter"
	"maps"
	"reflect"
	"slices"

	"go.dokimi.dev/eidos/core/symbol"
)

// BagSource restores a subject's recorded claims the first time a read,
// a write or a withdrawal needs the subject's bag. The sealed state of
// a previous run implements it.
type BagSource interface {
	// Claims returns every recorded claim on the subject, its drops and
	// its group drops included, and none for a subject nothing claimed.
	Claims(subject symbol.Identity) ([]StoredClaim, error)
	// Present returns the subjects on which a key read present when the
	// source was recorded, in identity order. It lists every subject whose
	// recorded claims make the key read present, and no other subject,
	// because a restored store reads the key absent on a subject that the
	// list leaves out without loading the subject's claims.
	Present(k KeyName) ([]symbol.Identity, error)
}

// StoredClaim is one recorded claim: its key or its group, its
// envelope, and its value, nil for a drop. A group drop names its group
// and no key.
type StoredClaim struct {
	Key   KeyName
	Group GroupName
	Claim Claim
	Value any
	Drop  bool
}

// Restore returns a fact store over r that restores bags from src on
// first use: a read, a write or a withdrawal of a subject loads the
// subject's recorded claims before it proceeds, and [Facts.ByKey]
// merges the recorded presence with the transitions of the run. A read
// of a fact through [Get] or [Fact] on a subject that the run has not
// restored or written does not load the subject's claims where the
// recorded presence of the key leaves the subject out, and reads the fact
// absent. A restored
// claim ranks against the run's own claims exactly as it did when it was
// made, because its envelope is recorded whole.
//
// A failure of the source is not returned by the read that met it: the
// bag reads as empty, and [Facts.Damaged] returns the failure, so the
// run discards what it derived and runs cold. The source is not asked
// for the presence of a key nothing registered.
//
// # Allocation contract
//
// Restore allocates what [NewFacts] does and the function that loads a
// key's recorded presence: three allocations.
func Restore(r *Registry, src BagSource) *Facts {
	f := NewFacts(r)
	f.source = src
	f.index.recorded = func(k KeyID) []symbol.Identity {
		name, registered := r.nameOf(k)
		if !registered {
			return nil
		}
		ids, err := src.Present(name)
		if err != nil {
			f.damaged(fmt.Errorf("meta: restore the presence of %s: %w", name, err))
			return nil
		}
		return ids
	}
	return f
}

// restoreBag fills a subject's bag from the recorded source and ranks
// each restored state. A claim under a key the registry does not
// resolve, or with a value of another type than the key's, damages the
// store, and the bag keeps the claims that restored.
func (f *Facts) restoreBag(id symbol.Identity, b *bag) {
	claims, err := f.source.Claims(id)
	if err != nil {
		f.damaged(fmt.Errorf("meta: restore the claims on %s: %w", id, err))
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, sc := range claims {
		entry := stored{claim: sc.Claim, value: cloneValue(sc.Value), drop: sc.Drop, restored: true}
		if sc.Group != "" {
			entry.drop = true
			b.group(sc.Group).add(entry)
			continue
		}
		k, registered := f.registry.Resolve(sc.Key)
		switch {
		case !registered:
			f.damaged(fmt.Errorf("meta: restore the claims on %s: %s names a key nothing registered", id, sc.Key))
			continue
		case !sc.Drop && reflect.TypeOf(sc.Value) != f.registry.typeOf(k):
			f.damaged(fmt.Errorf("meta: restore the claims on %s: %s is a %s key, and the value is a %T",
				id, sc.Key, f.registry.typeOf(k), sc.Value))
			continue
		}
		b.key(k).add(entry)
	}
	if b.hasFirst {
		b.first.rewin()
	}
	for _, state := range b.perKey {
		state.rewin()
	}
	for _, state := range b.perGroup {
		state.rewin()
	}
}

// Bags enumerates every bag that keeps a claim, each subject once with
// its claims in the form [BagSource.Claims] returns them: the claims of
// each key, keys in name order, then the drops of each group, groups in
// name order, and the claims of one fact or one group in rank order. A
// source that returns these claims restores a store that reads, ranks
// and withdraws them as this one does. Subjects arrive in no fixed
// order. In a store [Restore] returned, the bags are the ones the run
// restored or wrote.
//
// A list value and a claim's derivation are copies, so a caller cannot
// write into a bag. The yielded slice is the range's own until the body
// returns, and the next bag reuses its storage.
//
// # Concurrency
//
// A range takes each bag's read lock while it copies the bag, so it
// runs beside writes and yields each bag as one write left it.
//
// # Allocation contract
//
// A range allocates the slice it yields, which grows to the largest bag,
// the list of the keys of a bag of more than four, the list of a bag's
// groups, and a copy of each list value and of each non-empty
// derivation: one allocation for a store of bags of one claim without a
// derivation. The returned function inlines into the range.
func (f *Facts) Bags() iter.Seq2[symbol.Identity, []StoredClaim] {
	return func(yield func(symbol.Identity, []StoredClaim) bool) { f.eachBag(yield) }
}

// eachBag yields each bag that keeps a claim, in the form [Facts.Bags]
// documents, and stops when yield returns false.
func (f *Facts) eachBag(yield func(symbol.Identity, []StoredClaim) bool) {
	var claims []StoredClaim
	f.bags.Range(func(key, value any) bool {
		id, _ := key.(symbol.Identity)
		b, _ := value.(*bag)
		claims = f.appendBag(claims[:0], b)
		return len(claims) == 0 || yield(id, claims)
	})
}

// appendBag appends a bag's claims to dst in the order [Facts.Bags]
// documents, under the bag's read lock. A state exists only under a key
// the registry resolved, so every key's name is registered.
func (f *Facts) appendBag(dst []StoredClaim, b *bag) []StoredClaim {
	b.mu.RLock()
	defer b.mu.RUnlock()

	type keyed struct {
		name  KeyName
		state *factState
	}
	var inline [4]keyed
	keys := inline[:0]
	if b.hasFirst {
		name, _ := f.registry.nameOf(b.firstID)
		keys = append(keys, keyed{name: name, state: &b.first})
	}
	for k, state := range b.perKey {
		name, _ := f.registry.nameOf(k)
		keys = append(keys, keyed{name: name, state: state})
	}
	slices.SortFunc(keys, func(a, b keyed) int { return cmp.Compare(a.name, b.name) })
	for _, k := range keys {
		dst = k.state.appendClaims(dst, k.name, "")
	}
	if len(b.perGroup) == 0 {
		return dst
	}
	for _, g := range slices.Sorted(maps.Keys(b.perGroup)) {
		dst = b.perGroup[g].appendClaims(dst, "", g)
	}
	return dst
}

// appendClaims appends the state's claims to dst in rank order, each
// under the key or the group the state keeps, with a copy of its
// derivation and of its value.
func (s *factState) appendClaims(dst []StoredClaim, k KeyName, g GroupName) []StoredClaim {
	start := len(dst)
	for _, held := range s.claims {
		claim := held.claim
		claim.Derived = slices.Clone(claim.Derived)
		dst = append(dst, StoredClaim{Key: k, Group: g, Claim: claim, Value: cloneValue(held.value), Drop: held.drop})
	}
	slices.SortFunc(dst[start:], func(a, b StoredClaim) int { return rank(a.Claim, b.Claim) })
	return dst
}
