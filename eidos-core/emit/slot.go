// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package emit

// Slot is a typed append point on an emit declaration.
//
// One plugin rarely owns a generated file: a repository generator
// emits a struct, and a tracing plugin wants a field on it without
// either knowing the other exists. A slot is the primitive that
// lets them compose, because it accumulates rather than overwrites.
//
// The element type is the slot's contract. Appending a method into
// a field slot does not compile, so a mistake is caught where it is
// made rather than in the rendered output.
//
// The zero Slot is ready to append to. A Slot is not safe for
// concurrent use: plans run in parallel, but each owns its own emit
// declarations.
type Slot[T any] struct {
	items []T
}

// Append adds values to the end of the slot.
func (s *Slot[T]) Append(values ...T) { s.items = append(s.items, values...) }

// Items answers the slot's contents in insertion order.
//
// The result aliases the slot's storage: read it, range over it,
// and do not retain it across a later Append.
func (s *Slot[T]) Items() []T { return s.items }

// Len answers how many values the slot holds.
func (s *Slot[T]) Len() int { return len(s.items) }
