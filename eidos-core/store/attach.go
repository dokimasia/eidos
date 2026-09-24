// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"cmp"
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
	"sync"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// attachSet is one class of raw attachment: each subject's items,
// appended under the subject's own lock while the load runs, then
// sorted into one total order and fixed at the seal. Directives and
// stamps are its two classes.
type attachSet[T any] struct {
	// pending maps each subject to its *pending[T] until the seal. It
	// is a sync.Map because concurrent carriers attach to disjoint
	// subjects far more often than to one.
	pending sync.Map
	sealed  map[symbol.Identity][]T
	order   []symbol.Identity
}

// pending is one subject's items as they arrive.
type pending[T any] struct {
	mu    sync.Mutex
	items []T
}

// attach records items on a subject: refused for a zero subject and
// an empty attachment as the defects they are, and after the seal
// under the frozen write code. what names the class in each refusal.
func attach[T any](g *Graph, set *attachSet[T], subject symbol.Identity, items []T, what string) error {
	if subject.IsZero() {
		return errors.New("store: " + what + " on a zero subject index nowhere")
	}
	if len(items) == 0 {
		return errors.New("store: no " + what + " to attach")
	}

	g.seal.RLock()
	defer g.seal.RUnlock()

	if g.frozen {
		return &RefusedError{
			Code: FrozenWrite,
			Msg:  what + " on " + subject.String() + " are attached after Freeze",
		}
	}
	held, ok := set.pending.Load(subject)
	if !ok {
		held, _ = set.pending.LoadOrStore(subject, &pending[T]{})
	}
	entry, _ := held.(*pending[T])
	entry.mu.Lock()
	entry.items = append(entry.items, items...)
	entry.mu.Unlock()
	return nil
}

// seal sorts each subject's items by compare and fixes the subjects
// in identity order. compare is a total order, so attachments that
// arrived concurrently in any interleaving seal to one sequence.
func (s *attachSet[T]) seal(compare func(a, b T) int) {
	s.sealed = map[symbol.Identity][]T{}
	s.pending.Range(func(key, value any) bool {
		id, _ := key.(symbol.Identity)
		entry, _ := value.(*pending[T])
		items := slices.Clone(entry.items)
		slices.SortFunc(items, compare)
		s.sealed[id] = items
		return true
	})
	s.order = slices.SortedFunc(maps.Keys(s.sealed), symbol.Identity.Compare)
}

// of returns one subject's sealed items, nothing before the seal.
// The slice is the graph's own storage.
func (s *attachSet[T]) of(id symbol.Identity) []T { return s.sealed[id] }

// all enumerates every subject that has items, paired with its
// items, in identity order.
func (s *attachSet[T]) all() iter.Seq2[symbol.Identity, []T] {
	return func(yield func(symbol.Identity, []T) bool) {
		for _, id := range s.order {
			if !yield(id, s.sealed[id]) {
				return
			}
		}
	}
}

// compareRaw orders two raw directive instances by position, then
// name, then arguments. The order is total, so instances at one
// position seal in one order whatever order they arrived in.
func compareRaw(a, b directive.Raw) int {
	if c := a.Pos.Compare(b.Pos); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Name, b.Name); c != 0 {
		return c
	}
	return slices.CompareFunc(a.Args, b.Args, compareRawArg)
}

// compareRawArg orders two arguments by key, value, then column.
func compareRawArg(a, b directive.RawArg) int {
	if c := cmp.Compare(a.Key, b.Key); c != 0 {
		return c
	}
	if c := compareRawValue(a.Value, b.Value); c != 0 {
		return c
	}
	return cmp.Compare(a.Col, b.Col)
}

// compareRawValue orders two argument values by text, then quoting,
// then list elements.
func compareRawValue(a, b directive.RawValue) int {
	if c := cmp.Compare(a.Text, b.Text); c != 0 {
		return c
	}
	if a.Quoted != b.Quoted {
		if a.Quoted {
			return 1
		}
		return -1
	}
	return slices.CompareFunc(a.List, b.List, compareRawValue)
}

// compareStamp orders two raw stamps by position, key, origin, then
// the value's spelling: a total order, because the seal's order is
// the claim sequence arbitration reads.
func compareStamp(a, b meta.RawStamp) int {
	if c := a.Pos.Compare(b.Pos); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Key, b.Key); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Origin, b.Origin); c != 0 {
		return c
	}
	return strings.Compare(fmt.Sprint(a.Value), fmt.Sprint(b.Value))
}
