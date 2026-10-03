// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"maps"
	"slices"
	"sync"

	"go.dokimi.dev/eidos/core/symbol"
)

// factIndex returns the subjects on which each key reads present. It
// is maintained at stamp time so a fact-gated rule visits its matches
// and not the whole graph, and it has a lock of its own: presence
// transitions arrive from many bags at once.
type factIndex struct {
	mu     sync.Mutex
	perKey map[KeyID]*keyIndex
	// recorded returns the subjects on which a key read present when a
	// recorded source was written, nil for an index that restores
	// nothing. The index reads it once per key, on the key's first
	// enumeration.
	recorded func(k KeyID) []symbol.Identity
}

// keyIndex is one key's presence set with its enumeration cached:
// dispatch enumerates once per rule, so the sort happens once per
// transition and not once per call.
type keyIndex struct {
	members map[symbol.Identity]struct{}
	// sorted is the cached enumeration, rebuilt when stale. A rebuild
	// replaces the slice and never mutates it, so a handed-out
	// iterator keeps its snapshot.
	sorted []symbol.Identity
	stale  bool
	// loaded reports that members contain the recorded presence, which
	// an index that restores nothing has from the start. pending
	// contains the transitions recorded before it loaded, by subject,
	// each subject's last transition replacing the earlier ones.
	loaded  bool
	pending map[symbol.Identity]bool
}

// newFactIndex returns an empty index, which loads each key's
// recorded presence from recorded on the key's first enumeration where
// recorded is set.
func newFactIndex(recorded func(KeyID) []symbol.Identity) *factIndex {
	return &factIndex{perKey: map[KeyID]*keyIndex{}, recorded: recorded}
}

// record notes a presence transition for (subject, key). A
// transition marks the cached enumeration stale; a write that
// changes nothing does not. A transition that arrives before the key's
// recorded presence loaded waits until the enumeration merges it.
func (x *factIndex) record(id symbol.Identity, k KeyID, present bool) {
	x.mu.Lock()
	defer x.mu.Unlock()

	entry := x.perKey[k]
	if entry == nil {
		if !present && x.recorded == nil {
			return
		}
		entry = &keyIndex{members: map[symbol.Identity]struct{}{}, loaded: x.recorded == nil}
		x.perKey[k] = entry
	}
	if !entry.loaded {
		if entry.pending == nil {
			entry.pending = map[symbol.Identity]bool{}
		}
		entry.pending[id] = present
		entry.stale = true
		return
	}
	if present {
		if _, held := entry.members[id]; held {
			return
		}
		entry.members[id] = struct{}{}
	} else {
		if _, held := entry.members[id]; !held {
			return
		}
		delete(entry.members, id)
	}
	entry.stale = true
}

// enumerate returns the subjects presently carrying k, in identity
// order, from the cache where it is fresh. The first enumeration of a
// key in an index that restores loads the recorded presence and merges
// the transitions recorded before it.
func (x *factIndex) enumerate(k KeyID) []symbol.Identity {
	x.mu.Lock()
	defer x.mu.Unlock()

	entry := x.perKey[k]
	if entry == nil {
		if x.recorded == nil {
			return nil
		}
		entry = &keyIndex{members: map[symbol.Identity]struct{}{}}
		x.perKey[k] = entry
	}
	if !entry.loaded {
		for _, id := range x.recorded(k) {
			entry.members[id] = struct{}{}
		}
		for id, present := range entry.pending {
			if present {
				entry.members[id] = struct{}{}
			} else {
				delete(entry.members, id)
			}
		}
		entry.pending, entry.loaded, entry.stale = nil, true, true
	}
	if entry.stale {
		entry.sorted = slices.SortedFunc(maps.Keys(entry.members), symbol.Identity.Compare)
		entry.stale = false
	}
	return entry.sorted
}
