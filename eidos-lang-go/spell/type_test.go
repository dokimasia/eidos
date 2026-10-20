// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
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
	// streamAllocs is an asynchronous stream of a predeclared type: the
	// list of children, the element's reference, the iterator's reference,
	// its list of arguments and the reference of error.
	streamAllocs = 5
)

// The declarations of the source language that the cases reference. state
// is a type that session declares.
var (
	session = symbol.Identity{Lang: sourceLang, Package: "svc/store", Name: sessionName, Kind: symbol.KindInterface}
	page    = symbol.Identity{Lang: sourceLang, Package: "svc/store", Name: pageName, Kind: symbol.KindInterface}
	result  = symbol.Identity{Lang: sourceLang, Package: "svc/store", Name: resultName, Kind: symbol.KindSum}
	state   = symbol.Identity{
		Lang: sourceLang, Package: "svc/store", Owner: sessionName, Name: "State", Kind: symbol.KindEnum,
	}
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
	// paramShape is the shape that the fold gives a reference to the type
	// parameter paramName of page.
	paramShape = rules.TypeShape{
		Form: symbol.FormOpaque, Spelling: paramName,
		Ref: symbol.Identity{
			Lang: sourceLang, Package: "svc/store", Owner: pageName, Name: paramName, Kind: symbol.KindTypeParam,
		},
	}
)

// paramName is the name of the type parameter of the cases.
const paramName = "T"

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
				name: "returns a slice for an optional of a list",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "string[] | undefined",
					Elems: []rules.TypeShape{
						{Form: symbol.FormList, Spelling: "string[]", Elems: []rules.TypeShape{text}},
					},
				},
				want: &emit.TypeRef{
					Spelling: "[]string", Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: "string"}},
				},
			},
			{
				name: "returns a slice of byte for an optional of bytes",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "Uint8Array | undefined",
					Elems: []rules.TypeShape{rules.Leaf(symbol.FormBytes, "Uint8Array")},
				},
				want: &emit.TypeRef{
					Spelling: "[]byte", Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: "byte"}},
				},
			},
			{
				name: "returns a map for an optional of a map",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "Record<string, number> | undefined",
					Elems: []rules.TypeShape{{
						Form: symbol.FormMap, Spelling: "Record<string, number>",
						Elems: []rules.TypeShape{text, number},
					}},
				},
				want: &emit.TypeRef{
					Spelling: "map[string]float64", Form: symbol.FormMap,
					Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: "float64"}},
				},
			},
			{
				name: "returns a function type for an optional of a function type",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "(() => void) | undefined",
					Elems: []rules.TypeShape{{Form: symbol.FormFunc, Spelling: "() => void"}},
				},
				want: &emit.TypeRef{Spelling: "func()", Form: symbol.FormFunc},
			},
			{
				name: "returns a channel for an optional of a synchronous stream",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "Iterator<number> | undefined",
					Elems: []rules.TypeShape{
						{Form: symbol.FormStream, Spelling: "Iterator<number>", Elems: []rules.TypeShape{number}},
					},
				},
				want: &emit.TypeRef{
					Spelling: "chan float64", Form: symbol.FormStream, Elems: []*emit.TypeRef{{Spelling: "float64"}},
				},
			},
			{
				name: "returns any for an optional of the top type",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "unknown",
					Elems: []rules.TypeShape{rules.Leaf(symbol.FormDynamic, "unknown")},
				},
				want: &emit.TypeRef{Spelling: "any"},
			},
			{
				name: "returns error for an optional of the well-known error",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "Error | undefined",
					Elems: []rules.TypeShape{rules.Reference("Error", rules.WellKnownError)},
				},
				want: &emit.TypeRef{Spelling: "error"},
			},
			{
				name: "returns the translated reference for an optional of a sum",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "Result | undefined",
					Elems: []rules.TypeShape{{Form: symbol.FormSum, Spelling: resultName, Ref: result}},
				},
				want: &emit.TypeRef{Spelling: resultName, Target: result},
			},
			{
				name: "returns the translated reference for an optional of a reference to an interface",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "Session | undefined", Elems: []rules.TypeShape{sessionTo},
				},
				want: &emit.TypeRef{Spelling: sessionName, Target: session},
			},
			{
				name: "returns a pointer for an optional of a reference to an enum",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "Session.State | undefined",
					Elems: []rules.TypeShape{rules.Reference("Session.State", state)},
				},
				want: &emit.TypeRef{
					Spelling: "*SessionState", Form: symbol.FormOptional,
					Elems: []*emit.TypeRef{{Spelling: "SessionState", Target: state}},
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
			{name: "returns the name of a type parameter", give: paramShape, want: &emit.TypeRef{Spelling: paramName}},
			{
				name: "returns a slice of a type parameter for a list",
				give: rules.TypeShape{Form: symbol.FormList, Spelling: "T[]", Elems: []rules.TypeShape{paramShape}},
				want: &emit.TypeRef{
					Spelling: "[]" + paramName, Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: paramName}},
				},
			},
			{
				name: "returns a pointer for an optional of a type parameter",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "T | undefined", Elems: []rules.TypeShape{paramShape},
				},
				want: &emit.TypeRef{
					Spelling: "*" + paramName, Form: symbol.FormOptional, Elems: []*emit.TypeRef{{Spelling: paramName}},
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
				name: "returns an iterator over pairs of an element and an error for an asynchronous stream",
				give: rules.TypeShape{
					Form: symbol.FormStream, Spelling: "AsyncIterable<number>", Async: true,
					Elems: []rules.TypeShape{number},
				},
				want: &emit.TypeRef{
					Spelling: "iter.Seq2", Package: "iter",
					Args: []*emit.TypeRef{{Spelling: "float64"}, {Spelling: "error"}},
				},
			},
			{
				name: "returns any for the top type",
				give: rules.Leaf(symbol.FormDynamic, "unknown"),
				want: &emit.TypeRef{Spelling: "any"},
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
			{
				name: "returns struct{} for the well-known empty value",
				give: rules.Reference("google.protobuf.Empty", rules.WellKnownEmpty),
				want: &emit.TypeRef{Spelling: "struct{}", Form: symbol.FormInline},
			},
			{
				name: "returns error for the well-known error",
				give: rules.Reference("Error", rules.WellKnownError),
				want: &emit.TypeRef{Spelling: "error"},
			},
			{
				name: "returns context.Context for the well-known context",
				give: rules.Reference("context", rules.WellKnownContext),
				want: &emit.TypeRef{Spelling: "context.Context", Package: "context"},
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
				name: "returns an error for an immutable list",
				give: rules.TypeShape{
					Form: symbol.FormList, Spelling: "readonly string[]", Mutability: symbol.MutabilityImmutable,
					Elems: []rules.TypeShape{text},
				},
				want: "golang: a read-only list has no Go spelling",
			},
			{
				name: "returns an error for an immutable array",
				give: rules.TypeShape{
					Form: symbol.FormArray, Spelling: "readonly [number, number]", Length: 2,
					Mutability: symbol.MutabilityImmutable, Elems: []rules.TypeShape{number},
				},
				want: "golang: a read-only array has no Go spelling",
			},
			{
				name: "returns an error for an immutable map",
				give: rules.TypeShape{
					Form: symbol.FormMap, Spelling: "ReadonlyMap", Mutability: symbol.MutabilityImmutable,
					Elems: []rules.TypeShape{text, number},
				},
				want: "golang: a read-only map has no Go spelling",
			},
			{
				name: "returns an error for an optional of two children",
				give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "?", Elems: []rules.TypeShape{text, text}},
				want: "golang: an optional takes one child, and the shape has 2",
			},
			{
				name: "returns an error for a map of one child",
				give: rules.TypeShape{Form: symbol.FormMap, Spelling: "Record<string>", Elems: []rules.TypeShape{text}},
				want: "golang: a map takes two children, and the shape has 1",
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

		grpc, err := plugin.NewPolicy(golang.Target, golang.Policies(),
			map[plugin.PolicyKey]plugin.Choice{golang.GRPC: golang.GRPCGo})
		assert.NoError(t, err, "the binding grpc-go is a choice of the policy")
		connect, err := plugin.NewPolicy(golang.Target, golang.Policies(),
			map[plugin.PolicyKey]plugin.Choice{golang.GRPC: golang.GRPCConnect})
		assert.NoError(t, err, "the binding connect-go is a choice of the policy")
		request := symbol.Identity{Lang: "protobuf", Package: "svc", Name: "GetRequest", Kind: symbol.KindStruct}
		state := symbol.Identity{
			Lang:    "protobuf",
			Package: "svc",
			Owner:   "Session",
			Name:    "State",
			Kind:    symbol.KindEnum,
		}
		snake := symbol.Identity{Lang: "protobuf", Package: "svc", Name: "get_request", Kind: symbol.KindStruct}
		bound := []struct {
			name string
			give rules.TypeShape
			want string
		}{
			{
				name: "returns a pointer for a message",
				give: rules.Reference("GetRequest", request),
				want: "*GetRequest",
			},
			{
				name: "returns one pointer for an optional of a message",
				give: rules.TypeShape{
					Form:  symbol.FormOptional,
					Elems: []rules.TypeShape{rules.Reference("x", request)},
				},
				want: "*GetRequest",
			},
			{
				name: "joins the names of a nested enum with an underscore", give: rules.Reference("x", state),
				want: "Session_State",
			},
			{
				name: "writes a name of lower-case words in camel case", give: rules.Reference("x", snake),
				want: "*GetRequest",
			},
			{
				name: "returns the known type of a timestamp of protobuf",
				give: rules.Reference(".google.protobuf.Timestamp", rules.WellKnownTimestamp),
				want: "*timestamppb.Timestamp",
			},
			{
				name: "returns the known type of a wrapper",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "google.protobuf.StringValue",
					Elems: []rules.TypeShape{text},
				},
				want: "*wrapperspb.StringValue",
			},
			{
				name: "returns the enum of the null value",
				give: rules.Opaque(&node.TypeRef{Spelling: "google.protobuf.NullValue"}), want: "structpb.NullValue",
			},
			{
				name: "starts a name of a leading underscore with X",
				give: rules.Reference("x", symbol.Identity{Lang: "protobuf", Name: "_hidden", Kind: symbol.KindEnum}),
				want: "XHidden",
			},
			{
				name: "keeps the digits of a name",
				give: rules.Reference("x", symbol.Identity{Lang: "protobuf", Name: "v2_state", Kind: symbol.KindEnum}),
				want: "V2State",
			},
		}
		for _, tt := range bound {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, grpc)
				assert.NoError(t, err, "the binding spells the reference")
				assert.Equal(t, got.Spelling, tt.want, "the name of protoc-gen-go")
			})
		}

		const timestampPath = "google.golang.org/protobuf/types/known/timestamppb"
		references := []struct {
			name       string
			give       rules.TypeShape
			givePolicy plugin.Policy
			want       *emit.TypeRef
		}{
			{
				name: "returns a pointer to a bound reference for a message under grpc-go",
				give: rules.Reference("GetRequest", request), givePolicy: grpc,
				want: &emit.TypeRef{
					Spelling: "*GetRequest", Form: symbol.FormOptional,
					Elems: []*emit.TypeRef{{Spelling: "GetRequest", Target: request, Bound: true}},
				},
			},
			{
				name: "returns a bound reference for an enum under connect-go",
				give: rules.Reference("x", state), givePolicy: connect,
				want: &emit.TypeRef{Spelling: "Session_State", Target: state, Bound: true},
			},
			{
				name: "returns a reference with the import of a known type under a binding",
				give: rules.Reference(".google.protobuf.Timestamp", rules.WellKnownTimestamp), givePolicy: grpc,
				want: &emit.TypeRef{
					Spelling: "*timestamppb.Timestamp", Form: symbol.FormOptional,
					Elems: []*emit.TypeRef{{Spelling: "timestamppb.Timestamp", Package: timestampPath}},
				},
			},
		}
		for _, tt := range references {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, tt.givePolicy)
				assert.NoError(t, err, "the binding spells the reference")
				assert.Equal(t, got, tt.want, "the reference")
			})
		}

		t.Run("returns an error for a oneof under a binding", func(t *testing.T) {
			t.Parallel()

			oneof := symbol.Identity{
				Lang:    "protobuf",
				Package: "svc",
				Owner:   "Session",
				Name:    "auth",
				Kind:    symbol.KindSum,
			}
			_, err := spell.Type(rules.TypeShape{Form: symbol.FormSum, Ref: oneof}, grpc)
			assert.HasError(t, err, "protoc-gen-go declares the interface of a oneof unexported")
		})

		t.Run("returns the translated reference of a message without a binding", func(t *testing.T) {
			t.Parallel()

			got, err := spell.Type(rules.Reference("GetRequest", request), plugin.Policy{})
			assert.NoError(t, err, "the reference spells")
			assert.Equal(t, got.Spelling, "GetRequest", "the flat name")
		})
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
		{
			name:   "an asynchronous stream",
			allocs: streamAllocs,
			want:   "iter.Seq2",
			give: rules.TypeShape{
				Form: symbol.FormStream, Spelling: "AsyncIterable<number>", Async: true,
				Elems: []rules.TypeShape{number},
			},
		},
	}
}
