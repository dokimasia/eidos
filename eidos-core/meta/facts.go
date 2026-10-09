// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"fmt"
	"iter"
	"slices"
	"sync"

	"go.dokimi.dev/eidos/core/symbol"
)

// keyEntry is what a read consults of one key's spec: its fact group
// and its kind restriction.
type keyEntry struct {
	group GroupName
	kinds []symbol.Kind
}

// Facts is the run's stamped facts: one bag per subject, keyed by the
// subject's identity in a [sync.Map].
//
// # Concurrency
//
// Facts is safe for concurrent use. Writes to one subject serialize on
// that subject's bag lock, and readers of the bag share it. A write that
// changes whether a fact is present also takes the fact index's lock,
// which is one lock for the store. Rank decides every winner, so a
// parallel run returns what a serial one does, whatever order the writes
// arrived in.
//
// # Allocation contract
//
// A read allocates nothing, except the copy of a list value. A write
// allocates what the store keeps: a subject's bag on its first claim,
// the boxed value of each new claim, and the containers that grow with
// the claims. Each method states its count. In a store [Restore]
// returned, the first touch of a subject also allocates the bag and the
// claims the source restores into it. A read of a fact that the source's
// presence leaves out, on a subject the run has not touched, allocates
// nothing.
type Facts struct {
	registry *Registry
	// keys is each key's group and kind restriction by id, copied from
	// the registry so a read consults one entry and copies no spec.
	// Registration completes before the first write, which is what makes
	// the copy safe.
	keys []keyEntry
	// bags contains each subject's bag. Writers mostly write disjoint
	// subjects, which is what keeps two bags from contending on one
	// lock.
	bags sync.Map
	// index enumerates [Facts.ByKey]. Each bag records its presence
	// transitions into it under the bag's write lock, so two racing
	// writes on one subject cannot record their transitions out of
	// order.
	index factIndex
	// source restores each bag's recorded claims on first use, nil for
	// a store NewFacts returned.
	source BagSource
	// damage is the first failure of the source, which [Facts.Damaged]
	// returns.
	damageMu sync.Mutex
	damage   error
}

// NewFacts returns an empty fact store reading specs from r.
// Registration completes before the first write. The store copies the
// group and the kind restriction of every key r registered, and never
// locks r.
//
// # Allocation contract
//
// NewFacts allocates twice: the store, and its table of the keys'
// groups and kind restrictions.
func NewFacts(r *Registry) *Facts {
	keys := make([]keyEntry, len(r.specs)+1)
	for i, spec := range r.specs {
		keys[i+1] = keyEntry{group: spec.Group, kinds: spec.Kinds}
	}
	return &Facts{registry: r, keys: keys}
}

// Stamp records one claim of v under k.
//
// A claim identical to one the store keeps, from the same rank source
// with an equal value, changes nothing. Values compare per vocabulary
// term, and a list compares element-wise. The store keeps a copy of a
// list, so the caller may reuse its slice.
//
// Error modes:
//   - a key nothing registered;
//   - a named handle, because a registrant writes through the handle
//     that [Register] returned;
//   - a subject kind the key does not admit;
//   - a false boolean. Absence is the negative, so a fact turns false
//     only through a drop or a withdrawal;
//   - a second value from a rank source that already claimed the fact,
//     because rank cannot order the two.
//
// # Allocation contract
//
// An identical re-stamp allocates nothing. A new claim allocates its
// boxed value: one allocation for a string or an identity, two for a
// list and its copy, and none for a boolean or an integer below 256,
// which Go boxes without allocating. A subject's first claim also
// allocates the bag, the boxed identity and the [sync.Map] entry, three
// allocations, and the map's trie nodes where two hashes share a
// prefix: 0.38 per subject on average at 200,000 subjects. The first
// claim on a second key of a subject allocates the bag's key map and
// the key's state. A claim from a second rank source allocates the
// fact's claim slice, which grows by doubling. A claim that turns a fact
// present adds the subject to the key's index, whose map grows by
// doubling.
func Stamp[T FactValue](f *Facts, k Key[T], v T, c Claim) error {
	if k.ID() == 0 && k.Name() != "" {
		return fmt.Errorf("meta: %s is a named handle, and a write goes through the handle that registration returns",
			k.Name())
	}
	spec, err := f.spec(k.ID())
	if err != nil {
		return err
	}
	if err := admitClaim(spec, k.Name(), any(v), c); err != nil {
		return err
	}
	return stampValue(f, k, v, c)
}

// stampValue admits one typed claim under its subject's bag lock. A
// claim the state keeps from the same rank source with an equal value
// returns before the value is boxed, so an identical re-stamp allocates
// nothing.
func stampValue[T FactValue](f *Facts, k Key[T], v T, c Claim) error {
	b := f.bag(c.Subject)
	b.mu.Lock()
	defer b.mu.Unlock()

	state := b.key(k.ID())
	if held, kept := state.from(c); kept && !held.drop && equalValue(held.value, any(v)) {
		return nil
	}
	return f.admitLocked(b, c.Subject, k.ID(), k.Name(), state, stored{claim: c, value: cloneValue(any(v))})
}

// Registry returns the registry the store was created over: what
// a reader resolves a key by name against. It allocates nothing.
func (f *Facts) Registry() *Registry { return f.registry }

// DropKey claims the fact's absence.
//
// A drop is a claim like any other and ranks like one, so a
// directive-authority drop ranks above a plugin stamp whenever the stamp
// arrives, and below a manual write.
//
// Error modes: a key nothing registered, and a stamp from the same rank
// source on the fact.
//
// # Allocation contract
//
// An identical re-drop allocates nothing. A new drop allocates what a
// new claim of [Stamp] does, without a boxed value.
func (f *Facts) DropKey(k KeyID, c Claim) error {
	spec, err := f.spec(k)
	if err != nil {
		return err
	}
	return f.write(c.Subject, k, spec.Name, stored{claim: c, drop: true})
}

// DropGroup claims absence for every member of g, including members
// whose stamps arrive after the drop: the tombstone covers the group,
// so arbitration finds it whichever member is read.
//
// Error modes: a group nothing registered into.
//
// # Allocation contract
//
// An identical re-drop allocates nothing. A new drop allocates the bag's
// group map and the group's state on the subject's first group drop, and
// what a new claim of [Stamp] does to the bag and the index otherwise.
func (f *Facts) DropGroup(g GroupName, c Claim) error {
	members := f.registry.groups[g]
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
		f.recordMembers(c.Subject, b, g, members)
	}
	return nil
}

// Withdraw removes the claim on (c.Subject, k) that has c's rank
// source, meaning its authority, bucket, plugin and order, and ranks the
// remaining claims again: what a warm run does before it executes a
// match again, and for a match that disappeared. A fact whose presence
// the withdrawal changes moves in [Facts.ByKey].
//
// Error modes: a key nothing registered. Withdrawing a claim the store
// does not contain is not an error.
//
// # Allocation contract
//
// Withdraw allocates nothing, except where the withdrawal turns a fact
// present and its index's map grows.
func (f *Facts) Withdraw(k KeyID, c Claim) error {
	if _, err := f.spec(k); err != nil {
		return err
	}
	b, held := f.peek(c.Subject)
	if !held {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if state, held := b.state(k); held && state.withdraw(c) {
		f.index.record(c.Subject, k, b.presentLocked(f.group(k), k))
	}
	return nil
}

// WithdrawGroup removes the group drop on c.Subject that has c's rank
// source, and records every member's presence again.
//
// Error modes: a group nothing registered into. Withdrawing a drop the
// store does not contain is not an error.
//
// # Allocation contract
//
// WithdrawGroup allocates nothing, except where the withdrawal turns a
// member present and its index's map grows.
func (f *Facts) WithdrawGroup(g GroupName, c Claim) error {
	members := f.registry.groups[g]
	if len(members) == 0 {
		return fmt.Errorf("meta: group %q holds no keys: nothing registered into it", g)
	}
	b, held := f.peek(c.Subject)
	if !held {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if state, held := b.perGroup[g]; held && state.withdraw(c) {
		f.recordMembers(c.Subject, b, g, members)
	}
	return nil
}

// Damaged returns the first failure of the recorded source a store
// [Restore] returned met, and nil where every read of it succeeded or
// the store restores nothing. A bag the source failed to restore reads
// as empty, so a run that finds Damaged set after a phase discards what
// it derived and runs cold. It allocates nothing.
func (f *Facts) Damaged() error {
	f.damageMu.Lock()
	defer f.damageMu.Unlock()
	return f.damage
}

// Get returns the winning value, untracked, and false where the
// winner is a drop or nothing was stamped. A list is copied out, so a
// caller cannot write into a bag. A named handle resolves in the store's
// registry, and reads absent where the registry has no key of its name
// and type.
//
// # Allocation contract
//
// Get allocates nothing for a scalar value and two allocations for a
// list: the copy and its box.
func Get[T FactValue](f *Facts, id symbol.Identity, k Key[T]) (T, bool) {
	var zero T
	k, registered := k.resolved(f.registry)
	if !registered {
		return zero, false
	}
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

// Fact returns what [Get] does and records the read at (subject, key)
// into rec. It records a miss as well, so the reader runs again when the
// fact appears. A subject of a kind the key does not admit reads absent
// and records nothing, because [Stamp] refuses every claim on it. Every
// plugin reads through Fact. [Get] is the kernel's untracked path. A
// named handle resolves as Get resolves it, and a name that the registry
// does not contain reads absent and records the read by its name.
//
// # Allocation contract
//
// Fact allocates what [Get] does, and what rec allocates to record the
// read.
func Fact[T FactValue](f *Facts, rec Recorder, id symbol.Identity, k Key[T]) (T, bool) {
	bound, _ := k.resolved(f.registry)
	if !f.admits(bound.ID(), id.Kind) {
		var zero T
		return zero, false
	}
	rec.RecordFact(id, k.Name())
	return Get(f, id, bound)
}

// Recorder records fact reads. The store's read set implements it,
// so one artifact's declaration reads and fact reads arrive in one
// set.
type Recorder interface {
	RecordFact(subject symbol.Identity, key KeyName)
}

// ByKey enumerates the subjects on which k reads present when the range
// starts, in identity order. The index is maintained at stamp time,
// which is what lets a fact-gated rule visit its matches and not the
// whole graph.
//
// # Allocation contract
//
// The first enumeration after a presence transition sorts the key's
// subjects into a new slice. Every other enumeration allocates nothing,
// because the returned function inlines into the range.
func (f *Facts) ByKey(k KeyID) iter.Seq[symbol.Identity] {
	return func(yield func(symbol.Identity) bool) { f.eachWithKey(k, yield) }
}

// eachWithKey yields the subjects on which k reads present, in identity
// order, from the index's snapshot. It stops when yield returns false.
func (f *Facts) eachWithKey(k KeyID, yield func(symbol.Identity) bool) {
	for _, id := range f.index.enumerate(k) {
		if !yield(id) {
			return
		}
	}
}

// recordMembers records the presence of every member of a group on one
// subject: a group tombstone can flip any of them. The caller has
// locked b.mu.
func (f *Facts) recordMembers(id symbol.Identity, b *bag, g GroupName, members []KeyID) {
	for _, member := range members {
		f.index.record(id, member, b.presentLocked(g, member))
	}
}

// damaged records the first failure of the recorded source.
func (f *Facts) damaged(err error) {
	f.damageMu.Lock()
	defer f.damageMu.Unlock()
	if f.damage == nil {
		f.damage = err
	}
}

// spec returns a key's spec, or an error for a key nothing registered.
func (f *Facts) spec(k KeyID) (KeySpec, error) {
	spec, known := f.registry.Spec(k)
	if !known {
		return KeySpec{}, fmt.Errorf("meta: key %d is not registered: every write goes through a handle", k)
	}
	return spec, nil
}

// group returns a key's fact group from the table, without reading the
// registry or copying a spec. A key the table does not contain has no
// group.
func (f *Facts) group(k KeyID) GroupName {
	if int(k) >= len(f.keys) {
		return ""
	}
	return f.keys[k].group
}

// admits reports whether a key's kind restriction admits a subject's
// kind, from the table. A key the table does not contain admits every
// kind, so its read records and [Get] reads it absent.
func (f *Facts) admits(k KeyID, kind symbol.Kind) bool {
	if int(k) >= len(f.keys) {
		return true
	}
	return kindAdmitted(f.keys[k].kinds, kind)
}

// bag returns the subject's bag, creating it on first touch: the
// write path's own lookup. A read goes through [Facts.peek], so a
// miss on a subject nothing stamped allocates nothing. In a store
// [Restore] returned, the first touch restores the bag's recorded
// claims.
func (f *Facts) bag(id symbol.Identity) *bag {
	held, ok := f.bags.Load(id)
	if !ok {
		held, _ = f.bags.LoadOrStore(id, &bag{})
	}
	b, _ := held.(*bag)
	if f.source != nil {
		b.restore.Do(func() { f.restoreBag(id, b) })
	}
	return b
}

// peek returns the subject's bag and false where nothing was ever
// stamped, allocating nothing. In a store [Restore] returned, a
// subject's recorded claims may exist without a bag, so peek restores
// the bag and reports true.
func (f *Facts) peek(id symbol.Identity) (*bag, bool) {
	if f.source != nil {
		return f.bag(id), true
	}
	held, ok := f.bags.Load(id)
	if !ok {
		return nil, false
	}
	b, _ := held.(*bag)
	return b, true
}

// write admits one stored claim under (subject, key) and maintains
// the index through the presence transition.
func (f *Facts) write(id symbol.Identity, k KeyID, keyName KeyName, entry stored) error {
	b := f.bag(id)
	b.mu.Lock()
	defer b.mu.Unlock()

	return f.admitLocked(b, id, k, keyName, b.key(k), entry)
}

// admitLocked admits one stored claim into the state of (subject, key)
// in b and maintains the index through the presence transition. The
// caller has locked b.mu.
func (f *Facts) admitLocked(
	b *bag, id symbol.Identity, k KeyID, keyName KeyName, state *factState, entry stored,
) error {
	changed, err := state.admit(entry)
	if err != nil {
		return fmt.Errorf("meta: %s on %s %w", keyName, id, err)
	}
	if changed {
		f.index.record(id, k, b.presentLocked(f.group(k), k))
	}
	return nil
}

// lookup returns the winning value for (subject, key), and false
// where the winner is a drop or nothing was stamped. In a store [Restore]
// returned, a subject that the run has not restored or written reads the
// key absent without a restore where the source's presence of the key
// does not list the subject.
func (f *Facts) lookup(id symbol.Identity, k KeyID) (any, bool) {
	if f.source != nil {
		if _, touched := f.bags.Load(id); !touched {
			_, present := slices.BinarySearchFunc(f.index.recorded(k), id, symbol.Identity.Compare)
			if !present {
				return nil, false
			}
		}
	}
	b, held := f.peek(id)
	if !held {
		return nil, false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.presentLocked(f.group(k), k) {
		return nil, false
	}
	state, _ := b.state(k)
	return state.claims[state.winner].value, true
}

// kindAdmitted reports whether a key's kind restriction admits a
// subject's kind. An empty restriction admits every kind.
func kindAdmitted(kinds []symbol.Kind, kind symbol.Kind) bool {
	return len(kinds) == 0 || slices.Contains(kinds, kind)
}
