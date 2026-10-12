// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"encoding/json"
	"fmt"

	"go.dokimi.dev/eidos/core/symbol"
)

// sealedAppend is the panic of a direct write into a sealed slot.
const sealedAppend = "emit: the slot belongs to a value in the store of a plan, " +
	"so a plugin appends to it through the slot view of its emitter, which buffers the write and names the writer"

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
// # Sealing
//
// The store of a plan seals every slot of a value that a unit places,
// through [Seal]. While the phase calls of the plan run, [Slot.Append]
// and [Slot.Replace] panic on a sealed slot, so a plugin writes into a
// value of the store through the slot view of its emitter alone. The
// view buffers the write with its invocation, applies it through
// [AppendSealed] in canonical match order, and names the plugin among
// the contributors of the unit. The store lifts the seal when the
// settle starts, so the lowering hooks of a backend rewrite the
// declarations of their target. A slot of a value that no store
// contains is not sealed.
//
// # Concurrency
//
// The zero Slot is ready to append to. A Slot is not safe for
// concurrent use. Plans run in parallel, and each plan has its own
// emit declarations.
//
// # Allocation contract
//
// An append allocates only where the storage of the slot grows. The
// check of the seal allocates nothing.
type Slot[T any] struct {
	items []T
	// seal is the flag of the store that contains the value of the slot,
	// and nil for a slot that no store sealed.
	seal *bool
}

// SlotOf returns a slot whose contents are values, in order, for the
// composite literal of an emit value. The slot keeps values as its
// storage, so the caller does not reuse the slice that it passes. The
// slot is not sealed. SlotOf allocates nothing beyond the slice of a
// variadic call.
func SlotOf[T any](values ...T) Slot[T] { return Slot[T]{items: values} }

// Append adds values to the end of the slot. It panics on a sealed
// slot, as the type states. It allocates only where the storage of the
// slot grows.
func (s *Slot[T]) Append(values ...T) {
	if s.seal != nil && *s.seal {
		panic(sealedAppend)
	}
	s.items = append(s.items, values...)
}

// Replace sets the contents of the slot to values, in order, for a
// lowering that rewrites a list. It reuses the storage of the slot, and
// clears the entries past the new contents, so the slot keeps no
// reference to a removed value. It panics on a sealed slot, as
// [Slot.Append] does. It allocates only where values do not fit the
// storage of the slot.
func (s *Slot[T]) Replace(values ...T) {
	if s.seal != nil && *s.seal {
		panic(sealedAppend)
	}
	before := len(s.items)
	s.items = append(s.items[:0], values...)
	if before > len(s.items) {
		clear(s.items[len(s.items):before])
	}
}

// Items returns the contents of the slot in insertion order.
//
// The result aliases the storage of the slot. A caller reads it or
// ranges over it, and does not retain it across a later Append.
func (s *Slot[T]) Items() []T { return s.items }

// Len returns the number of values in the slot.
func (s *Slot[T]) Len() int { return len(s.items) }

// IsZero reports whether the slot is empty, so a struct field tagged
// omitzero leaves an empty slot out of its JSON encoding. The encoder
// calls IsZero on an addressable slot without a copy of the slot, so the
// check allocates nothing there.
func (s *Slot[T]) IsZero() bool { return len(s.items) == 0 }

// MarshalJSON encodes the slot as the array of its contents.
//
// A slot of declarations encodes through [Symbols], so each element
// includes the kind that a decoder needs. A slot of one concrete kind
// encodes from the struct tags of that kind.
func (s Slot[T]) MarshalJSON() ([]byte, error) {
	var (
		b   []byte
		err error
	)
	if declarations, ok := any(s.items).([]symbol.Symbol); ok {
		b, err = json.Marshal(Symbols(declarations))
	} else {
		b, err = json.Marshal(s.items)
	}
	if err != nil {
		return nil, fmt.Errorf("emit: encode a slot: %w", err)
	}
	return b, nil
}

// UnmarshalJSON decodes an array into the slot and replaces its
// contents. Malformed JSON leaves the slot unchanged.
//
// A slot of declarations decodes through [Symbols], which reads the
// kind of each element before it allocates the element. A slot of one
// concrete kind decodes directly into that kind.
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

// AppendSealed adds values to the end of a slot, whether the slot is
// sealed or not. The emitter applies a buffered append through it.
// AppendSealed is reserved to the kernel, and the lint rule of the
// repository refuses a call from outside the kernel. It allocates only
// where the storage of the slot grows.
func AppendSealed[T any](s *Slot[T], values ...T) { s.items = append(s.items, values...) }
