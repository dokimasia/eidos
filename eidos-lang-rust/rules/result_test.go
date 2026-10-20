// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust"
	rustrules "go.dokimi.dev/eidos/lang/rust/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The ceilings of Result: a result type with two type arguments allocates
// nothing, and a result of a module allocates the reference of its failure
// and the spelling of the failure where the result is a path.
const (
	resultAllocs     = 0
	ioResultAllocs   = 2
	resultName       = "Result"
	storeErrorSpells = "StoreError"
)

// The aliases of the fixture that the specs of Result read.
var (
	idsID   = symbol.Identity{Lang: rust.Lang, Package: utilPkg, Name: "Ids", Kind: symbol.KindAlias}
	deep0ID = symbol.Identity{Lang: rust.Lang, Package: utilPkg, Name: "Deep0", Kind: symbol.KindAlias}
)

// TestResult pins the value and the failure of each result type, and the
// references of other types.
func TestResult(t *testing.T) {
	t.Parallel()

	t.Run("Result", func(t *testing.T) {
		t.Parallel()

		u8 := &node.TypeRef{Spelling: u8Name}
		text := &node.TypeRef{Spelling: stringName}
		unit := &node.TypeRef{Spelling: "()"}
		tests := []struct {
			name        string
			give        *node.TypeRef
			wantValue   *node.TypeRef
			wantFailure *node.TypeRef
		}{
			{
				name: "returns the two type arguments of Result",
				give: &node.TypeRef{
					Spelling: resultName,
					Args:     []*node.TypeRef{u8, text},
				},
				wantValue:   u8,
				wantFailure: text,
			},
			{
				name: "returns the two type arguments of a path of std to Result",
				give: &node.TypeRef{
					Spelling: "std::result::Result", Package: "std/result", Args: []*node.TypeRef{u8, text},
				},
				wantValue: u8, wantFailure: text,
			},
			{
				name: "returns no value for a Result of the unit type",
				give: &node.TypeRef{Spelling: resultName, Args: []*node.TypeRef{unit, text}}, wantFailure: text,
			},
			{
				name:        "returns the Error of the module for io::Result",
				give:        &node.TypeRef{Spelling: "io::Result", Package: "std/io", Args: []*node.TypeRef{u8}},
				wantValue:   u8,
				wantFailure: &node.TypeRef{Spelling: "io::Error", Package: "std/io"},
			},
			{
				name:        "returns the Error of the module for a Result that a use binds",
				give:        &node.TypeRef{Spelling: resultName, Package: "std/io", Args: []*node.TypeRef{u8}},
				wantValue:   u8,
				wantFailure: &node.TypeRef{Spelling: errorName, Package: "std/io"},
			},
			{
				name:        "returns no value for fmt::Result",
				give:        &node.TypeRef{Spelling: "std::fmt::Result", Package: "std/fmt"},
				wantFailure: &node.TypeRef{Spelling: "std::fmt::Error", Package: "std/fmt"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				value, failure, ok := rustrules.Rules{}.Result(tt.give, rules.View{})
				assert.True(t, ok, "the reference is a result type")
				assert.Equal(t, value, tt.wantValue, "the value of the result")
				assert.Equal(t, failure, tt.wantFailure, "the failure of the result")
			})
		}

		t.Run("returns the value and the failure of the target of an alias of the crate", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			ret := f.method(t, storeID, "get").Returns[0].Type
			value, failure, ok := rustrules.Rules{}.Result(ret, f.view)
			assert.True(t, ok, "the alias of the crate is a result type")
			assert.Equal(t, value.Target, orderID, "the value is the argument of the alias")
			assert.Equal(t, failure.Spelling, storeErrorSpells, "the failure is the failure of the target")
			assert.Contains(t, slices.Collect(f.reads.Identities()), resultID, "the view records the read of the alias")
		})

		t.Run(
			"returns the default of a type parameter that a reference leaves out past its lifetime",
			func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				outcome := symbol.Identity{Lang: rust.Lang, Package: errorsPkg, Name: "Outcome", Kind: symbol.KindAlias}
				ref := &node.TypeRef{Spelling: "Outcome", Target: outcome, Args: []*node.TypeRef{{Spelling: "'static"}}}
				value, failure, ok := rustrules.Rules{}.Result(ref, f.view)
				assert.True(t, ok, "the alias is a result type")
				assert.NotNil(t, value, "the result has a value")
				assert.Equal(t, value.Spelling, u8Name, "the value is the default of the parameter")
				assert.NotNil(t, failure, "the result has a failure")
				assert.Equal(t, failure.Form, symbol.FormBorrow, "the failure is the borrow of the target")
			},
		)

		t.Run("returns the type argument of a reference to an alias past its lifetime", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			outcome := symbol.Identity{Lang: rust.Lang, Package: errorsPkg, Name: "Outcome", Kind: symbol.KindAlias}
			ref := &node.TypeRef{Spelling: "Outcome", Target: outcome, Args: []*node.TypeRef{
				{Spelling: "'static"}, {Spelling: "u16"},
			}}
			value, _, ok := rustrules.Rules{}.Result(ref, f.view)
			assert.True(t, ok, "the alias is a result type")
			assert.NotNil(t, value, "the result has a value")
			assert.Equal(t, value.Spelling, "u16", "the value is the type argument of the reference")
		})

		others := []struct {
			name string
			give *node.TypeRef
		}{
			{name: "reports false for a nil reference"},
			{
				name: "reports false for a structural reference",
				give: &node.TypeRef{Spelling: "(u8,)", Form: symbol.FormTuple},
			},
			{name: "reports false for Option", give: &node.TypeRef{Spelling: optionName, Args: []*node.TypeRef{u8}}},
			{name: "reports false for a type of the crate", give: &node.TypeRef{Spelling: "Order", Target: orderID}},
			{name: "reports false for an alias of another type", give: &node.TypeRef{Spelling: "Ids", Target: idsID}},
			{
				name: "reports false past the aliases that the rules follow",
				give: &node.TypeRef{Spelling: "Deep0", Target: deep0ID},
			},
		}
		for _, tt := range others {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				_, _, ok := rustrules.Rules{}.Result(tt.give, f.view)
				assert.False(t, ok, "the reference is no result type")
			})
		}
	})
}

// TestResultAllocs checks the ceilings of Result in the ordinary run, which
// does not run benchmarks.
func TestResultAllocs(t *testing.T) {
	checkAllocs(t, resultCalls())
}

// BenchmarkResult measures Result for a result with two type arguments and
// for a result of a module under their ceilings.
func BenchmarkResult(b *testing.B) {
	benchCalls(b, resultCalls())
}

// resultCalls returns the calls of Result that the allocation test and the
// benchmark measure.
func resultCalls() []allocCall {
	r := rustrules.Rules{}
	u8 := &node.TypeRef{Spelling: u8Name}
	tests := []struct {
		caseName string
		give     *node.TypeRef
		allocs   uint64
	}{
		{
			caseName: "a result with two type arguments",
			give:     &node.TypeRef{Spelling: resultName, Args: []*node.TypeRef{u8, u8}}, allocs: resultAllocs,
		},
		{
			caseName: "a result of a module",
			give:     &node.TypeRef{Spelling: "io::Result", Package: "std/io", Args: []*node.TypeRef{u8}},
			allocs:   ioResultAllocs,
		},
	}
	calls := make([]allocCall, 0, len(tests))
	for _, tt := range tests {
		var ok bool
		calls = append(calls, allocCall{
			name: "Result", caseName: tt.caseName, allocs: tt.allocs,
			call:  func() { _, _, ok = r.Result(tt.give, rules.View{}) },
			check: func(tb assert.TB) { assert.True(tb, ok, "Result splits "+tt.caseName) },
		})
	}
	return calls
}
