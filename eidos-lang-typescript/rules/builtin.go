// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// numberBits is the width of TypeScript's number, a float of double
// precision.
const numberBits = 64

// Builtin classifies a name of TypeScript's global scope that the
// resolution step left without a target:
//
//   - string, number and boolean as Text, a float of 64 bits and Bool;
//   - any and unknown as the top type;
//   - Date as the well-known timestamp, and Uint8Array as Bytes;
//   - Array with a type argument as a mutable list, and ReadonlyArray as
//     an immutable list;
//   - Record and Map with type arguments as a mutable map, and
//     ReadonlyMap as an immutable map;
//   - AsyncIterable, AsyncIterableIterator and AsyncGenerator with a type
//     argument as an asynchronous stream.
//
// The kernel's fold makes the type arguments the children of a list, a
// map or a stream. It folds a reference with another number of arguments
// to Opaque. Every other spelling is Opaque. That includes bigint,
// Promise and a literal type. A name that an import binds is not a
// global, so it is Opaque too. Builtin allocates nothing.
func (Rules) Builtin(ref *node.TypeRef, _ rules.View) rules.TypeShape {
	if ref == nil || ref.Form != symbol.FormNamed || !ref.Target.IsZero() || ref.Package != "" {
		return rules.Opaque(ref)
	}
	switch ref.Spelling {
	case typescript.TypeString:
		return rules.Leaf(symbol.FormText, ref.Spelling)
	case typescript.TypeNumber:
		return rules.Scalar(ref.Spelling, rules.ScalarFloat, numberBits)
	case typescript.TypeBoolean:
		return rules.Leaf(symbol.FormBool, ref.Spelling)
	case typescript.TypeAny, typescript.TypeUnknown:
		return rules.Leaf(symbol.FormDynamic, ref.Spelling)
	case typescript.TypeDate:
		return rules.Reference(ref.Spelling, rules.WellKnownTimestamp)
	case typescript.TypeUint8Array:
		return rules.Leaf(symbol.FormBytes, ref.Spelling)
	}
	if len(ref.Args) == 0 {
		return rules.Opaque(ref)
	}
	switch ref.Spelling {
	case typescript.TypeArray:
		return rules.TypeShape{Form: symbol.FormList, Spelling: ref.Spelling, Mutability: symbol.MutabilityMutable}
	case typescript.TypeReadonlyArray:
		return rules.TypeShape{Form: symbol.FormList, Spelling: ref.Spelling, Mutability: symbol.MutabilityImmutable}
	case typescript.TypeRecord, typescript.TypeMap:
		return rules.TypeShape{Form: symbol.FormMap, Spelling: ref.Spelling, Mutability: symbol.MutabilityMutable}
	case typescript.TypeReadonlyMap:
		return rules.TypeShape{Form: symbol.FormMap, Spelling: ref.Spelling, Mutability: symbol.MutabilityImmutable}
	case typescript.TypeAsyncIterable, typescript.TypeAsyncIterableIterator, typescript.TypeAsyncGenerator:
		return rules.TypeShape{Form: symbol.FormStream, Spelling: ref.Spelling, Async: true}
	default:
		return rules.Opaque(ref)
	}
}
