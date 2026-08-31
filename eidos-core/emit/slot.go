// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package emit

import (
	"encoding/json"

	"go.dokimi.dev/eidos/core/symbol"
)

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

// Items returns the slot's contents in insertion order.
//
// The result aliases the slot's storage: read it, range over it,
// and do not retain it across a later Append.
func (s *Slot[T]) Items() []T { return s.items }

// Len returns how many values the slot holds.
func (s *Slot[T]) Len() int { return len(s.items) }

// IsZero reports whether the slot holds nothing, which is what lets
// an encoder omit an untouched slot.
func (s Slot[T]) IsZero() bool { return len(s.items) == 0 }

// MarshalJSON encodes the slot as the array of its contents.
//
// A slot of declarations encodes through [Symbols], so each element
// carries the kind a decoder needs. A slot of one concrete kind
// encodes from that kind's own struct tags.
func (s Slot[T]) MarshalJSON() ([]byte, error) {
	if declarations, ok := any(s.items).([]symbol.Symbol); ok {
		return json.Marshal(Symbols(declarations))
	}
	return json.Marshal(s.items)
}

// UnmarshalJSON decodes an array into the slot, replacing whatever
// it held.
//
// A slot of declarations decodes through [Symbols], which reads
// each element's kind before allocating it. A slot of one concrete
// kind decodes directly, because the decoder already knows what to
// make.
func (s *Slot[T]) UnmarshalJSON(data []byte) error {
	if into, ok := any(&s.items).(*[]symbol.Symbol); ok {
		var declarations Symbols
		if err := json.Unmarshal(data, &declarations); err != nil {
			return err
		}
		*into = declarations
		return nil
	}
	return json.Unmarshal(data, &s.items)
}
