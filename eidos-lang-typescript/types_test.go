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

// keywordTypes and globalTypes list every constant of the two groups.
var (
	keywordTypes = []string{
		typescript.TypeAny, typescript.TypeUnknown, typescript.TypeNever, typescript.TypeObject,
		typescript.TypeSymbol, typescript.TypeVoid, typescript.TypeNull, typescript.TypeUndefined,
		typescript.TypeThis, typescript.TypeString, typescript.TypeNumber, typescript.TypeBoolean,
		typescript.TypeBigInt,
	}
	globalTypes = []string{
		typescript.TypeArray, typescript.TypeReadonlyArray, typescript.TypeRecord, typescript.TypePartial,
		typescript.TypeMap, typescript.TypeReadonlyMap, typescript.TypeSet, typescript.TypePromise,
		typescript.TypeDate, typescript.TypeUint8Array, typescript.TypeAsyncIterable,
		typescript.TypeAsyncIterableIterator, typescript.TypeAsyncGenerator, typescript.TypeAbortSignal,
	}
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
}

// TestTypesAllocs checks that both predicates do not allocate. The
// ordinary test run does not run benchmarks, so this test checks the
// ceilings.
func TestTypesAllocs(t *testing.T) {
	var keyword, global bool
	expect.MaxAllocs(t, func() { keyword = typescript.KeywordType(typescript.TypeBigInt) }, 0,
		"KeywordType allocates nothing")
	expect.MaxAllocs(t, func() { global = typescript.GlobalType(typescript.TypeAbortSignal) }, 0,
		"GlobalType allocates nothing")
	expect.True(t, keyword, "bigint is a keyword type")
	expect.True(t, global, "AbortSignal is a global type")
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
}
