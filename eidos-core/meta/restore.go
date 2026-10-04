// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"fmt"
	"reflect"

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
	// source was recorded, in identity order.
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
// merges the recorded presence with the transitions of the run. A
// restored claim ranks against the run's own claims exactly as it did
// when it was made, because its envelope is recorded whole.
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
