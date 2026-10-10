// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/go/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// sourceLang is the language whose declarations the cases translate into
// Go.
const sourceLang symbol.Lang = "typescript"

// The spellings of the source language's shapes.
const (
	textSpelling     = "string"
	numberSpelling   = "number"
	sessionName      = "Session"
	pageName         = "Page"
	resultName       = "Result"
	sessionsSpelling = "Session[]"
)

// The allocations of the spellings that the allocation test and the
// benchmark measure.
const (
	// leafAllocs is a predeclared type: the reference.
	leafAllocs = 1
	// listAllocs is a slice of a translated reference: the slice's
	// reference, its list of children, the child's reference and the
	// slice's spelling.
	listAllocs = 4
	// argsAllocs is a translated reference with one argument: the
	// reference, its list of arguments and the argument's reference.
	argsAllocs = 3
	// mapAllocs is a map of two predeclared types: the map's reference,
	// its list of children, the two children's references and the map's
	// spelling.
	mapAllocs = 5
)

// The declarations of the source language that the cases reference.
var (
	session = symbol.Identity{Lang: sourceLang, Package: "svc/store", Name: sessionName, Kind: symbol.KindInterface}
	page    = symbol.Identity{Lang: sourceLang, Package: "svc/store", Name: pageName, Kind: symbol.KindInterface}
	result  = symbol.Identity{Lang: sourceLang, Package: "svc/store", Name: resultName, Kind: symbol.KindSum}
)

// The cases compose text, a float of 64 bits, a reference to session,
// and a union of text and the float.
var (
	text      = rules.Leaf(symbol.FormText, textSpelling)
	number    = rules.Scalar(numberSpelling, rules.ScalarFloat, 64)
	sessionTo = rules.Reference(sessionName, session)
	union     = rules.TypeShape{
		Form: symbol.FormUnion, Spelling: "string | number", Elems: []rules.TypeShape{text, number},
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

// TestType checks the Go spelling of each canonical form by the hub's
// table. It also checks the refusal of each form without a Go spelling,
// and the child that each refusal is about.
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
				name: "returns int8 for an integer of 8 bits",
				give: rules.Scalar("i8", rules.ScalarInt, 8), want: &emit.TypeRef{Spelling: "int8"},
			},
			{
				name: "returns int16 for an integer of 16 bits",
				give: rules.Scalar("i16", rules.ScalarInt, 16), want: &emit.TypeRef{Spelling: "int16"},
			},
			{
				name: "returns int32 for an integer of 32 bits",
				give: rules.Scalar("int", rules.ScalarInt, 32), want: &emit.TypeRef{Spelling: "int32"},
			},
			{
				name: "returns int64 for an integer of 64 bits",
				give: rules.Scalar("long", rules.ScalarInt, 64), want: &emit.TypeRef{Spelling: "int64"},
			},
			{
				name: "returns int for an integer of the platform's width",
				give: rules.Scalar("isize", rules.ScalarInt, 0), want: &emit.TypeRef{Spelling: "int"},
			},
			{
				name: "returns uint8 for an unsigned integer of 8 bits",
				give: rules.Scalar("u8", rules.ScalarUint, 8), want: &emit.TypeRef{Spelling: "uint8"},
			},
			{
				name: "returns uint16 for an unsigned integer of 16 bits",
				give: rules.Scalar("u16", rules.ScalarUint, 16), want: &emit.TypeRef{Spelling: "uint16"},
			},
			{
				name: "returns uint32 for an unsigned integer of 32 bits",
				give: rules.Scalar("u32", rules.ScalarUint, 32), want: &emit.TypeRef{Spelling: "uint32"},
			},
			{
				name: "returns uint64 for an unsigned integer of 64 bits",
				give: rules.Scalar("u64", rules.ScalarUint, 64), want: &emit.TypeRef{Spelling: "uint64"},
			},
			{
				name: "returns uint for an unsigned integer of the platform's width",
				give: rules.Scalar("usize", rules.ScalarUint, 0), want: &emit.TypeRef{Spelling: "uint"},
			},
			{
				name: "returns float32 for a float of 32 bits",
				give: rules.Scalar("f32", rules.ScalarFloat, 32), want: &emit.TypeRef{Spelling: "float32"},
			},
			{
				name: "returns float64 for a float of 64 bits",
				give: number, want: &emit.TypeRef{Spelling: "float64"},
			},
			{
				name: "returns bool for Bool",
				give: rules.Leaf(symbol.FormBool, "boolean"), want: &emit.TypeRef{Spelling: "bool"},
			},
			{
				name: "returns string for Text",
				give: text, want: &emit.TypeRef{Spelling: "string"},
			},
			{
				name: "returns a slice of byte for Bytes",
				give: rules.Leaf(symbol.FormBytes, "Uint8Array"),
				want: &emit.TypeRef{
					Spelling: "[]byte", Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: "byte"}},
				},
			},
			{
				name: "returns a pointer for an optional",
				give: rules.TypeShape{
					Form:     symbol.FormOptional,
					Spelling: "string | undefined",
					Elems:    []rules.TypeShape{text},
				},
				want: &emit.TypeRef{
					Spelling: "*string", Form: symbol.FormOptional, Elems: []*emit.TypeRef{{Spelling: "string"}},
				},
			},
			{
				name: "returns a slice of a translated reference for a list",
				give: rules.TypeShape{
					Form:     symbol.FormList,
					Spelling: sessionsSpelling,
					Elems:    []rules.TypeShape{sessionTo},
				},
				want: &emit.TypeRef{
					Spelling: "[]Session", Form: symbol.FormList,
					Elems: []*emit.TypeRef{{Spelling: sessionName, Target: session}},
				},
			},
			{
				name: "returns an array of the length of an array",
				give: rules.TypeShape{
					Form: symbol.FormArray, Spelling: "[boolean, boolean]", Length: 2,
					Elems: []rules.TypeShape{rules.Leaf(symbol.FormBool, "boolean")},
				},
				want: &emit.TypeRef{
					Spelling: "[2]bool", Form: symbol.FormArray, Length: 2, Elems: []*emit.TypeRef{{Spelling: "bool"}},
				},
			},
			{
				name: "returns a map for a map",
				give: rules.TypeShape{
					Form: symbol.FormMap, Spelling: "Record<string, number>", Elems: []rules.TypeShape{text, number},
				},
				want: &emit.TypeRef{
					Spelling: "map[string]float64", Form: symbol.FormMap,
					Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: "float64"}},
				},
			},
			{
				name: "returns a function type of one result",
				give: rules.TypeShape{
					Form: symbol.FormFunc, Spelling: "(a: string) => number", Split: 1,
					Elems: []rules.TypeShape{text, number},
				},
				want: &emit.TypeRef{
					Spelling: "func(string) float64", Form: symbol.FormFunc, Split: 1,
					Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: "float64"}},
				},
			},
			{
				name: "returns a function type of two results in parentheses",
				give: rules.TypeShape{
					Form: symbol.FormFunc, Spelling: "(a: string) => [number, string]", Split: 1,
					Elems: []rules.TypeShape{text, number, text},
				},
				want: &emit.TypeRef{
					Spelling: "func(string) (float64, string)", Form: symbol.FormFunc, Split: 1,
					Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: "float64"}, {Spelling: "string"}},
				},
			},
			{
				name: "returns a function type without a result",
				give: rules.TypeShape{
					Form: symbol.FormFunc, Spelling: "(a: string, b: string) => void", Split: 2,
					Elems: []rules.TypeShape{text, text},
				},
				want: &emit.TypeRef{
					Spelling: "func(string, string)", Form: symbol.FormFunc, Split: 2,
					Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: "string"}},
				},
			},
			{
				name: "returns a channel for a synchronous stream",
				give: rules.TypeShape{
					Form:     symbol.FormStream,
					Spelling: "Iterator<number>",
					Elems:    []rules.TypeShape{number},
				},
				want: &emit.TypeRef{
					Spelling: "chan float64", Form: symbol.FormStream, Elems: []*emit.TypeRef{{Spelling: "float64"}},
				},
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
				name: "returns any for a wildcard without a bound",
				give: rules.TypeShape{Form: symbol.FormWildcard, Spelling: "?"},
				want: &emit.TypeRef{Spelling: "any"},
			},
			{
				name: "returns a translated reference for a reference",
				give: sessionTo, want: &emit.TypeRef{Spelling: sessionName, Target: session},
			},
			{
				name: "returns a translated reference with its translated arguments",
				give: rules.Reference(pageName, page, text),
				want: &emit.TypeRef{Spelling: pageName, Target: page, Args: []*emit.TypeRef{{Spelling: "string"}}},
			},
			{
				name: "returns the spelling of a translated reference's arguments in a slice",
				give: rules.TypeShape{
					Form: symbol.FormList, Spelling: "Page<string>[]",
					Elems: []rules.TypeShape{rules.Reference(pageName, page, text)},
				},
				want: &emit.TypeRef{
					Spelling: "[]Page[string]",
					Form:     symbol.FormList,
					Elems: []*emit.TypeRef{
						{Spelling: pageName, Target: page, Args: []*emit.TypeRef{{Spelling: "string"}}},
					},
				},
			},
			{
				name: "returns a translated reference for a sum",
				give: rules.TypeShape{Form: symbol.FormSum, Spelling: resultName, Ref: result},
				want: &emit.TypeRef{Spelling: resultName, Target: result},
			},
			{
				name: "returns time.Time for the well-known timestamp",
				give: rules.Reference("Date", rules.WellKnownTimestamp),
				want: &emit.TypeRef{Spelling: "time.Time", Package: "time"},
			},
			{
				name: "returns time.Duration for the well-known duration",
				give: rules.Reference("Duration", rules.WellKnownDuration),
				want: &emit.TypeRef{Spelling: "time.Duration", Package: "time"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, plugin.Policy{})
				assert.NoError(t, err, "Go spells the form")
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
					Spelling: "[string, number]",
					Elems:    []rules.TypeShape{text, number},
				},
				want: "golang: a tuple has no Go spelling",
			},
			{
				name: "returns an error for a union",
				give: union, want: "golang: a union has no Go spelling",
			},
			{
				name: "returns an error for an intersection",
				give: rules.TypeShape{
					Form:     symbol.FormIntersection,
					Spelling: "A & B",
					Elems:    []rules.TypeShape{sessionTo, sessionTo},
				},
				want: "golang: an intersection has no Go spelling",
			},
			{
				name: "returns an error for an asynchronous stream",
				give: rules.TypeShape{
					Form:     symbol.FormStream,
					Spelling: "AsyncIterable<number>",
					Async:    true,
					Elems:    []rules.TypeShape{number},
				},
				want: "golang: an asynchronous stream has no Go spelling",
			},
			{
				name: "returns an error for a borrow",
				give: rules.TypeShape{Form: symbol.FormBorrow, Spelling: "&str", Elems: []rules.TypeShape{text}},
				want: "golang: a borrow has no Go spelling",
			},
			{
				name: "returns an error for a wildcard with a lower bound",
				give: rules.TypeShape{
					Form: symbol.FormWildcard, Spelling: "? super Session", Variance: symbol.VarianceIn,
					Elems: []rules.TypeShape{sessionTo},
				},
				want: "golang: a wildcard with a lower bound has no Go spelling",
			},
			{
				name: "returns an error for an inline type",
				give: rules.TypeShape{Form: symbol.FormInline, Spelling: "{ id: string }"},
				want: "golang: an inline type has no Go spelling",
			},
			{
				name: "returns an error for an opaque type",
				give: rules.Opaque(nil),
				want: "golang: a type that the rules of its language do not classify has no Go spelling",
			},
			{
				name: "returns an error for an integer of 128 bits",
				give: rules.Scalar("i128", rules.ScalarInt, 128), want: "golang: no Go int type has 128 bits",
			},
			{
				name: "returns an error for a float of the platform's width",
				give: rules.Scalar("float", rules.ScalarFloat, 0),
				want: "golang: no Go float type has the platform's width",
			},
			{
				name: "returns an error for an array of an unstated length",
				give: rules.TypeShape{Form: symbol.FormArray, Spelling: "int[]", Elems: []rules.TypeShape{number}},
				want: "golang: an array of an unstated length has no Go spelling",
			},
			{
				name: "returns an error for an optional of two children",
				give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "?", Elems: []rules.TypeShape{text, text}},
				want: "golang: an optional takes one child, and the shape has 2",
			},
			{
				name: "returns an error for a map of one child",
				give: rules.TypeShape{Form: symbol.FormMap, Spelling: "Record<string>", Elems: []rules.TypeShape{text}},
				want: "golang: a map takes two children, a key and a value, and the shape has 1",
			},
			{
				name: "returns an error for a function type of more parameters than children",
				give: rules.TypeShape{Form: symbol.FormFunc, Spelling: "() => void", Split: 1},
				want: "golang: a function type splits its 0 children at 1",
			},
			{
				name: "returns an error about a child without a Go spelling",
				give: rules.TypeShape{
					Form: symbol.FormList, Spelling: "(string | number)[]", Elems: []rules.TypeShape{union},
				},
				want: "golang: string | number: a union has no Go spelling",
			},
			{
				name: "returns an error about an argument without a Go spelling",
				give: rules.Reference(pageName, page, union),
				want: "golang: string | number: a union has no Go spelling",
			},
			{
				name: "returns an error about the child of a function type",
				give: rules.TypeShape{
					Form: symbol.FormFunc, Spelling: "(u: string | number) => void", Split: 1,
					Elems: []rules.TypeShape{union},
				},
				want: "golang: string | number: a union has no Go spelling",
			},
			{
				name: "returns an error about the child of a map",
				give: rules.TypeShape{
					Form:     symbol.FormMap,
					Spelling: "Record<string, string | number>",
					Elems:    []rules.TypeShape{text, union},
				},
				want: "golang: string | number: a union has no Go spelling",
			},
			{
				name: "returns an error about the bound of a wildcard",
				give: rules.TypeShape{
					Form: symbol.FormWildcard, Spelling: "? extends string | number", Variance: symbol.VarianceOut,
					Elems: []rules.TypeShape{union},
				},
				want: "golang: string | number: a union has no Go spelling",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, plugin.Policy{})
				assert.HasError(t, err, "Go has no spelling of the form")
				assert.Equal(t, err.Error(), tt.want, "the error has the reason for the refusal")
				assert.Nil(t, got, "a refusal returns no reference")
			})
		}

		keys := []struct {
			name string
			give rules.TypeShape
			want string
		}{
			{
				name: "returns an error for a map key of a list",
				give: rules.TypeShape{Form: symbol.FormList, Spelling: "string[]", Elems: []rules.TypeShape{text}},
				want: "golang: string[]: Go does not compare the values of a list, so the type cannot be a map key",
			},
			{
				name: "returns an error for a map key of a map",
				give: rules.TypeShape{
					Form:     symbol.FormMap,
					Spelling: "Map<string, string>",
					Elems:    []rules.TypeShape{text, text},
				},
				want: "golang: Map<string, string>: Go does not compare the values of a map, " +
					"so the type cannot be a map key",
			},
			{
				name: "returns an error for a map key of a function type",
				give: rules.TypeShape{Form: symbol.FormFunc, Spelling: "() => void"},
				want: "golang: () => void: Go does not compare the values of a function type, " +
					"so the type cannot be a map key",
			},
			{
				name: "returns an error for a map key of bytes",
				give: rules.Leaf(symbol.FormBytes, "Uint8Array"),
				want: "golang: Uint8Array: Go does not compare the values of bytes, so the type cannot be a map key",
			},
		}
		for _, tt := range keys {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Type(rules.TypeShape{
					Form: symbol.FormMap, Spelling: "Map", Elems: []rules.TypeShape{tt.give, text},
				}, plugin.Policy{})
				assert.HasError(t, err, "Go does not compare the values of the key's form")
				assert.Equal(t, err.Error(), tt.want, "the error contains the spelling of the key")
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
		assert.Equal(t, got.Spelling, tt.want, "Type spells "+tt.name+" in Go")
	}
}

// BenchmarkType measures the spoke that every translation into Go calls.
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
				assert.Equal(b, got.Spelling, tt.want, "Type spells "+tt.name+" in Go")
			})
		}
	})
}

// typeCalls returns the spellings that the allocation test and the
// benchmark measure: a leaf, a list, a reference with an argument and a
// map.
func typeCalls() []typeCall {
	return []typeCall{
		{name: "a scalar", give: number, allocs: leafAllocs, want: "float64"},
		{
			name:   "a list of a reference",
			allocs: listAllocs,
			want:   "[]Session",
			give: rules.TypeShape{
				Form:     symbol.FormList,
				Spelling: sessionsSpelling,
				Elems:    []rules.TypeShape{sessionTo},
			},
		},
		{
			name: "a reference with an argument", give: rules.Reference(pageName, page, text), allocs: argsAllocs,
			want: pageName,
		},
		{
			name:   "a map",
			allocs: mapAllocs,
			want:   "map[string]float64",
			give: rules.TypeShape{
				Form:     symbol.FormMap,
				Spelling: "Record<string, number>",
				Elems:    []rules.TypeShape{text, number},
			},
		},
	}
}
