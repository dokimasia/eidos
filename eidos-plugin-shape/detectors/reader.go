// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package detectors

import (
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Reader reports a callable that takes one key and returns the value of
// the key beside its error. The key is a single value, so a list, a map, a
// function or an interface is no key. An inline record is a key.
func Reader(c rules.Callable, b rules.Bound) bool {
	s := signatureOf(c)
	if !s.fails || s.inputs != 1 || s.values != 1 {
		return false
	}
	switch b.TypeOf(s.input.Ref).Form {
	case symbol.FormList, symbol.FormBytes, symbol.FormMap, symbol.FormFunc:
		return false
	case symbol.FormInline:
		return !isInterface(b, s.input.Ref)
	default:
		return true
	}
}

// ReaderNoError reports a callable that takes one key, which is not
// variadic, and returns the value of the key without an error model.
func ReaderNoError(c rules.Callable, _ rules.Bound) bool {
	s := signatureOf(c)
	return !s.fails && s.inputs == 1 && !s.input.Variadic &&
		len(c.Returns) == 1 && c.Returns[0].Role == rules.ReturnValue
}

// ReaderWithBool reports a callable that takes one key and returns two
// values, the second of which is a truth value.
func ReaderWithBool(c rules.Callable, b rules.Bound) bool {
	s := signatureOf(c)
	return s.inputs == 1 && len(c.Returns) == 2 && b.TypeOf(c.Returns[1].Ref).Form == symbol.FormBool
}

// PointerReader reports a callable that takes one key and returns one
// value that can be absent, without an error model.
func PointerReader(c rules.Callable, b rules.Bound) bool {
	s := signatureOf(c)
	return !s.fails && s.inputs == 1 && len(c.Returns) == 1 &&
		b.TypeOf(c.Returns[0].Ref).Form == symbol.FormOptional
}

// BatchReader reports a callable that takes one variadic list of keys and
// returns one list of values beside its error.
func BatchReader(c rules.Callable, b rules.Bound) bool {
	s := signatureOf(c)
	if !s.fails || s.inputs != 1 || !s.input.Variadic || s.values != 1 {
		return false
	}
	form := b.TypeOf(s.value.Ref).Form
	return form == symbol.FormList || form == symbol.FormBytes
}

// MultiReader reports a callable that takes one key and returns two or
// more values beside its error.
func MultiReader(c rules.Callable, _ rules.Bound) bool {
	s := signatureOf(c)
	return s.fails && s.inputs == 1 && s.values >= 2
}

// Lookup reports a callable that takes one key and returns three values,
// the third of which is a truth value.
func Lookup(c rules.Callable, b rules.Bound) bool {
	s := signatureOf(c)
	return s.inputs == 1 && len(c.Returns) == 3 && b.TypeOf(c.Returns[2].Ref).Form == symbol.FormBool
}

// StreamReader reports a callable that returns exactly one stream and
// takes at most one input.
func StreamReader(c rules.Callable, _ rules.Bound) bool {
	s := signatureOf(c)
	return s.inputs <= 1 && len(c.Returns) == 1 && c.Returns[0].Role == rules.ReturnStream
}
