// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings of the global types that the rules classify.
const (
	spellString                = "string"
	spellNumber                = "number"
	spellBoolean               = "boolean"
	spellBigInt                = "bigint"
	spellDate                  = "Date"
	spellUint8Array            = "Uint8Array"
	spellArray                 = "Array"
	spellReadonlyArray         = "ReadonlyArray"
	spellRecord                = "Record"
	spellMap                   = "Map"
	spellReadonlyMap           = "ReadonlyMap"
	spellAsyncIterable         = "AsyncIterable"
	spellAsyncIterableIterator = "AsyncIterableIterator"
	spellAsyncGenerator        = "AsyncGenerator"
	spellAbortSignal           = "AbortSignal"
)

// numberBits is the width of TypeScript's number, a float of double
// precision.
const numberBits = 64

// globalTypes lists the global types of the builtin table, bigint
// included. A type name resolves to one of them when no declaration in
// scope has the name.
var globalTypes = []string{
	spellString, spellNumber, spellBoolean, spellBigInt, spellDate, spellUint8Array, spellArray,
	spellReadonlyArray, spellRecord, spellMap, spellReadonlyMap, spellAsyncIterable,
	spellAsyncIterableIterator, spellAsyncGenerator, spellAbortSignal,
}

// Builtin classifies a name of TypeScript's global scope that the
// resolution step left without a target:
//
//   - string, number and boolean as Text, a float of 64 bits and Bool;
//   - Date as the well-known timestamp, and Uint8Array as Bytes;
//   - Array and ReadonlyArray with a type argument as a list;
//   - Record, Map and ReadonlyMap with type arguments as a map;
//   - AsyncIterable, AsyncIterableIterator and AsyncGenerator with a type
//     argument as an asynchronous stream.
//
// The kernel's fold makes the type arguments the children of a list, a
// map or a stream. It folds a reference with another number of arguments
// to Opaque. Every other spelling is Opaque. That includes bigint, any,
// unknown, Promise and a literal type. A name that an import binds is not
// a global, so it is Opaque too. Builtin allocates nothing.
func (Rules) Builtin(ref *node.TypeRef, _ rules.View) rules.TypeShape {
	if ref == nil || ref.Form != symbol.FormNamed || !ref.Target.IsZero() || ref.Package != "" {
		return rules.Opaque(ref)
	}
	switch ref.Spelling {
	case spellString:
		return rules.Leaf(symbol.FormText, ref.Spelling)
	case spellNumber:
		return rules.Scalar(ref.Spelling, rules.ScalarFloat, numberBits)
	case spellBoolean:
		return rules.Leaf(symbol.FormBool, ref.Spelling)
	case spellDate:
		return rules.Reference(ref.Spelling, rules.WellKnownTimestamp)
	case spellUint8Array:
		return rules.Leaf(symbol.FormBytes, ref.Spelling)
	}
	if len(ref.Args) == 0 {
		return rules.Opaque(ref)
	}
	switch ref.Spelling {
	case spellArray, spellReadonlyArray:
		return rules.TypeShape{Form: symbol.FormList, Spelling: ref.Spelling}
	case spellRecord, spellMap, spellReadonlyMap:
		return rules.TypeShape{Form: symbol.FormMap, Spelling: ref.Spelling}
	case spellAsyncIterable, spellAsyncIterableIterator, spellAsyncGenerator:
		return rules.TypeShape{Form: symbol.FormStream, Spelling: ref.Spelling, Async: true}
	default:
		return rules.Opaque(ref)
	}
}
