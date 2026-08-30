// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package meta

import (
	"fmt"
	"iter"
	"slices"
	"sync"

	"go.dokimi.dev/eidos/core/symbol"
)

// Facts is the run's stamped facts: one bag per subject.
//
// Facts is safe for concurrent use and serializes writes per bag,
// so two annotators stamping two subjects do not contend, and
// readers of one bag share its lock. Rank decides every winner, so
// a parallel run answers what a serial one does, whatever order the
// writes arrived.
type Facts struct {
	registry *Registry
	// groupOf holds each key's group by id, precomputed from the
	// registry so a read consults one slice entry rather than
	// copying a spec. Registration completes before the first
	// write, which is what makes the snapshot safe.
	groupOf []GroupName
	// bags holds each subject's bag. Writers mostly own disjoint
	// subjects, which is what keeps two bags from contending on one
	// lock.
	bags sync.Map
	// index answers ByKey; bags feed it presence transitions under
	// their own write lock, so two racing writes on one subject
	// cannot record their transitions out of order.
	index *factIndex
}

// NewFacts answers an empty fact store reading specs from r.
// Registration completes before the first write; the store
// snapshots what it needs and never locks the registry.
func NewFacts(r *Registry) *Facts {
	groupOf := make([]GroupName, len(r.specs)+1)
	for i, spec := range r.specs {
		groupOf[i+1] = spec.Group
	}
	return &Facts{registry: r, groupOf: groupOf, index: newFactIndex()}
}

// Stamp records one claim of v under k.
//
// It refuses a zero key, a subject kind the key does not admit, and
// a false boolean — absence is the negative, so false is never
// stamped and deletion stays load-bearing. A claim identical to one
// already held, same rank source and equal value, changes nothing.
// Values compare per vocabulary term; slices compare element-wise
// and are copied in.
func Stamp[T FactValue](f *Facts, k Key[T], v T, c Claim) error {
	spec, err := f.spec(k.ID())
	if err != nil {
		return err
	}
	if flag, isBool := any(v).(bool); isBool && !flag {
		return fmt.Errorf("meta: %s stamps false on %s: absence is the negative, drop the fact instead",
			k.Name(), c.Subject)
	}
	if !kindAdmitted(spec.Kinds, c.Subject.Kind) {
		return fmt.Errorf("meta: %s does not admit kind %s, which %s is",
			k.Name(), c.Subject.Kind, c.Subject)
	}
	return f.write(c.Subject, k.ID(), k.Name(), stored{claim: c, value: cloneValue(any(v))})
}

// DropKey claims the fact's absence.
//
// A drop is a claim like any other: it wins and loses by rank, so a
// directive-authority drop beats a plugin stamp whenever the stamp
// arrives, and loses to a manual write.
func (f *Facts) DropKey(k KeyID, c Claim) error {
	spec, err := f.spec(k)
	if err != nil {
		return err
	}
	return f.write(c.Subject, k, spec.Name, stored{claim: c, drop: true})
}

// DropGroup claims absence for every member of g, including members
// whose stamps land after the drop: the tombstone covers the group,
// so arbitration finds it whichever member is read.
func (f *Facts) DropGroup(g GroupName, c Claim) error {
	members := slices.Collect(f.registry.Group(g))
	if len(members) == 0 {
		return fmt.Errorf("meta: group %q holds no keys: nothing registered into it", g)
	}

	b := f.bag(c.Subject)
	b.mu.Lock()
	defer b.mu.Unlock()

	changed, err := b.group(g).admit(stored{claim: c, drop: true})
	if err != nil {
		return fmt.Errorf("meta: group %s on %s %w", g, c.Subject, err)
	}
	if changed {
		// The tombstone can flip any member the bag holds claims
		// for, so every member's presence re-records.
		for _, member := range members {
			f.index.record(c.Subject, member, b.presentLocked(g, member))
		}
	}
	return nil
}

// Get answers the winning value, untracked, and false where the
// winner is a drop or nothing was stamped. Slice values are copied
// out, so a caller cannot reach into a bag.
func Get[T FactValue](f *Facts, id symbol.Identity, k Key[T]) (T, bool) {
	var zero T
	value, held := f.lookup(id, k.ID())
	if !held {
		return zero, false
	}
	typed, isT := cloneValue(value).(T)
	if !isT {
		return zero, false
	}
	return typed, true
}

// Fact answers what [Get] does and records the read at
// (subject, key) into rec. A miss records too: the reader asked, so
// it runs again when the fact appears. It is the read every plugin
// makes; Get is the kernel's own untracked path.
func Fact[T FactValue](f *Facts, rec Recorder, id symbol.Identity, k Key[T]) (T, bool) {
	rec.RecordFact(id, k.Name())
	return Get(f, id, k)
}

// Recorder records fact reads. The store's read set implements it,
// so one artifact's declaration reads and fact reads land in one
// set.
type Recorder interface {
	RecordFact(subject symbol.Identity, key KeyName)
}

// ByKey enumerates the subjects on which k presently reads present,
// in identity order. The index is maintained at stamp time, which
// is what lets a fact-gated rule visit its matches rather than the
// graph.
func (f *Facts) ByKey(k KeyID) iter.Seq[symbol.Identity] {
	return slices.Values(f.index.enumerate(k))
}

// spec answers a key's spec or the refusal naming what was wrong.
func (f *Facts) spec(k KeyID) (KeySpec, error) {
	spec, known := f.registry.Spec(k)
	if !known {
		return KeySpec{}, fmt.Errorf("meta: key %d is not registered: every write goes through a handle", k)
	}
	return spec, nil
}

// group answers a key's fact group without copying its spec, which
// is what keeps the read path off the registry.
func (f *Facts) group(k KeyID) GroupName {
	if int(k) >= len(f.groupOf) {
		return ""
	}
	return f.groupOf[k]
}

// bag answers the subject's bag, creating it on first touch.
func (f *Facts) bag(id symbol.Identity) *bag {
	if held, ok := f.bags.Load(id); ok {
		b, _ := held.(*bag)
		return b
	}
	held, _ := f.bags.LoadOrStore(id, &bag{perKey: map[KeyID]*factState{}})
	b, _ := held.(*bag)
	return b
}

// write admits one stored claim under (subject, key) and maintains
// the index through the presence transition.
func (f *Facts) write(id symbol.Identity, k KeyID, keyName KeyName, entry stored) error {
	b := f.bag(id)
	b.mu.Lock()
	defer b.mu.Unlock()

	changed, err := b.key(k).admit(entry)
	if err != nil {
		return fmt.Errorf("meta: %s on %s %w", keyName, id, err)
	}
	if changed {
		f.index.record(id, k, b.presentLocked(f.group(k), k))
	}
	return nil
}

// lookup answers the winning value for (subject, key), and false
// where the winner is a drop or nothing was stamped.
func (f *Facts) lookup(id symbol.Identity, k KeyID) (any, bool) {
	b := f.bag(id)
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.presentLocked(f.group(k), k) {
		return nil, false
	}
	state := b.perKey[k]
	return state.claims[state.winner].value, true
}

// kindAdmitted reports whether a key's kind restriction admits a
// subject's kind. An empty restriction admits every kind.
func kindAdmitted(kinds []symbol.Kind, kind symbol.Kind) bool {
	return len(kinds) == 0 || slices.Contains(kinds, kind)
}
