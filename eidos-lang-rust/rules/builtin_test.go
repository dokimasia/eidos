// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	rustrules "go.dokimi.dev/eidos/lang/rust/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The ceilings of Builtin: a primitive allocates nothing, and an iterator
// allocates the list of its child, the memo of the binding that folds the
// type of its binding Item, the first group of the memo and the shape that
// the memo stores out of line.
const (
	primitiveAllocs = 0
	iteratorAllocs  = 4
)

// The spellings of the types of the standard library that the specs build
// references to.
const (
	optionName   = "Option"
	vecName      = "Vec"
	boxName      = "Box"
	errorName    = "Error"
	iteratorName = "Iterator"
	durationName = "Duration"
	u8Name       = "u8"
	stringName   = "String"
	strName      = "str"
	charName     = "char"
)

// TestBuiltin pins the shape of each type of the standard library that the
// rules fold, and of the references that fold to Opaque.
func TestBuiltin(t *testing.T) {
	t.Parallel()

	t.Run("Builtin", func(t *testing.T) {
		t.Parallel()

		u8 := &node.TypeRef{Spelling: u8Name}
		item := &node.TypeRef{Spelling: "Item=u8", Args: []*node.TypeRef{u8}}
		dynError := &node.TypeRef{
			Spelling: "dyn Error+Send+Sync", Form: symbol.FormIntersection,
			Elems: []*node.TypeRef{
				{Spelling: errorName, Package: "std/error"}, {Spelling: "Send"}, {Spelling: "std::marker::Sync"},
			},
		}
		tests := []struct {
			name string
			give *node.TypeRef
			want rules.TypeShape
		}{
			{
				name: "returns a bool for bool",
				give: &node.TypeRef{Spelling: "bool"},
				want: rules.Leaf(symbol.FormBool, "bool"),
			},
			{
				name: "returns a signed integer of its width for i32",
				give: &node.TypeRef{Spelling: "i32"}, want: rules.Scalar("i32", rules.ScalarInt, 32),
			},
			{
				name: "returns a signed integer of 128 bits for i128",
				give: &node.TypeRef{Spelling: "i128"}, want: rules.Scalar("i128", rules.ScalarInt, 128),
			},
			{
				name: "returns an unsigned integer of the width of the platform for usize",
				give: &node.TypeRef{Spelling: "usize"}, want: rules.Scalar("usize", rules.ScalarUint, 0),
			},
			{
				name: "returns a float of its width for f32",
				give: &node.TypeRef{Spelling: "f32"}, want: rules.Scalar("f32", rules.ScalarFloat, 32),
			},
			{
				name: "returns the primitive of a path through std::primitive",
				give: &node.TypeRef{Spelling: "std::primitive::u8", Package: "std/primitive"},
				want: rules.Scalar("std::primitive::u8", rules.ScalarUint, 8),
			},
			{
				name: "returns text for char",
				give: &node.TypeRef{Spelling: charName},
				want: rules.Leaf(symbol.FormText, charName),
			},
			{
				name: "returns text for str",
				give: &node.TypeRef{Spelling: strName},
				want: rules.Leaf(symbol.FormText, strName),
			},
			{
				name: "returns text for String", give: &node.TypeRef{Spelling: stringName},
				want: rules.Leaf(symbol.FormText, stringName),
			},
			{
				name: "returns text for PathBuf of std::path",
				give: &node.TypeRef{Spelling: "PathBuf", Package: "std/path"},
				want: rules.Leaf(symbol.FormText, "PathBuf"),
			},
			{
				name: "returns text for a Cow of str past its lifetime",
				give: &node.TypeRef{Spelling: "Cow", Package: "std/borrow", Args: []*node.TypeRef{
					{Spelling: "'a"}, {Spelling: strName},
				}},
				want: rules.Leaf(symbol.FormText, "Cow"),
			},
			{
				name: "returns the well-known empty value for the unit type", give: &node.TypeRef{Spelling: "()"},
				want: rules.Reference("()", rules.WellKnownEmpty),
			},
			{
				name: "returns a union without members for the never type", give: &node.TypeRef{Spelling: "!"},
				want: rules.TypeShape{Form: symbol.FormUnion, Spelling: "!"},
			},
			{
				name: "returns an optional for Option",
				give: &node.TypeRef{Spelling: optionName, Args: []*node.TypeRef{u8}},
				want: rules.TypeShape{Form: symbol.FormOptional, Spelling: optionName},
			},
			{
				name: "returns a mutable list for Vec",
				give: &node.TypeRef{Spelling: vecName, Args: []*node.TypeRef{u8}},
				want: rules.TypeShape{Form: symbol.FormList, Spelling: vecName, Mutability: symbol.MutabilityMutable},
			},
			{
				name: "returns a mutable list for a path of alloc to Vec",
				give: &node.TypeRef{Spelling: "alloc::vec::Vec", Package: "alloc/vec", Args: []*node.TypeRef{u8}},
				want: rules.TypeShape{
					Form: symbol.FormList, Spelling: "alloc::vec::Vec", Mutability: symbol.MutabilityMutable,
				},
			},
			{
				name: "returns a mutable list for VecDeque of std::collections",
				give: &node.TypeRef{Spelling: "VecDeque", Package: collectionsPkg, Args: []*node.TypeRef{u8}},
				want: rules.TypeShape{
					Form:       symbol.FormList,
					Spelling:   "VecDeque",
					Mutability: symbol.MutabilityMutable,
				},
			},
			{
				name: "returns a mutable map for HashMap of std::collections",
				give: &node.TypeRef{Spelling: "HashMap", Package: collectionsPkg, Args: []*node.TypeRef{u8, u8}},
				want: rules.TypeShape{Form: symbol.FormMap, Spelling: "HashMap", Mutability: symbol.MutabilityMutable},
			},
			{
				name: "returns a mutable borrow for Box",
				give: &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{u8}},
				want: rules.TypeShape{Form: symbol.FormBorrow, Spelling: boxName, Mutability: symbol.MutabilityMutable},
			},
			{
				name: "returns an immutable borrow for Arc of std::sync",
				give: &node.TypeRef{Spelling: "Arc", Package: "std/sync", Args: []*node.TypeRef{u8}},
				want: rules.TypeShape{Form: symbol.FormBorrow, Spelling: "Arc", Mutability: symbol.MutabilityImmutable},
			},
			{
				name: "returns the well-known error for a Box of a trait object of the error trait",
				give: &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{dynError}},
				want: rules.Reference(boxName, rules.WellKnownError),
			},
			{
				name: "returns a mutable borrow for a Box of a trait object of another trait",
				give: &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{{
					Spelling: "dyn Debug", Form: symbol.FormIntersection, Elems: []*node.TypeRef{{Spelling: "Debug"}},
				}}},
				want: rules.TypeShape{Form: symbol.FormBorrow, Spelling: boxName, Mutability: symbol.MutabilityMutable},
			},
			{
				name: "returns a mutable borrow for a Box of a trait object of two error traits",
				give: &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{{
					Spelling: "dyn Error+Error", Form: symbol.FormIntersection,
					Elems: []*node.TypeRef{{Spelling: errorName}, {Spelling: errorName}},
				}}},
				want: rules.TypeShape{Form: symbol.FormBorrow, Spelling: boxName, Mutability: symbol.MutabilityMutable},
			},
			{
				name: "returns a mutable borrow for a Box of a trait object of a trait of the crate",
				give: &node.TypeRef{Spelling: boxName, Args: []*node.TypeRef{{
					Spelling: "dyn Store", Form: symbol.FormIntersection,
					Elems: []*node.TypeRef{{Spelling: "Store", Target: storeID}},
				}}},
				want: rules.TypeShape{Form: symbol.FormBorrow, Spelling: boxName, Mutability: symbol.MutabilityMutable},
			},
			{
				name: "returns a mutable borrow for a Box of a type that is no trait object",
				give: &node.TypeRef{
					Spelling: boxName,
					Args:     []*node.TypeRef{{Spelling: "[u8]", Form: symbol.FormList}},
				},
				want: rules.TypeShape{Form: symbol.FormBorrow, Spelling: boxName, Mutability: symbol.MutabilityMutable},
			},
			{
				name: "returns the well-known duration for Duration of std::time",
				give: &node.TypeRef{Spelling: durationName, Package: timePkg},
				want: rules.Reference(durationName, rules.WellKnownDuration),
			},
			{
				name: "returns the well-known duration for Duration of core::time",
				give: &node.TypeRef{Spelling: "core::time::Duration", Package: "core/time"},
				want: rules.Reference("core::time::Duration", rules.WellKnownDuration),
			},
			{
				name: "returns the well-known timestamp for SystemTime of std::time",
				give: &node.TypeRef{Spelling: "SystemTime", Package: timePkg},
				want: rules.Reference("SystemTime", rules.WellKnownTimestamp),
			},
			{
				name: "returns the well-known error for io::Error",
				give: &node.TypeRef{Spelling: "io::Error", Package: "std/io"},
				want: rules.Reference("io::Error", rules.WellKnownError),
			},
			{
				name: "returns a stream of the type of the binding Item for Iterator",
				give: &node.TypeRef{Spelling: iteratorName, Args: []*node.TypeRef{item}},
				want: rules.TypeShape{
					Form: symbol.FormStream, Spelling: iteratorName,
					Elems: []rules.TypeShape{rules.Scalar(u8Name, rules.ScalarUint, 8)},
				},
			},
			{
				name: "returns a stream of the binding Item after another binding",
				give: &node.TypeRef{Spelling: "IntoIterator", Args: []*node.TypeRef{
					{Spelling: "IntoIter=I", Args: []*node.TypeRef{{Spelling: "I"}}}, item,
				}},
				want: rules.TypeShape{
					Form: symbol.FormStream, Spelling: "IntoIterator",
					Elems: []rules.TypeShape{rules.Scalar(u8Name, rules.ScalarUint, 8)},
				},
			},
			{
				name: "returns a stream of the binding Item after an argument that is no binding",
				give: &node.TypeRef{Spelling: iteratorName, Args: []*node.TypeRef{nil, item}},
				want: rules.TypeShape{
					Form: symbol.FormStream, Spelling: iteratorName,
					Elems: []rules.TypeShape{rules.Scalar(u8Name, rules.ScalarUint, 8)},
				},
			},
			{
				name: "returns a stream of an opaque value for an Iterator without a binding Item",
				give: &node.TypeRef{Spelling: iteratorName},
				want: rules.TypeShape{
					Form:     symbol.FormStream,
					Spelling: iteratorName,
					Elems:    []rules.TypeShape{{Form: symbol.FormOpaque}},
				},
			},
			{
				name: "returns an asynchronous stream for Stream of futures",
				give: &node.TypeRef{Spelling: "Stream", Package: "futures/stream", Args: []*node.TypeRef{item}},
				want: rules.TypeShape{
					Form: symbol.FormStream, Spelling: "Stream", Async: true,
					Elems: []rules.TypeShape{rules.Scalar(u8Name, rules.ScalarUint, 8)},
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, rustrules.Rules{}.Builtin(tt.give, rules.View{}), tt.want, "the shape of the reference")
			})
		}

		opaque := []struct {
			name string
			give *node.TypeRef
		}{
			{name: "returns Opaque for a nil reference"},
			{
				name: "returns Opaque for a reference with a target",
				give: &node.TypeRef{Spelling: "Order", Target: orderID},
			},
			{
				name: "returns Opaque for a structural reference",
				give: &node.TypeRef{Spelling: "(u8,)", Form: symbol.FormTuple},
			},
			{
				name: "returns Opaque for a set",
				give: &node.TypeRef{Spelling: "HashSet", Package: collectionsPkg, Args: []*node.TypeRef{u8}},
			},
			{name: "returns Opaque for Instant", give: &node.TypeRef{Spelling: "Instant", Package: timePkg}},
			{
				name: "returns Opaque for a type of another crate",
				give: &node.TypeRef{Spelling: "Utc", Package: "chrono"},
			},
			{
				name: "returns Opaque for a name of the table in another crate",
				give: &node.TypeRef{Spelling: durationName, Package: "chrono"},
			},
			{
				name: "returns Opaque for a name of the table in another module",
				give: &node.TypeRef{Spelling: "HashMap", Package: "std/sync"},
			},
			{name: "returns Opaque for an associated type binding", give: item},
			{name: "returns Opaque for a Vec without its type argument", give: &node.TypeRef{Spelling: vecName}},
			{
				name: "returns Opaque for a Vec with an allocator",
				give: &node.TypeRef{Spelling: vecName, Args: []*node.TypeRef{u8, {Spelling: "A"}}},
			},
			{
				name: "returns Opaque for a Cow of a slice",
				give: &node.TypeRef{Spelling: "Cow", Args: []*node.TypeRef{{Spelling: "[u8]", Form: symbol.FormList}}},
			},
			{
				name: "returns Opaque for Stream of the standard library",
				give: &node.TypeRef{Spelling: "Stream", Package: "std/stream"},
			},
		}
		for _, tt := range opaque {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := rustrules.Rules{}.Builtin(tt.give, rules.View{})
				assert.Equal(t, got.Form, symbol.FormOpaque, "the reference has no shape of its own")
			})
		}
	})
}

// TestBuiltinAllocs checks the ceilings of Builtin in the ordinary run, which
// does not run benchmarks.
func TestBuiltinAllocs(t *testing.T) {
	checkAllocs(t, builtinCalls())
}

// BenchmarkBuiltin measures Builtin for a primitive, a generic type and an
// iterator under their ceilings.
func BenchmarkBuiltin(b *testing.B) {
	benchCalls(b, builtinCalls())
}

// builtinCalls returns the calls of Builtin that the allocation test and the
// benchmark measure.
func builtinCalls() []allocCall {
	r := rustrules.Rules{}
	u8 := &node.TypeRef{Spelling: u8Name}
	tests := []struct {
		caseName string
		give     *node.TypeRef
		allocs   uint64
		want     symbol.TypeForm
	}{
		{caseName: "a primitive", give: u8, allocs: primitiveAllocs, want: symbol.FormScalar},
		{
			caseName: "a generic type", give: &node.TypeRef{Spelling: vecName, Args: []*node.TypeRef{u8}},
			allocs: primitiveAllocs, want: symbol.FormList,
		},
		{
			caseName: "an iterator",
			give: &node.TypeRef{Spelling: iteratorName, Args: []*node.TypeRef{
				{Spelling: "Item=u8", Args: []*node.TypeRef{u8}},
			}},
			allocs: iteratorAllocs, want: symbol.FormStream,
		},
	}
	calls := make([]allocCall, 0, len(tests))
	for _, tt := range tests {
		var got rules.TypeShape
		calls = append(calls, allocCall{
			name: "Builtin", caseName: tt.caseName, allocs: tt.allocs,
			call:  func() { got = r.Builtin(tt.give, rules.View{}) },
			check: func(tb assert.TB) { assert.Equal(tb, got.Form, tt.want, "Builtin folds "+tt.caseName) },
		})
	}
	return calls
}
