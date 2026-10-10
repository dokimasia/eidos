// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/rust/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// sourceLang is the language whose declarations the cases translate into
// Rust.
const sourceLang symbol.Lang = "golang"

// The spellings of the source language's shapes.
const (
	textSpelling  = "string"
	int32Spelling = "int32"
	int64Spelling = "int64"
	sessionName   = "Session"
	pageName      = "Page"
	resultName    = "Result"
)

// The modules of the library types that the cases expect, in the
// slash-separated form.
const (
	collectionsModule = "std/collections"
	timeModule        = "std/time"
)

// The allocations of the spellings that the allocation test and the
// benchmark measure.
const (
	// leafAllocs is a primitive: the reference.
	leafAllocs = 1
	// listAllocs is a list of a translated reference: the vector's
	// reference, its list of arguments and the argument's reference.
	listAllocs = 3
	// mapAllocs is a map of text onto a 64-bit integer: the map's
	// reference, its list of arguments and the two arguments' references.
	mapAllocs = 4
	// functionAllocs is a function type of one parameter and one result:
	// the box and its list of arguments, the function type and its list
	// of children, the two children's references, and the function
	// type's spelling.
	functionAllocs = 7
)

// The declarations of the source language that the cases reference. state
// is a type that session declares.
var (
	session = symbol.Identity{Lang: sourceLang, Package: "svc", Name: sessionName, Kind: symbol.KindStruct}
	page    = symbol.Identity{Lang: sourceLang, Package: "svc", Name: pageName, Kind: symbol.KindStruct}
	result  = symbol.Identity{Lang: sourceLang, Package: "svc", Name: resultName, Kind: symbol.KindSum}
	state   = symbol.Identity{
		Lang: sourceLang, Package: "svc", Owner: sessionName, Name: "State", Kind: symbol.KindEnum,
	}
)

// The shapes the cases compose.
var (
	text      = rules.Leaf(symbol.FormText, textSpelling)
	int32Of   = rules.Scalar(int32Spelling, rules.ScalarInt, 32)
	int64Of   = rules.Scalar(int64Spelling, rules.ScalarInt, 64)
	sessionTo = rules.Reference(sessionName, session)
	union     = rules.TypeShape{
		Form:     symbol.FormUnion,
		Spelling: "string | int32",
		Elems:    []rules.TypeShape{text, int32Of},
	}
)

// typeCall is one spelling that the allocation test and the benchmark
// measure: the shape, its ceiling and the spelling that it returns.
type typeCall struct {
	name   string
	give   rules.TypeShape
	allocs uint64
	want   string
}

// TestType checks the Rust spelling of each canonical form by the hub's
// table. It also checks the refusal of each form without a Rust
// spelling, and the child that each refusal is about.
func TestType(t *testing.T) {
	t.Parallel()

	t.Run("Type", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rules.TypeShape
			want *emit.TypeRef
		}{
			{
				name: "returns i8 for an integer of 8 bits",
				give: rules.Scalar("int8", rules.ScalarInt, 8), want: &emit.TypeRef{Spelling: "i8"},
			},
			{
				name: "returns i16 for an integer of 16 bits",
				give: rules.Scalar("int16", rules.ScalarInt, 16), want: &emit.TypeRef{Spelling: "i16"},
			},
			{
				name: "returns i32 for an integer of 32 bits",
				give: int32Of, want: &emit.TypeRef{Spelling: "i32"},
			},
			{
				name: "returns i64 for an integer of 64 bits",
				give: int64Of, want: &emit.TypeRef{Spelling: "i64"},
			},
			{
				name: "returns i128 for an integer of 128 bits",
				give: rules.Scalar("int128", rules.ScalarInt, 128), want: &emit.TypeRef{Spelling: "i128"},
			},
			{
				name: "returns isize for an integer of the platform's width",
				give: rules.Scalar("int", rules.ScalarInt, 0), want: &emit.TypeRef{Spelling: "isize"},
			},
			{
				name: "returns u8 for an unsigned integer of 8 bits",
				give: rules.Scalar("uint8", rules.ScalarUint, 8), want: &emit.TypeRef{Spelling: "u8"},
			},
			{
				name: "returns u64 for an unsigned integer of 64 bits",
				give: rules.Scalar("uint64", rules.ScalarUint, 64), want: &emit.TypeRef{Spelling: "u64"},
			},
			{
				name: "returns usize for an unsigned integer of the platform's width",
				give: rules.Scalar("uint", rules.ScalarUint, 0), want: &emit.TypeRef{Spelling: "usize"},
			},
			{
				name: "returns f32 for a float of 32 bits",
				give: rules.Scalar("float32", rules.ScalarFloat, 32), want: &emit.TypeRef{Spelling: "f32"},
			},
			{
				name: "returns f64 for a float of 64 bits",
				give: rules.Scalar("float64", rules.ScalarFloat, 64), want: &emit.TypeRef{Spelling: "f64"},
			},
			{
				name: "returns bool for Bool",
				give: rules.Leaf(symbol.FormBool, "boolean"), want: &emit.TypeRef{Spelling: "bool"},
			},
			{
				name: "returns String for Text",
				give: text, want: &emit.TypeRef{Spelling: "String"},
			},
			{
				name: "returns a vector of u8 for Bytes",
				give: rules.Leaf(symbol.FormBytes, "[]byte"),
				want: &emit.TypeRef{Spelling: "Vec", Args: []*emit.TypeRef{{Spelling: "u8"}}},
			},
			{
				name: "returns Option for an optional",
				give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "*string", Elems: []rules.TypeShape{text}},
				want: &emit.TypeRef{Spelling: "Option", Args: []*emit.TypeRef{{Spelling: "String"}}},
			},
			{
				name: "returns a vector of a translated reference for a list",
				give: rules.TypeShape{
					Form:     symbol.FormList,
					Spelling: "[]Session",
					Elems:    []rules.TypeShape{sessionTo},
				},
				want: &emit.TypeRef{Spelling: "Vec", Args: []*emit.TypeRef{{Spelling: sessionName, Target: session}}},
			},
			{
				name: "returns an array of its length for an array",
				give: rules.TypeShape{
					Form:     symbol.FormArray,
					Spelling: "[4]int32",
					Length:   4,
					Elems:    []rules.TypeShape{int32Of},
				},
				want: &emit.TypeRef{
					Spelling: "[i32; 4]", Form: symbol.FormArray, Length: 4, Elems: []*emit.TypeRef{{Spelling: "i32"}},
				},
			},
			{
				name: "returns the arguments of a translated reference in an array",
				give: rules.TypeShape{
					Form: symbol.FormArray, Spelling: "[2]Page[int64]", Length: 2,
					Elems: []rules.TypeShape{rules.Reference(pageName, page, int64Of)},
				},
				want: &emit.TypeRef{
					Spelling: "[Page<i64>; 2]",
					Form:     symbol.FormArray,
					Length:   2,
					Elems: []*emit.TypeRef{
						{Spelling: pageName, Target: page, Args: []*emit.TypeRef{{Spelling: "i64"}}},
					},
				},
			},
			{
				name: "returns HashMap for a map",
				give: rules.TypeShape{
					Form:     symbol.FormMap,
					Spelling: "map[string]int64",
					Elems:    []rules.TypeShape{text, int64Of},
				},
				want: &emit.TypeRef{
					Spelling: "HashMap", Package: collectionsModule,
					Args: []*emit.TypeRef{{Spelling: "String"}, {Spelling: "i64"}},
				},
			},
			{
				name: "returns a tuple for a tuple",
				give: rules.TypeShape{
					Form:     symbol.FormTuple,
					Spelling: "[string, int64]",
					Elems:    []rules.TypeShape{text, int64Of},
				},
				want: &emit.TypeRef{
					Spelling: "(String, i64)", Form: symbol.FormTuple,
					Elems: []*emit.TypeRef{{Spelling: "String"}, {Spelling: "i64"}},
				},
			},
			{
				name: "returns a tuple of one member with its trailing comma",
				give: rules.TypeShape{Form: symbol.FormTuple, Spelling: "[string]", Elems: []rules.TypeShape{text}},
				want: &emit.TypeRef{
					Spelling: "(String,)",
					Form:     symbol.FormTuple,
					Elems:    []*emit.TypeRef{{Spelling: "String"}},
				},
			},
			{
				name: "returns a boxed Fn of one parameter and one result",
				give: rules.TypeShape{
					Form:     symbol.FormFunc,
					Spelling: "func(string) int64",
					Split:    1,
					Elems:    []rules.TypeShape{text, int64Of},
				},
				want: &emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{{
					Spelling: "dyn Fn(String) -> i64", Form: symbol.FormFunc, Split: 1,
					Elems: []*emit.TypeRef{{Spelling: "String"}, {Spelling: "i64"}},
				}}},
			},
			{
				name: "returns a boxed Fn without an arrow for a function type without a result",
				give: rules.TypeShape{
					Form:     symbol.FormFunc,
					Spelling: "func(string)",
					Split:    1,
					Elems:    []rules.TypeShape{text},
				},
				want: &emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{
					{
						Spelling: "dyn Fn(String)",
						Form:     symbol.FormFunc,
						Split:    1,
						Elems:    []*emit.TypeRef{{Spelling: "String"}},
					},
				}},
			},
			{
				name: "returns a boxed Fn that returns a tuple of two results",
				give: rules.TypeShape{
					Form: symbol.FormFunc, Spelling: "func() (int64, string)", Elems: []rules.TypeShape{int64Of, text},
				},
				want: &emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{{
					Spelling: "dyn Fn() -> (i64, String)", Form: symbol.FormFunc,
					Elems: []*emit.TypeRef{{Spelling: "i64"}, {Spelling: "String"}},
				}}},
			},
			{
				name: "returns the upper bound of a wildcard",
				give: rules.TypeShape{
					Form: symbol.FormWildcard, Spelling: "? extends Session", Variance: symbol.VarianceOut,
					Elems: []rules.TypeShape{sessionTo},
				},
				want: &emit.TypeRef{Spelling: sessionName, Target: session},
			},
			{
				name: "returns a translated reference with its translated arguments",
				give: rules.Reference(pageName, page, int64Of),
				want: &emit.TypeRef{Spelling: pageName, Target: page, Args: []*emit.TypeRef{{Spelling: "i64"}}},
			},
			{
				name: "returns a translated reference for a sum",
				give: rules.TypeShape{Form: symbol.FormSum, Spelling: resultName, Ref: result},
				want: &emit.TypeRef{Spelling: resultName, Target: result},
			},
			{
				name: "returns SystemTime for the well-known timestamp",
				give: rules.Reference("time.Time", rules.WellKnownTimestamp),
				want: &emit.TypeRef{Spelling: "SystemTime", Package: timeModule},
			},
			{
				name: "returns Duration for the well-known duration",
				give: rules.Reference("time.Duration", rules.WellKnownDuration),
				want: &emit.TypeRef{Spelling: "Duration", Package: timeModule},
			},
			{
				name: "returns the unit type for the well-known empty value",
				give: rules.Reference("google.protobuf.Empty", rules.WellKnownEmpty),
				want: &emit.TypeRef{Spelling: "()", Form: symbol.FormTuple},
			},
			{
				name: "returns the flat name for a reference to a nested type",
				give: rules.Reference("Session.State", state),
				want: &emit.TypeRef{Spelling: "SessionState", Target: state},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, plugin.Policy{})
				assert.NoError(t, err, "Rust spells the form")
				assert.Equal(t, got, tt.want, "the reference")
			})
		}

		refusals := []struct {
			name string
			give rules.TypeShape
			want string
		}{
			{
				name: "returns an error for a union",
				give: union, want: "rust: a union has no Rust spelling",
			},
			{
				name: "returns an error for an intersection",
				give: rules.TypeShape{Form: symbol.FormIntersection, Spelling: "A & B"},
				want: "rust: an intersection has no Rust spelling",
			},
			{
				name: "returns an error for a synchronous stream",
				give: rules.TypeShape{
					Form:     symbol.FormStream,
					Spelling: "chan int32",
					Elems:    []rules.TypeShape{int32Of},
				},
				want: "rust: a synchronous stream has no Rust spelling",
			},
			{
				name: "returns an error for an asynchronous stream",
				give: rules.TypeShape{
					Form:     symbol.FormStream,
					Spelling: "AsyncIterable<number>",
					Async:    true,
					Elems:    []rules.TypeShape{int32Of},
				},
				want: "rust: an asynchronous stream has no Rust spelling",
			},
			{
				name: "returns an error for a borrow",
				give: rules.TypeShape{Form: symbol.FormBorrow, Spelling: "T", Elems: []rules.TypeShape{text}},
				want: "rust: a borrow of another language has no lifetime, and a Rust borrow needs one",
			},
			{
				name: "returns an error for a wildcard with a lower bound",
				give: rules.TypeShape{
					Form: symbol.FormWildcard, Spelling: "? super Session", Variance: symbol.VarianceIn,
					Elems: []rules.TypeShape{sessionTo},
				},
				want: "rust: a wildcard without an upper bound has no Rust spelling",
			},
			{
				name: "returns an error for a wildcard without a bound",
				give: rules.TypeShape{Form: symbol.FormWildcard, Spelling: "?"},
				want: "rust: a wildcard without an upper bound has no Rust spelling",
			},
			{
				name: "returns an error for an inline type",
				give: rules.TypeShape{Form: symbol.FormInline, Spelling: "struct{}"},
				want: "rust: an inline type has no Rust spelling",
			},
			{
				name: "returns an error for an opaque type",
				give: rules.Opaque(nil),
				want: "rust: a type that the rules of its language do not classify has no Rust spelling",
			},
			{
				name: "returns an error for the top type",
				give: rules.Leaf(symbol.FormDynamic, "any"), want: "rust: the top type has no Rust spelling",
			},
			{
				name: "returns an error for an integer of 256 bits",
				give: rules.Scalar("int256", rules.ScalarInt, 256), want: "rust: no Rust int type has 256 bits",
			},
			{
				name: "returns an error for a float of the platform's width",
				give: rules.Scalar("float", rules.ScalarFloat, 0),
				want: "rust: no Rust float type has the platform's width",
			},
			{
				name: "returns an error for an array of an unstated length",
				give: rules.TypeShape{Form: symbol.FormArray, Spelling: "string[]", Elems: []rules.TypeShape{text}},
				want: "rust: an array of an unstated length has no Rust spelling",
			},
			{
				name: "returns an error for an optional of two children",
				give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "?", Elems: []rules.TypeShape{text, text}},
				want: "rust: an optional takes one child, and the shape has 2",
			},
			{
				name: "returns an error for a map of one child",
				give: rules.TypeShape{Form: symbol.FormMap, Spelling: "map[string]", Elems: []rules.TypeShape{text}},
				want: "rust: a map takes two children, a key and a value, and the shape has 1",
			},
			{
				name: "returns an error for a function type of more parameters than children",
				give: rules.TypeShape{Form: symbol.FormFunc, Spelling: "func()", Split: 1},
				want: "rust: a function type splits its 0 children at 1",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, plugin.Policy{})
				assert.HasError(t, err, "Rust has no spelling of the form")
				assert.Equal(t, err.Error(), tt.want, "the error has the reason for the refusal")
				assert.Nil(t, got, "a refusal returns no reference")
			})
		}

		children := []struct {
			name string
			give func(child rules.TypeShape) rules.TypeShape
		}{
			{
				name: "returns an error about the child of a list",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormList, Elems: []rules.TypeShape{c}}
				},
			},
			{
				name: "returns an error about the value of a map",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormMap, Elems: []rules.TypeShape{text, c}}
				},
			},
			{
				name: "returns an error about a member of a tuple",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormTuple, Elems: []rules.TypeShape{text, c}}
				},
			},
			{
				name: "returns an error about the parameter of a function type",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormFunc, Split: 1, Elems: []rules.TypeShape{c}}
				},
			},
			{
				name: "returns an error about the bound of a wildcard",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{
						Form:     symbol.FormWildcard,
						Variance: symbol.VarianceOut,
						Elems:    []rules.TypeShape{c},
					}
				},
			},
			{
				name: "returns an error about an argument of a reference",
				give: func(c rules.TypeShape) rules.TypeShape { return rules.Reference(pageName, page, c) },
			},
		}
		for _, tt := range children {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Type(tt.give(union), plugin.Policy{})
				assert.HasError(t, err, "Rust has no spelling of the child")
				assert.Equal(
					t,
					err.Error(),
					"rust: string | int32: a union has no Rust spelling",
					"the error starts with the spelling of the child",
				)
			})
		}
	})
}

// Type allocates each reference that it returns, its list of children
// or arguments, and the spelling of a structural reference. The ordinary
// run, which runs no benchmark, checks those ceilings here.
func TestTypeAllocs(t *testing.T) {
	for _, tt := range typeCalls() {
		var (
			got *emit.TypeRef
			err error
		)
		assert.MaxAllocs(t, func() { got, err = spell.Type(tt.give, plugin.Policy{}) }, tt.allocs,
			"Type of "+tt.name+" allocates within its ceiling")
		assert.NoError(t, err, "Type spells "+tt.name)
		assert.Equal(t, got.Spelling, tt.want, "Type spells "+tt.name+" in Rust")
	}
}

// BenchmarkType measures the spoke that every translation into Rust
// calls.
func BenchmarkType(b *testing.B) {
	b.Run("Type", func(b *testing.B) {
		for _, tt := range typeCalls() {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					got *emit.TypeRef
					err error
				)
				for c.Loop() {
					got, err = spell.Type(tt.give, plugin.Policy{})
				}
				assert.NoError(b, err, "Type spells "+tt.name)
				assert.Equal(b, got.Spelling, tt.want, "Type spells "+tt.name+" in Rust")
			})
		}
	})
}

// typeCalls returns the spellings that the allocation test and the
// benchmark measure: a primitive, a list, a map and a function type.
func typeCalls() []typeCall {
	return []typeCall{
		{name: "an integer of 32 bits", give: int32Of, allocs: leafAllocs, want: "i32"},
		{
			name: "a list of a reference", allocs: listAllocs, want: "Vec",
			give: rules.TypeShape{Form: symbol.FormList, Spelling: "[]Session", Elems: []rules.TypeShape{sessionTo}},
		},
		{
			name:   "a map",
			allocs: mapAllocs,
			want:   "HashMap",
			give: rules.TypeShape{
				Form:     symbol.FormMap,
				Spelling: "map[string]int64",
				Elems:    []rules.TypeShape{text, int64Of},
			},
		},
		{
			name: "a function type", allocs: functionAllocs, want: "Box",
			give: rules.TypeShape{
				Form:     symbol.FormFunc,
				Spelling: "func(string) int64",
				Split:    1,
				Elems:    []rules.TypeShape{text, int64Of},
			},
		},
	}
}
