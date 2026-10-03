// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"cmp"
	"slices"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/symbol"
)

// FactRef names one fact: a subject and a key.
type FactRef struct {
	Subject symbol.Identity
	Key     KeyName
}

// Compare orders two facts by subject, then by key, returning a negative
// number, zero or a positive one as r sorts before, with or after other.
func (r FactRef) Compare(other FactRef) int {
	return cmp.Or(r.Subject.Compare(other.Subject), cmp.Compare(r.Key, other.Key))
}

// ClaimedBy returns the facts a plugin claimed through this store in
// the current run, sorted by subject, then key, each once. A claim a
// recorded source restored is not the current run's, and a group drop
// names no fact. It walks every bag the run touched, so it costs one
// pass over the run's facts: what a run asks once per phase call of a
// plugin that journals no invocation.
func (f *Facts) ClaimedBy(p diag.Origin) []FactRef {
	var out []FactRef
	f.bags.Range(func(key, value any) bool {
		id, _ := key.(symbol.Identity)
		b, _ := value.(*bag)
		b.mu.RLock()
		if b.hasFirst {
			out = f.claimedIn(out, id, b.firstID, &b.first, p)
		}
		for k, state := range b.perKey {
			out = f.claimedIn(out, id, k, state, p)
		}
		b.mu.RUnlock()
		return true
	})
	slices.SortFunc(out, FactRef.Compare)
	return out
}

// claimedIn appends the fact of one key's state where the plugin made a
// claim on it in the current run. The caller has locked the bag.
func (f *Facts) claimedIn(out []FactRef, id symbol.Identity, k KeyID, state *factState, p diag.Origin) []FactRef {
	for _, held := range state.claims {
		if !held.restored && held.claim.Plugin == p {
			return append(out, FactRef{Subject: id, Key: f.nameOf[k]})
		}
	}
	return out
}
