// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import "slices"

// The keyword types of TypeScript. No declaration can take one of these
// names. null and undefined are also the literals of their types' one
// value.
const (
	TypeAny       = "any"
	TypeUnknown   = "unknown"
	TypeNever     = "never"
	TypeObject    = "object"
	TypeSymbol    = "symbol"
	TypeVoid      = "void"
	TypeNull      = "null"
	TypeUndefined = "undefined"
	TypeThis      = "this"
	TypeString    = "string"
	TypeNumber    = "number"
	TypeBoolean   = "boolean"
	TypeBigInt    = "bigint"
)

// The global types of TypeScript's standard library that the satellite
// reads or writes. A declaration of a module can take one of these names,
// and the declaration then hides the global type inside the module.
const (
	TypeArray                 = "Array"
	TypeReadonlyArray         = "ReadonlyArray"
	TypeRecord                = "Record"
	TypePartial               = "Partial"
	TypeReadonly              = "Readonly"
	TypeMap                   = "Map"
	TypeReadonlyMap           = "ReadonlyMap"
	TypeSet                   = "Set"
	TypeReadonlySet           = "ReadonlySet"
	TypePromise               = "Promise"
	TypeDate                  = "Date"
	TypeUint8Array            = "Uint8Array"
	TypeIterable              = "Iterable"
	TypeAsyncIterable         = "AsyncIterable"
	TypeAsyncIterableIterator = "AsyncIterableIterator"
	TypeAsyncGenerator        = "AsyncGenerator"
	TypeAbortSignal           = "AbortSignal"
	TypeErrorOptions          = "ErrorOptions"
)

// The global error classes of TypeScript's standard library. They are
// global types too, and a class that extends one of them is an error
// type.
const (
	TypeError          = "Error"
	TypeAggregateError = "AggregateError"
	TypeEvalError      = "EvalError"
	TypeRangeError     = "RangeError"
	TypeReferenceError = "ReferenceError"
	TypeSyntaxError    = "SyntaxError"
	TypeTypeError      = "TypeError"
	TypeURIError       = "URIError"
)

// keywordTypes lists the keyword types in the order of their constants.
var keywordTypes = []string{
	TypeAny, TypeUnknown, TypeNever, TypeObject, TypeSymbol, TypeVoid, TypeNull, TypeUndefined, TypeThis,
	TypeString, TypeNumber, TypeBoolean, TypeBigInt,
}

// errorClasses lists the global error classes in the order of their
// constants.
var errorClasses = []string{
	TypeError, TypeAggregateError, TypeEvalError, TypeRangeError, TypeReferenceError, TypeSyntaxError,
	TypeTypeError, TypeURIError,
}

// globalTypes lists the global types in the order of their constants, the
// error classes last.
var globalTypes = slices.Concat([]string{
	TypeArray, TypeReadonlyArray, TypeRecord, TypePartial, TypeReadonly, TypeMap, TypeReadonlyMap, TypeSet,
	TypeReadonlySet, TypePromise, TypeDate, TypeUint8Array, TypeIterable, TypeAsyncIterable,
	TypeAsyncIterableIterator, TypeAsyncGenerator, TypeAbortSignal, TypeErrorOptions,
}, errorClasses)

// KeywordType reports whether name is a keyword type of TypeScript. It
// allocates nothing.
func KeywordType(name string) bool {
	return slices.Contains(keywordTypes, name)
}

// GlobalType reports whether name is one of the global types that the
// satellite reads or writes. The rules resolve such a name without a
// declaration in scope, and the backend writes it without an import. It
// allocates nothing.
func GlobalType(name string) bool {
	return slices.Contains(globalTypes, name)
}

// ErrorClass reports whether name is one of the global error classes of
// TypeScript, such as Error and TypeError. It allocates nothing.
func ErrorClass(name string) bool {
	return slices.Contains(errorClasses, name)
}
