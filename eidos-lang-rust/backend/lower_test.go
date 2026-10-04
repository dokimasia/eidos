// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/rust/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The cases pin the wrapper a failure folds into, the unit type a
// callable without a result wraps, and the prefix every refusal states.
const (
	resultType   = "Result"
	unitType     = "()"
	refusalScope = "rust: "
)

// The cases lower these callables, the types they state, and the
// failures they announce.
const (
	fetchName    = "fetch"
	closeName    = "close"
	trackName    = "track"
	storeName    = "store"
	rowType      = "row"
	countType    = "u32"
	fetchError   = "fetchError"
	closeError   = "closeError"
	notFoundType = "notFound"
	timeoutType  = "timeout"
	rowComment   = "the row"
)

// The allocations of a lowering.
const (
	// wrapAllocs is a function of one result announcing one failure: the
	// folded return, its Result reference, its two arguments, and its list
	// of one.
	wrapAllocs = 1 + 1 + 1 + 1
	// allocRuns is the calls assert.MaxAllocs makes: one to warm up and
	// one hundred it counts.
	allocRuns = 101
)

// The lowering folds an announced failure into the Result return,
// refuses what a result cannot state, and passes everything else
// through untouched.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a function announcing no failure through unchanged", func(t *testing.T) {
			t.Parallel()

			f := thrower(nil)
			lowered(t, f)
			assert.Length(t, f.Returns, 0, "no return is added")
		})

		t.Run("passes a method with a receiver through unchanged", func(t *testing.T) {
			t.Parallel()

			lowered(t, &emit.Method{Name: trackName, Receives: &emit.TypeRef{Spelling: rowType}})
		})

		t.Run("folds a function's result with its failure into one Result return", func(t *testing.T) {
			t.Parallel()

			valued := &emit.TypeRef{Spelling: rowType}
			f := thrower([]*emit.Return{{Type: valued}}, fetchError)
			failure := f.Throws[0]
			lowered(t, f)
			assert.Length(t, f.Returns, 1, "into the one wrapped return")
			wrapped := f.Returns[0].Type
			assert.Equal(t, wrapped.Spelling, resultType, "as a Result")
			assert.Length(t, wrapped.Args, 2, "of value and failure")
			assert.True(t, wrapped.Args[0] == valued,
				"the value reference moves whole, so the settle keeps following it")
			assert.True(t, wrapped.Args[1] == failure, "and so does the failure's")
		})

		t.Run("consumes the failure a function announces", func(t *testing.T) {
			t.Parallel()

			f := thrower([]*emit.Return{{Type: &emit.TypeRef{Spelling: rowType}}}, fetchError)
			lowered(t, f)
			assert.Length(t, f.Throws, 0, "a second settle finds nothing to lower")
		})

		t.Run("moves the result's trailing comment to the folded return", func(t *testing.T) {
			t.Parallel()

			f := thrower([]*emit.Return{{Type: &emit.TypeRef{Spelling: rowType}, Comment: rowComment}},
				fetchError)
			lowered(t, f)
			assert.Equal(t, f.Returns[0].Comment, rowComment, "the comment follows the value")
		})

		t.Run("wraps the unit type for a member method announcing a failure without a result", func(t *testing.T) {
			t.Parallel()

			host := &emit.Interface{Name: storeName}
			host.Methods.Append(&emit.Method{
				Name:   closeName,
				Throws: []*emit.TypeRef{{Spelling: closeError}},
			})
			lowered(t, host)
			m := host.Methods.Items()[0]
			assert.Length(t, m.Throws, 0, "a host's member methods lower with it")
			assert.Equal(t, m.Returns[0].Type.Args[0].Spelling, unitType,
				"nothing returned wraps the unit type")
		})

		refusals := []struct {
			name string
			give symbol.Symbol
		}{
			{
				name: "returns an error for a function announcing two failures",
				give: thrower(nil, notFoundType, timeoutType),
			},
			{
				name: "returns an error for a function returning two values beside a failure",
				give: thrower([]*emit.Return{
					{Type: &emit.TypeRef{Spelling: rowType}},
					{Type: &emit.TypeRef{Spelling: countType}},
				}, fetchError),
			},
			{
				name: "returns an error for a function whose result states no type beside a failure",
				give: thrower([]*emit.Return{{Name: rowType}}, fetchError),
			},
			{
				name: "returns an error for a file-level method without a receiver",
				give: &emit.Method{Name: trackName},
			},
			{
				name: "returns an error for a file-level method whose receiver spells no type",
				give: &emit.Method{Name: trackName, Receives: &emit.TypeRef{}},
			},
			{
				name: "returns an error for a method announcing two failures",
				give: &emit.Method{Name: trackName, Receives: &emit.TypeRef{Spelling: rowType}, Throws: twoFailures()},
			},
			{
				name: "returns an error for a struct declaring one method twice",
				give: structOf(&emit.Method{Name: trackName}, &emit.Method{Name: trackName}),
			},
			{
				name: "returns an error for a struct whose method announces two failures",
				give: structOf(&emit.Method{Name: closeName, Throws: twoFailures()}),
			},
			{
				name: "returns an error for a trait declaring one method twice",
				give: traitOf(&emit.Method{Name: trackName}, &emit.Method{Name: trackName}),
			},
			{
				name: "returns an error for a trait whose method announces two failures",
				give: traitOf(&emit.Method{Name: closeName, Throws: twoFailures()}),
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.Lower(tt.give)
				assert.HasError(t, err, "a result or an impl block cannot state it")
			})
		}

		t.Run("returns an error under the language's identity", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Lower(thrower(nil, notFoundType, timeoutType))
			assert.HasError(t, err, "two failures refuse")
			assert.HasPrefix(t, err.Error(), refusalScope, "naming the target that refused")
		})
	})
}

// A declaration Rust states as it is allocates nothing, and a fold
// allocates the Result return. A fold consumes its failure, so each
// counted call lowers a fresh function. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestLowerAllocs(t *testing.T) {
	constant := &emit.Constant{Name: storeName, Type: &emit.TypeRef{Spelling: countType}, Value: "8"}
	assert.MaxAllocs(t, func() { _, _ = backend.Lower(constant) }, 0, "Lower passes the constant within its ceiling")

	fns, next := make([]*emit.Function, allocRuns), 0
	for i := range fns {
		fns[i] = valuedThrower()
	}
	assert.MaxAllocs(t, func() {
		_, _ = backend.Lower(fns[next])
		next++
	}, wrapAllocs, "Lower folds the failure within its ceiling")
}

// BenchmarkLower measures the lowering the settle runs over every
// declaration, a fresh function built outside the count for each fold.
func BenchmarkLower(b *testing.B) {
	b.Run("Lower", func(b *testing.B) {
		constant := &emit.Constant{Name: storeName, Type: &emit.TypeRef{Spelling: countType}, Value: "8"}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			out []symbol.Symbol
			err error
		)
		for c.Loop() {
			out, err = backend.Lower(constant)
		}
		assert.NoError(b, err, "Lower passes the constant")
		assert.Length(b, out, 0, "Lower keeps the constant in place")
	})

	b.Run("Lower/a function that throws", func(b *testing.B) {
		f := valuedThrower()
		c := bench.Start(b).MaxAllocs(wrapAllocs)
		defer c.End()
		for c.Loop() {
			_, _ = backend.Lower(f)
			c.Excluding(func() { f = valuedThrower() })
		}
		lowered(b, f)
		assert.Equal(b, f.Returns[0].Type.Spelling, resultType, "Lower folds the failure into a Result")
	})
}

// thrower returns a function announcing the given failures beside
// the given results.
func thrower(returns []*emit.Return, failures ...string) *emit.Function {
	f := &emit.Function{Name: fetchName, Returns: returns}
	for _, name := range failures {
		f.Throws = append(f.Throws, &emit.TypeRef{Spelling: name})
	}
	return f
}

// valuedThrower returns a function of one row result announcing one
// failure.
func valuedThrower() *emit.Function {
	return thrower([]*emit.Return{{Type: &emit.TypeRef{Spelling: rowType}}}, fetchError)
}

// twoFailures returns the two failures a result cannot fold.
func twoFailures() []*emit.TypeRef {
	return []*emit.TypeRef{{Spelling: notFoundType}, {Spelling: timeoutType}}
}

// structOf returns a struct declaring the given methods.
func structOf(methods ...*emit.Method) *emit.Struct {
	s := &emit.Struct{Name: storeName}
	s.Methods.Append(methods...)
	return s
}

// traitOf returns a trait declaring the given methods.
func traitOf(methods ...*emit.Method) *emit.Interface {
	i := &emit.Interface{Name: storeName}
	i.Methods.Append(methods...)
	return i
}

// lowered lowers a declaration the lowering rewrites in place.
func lowered(tb assert.TB, s symbol.Symbol) {
	tb.Helper()

	out, err := backend.Lower(s)
	assert.NoError(tb, err, "the declaration lowers")
	assert.Length(tb, out, 0, "in place: a nil list keeps the declaration")
}
