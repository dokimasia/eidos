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

// Facts is the run's stamped facts: one bag per subject.
//
// Facts is safe for concurrent use. Writes to one subject serialize
// on that subject's bag lock, and readers of the bag share it. A
// write that changes whether a fact is present also takes the fact
// index's lock, which is one lock for the store. Rank decides every
// winner, so a parallel run returns what a serial one does, whatever
// order the writes arrived.
type Facts struct {
	registry *Registry
	// groupOf, kindsOf and nameOf are each key's group, kind
	// restriction and name by id, precomputed from the registry so a
	// read consults one slice entry and copies no spec. Registration
	// completes before the first write, which is what makes the
	// snapshot safe.
	groupOf []GroupName
	kindsOf [][]symbol.Kind
	nameOf  []KeyName
	// bags contains each subject's bag. Writers mostly write disjoint
	// subjects, which is what keeps two bags from contending on one
	// lock.
	bags sync.Map
	// index returns ByKey; bags feed it presence transitions under
	// their own write lock, so two racing writes on one subject
	// cannot record their transitions out of order.
	index *factIndex
	// source restores each bag's recorded claims on first use, nil for
	// a store NewFacts returned.
	source BagSource
	// damage is the first failure of the source, which [Facts.Damaged]
	// returns.
	damageMu sync.Mutex
	damage   error
}

// NewFacts returns an empty fact store reading specs from r.
// Registration completes before the first write; the store
// snapshots what it needs and never locks the registry.
func NewFacts(r *Registry) *Facts {
	groupOf := make([]GroupName, len(r.specs)+1)
	kindsOf := make([][]symbol.Kind, len(r.specs)+1)
	nameOf := make([]KeyName, len(r.specs)+1)
	for i, spec := range r.specs {
		groupOf[i+1] = spec.Group
		kindsOf[i+1] = spec.Kinds
		nameOf[i+1] = spec.Name
	}
	return &Facts{registry: r, groupOf: groupOf, kindsOf: kindsOf, nameOf: nameOf, index: newFactIndex(nil)}
}

// Stamp records one claim of v under k.
//
// It refuses a zero key, a subject kind the key does not admit, and
// a false boolean — absence is the negative, so false is never
// stamped and deletion remains load-bearing. A claim identical to one
// already kept, same rank source and equal value, changes nothing.
// Values compare per vocabulary term; slices compare element-wise
// and are copied in.
func Stamp[T FactValue](f *Facts, k Key[T], v T, c Claim) error {
	spec, err := f.spec(k.ID())
	if err != nil {
		return err
	}
	if err := admitClaim(spec, k.Name(), any(v), c); err != nil {
		return err
	}
	return f.write(c.Subject, k.ID(), k.Name(), stored{claim: c, value: cloneValue(any(v))})
}

// Registry returns the registry the store was created over: what
// a reader resolves a key by name against.
func (f *Facts) Registry() *Registry { return f.registry }

// DropKey claims the fact's absence.
//
// A drop is a claim like any other and ranks like one, so a
// directive-authority drop ranks above a plugin stamp whenever the stamp
// arrives, and below a manual write.
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
func (f *Facts) Withdraw(k KeyID, c Claim) error {
	if _, err := f.spec(k); err != nil {
		return err
	}
	b := f.bag(c.Subject)
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
func (f *Facts) WithdrawGroup(g GroupName, c Claim) error {
	members := slices.Collect(f.registry.Group(g))
	if len(members) == 0 {
		return fmt.Errorf("meta: group %q holds no keys: nothing registered into it", g)
	}
	b := f.bag(c.Subject)
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
// it derived and runs cold.
func (f *Facts) Damaged() error {
	f.damageMu.Lock()
	defer f.damageMu.Unlock()
	return f.damage
}

// Get returns the winning value, untracked, and false where the
// winner is a drop or nothing was stamped. Slice values are copied
// out, so a caller cannot write into a bag.
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

// Fact returns what [Get] does and records the read at
// (subject, key) into rec. A miss records too: the reader asked, so
// it runs again when the fact appears. A subject of a kind the key
// does not admit reads absent and records nothing, because [Stamp]
// refuses every claim on it. It is the read every plugin makes; Get
// is the kernel's own untracked path.
func Fact[T FactValue](f *Facts, rec Recorder, id symbol.Identity, k Key[T]) (T, bool) {
	if !f.admits(k.ID(), id.Kind) {
		var zero T
		return zero, false
	}
	rec.RecordFact(id, k.Name())
	return Get(f, id, k)
}

// Recorder records fact reads. The store's read set implements it,
// so one artifact's declaration reads and fact reads arrive in one
// set.
type Recorder interface {
	RecordFact(subject symbol.Identity, key KeyName)
}

// ByKey enumerates the subjects on which k presently reads present,
// in identity order. The index is maintained at stamp time, which
// is what lets a fact-gated rule visit its matches and not the whole
// graph.
func (f *Facts) ByKey(k KeyID) iter.Seq[symbol.Identity] {
	return slices.Values(f.index.enumerate(k))
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

// spec returns a key's spec or the refusal naming what was wrong.
func (f *Facts) spec(k KeyID) (KeySpec, error) {
	spec, known := f.registry.Spec(k)
	if !known {
		return KeySpec{}, fmt.Errorf("meta: key %d is not registered: every write goes through a handle", k)
	}
	return spec, nil
}

// group returns a key's fact group without copying its spec, which
// is what keeps the read path off the registry.
func (f *Facts) group(k KeyID) GroupName {
	if int(k) >= len(f.groupOf) {
		return ""
	}
	return f.groupOf[k]
}

// admits reports whether a key's kind restriction admits a subject's
// kind, from the snapshot. A key the snapshot does not contain admits
// every kind, so its read records and [Get] reads it absent.
func (f *Facts) admits(k KeyID, kind symbol.Kind) bool {
	if int(k) >= len(f.kindsOf) {
		return true
	}
	return kindAdmitted(f.kindsOf[k], kind)
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

	changed, err := b.key(k).admit(entry)
	if err != nil {
		return fmt.Errorf("meta: %s on %s %w", keyName, id, err)
	}
	if changed {
		f.index.record(id, k, b.presentLocked(f.group(k), k))
	}
	return nil
}

// lookup returns the winning value for (subject, key), and false
// where the winner is a drop or nothing was stamped.
func (f *Facts) lookup(id symbol.Identity, k KeyID) (any, bool) {
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
