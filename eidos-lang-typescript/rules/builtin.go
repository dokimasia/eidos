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

// Builtin classifies a name of TypeScript's global scope: a name that the
// resolution step left without a target, and a name that it resolved to a
// declaration of a library, for which [typescript.Global] reports true:
//
//   - string, number and boolean as Text, a float of 64 bits and Bool;
//   - any and unknown as the top type;
//   - Date as the well-known timestamp, Error as the well-known error, and
//     Uint8Array as Bytes;
//   - Array with a type argument as a mutable list, and ReadonlyArray as
//     an immutable list;
//   - Record and Map with type arguments as a mutable map, and
//     ReadonlyMap as an immutable map;
//   - AsyncIterable, AsyncIterableIterator and AsyncGenerator with a type
//     argument as an asynchronous stream.
//
// The kernel's fold makes the type arguments the children of a list or a
// map, and it folds a list or a map with another number of arguments to
// Opaque. It makes the first type argument the child of a stream, such as
// the T of AsyncGenerator<T, TReturn, TNext>. Every other spelling is
// Opaque. That includes bigint, Promise and a literal type. A name that an
// import binds is not a global, so it is Opaque too. Builtin allocates
// nothing.
func (Rules) Builtin(ref *node.TypeRef, _ rules.View) rules.TypeShape {
	if ref == nil || ref.Form != symbol.FormNamed || !typescript.Global(ref.Target, ref.Package) {
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
	case typescript.TypeError:
		return rules.Reference(ref.Spelling, rules.WellKnownError)
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

// ShapeOf returns the shape of a reference to a declaration of a library of
// TypeScript: the shape that [Rules.Builtin] gives the global name, so a
// library type that the table of Builtin does not classify, such as
// Promise, folds to Opaque as it did without a target. ShapeOf reports false
// for a declaration outside a library, whose references fold to a reference
// to it. It allocates nothing.
func (r Rules) ShapeOf(ref *node.TypeRef, _ symbol.Symbol, v rules.View) (rules.TypeShape, bool) {
	if ref == nil || !typescript.Library(ref.Target.Package) {
		return rules.TypeShape{}, false
	}
	return r.Builtin(ref, v), true
}
