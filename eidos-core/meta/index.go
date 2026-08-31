// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package meta

import (
	"maps"
	"slices"
	"sync"

	"go.dokimi.dev/eidos/core/symbol"
)

// factIndex returns which subjects presently carry which keys. It is
// maintained at stamp time so a fact-gated rule visits its matches
// rather than the graph, and it owns its own lock: presence
// transitions arrive from many bags at once.
type factIndex struct {
	mu     sync.Mutex
	perKey map[KeyID]*keyIndex
}

// keyIndex is one key's presence set with its enumeration cached:
// dispatch enumerates once per rule, so the sort happens once per
// transition rather than once per call.
type keyIndex struct {
	members map[symbol.Identity]struct{}
	// sorted is the cached enumeration, rebuilt when stale. It is
	// replaced rather than mutated, so a handed-out iterator keeps
	// its snapshot.
	sorted []symbol.Identity
	stale  bool
}

// newFactIndex returns an index holding nothing.
func newFactIndex() *factIndex {
	return &factIndex{perKey: map[KeyID]*keyIndex{}}
}

// record notes a presence transition for (subject, key). A
// transition marks the cached enumeration stale; a write that
// changes nothing does not.
func (x *factIndex) record(id symbol.Identity, k KeyID, present bool) {
	x.mu.Lock()
	defer x.mu.Unlock()

	entry := x.perKey[k]
	if entry == nil {
		if !present {
			return
		}
		entry = &keyIndex{members: map[symbol.Identity]struct{}{}}
		x.perKey[k] = entry
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
// order, from the cache where it is fresh.
func (x *factIndex) enumerate(k KeyID) []symbol.Identity {
	x.mu.Lock()
	defer x.mu.Unlock()

	entry := x.perKey[k]
	if entry == nil {
		return nil
	}
	if entry.stale {
		entry.sorted = slices.SortedFunc(maps.Keys(entry.members), symbol.Identity.Compare)
		entry.stale = false
	}
	return entry.sorted
}
