// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/typescript"
)

// declaredName is a name that a module declares, and neither a keyword
// type nor a global type.
const declaredName = "Store"

// keywordTypes, globalTypes and errorClasses list every constant of the
// three groups. The error classes are global types too.
var (
	keywordTypes = []string{
		typescript.TypeAny, typescript.TypeUnknown, typescript.TypeNever, typescript.TypeObject,
		typescript.TypeSymbol, typescript.TypeVoid, typescript.TypeNull, typescript.TypeUndefined,
		typescript.TypeThis, typescript.TypeString, typescript.TypeNumber, typescript.TypeBoolean,
		typescript.TypeBigInt,
	}
	errorClasses = []string{
		typescript.TypeError, typescript.TypeAggregateError, typescript.TypeEvalError, typescript.TypeRangeError,
		typescript.TypeReferenceError, typescript.TypeSyntaxError, typescript.TypeTypeError, typescript.TypeURIError,
	}
	globalTypes = append([]string{
		typescript.TypeArray, typescript.TypeReadonlyArray, typescript.TypeRecord, typescript.TypePartial,
		typescript.TypeMap, typescript.TypeReadonlyMap, typescript.TypeSet, typescript.TypeReadonlySet,
		typescript.TypePromise,
		typescript.TypeDate, typescript.TypeUint8Array, typescript.TypeAsyncIterable,
		typescript.TypeAsyncIterableIterator, typescript.TypeAsyncGenerator, typescript.TypeAbortSignal,
	}, errorClasses...)
)

// TestTypes checks the two predicates over every constant of the keyword
// types and the global types.
func TestTypes(t *testing.T) {
	t.Parallel()

	t.Run("KeywordType", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for each keyword type", func(t *testing.T) {
			t.Parallel()

			for _, name := range keywordTypes {
				expect.True(t, typescript.KeywordType(name), name+" is a keyword type")
			}
		})

		t.Run("reports false for each global type", func(t *testing.T) {
			t.Parallel()

			for _, name := range globalTypes {
				expect.False(t, typescript.KeywordType(name), name+" is not a keyword type")
			}
		})

		t.Run("reports false for a declared name", func(t *testing.T) {
			t.Parallel()

			assert.False(t, typescript.KeywordType(declaredName), "a declared name is not a keyword type")
		})
	})

	t.Run("GlobalType", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for each global type", func(t *testing.T) {
			t.Parallel()

			for _, name := range globalTypes {
				expect.True(t, typescript.GlobalType(name), name+" is a global type")
			}
		})

		t.Run("reports false for each keyword type", func(t *testing.T) {
			t.Parallel()

			for _, name := range keywordTypes {
				expect.False(t, typescript.GlobalType(name), name+" is not a global type")
			}
		})

		t.Run("reports false for a declared name", func(t *testing.T) {
			t.Parallel()

			assert.False(t, typescript.GlobalType(declaredName), "a declared name is not a global type")
		})
	})

	t.Run("ErrorClass", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for each error class", func(t *testing.T) {
			t.Parallel()

			for _, name := range errorClasses {
				expect.True(t, typescript.ErrorClass(name), name+" is an error class")
			}
		})

		t.Run("reports false for a global type that is not an error class", func(t *testing.T) {
			t.Parallel()

			assert.False(t, typescript.ErrorClass(typescript.TypeDate), "Date is not an error class")
		})

		t.Run("reports false for a declared name", func(t *testing.T) {
			t.Parallel()

			assert.False(t, typescript.ErrorClass(declaredName), "a declared name is not an error class")
		})
	})
}

// TestTypesAllocs checks that the three predicates do not allocate. The
// ordinary test run does not run benchmarks, so this test checks the
// ceilings.
func TestTypesAllocs(t *testing.T) {
	var keyword, global, class bool
	expect.MaxAllocs(t, func() { keyword = typescript.KeywordType(typescript.TypeBigInt) }, 0,
		"KeywordType allocates nothing")
	expect.MaxAllocs(t, func() { global = typescript.GlobalType(typescript.TypeAbortSignal) }, 0,
		"GlobalType allocates nothing")
	expect.MaxAllocs(t, func() { class = typescript.ErrorClass(typescript.TypeURIError) }, 0,
		"ErrorClass allocates nothing")
	expect.True(t, keyword, "bigint is a keyword type")
	expect.True(t, global, "AbortSignal is a global type")
	expect.True(t, class, "URIError is an error class")
}

// BenchmarkTypes measures each predicate on the last name of its list.
// The backend calls GlobalType for every reference without a module.
func BenchmarkTypes(b *testing.B) {
	b.Run("KeywordType", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var keyword bool
		for c.Loop() {
			keyword = typescript.KeywordType(typescript.TypeBigInt)
		}
		assert.True(b, keyword, "bigint is a keyword type")
	})

	b.Run("GlobalType", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var global bool
		for c.Loop() {
			global = typescript.GlobalType(typescript.TypeAbortSignal)
		}
		assert.True(b, global, "AbortSignal is a global type")
	})

	b.Run("ErrorClass", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var class bool
		for c.Loop() {
			class = typescript.ErrorClass(typescript.TypeURIError)
		}
		assert.True(b, class, "URIError is an error class")
	})
}
