// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The spellings of the composites that the cases build: a map, a list, an
// empty tuple and two function types.
const (
	mapSpelling   = "map[string]int"
	listSpelling  = "[]"
	tupleSpelling = "()"
	funcSpelling  = "func(string) error"
	thunkSpelling = "func() error"
)

// The allocations of the composites, which TestTypeRefAllocs checks in the
// ordinary run and BenchmarkTypeRef in a benchmark run.
const (
	// compositeAllocs is a composite whose children are all stated: the
	// reference.
	compositeAllocs = 1
	// compositeNilAllocs is a composite with one child that the source does
	// not state: the reference and the empty child.
	compositeNilAllocs = 2
	// funcTypeAllocs is a function type without results: the reference.
	funcTypeAllocs = 1
	// funcTypeJoinAllocs is a function type with results: the joined
	// children and the reference.
	funcTypeJoinAllocs = 2
)

// typeAt is the position of every composite that the cases build.
var typeAt = position.Pos{File: "svc/store/row.go", Line: 3, Col: 9}

// The children that the cases pass: a string, an int and an error.
var (
	stringRef = &node.TypeRef{Spelling: "string"}
	intRef    = &node.TypeRef{Spelling: "int"}
	errorRef  = &node.TypeRef{Spelling: "error"}
)

// The composite references that frontends build through the kit keep the
// arity of their form.
func TestTypeRef(t *testing.T) {
	t.Parallel()

	t.Run("Composite", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the form with its children in order", func(t *testing.T) {
			t.Parallel()

			got := plugin.Composite(mapSpelling, typeAt, symbol.FormMap, stringRef, intRef)
			assert.Length(t, got.Elems, 2, "the map has two children")
			expect.Equal(t, got.Spelling, mapSpelling, "the reference has the spelling of its source")
			expect.Equal(t, got.Pos, typeAt, "the reference is at the position of its source")
			expect.Equal(t, got.Form, symbol.FormMap, "the reference has the form")
			expect.Equal(t, got.Elems[0], stringRef, "the first child is the key", assert.ByIdentity())
			expect.Equal(t, got.Elems[1], intRef, "the second child is the value", assert.ByIdentity())
		})

		t.Run("replaces a nil child with an empty reference at the position", func(t *testing.T) {
			t.Parallel()

			got := plugin.Composite(listSpelling, typeAt, symbol.FormList, nil)
			assert.Length(t, got.Elems, 1, "the list keeps its one child")
			assert.Equal(t, got.Elems[0], &node.TypeRef{Pos: typeAt},
				"the child is an empty reference at the position of the list")
		})

		t.Run("returns no children for a form without children", func(t *testing.T) {
			t.Parallel()

			got := plugin.Composite(tupleSpelling, typeAt, symbol.FormTuple)
			assert.Empty(t, got.Elems, "an empty tuple has no members")
		})
	})

	t.Run("FuncType", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the parameters and then the results", func(t *testing.T) {
			t.Parallel()

			got := plugin.FuncType(funcSpelling, typeAt, []*node.TypeRef{stringRef}, []*node.TypeRef{errorRef})
			assert.Length(t, got.Elems, 2, "the function type has two children")
			expect.Equal(t, got.Form, symbol.FormFunc, "the reference is a function type")
			expect.Equal(t, got.Spelling, funcSpelling, "the reference has the spelling of its source")
			expect.Equal(t, got.Elems[0], stringRef, "the first child is the parameter", assert.ByIdentity())
			expect.Equal(t, got.Elems[1], errorRef, "the second child is the result", assert.ByIdentity())
			expect.Equal(t, got.Split, 1, "the results begin after the one parameter")
		})

		t.Run("returns a split of zero for a function without parameters", func(t *testing.T) {
			t.Parallel()

			got := plugin.FuncType(thunkSpelling, typeAt, nil, []*node.TypeRef{errorRef})
			expect.Equal(t, got.Split, 0, "the results begin at the first child")
			expect.Length(t, got.Elems, 1, "the one result is the one child")
		})

		t.Run("replaces a nil result with an empty reference at the position", func(t *testing.T) {
			t.Parallel()

			got := plugin.FuncType(funcSpelling, typeAt, []*node.TypeRef{stringRef}, []*node.TypeRef{nil})
			assert.Length(t, got.Elems, 2, "the function type keeps its two children")
			assert.Equal(t, got.Elems[1], &node.TypeRef{Pos: typeAt},
				"the result is an empty reference at the position of the function type")
		})

		t.Run("keeps the parameters of a function type without results as its children", func(t *testing.T) {
			t.Parallel()

			params := []*node.TypeRef{stringRef}
			got := plugin.FuncType(funcSpelling, typeAt, params, nil)
			assert.Equal(t, got.Elems, params, "the children are the list of the parameters", assert.ByIdentity())
		})

		t.Run("leaves the storage of the parameters unchanged", func(t *testing.T) {
			t.Parallel()

			params := append(make([]*node.TypeRef, 0, 2), stringRef)
			plugin.FuncType(funcSpelling, typeAt, params, []*node.TypeRef{errorRef})
			assert.Nil(t, params[:2][1], "the result is not written into the spare room of the parameters")
		})
	})
}

// Each composite allocates its reference and the empty children that it
// adds, in the ordinary run, which runs no benchmark. The check runs
// alone, because the count includes every goroutine's allocations.
func TestTypeRefAllocs(t *testing.T) {
	stated := []*node.TypeRef{stringRef, intRef}
	var got *node.TypeRef
	assert.MaxAllocs(t, func() { got = plugin.Composite(mapSpelling, typeAt, symbol.FormMap, stated...) },
		compositeAllocs, "Composite allocates the reference")
	assert.Length(t, got.Elems, 2, "Composite returns the two children")
	assert.MaxAllocsWithSetup(t, func() []*node.TypeRef { return []*node.TypeRef{nil} },
		func(unstated []*node.TypeRef) {
			got = plugin.Composite(listSpelling, typeAt, symbol.FormList, unstated...)
		},
		compositeNilAllocs, "Composite allocates the reference and the empty child")
	assert.Length(t, got.Elems, 1, "Composite returns the one child")

	params, results := []*node.TypeRef{stringRef}, []*node.TypeRef{errorRef}
	assert.MaxAllocs(t, func() { got = plugin.FuncType(funcSpelling, typeAt, params, nil) },
		funcTypeAllocs, "FuncType allocates the reference of a function type without results")
	assert.Length(t, got.Elems, 1, "FuncType returns the parameter")
	assert.MaxAllocs(t, func() { got = plugin.FuncType(funcSpelling, typeAt, params, results) },
		funcTypeJoinAllocs, "FuncType allocates the joined children and the reference")
	assert.Length(t, got.Elems, 2, "FuncType returns the parameter and the result")
}

// BenchmarkTypeRef measures the composites that a frontend builds once for
// each structural reference that it lowers.
func BenchmarkTypeRef(b *testing.B) {
	b.Run("Composite", func(b *testing.B) {
		stated := []*node.TypeRef{stringRef, intRef}
		c := bench.Start(b).MaxAllocs(compositeAllocs)
		defer c.End()
		var got *node.TypeRef
		for c.Loop() {
			got = plugin.Composite(mapSpelling, typeAt, symbol.FormMap, stated...)
		}
		assert.Length(b, got.Elems, 2, "Composite returns the two children")
	})

	b.Run("FuncType", func(b *testing.B) {
		params, results := []*node.TypeRef{stringRef}, []*node.TypeRef{errorRef}
		c := bench.Start(b).MaxAllocs(funcTypeJoinAllocs)
		defer c.End()
		var got *node.TypeRef
		for c.Loop() {
			got = plugin.FuncType(funcSpelling, typeAt, params, results)
		}
		assert.Length(b, got.Elems, 2, "FuncType returns the parameter and the result")
	})
}
