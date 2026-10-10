// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/java/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// sourceLang is the language whose declarations the cases translate into
// Java.
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

// The packages of the library types that the cases expect, in the
// slash-separated form.
const (
	utilPackage     = "java/util"
	functionPackage = "java/util/function"
	timePackage     = "java/time"
)

// The allocations of the spellings that the allocation test and the
// benchmark measure.
const (
	// leafAllocs is a primitive: the reference.
	leafAllocs = 1
	// listAllocs is a list of a 32-bit integer: the list's reference, its
	// list of arguments and the boxed argument's reference.
	listAllocs = 3
	// arrayAllocs is an array of a translated reference: the array's
	// reference, its list of children, the child's reference and the
	// array's spelling.
	arrayAllocs = 4
	// mapAllocs is a map of text onto a 64-bit integer: the map's
	// reference, its list of arguments and the two arguments' references.
	mapAllocs = 4
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
	boolOf    = rules.Leaf(symbol.FormBool, "bool")
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

// TestType checks the Java spelling of each canonical form by the hub's
// table, with a boxed class where Java takes a class alone. It also
// checks the refusal of each form without a Java spelling, and the child
// that each refusal is about.
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
				name: "returns byte for an integer of 8 bits",
				give: rules.Scalar("int8", rules.ScalarInt, 8), want: &emit.TypeRef{Spelling: "byte"},
			},
			{
				name: "returns short for an integer of 16 bits",
				give: rules.Scalar("int16", rules.ScalarInt, 16), want: &emit.TypeRef{Spelling: "short"},
			},
			{
				name: "returns int for an integer of 32 bits",
				give: int32Of, want: &emit.TypeRef{Spelling: "int"},
			},
			{
				name: "returns long for an integer of 64 bits",
				give: int64Of, want: &emit.TypeRef{Spelling: "long"},
			},
			{
				name: "returns long for an integer of the platform's width",
				give: rules.Scalar("int", rules.ScalarInt, 0), want: &emit.TypeRef{Spelling: "long"},
			},
			{
				name: "returns the signed type of the width for an unsigned integer of 32 bits",
				give: rules.Scalar("uint32", rules.ScalarUint, 32), want: &emit.TypeRef{Spelling: "int"},
			},
			{
				name: "returns long for an unsigned integer of 64 bits",
				give: rules.Scalar("uint64", rules.ScalarUint, 64), want: &emit.TypeRef{Spelling: "long"},
			},
			{
				name: "returns float for a float of 32 bits",
				give: rules.Scalar("float32", rules.ScalarFloat, 32), want: &emit.TypeRef{Spelling: "float"},
			},
			{
				name: "returns double for a float of 64 bits",
				give: rules.Scalar("float64", rules.ScalarFloat, 64), want: &emit.TypeRef{Spelling: "double"},
			},
			{
				name: "returns boolean for Bool",
				give: boolOf, want: &emit.TypeRef{Spelling: "boolean"},
			},
			{
				name: "returns String for Text",
				give: text, want: &emit.TypeRef{Spelling: "String"},
			},
			{
				name: "returns Object for the top type",
				give: rules.Leaf(symbol.FormDynamic, "any"), want: &emit.TypeRef{Spelling: "Object"},
			},
			{
				name: "returns an array of byte for Bytes",
				give: rules.Leaf(symbol.FormBytes, "[]byte"),
				want: &emit.TypeRef{
					Spelling: "byte[]",
					Form:     symbol.FormArray,
					Elems:    []*emit.TypeRef{{Spelling: "byte"}},
				},
			},
			{
				name: "returns the boxed class of a primitive for an optional",
				give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "*int32", Elems: []rules.TypeShape{int32Of}},
				want: &emit.TypeRef{Spelling: "Integer"},
			},
			{
				name: "returns Boolean for an optional of Bool",
				give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "*bool", Elems: []rules.TypeShape{boolOf}},
				want: &emit.TypeRef{Spelling: "Boolean"},
			},
			{
				name: "returns the class for an optional of a class",
				give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "*string", Elems: []rules.TypeShape{text}},
				want: &emit.TypeRef{Spelling: "String"},
			},
			{
				name: "returns a list of a boxed class for a list of a primitive",
				give: rules.TypeShape{Form: symbol.FormList, Spelling: "[]int32", Elems: []rules.TypeShape{int32Of}},
				want: &emit.TypeRef{
					Spelling: "List",
					Package:  utilPackage,
					Args:     []*emit.TypeRef{{Spelling: "Integer"}},
				},
			},
			{
				name: "returns an array without a length for an array of a stated length",
				give: rules.TypeShape{
					Form:     symbol.FormArray,
					Spelling: "[3]int32",
					Length:   3,
					Elems:    []rules.TypeShape{int32Of},
				},
				want: &emit.TypeRef{
					Spelling: "int[]",
					Form:     symbol.FormArray,
					Elems:    []*emit.TypeRef{{Spelling: "int"}},
				},
			},
			{
				name: "returns an array for an array of an unstated length",
				give: rules.TypeShape{Form: symbol.FormArray, Spelling: "string[]", Elems: []rules.TypeShape{text}},
				want: &emit.TypeRef{
					Spelling: "String[]",
					Form:     symbol.FormArray,
					Elems:    []*emit.TypeRef{{Spelling: "String"}},
				},
			},
			{
				name: "returns the arguments of a translated reference in an array",
				give: rules.TypeShape{
					Form:     symbol.FormArray,
					Spelling: "[]Page[int64]",
					Elems:    []rules.TypeShape{rules.Reference(pageName, page, int64Of)},
				},
				want: &emit.TypeRef{
					Spelling: "Page<Long>[]",
					Form:     symbol.FormArray,
					Elems: []*emit.TypeRef{
						{Spelling: pageName, Target: page, Args: []*emit.TypeRef{{Spelling: "Long"}}},
					},
				},
			},
			{
				name: "returns a map of boxed classes for a map",
				give: rules.TypeShape{
					Form:     symbol.FormMap,
					Spelling: "map[string]int64",
					Elems:    []rules.TypeShape{text, int64Of},
				},
				want: &emit.TypeRef{
					Spelling: "Map",
					Package:  utilPackage,
					Args:     []*emit.TypeRef{{Spelling: "String"}, {Spelling: "Long"}},
				},
			},
			{
				name: "returns Runnable for a function type without parameters or results",
				give: rules.TypeShape{Form: symbol.FormFunc, Spelling: "func()"},
				want: &emit.TypeRef{Spelling: "Runnable"},
			},
			{
				name: "returns Supplier for a function type of one result",
				give: rules.TypeShape{
					Form:     symbol.FormFunc,
					Spelling: "func() int32",
					Elems:    []rules.TypeShape{int32Of},
				},
				want: &emit.TypeRef{
					Spelling: "Supplier",
					Package:  functionPackage,
					Args:     []*emit.TypeRef{{Spelling: "Integer"}},
				},
			},
			{
				name: "returns Consumer for a function type of one parameter",
				give: rules.TypeShape{
					Form:     symbol.FormFunc,
					Spelling: "func(string)",
					Split:    1,
					Elems:    []rules.TypeShape{text},
				},
				want: &emit.TypeRef{
					Spelling: "Consumer",
					Package:  functionPackage,
					Args:     []*emit.TypeRef{{Spelling: "String"}},
				},
			},
			{
				name: "returns Function for a function type of one parameter and one result",
				give: rules.TypeShape{
					Form:     symbol.FormFunc,
					Spelling: "func(string) int32",
					Split:    1,
					Elems:    []rules.TypeShape{text, int32Of},
				},
				want: &emit.TypeRef{
					Spelling: "Function", Package: functionPackage,
					Args: []*emit.TypeRef{{Spelling: "String"}, {Spelling: "Integer"}},
				},
			},
			{
				name: "returns BiConsumer for a function type of two parameters",
				give: rules.TypeShape{
					Form:     symbol.FormFunc,
					Spelling: "func(string, string)",
					Split:    2,
					Elems:    []rules.TypeShape{text, text},
				},
				want: &emit.TypeRef{
					Spelling: "BiConsumer", Package: functionPackage,
					Args: []*emit.TypeRef{{Spelling: "String"}, {Spelling: "String"}},
				},
			},
			{
				name: "returns BiFunction for a function type of two parameters and one result",
				give: rules.TypeShape{
					Form: symbol.FormFunc, Spelling: "func(string, string) bool", Split: 2,
					Elems: []rules.TypeShape{text, text, boolOf},
				},
				want: &emit.TypeRef{
					Spelling: "BiFunction", Package: functionPackage,
					Args: []*emit.TypeRef{{Spelling: "String"}, {Spelling: "String"}, {Spelling: "Boolean"}},
				},
			},
			{
				name: "returns the type of a borrow in the borrow's position",
				give: rules.TypeShape{Form: symbol.FormBorrow, Spelling: "&i32", Elems: []rules.TypeShape{int32Of}},
				want: &emit.TypeRef{Spelling: "int"},
			},
			{
				name: "returns an upper-bounded wildcard of a boxed bound",
				give: rules.TypeShape{
					Form: symbol.FormWildcard, Spelling: "impl Into<i32>", Variance: symbol.VarianceOut,
					Elems: []rules.TypeShape{int32Of},
				},
				want: &emit.TypeRef{
					Spelling: "? extends Integer", Form: symbol.FormWildcard, Variance: symbol.VarianceOut,
					Elems: []*emit.TypeRef{{Spelling: "Integer"}},
				},
			},
			{
				name: "returns a lower-bounded wildcard",
				give: rules.TypeShape{
					Form: symbol.FormWildcard, Spelling: "? super Session", Variance: symbol.VarianceIn,
					Elems: []rules.TypeShape{sessionTo},
				},
				want: &emit.TypeRef{
					Spelling: "? super Session", Form: symbol.FormWildcard, Variance: symbol.VarianceIn,
					Elems: []*emit.TypeRef{{Spelling: sessionName, Target: session}},
				},
			},
			{
				name: "returns the unbounded wildcard for a wildcard without a bound",
				give: rules.TypeShape{Form: symbol.FormWildcard, Spelling: "?"},
				want: &emit.TypeRef{Spelling: "?", Form: symbol.FormWildcard},
			},
			{
				name: "returns a translated reference with its boxed arguments",
				give: rules.Reference(pageName, page, int64Of),
				want: &emit.TypeRef{Spelling: pageName, Target: page, Args: []*emit.TypeRef{{Spelling: "Long"}}},
			},
			{
				name: "returns a translated reference for a sum",
				give: rules.TypeShape{Form: symbol.FormSum, Spelling: resultName, Ref: result},
				want: &emit.TypeRef{Spelling: resultName, Target: result},
			},
			{
				name: "returns Instant for the well-known timestamp",
				give: rules.Reference("time.Time", rules.WellKnownTimestamp),
				want: &emit.TypeRef{Spelling: "Instant", Package: timePackage},
			},
			{
				name: "returns Duration for the well-known duration",
				give: rules.Reference("time.Duration", rules.WellKnownDuration),
				want: &emit.TypeRef{Spelling: "Duration", Package: timePackage},
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
				assert.NoError(t, err, "Java spells the form")
				assert.Equal(t, got, tt.want, "the reference")
			})
		}

		refusals := []struct {
			name string
			give rules.TypeShape
			want string
		}{
			{
				name: "returns an error for a tuple",
				give: rules.TypeShape{
					Form:     symbol.FormTuple,
					Spelling: "(String, i32)",
					Elems:    []rules.TypeShape{text, int32Of},
				},
				want: "java: a tuple has no Java spelling",
			},
			{
				name: "returns an error for a union",
				give: union, want: "java: a union has no Java spelling",
			},
			{
				name: "returns an error for an intersection",
				give: rules.TypeShape{Form: symbol.FormIntersection, Spelling: "A & B"},
				want: "java: an intersection has no Java spelling",
			},
			{
				name: "returns an error for a synchronous stream",
				give: rules.TypeShape{
					Form:     symbol.FormStream,
					Spelling: "chan int32",
					Elems:    []rules.TypeShape{int32Of},
				},
				want: "java: a synchronous stream has no Java spelling",
			},
			{
				name: "returns an error for an asynchronous stream",
				give: rules.TypeShape{
					Form:     symbol.FormStream,
					Spelling: "AsyncIterable<number>",
					Async:    true,
					Elems:    []rules.TypeShape{int32Of},
				},
				want: "java: an asynchronous stream has no Java spelling",
			},
			{
				name: "returns an error for an inline type",
				give: rules.TypeShape{Form: symbol.FormInline, Spelling: "struct{}"},
				want: "java: an inline type has no Java spelling",
			},
			{
				name: "returns an error for an opaque type",
				give: rules.Opaque(nil),
				want: "java: a type that the rules of its language do not classify has no Java spelling",
			},
			{
				name: "returns an error for the well-known empty value",
				give: rules.Reference("google.protobuf.Empty", rules.WellKnownEmpty),
				want: "java: the well-known empty value has no Java spelling",
			},
			{
				name: "returns an error for an integer of 128 bits",
				give: rules.Scalar("i128", rules.ScalarInt, 128), want: "java: no Java int type has 128 bits",
			},
			{
				name: "returns an error for a float of the platform's width",
				give: rules.Scalar("float", rules.ScalarFloat, 0),
				want: "java: no Java float type has the platform's width",
			},
			{
				name: "returns an error for a function type of three parameters",
				give: rules.TypeShape{
					Form: symbol.FormFunc, Spelling: "func(string, string, string)", Split: 3,
					Elems: []rules.TypeShape{text, text, text},
				},
				want: "java: a function type of 3 parameters and 0 results has no Java functional interface",
			},
			{
				name: "returns an error for a function type of two results",
				give: rules.TypeShape{
					Form:     symbol.FormFunc,
					Spelling: "func() (string, string)",
					Elems:    []rules.TypeShape{text, text},
				},
				want: "java: a function type of 0 parameters and 2 results has no Java functional interface",
			},
			{
				name: "returns an error for a function type of more parameters than children",
				give: rules.TypeShape{Form: symbol.FormFunc, Spelling: "func()", Split: 1},
				want: "java: a function type splits its 0 children at 1",
			},
			{
				name: "returns an error for an optional of two children",
				give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "?", Elems: []rules.TypeShape{text, text}},
				want: "java: an optional takes one child, and the shape has 2",
			},
			{
				name: "returns an error for a map of one child",
				give: rules.TypeShape{Form: symbol.FormMap, Spelling: "map[string]", Elems: []rules.TypeShape{text}},
				want: "java: a map takes two children, a key and a value, and the shape has 1",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, plugin.Policy{})
				assert.HasError(t, err, "Java has no spelling of the form")
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
				name: "returns an error about the child of an array",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormArray, Elems: []rules.TypeShape{c}}
				},
			},
			{
				name: "returns an error about the child of a borrow",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormBorrow, Elems: []rules.TypeShape{c}}
				},
			},
			{
				name: "returns an error about the value of a map",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormMap, Elems: []rules.TypeShape{text, c}}
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
				assert.HasError(t, err, "Java has no spelling of the child")
				assert.Equal(
					t,
					err.Error(),
					"java: string | int32: a union has no Java spelling",
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
		assert.Equal(t, got.Spelling, tt.want, "Type spells "+tt.name+" in Java")
	}
}

// BenchmarkType measures the spoke that every translation into Java
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
				assert.Equal(b, got.Spelling, tt.want, "Type spells "+tt.name+" in Java")
			})
		}
	})
}

// typeCalls returns the spellings that the allocation test and the
// benchmark measure: a primitive, a list, an array and a map.
func typeCalls() []typeCall {
	return []typeCall{
		{name: "an integer of 32 bits", give: int32Of, allocs: leafAllocs, want: "int"},
		{
			name: "a list of a primitive", allocs: listAllocs, want: "List",
			give: rules.TypeShape{Form: symbol.FormList, Spelling: "[]int32", Elems: []rules.TypeShape{int32Of}},
		},
		{
			name: "an array of a reference", allocs: arrayAllocs, want: "Session[]",
			give: rules.TypeShape{Form: symbol.FormArray, Spelling: "[]Session", Elems: []rules.TypeShape{sessionTo}},
		},
		{
			name:   "a map",
			allocs: mapAllocs,
			want:   "Map",
			give: rules.TypeShape{
				Form:     symbol.FormMap,
				Spelling: "map[string]int64",
				Elems:    []rules.TypeShape{text, int64Of},
			},
		},
	}
}
