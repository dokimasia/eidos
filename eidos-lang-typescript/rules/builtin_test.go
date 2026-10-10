// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	tsrules "go.dokimi.dev/eidos/lang/typescript/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestBuiltin checks each row of the builtin table, which classifies the
// global types of TypeScript.
func TestBuiltin(t *testing.T) {
	t.Parallel()

	t.Run("Builtin", func(t *testing.T) {
		t.Parallel()

		arg := &node.TypeRef{Spelling: numberName}
		tests := []struct {
			name string
			give *node.TypeRef
			want rules.TypeShape
		}{
			{
				name: "classifies string as text", give: &node.TypeRef{Spelling: stringName},
				want: rules.Leaf(symbol.FormText, stringName),
			},
			{
				name: "classifies number as a float of 64 bits", give: &node.TypeRef{Spelling: numberName},
				want: rules.Scalar(numberName, rules.ScalarFloat, 64),
			},
			{
				name: "classifies boolean as a truth value", give: &node.TypeRef{Spelling: "boolean"},
				want: rules.Leaf(symbol.FormBool, "boolean"),
			},
			{
				name: "classifies any as the top type", give: &node.TypeRef{Spelling: "any"},
				want: rules.Leaf(symbol.FormDynamic, "any"),
			},
			{
				name: "classifies unknown as the top type", give: &node.TypeRef{Spelling: "unknown"},
				want: rules.Leaf(symbol.FormDynamic, "unknown"),
			},
			{
				name: "classifies Date as the well-known timestamp", give: &node.TypeRef{Spelling: "Date"},
				want: rules.Reference("Date", rules.WellKnownTimestamp),
			},
			{
				name: "classifies Uint8Array as bytes", give: &node.TypeRef{Spelling: "Uint8Array"},
				want: rules.Leaf(symbol.FormBytes, "Uint8Array"),
			},
			{
				name: "classifies Array with an argument as a list",
				give: &node.TypeRef{Spelling: "Array", Args: []*node.TypeRef{arg}},
				want: rules.TypeShape{Form: symbol.FormList, Spelling: "Array"},
			},
			{
				name: "classifies ReadonlyArray with an argument as a list",
				give: &node.TypeRef{Spelling: "ReadonlyArray", Args: []*node.TypeRef{arg}},
				want: rules.TypeShape{Form: symbol.FormList, Spelling: "ReadonlyArray"},
			},
			{
				name: "classifies Record with arguments as a map",
				give: &node.TypeRef{Spelling: "Record", Args: []*node.TypeRef{arg, arg}},
				want: rules.TypeShape{Form: symbol.FormMap, Spelling: "Record"},
			},
			{
				name: "classifies Map with arguments as a map",
				give: &node.TypeRef{Spelling: "Map", Args: []*node.TypeRef{arg, arg}},
				want: rules.TypeShape{Form: symbol.FormMap, Spelling: "Map"},
			},
			{
				name: "classifies ReadonlyMap with arguments as a map",
				give: &node.TypeRef{Spelling: "ReadonlyMap", Args: []*node.TypeRef{arg, arg}},
				want: rules.TypeShape{Form: symbol.FormMap, Spelling: "ReadonlyMap"},
			},
			{
				name: "classifies AsyncIterable with an argument as an asynchronous stream",
				give: &node.TypeRef{Spelling: "AsyncIterable", Args: []*node.TypeRef{arg}},
				want: rules.TypeShape{Form: symbol.FormStream, Spelling: "AsyncIterable", Async: true},
			},
			{
				name: "classifies AsyncIterableIterator with an argument as an asynchronous stream",
				give: &node.TypeRef{Spelling: "AsyncIterableIterator", Args: []*node.TypeRef{arg}},
				want: rules.TypeShape{Form: symbol.FormStream, Spelling: "AsyncIterableIterator", Async: true},
			},
			{
				name: "classifies AsyncGenerator with an argument as an asynchronous stream",
				give: &node.TypeRef{Spelling: "AsyncGenerator", Args: []*node.TypeRef{arg}},
				want: rules.TypeShape{Form: symbol.FormStream, Spelling: "AsyncGenerator", Async: true},
			},
			{
				name: "classifies Array without an argument as opaque", give: &node.TypeRef{Spelling: "Array"},
				want: rules.TypeShape{Form: symbol.FormOpaque, Spelling: "Array"},
			},
			{
				name: "classifies bigint as opaque", give: &node.TypeRef{Spelling: "bigint"},
				want: rules.TypeShape{Form: symbol.FormOpaque, Spelling: "bigint"},
			},
			{
				name: "classifies Promise as opaque",
				give: &node.TypeRef{Spelling: "Promise", Args: []*node.TypeRef{arg}},
				want: rules.TypeShape{Form: symbol.FormOpaque, Spelling: "Promise"},
			},
			{
				name: "classifies a literal type as opaque", give: &node.TypeRef{Spelling: `"read"`},
				want: rules.TypeShape{Form: symbol.FormOpaque, Spelling: `"read"`},
			},
			{
				name: "classifies a name that an import binds as opaque",
				give: &node.TypeRef{Spelling: "Date", Package: outsideLib},
				want: rules.TypeShape{Form: symbol.FormOpaque, Spelling: "Date"},
			},
			{
				name: "classifies a structural reference as opaque",
				give: &node.TypeRef{Spelling: "string[]", Form: symbol.FormList},
				want: rules.TypeShape{Form: symbol.FormOpaque, Spelling: "string[]"},
			},
			{name: "classifies no reference as opaque", give: nil, want: rules.TypeShape{Form: symbol.FormOpaque}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tsrules.New().Builtin(tt.give, rules.View{}), tt.want,
					"Builtin returns the shape of the reference")
			})
		}

		t.Run("folds the argument of Array into the list's child", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			list := &node.TypeRef{Spelling: "Array", Args: []*node.TypeRef{{Spelling: numberName}}}
			shape := rules.NewBound(tsrules.New(), f.view, nil).TypeOf(list)
			assert.Length(t, shape.Elems, 1, "a list has one child")
			expect.Equal(t, shape.Elems[0].Form, symbol.FormScalar, "the child is the number")
		})

		t.Run("folds a Record of one argument to opaque", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			record := &node.TypeRef{Spelling: "Record", Args: []*node.TypeRef{{Spelling: stringName}}}
			shape := rules.NewBound(tsrules.New(), f.view, nil).TypeOf(record)
			assert.Equal(t, shape.Form, symbol.FormOpaque, "a Record takes two arguments")
		})
	})
}

// TestBuiltinAllocs checks that the builtin table classifies a spelling
// without an allocation. The ordinary test run runs no benchmark, so
// this test checks the ceilings.
func TestBuiltinAllocs(t *testing.T) {
	checkAllocs(t, builtinCalls())
}

// BenchmarkBuiltin measures the classification of a builtin under its
// ceiling.
func BenchmarkBuiltin(b *testing.B) {
	benchCalls(b, builtinCalls())
}

// builtinCalls returns the measured calls of builtin.go. The calls
// classify a scalar and a list, one from each half of the table.
func builtinCalls() []allocCall {
	r := tsrules.New()
	number := &node.TypeRef{Spelling: numberName}
	list := &node.TypeRef{Spelling: "Array", Args: []*node.TypeRef{number}}
	var shape rules.TypeShape
	return []allocCall{
		{
			name: "Builtin", caseName: "a number",
			call: func() { shape = r.Builtin(number, rules.View{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, shape.Form, symbol.FormScalar, "Builtin classifies the number")
			},
		},
		{
			name: "Builtin", caseName: "an array",
			call: func() { shape = r.Builtin(list, rules.View{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, shape.Form, symbol.FormList, "Builtin classifies the array")
			},
		},
	}
}
