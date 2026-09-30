// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The lowering's pins: the wrapper a failure folds into, the unit
// type a callable without a result wraps, and the prefix every
// refusal states.
const (
	resultType   = "Result"
	unitType     = "()"
	refusalScope = "rust: "
)

// The lowering's fixture: the callables it rewrites, the types they
// state, and the failures they announce.
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

// thrower returns a function announcing the given failures beside
// the given results.
func thrower(returns []*emit.Return, failures ...string) *emit.Function {
	f := &emit.Function{Name: fetchName, Returns: returns}
	for _, name := range failures {
		f.Throws = append(f.Throws, &emit.TypeRef{Spelling: name})
	}
	return f
}

// lowered lowers a declaration the lowering rewrites in place.
func lowered(tb assert.TB, s symbol.Symbol) {
	tb.Helper()

	out, err := backend.Lower(s)
	assert.NoError(tb, err, "the declaration lowers")
	assert.Length(tb, out, 0, "in place: a nil list keeps the declaration")
}

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

		t.Run("folds a function's result and failure into one Result return", func(t *testing.T) {
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
