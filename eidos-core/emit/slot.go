// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"encoding/json"
	"fmt"

	"go.dokimi.dev/eidos/core/symbol"
)

// Slot is a typed append point on an emit declaration.
//
// More than one plugin can contribute to one generated file. A
// repository generator emits a struct, and a tracing plugin appends
// a field to it without either plugin knowing the other exists. A
// slot accumulates every value appended to it and never overwrites
// one.
//
// The element type is the slot's contract. Appending a method into
// a field slot does not compile, so the compiler reports the mistake
// at the call that makes it.
//
// The zero Slot is ready to append to. A Slot is not safe for
// concurrent use. Plans run in parallel, and each plan has its own
// emit declarations.
type Slot[T any] struct {
	items []T
}

// Append adds values to the end of the slot.
func (s *Slot[T]) Append(values ...T) { s.items = append(s.items, values...) }

// Items returns the slot's contents in insertion order.
//
// The result aliases the slot's storage: read it, range over it,
// and do not retain it across a later Append.
func (s *Slot[T]) Items() []T { return s.items }

// Len returns the number of values in the slot.
func (s *Slot[T]) Len() int { return len(s.items) }

// IsZero reports whether the slot is empty, so a struct field tagged
// omitzero leaves an empty slot out of its JSON encoding.
func (s Slot[T]) IsZero() bool { return len(s.items) == 0 }

// MarshalJSON encodes the slot as the array of its contents.
//
// A slot of declarations encodes through [Symbols], so each element
// includes the kind a decoder needs. A slot of one concrete kind
// encodes from that kind's own struct tags.
func (s Slot[T]) MarshalJSON() ([]byte, error) {
	if declarations, ok := any(s.items).([]symbol.Symbol); ok {
		return json.Marshal(Symbols(declarations))
	}
	return json.Marshal(s.items)
}

// UnmarshalJSON decodes an array into the slot and replaces its
// contents. Malformed JSON leaves the slot unchanged.
//
// A slot of declarations decodes through [Symbols], which reads
// each element's kind before allocating it. A slot of one concrete
// kind decodes directly into that kind.
func (s *Slot[T]) UnmarshalJSON(data []byte) error {
	if into, ok := any(&s.items).(*[]symbol.Symbol); ok {
		var declarations Symbols
		if err := json.Unmarshal(data, &declarations); err != nil {
			return fmt.Errorf("emit: decode a slot: %w", err)
		}
		*into = declarations
		return nil
	}
	if err := json.Unmarshal(data, &s.items); err != nil {
		return fmt.Errorf("emit: decode a slot: %w", err)
	}
	return nil
}
