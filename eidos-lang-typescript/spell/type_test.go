// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// sourceLang is the language whose declarations the cases translate into
// TypeScript.
const sourceLang symbol.Lang = "golang"

// The spellings of the source language's shapes.
const (
	textSpelling  = "string"
	floatSpelling = "float64"
	int64Spelling = "int64"
	sessionName   = "Session"
	pageName      = "Page"
	statusName    = "Status"
	resultName    = "Result"
)

// The allocations of the spellings that the allocation test and the
// benchmark measure.
const (
	// leafAllocs is a builtin type: the reference.
	leafAllocs = 1
	// listAllocs is a list of a translated reference: the list's
	// reference, its list of children, the child's reference and the
	// list's spelling.
	listAllocs = 4
	// optionalAllocs is an optional of text: the union's reference, its
	// list of children, the two children's references and the union's
	// spelling.
	optionalAllocs = 5
	// recordAllocs is a map of text onto a float: the record's
	// reference, its list of arguments and the two arguments' references.
	recordAllocs = 4
)

// The declarations of the source language that the cases reference.
var (
	session = symbol.Identity{Lang: sourceLang, Package: "svc", Name: sessionName, Kind: symbol.KindStruct}
	page    = symbol.Identity{Lang: sourceLang, Package: "svc", Name: pageName, Kind: symbol.KindStruct}
	status  = symbol.Identity{Lang: sourceLang, Package: "svc", Name: statusName, Kind: symbol.KindEnum}
	result  = symbol.Identity{Lang: sourceLang, Package: "svc", Name: resultName, Kind: symbol.KindSum}
)

// The shapes the cases compose.
var (
	text      = rules.Leaf(symbol.FormText, textSpelling)
	float     = rules.Scalar(floatSpelling, rules.ScalarFloat, 64)
	long      = rules.Scalar(int64Spelling, rules.ScalarInt, 64)
	sessionTo = rules.Reference(sessionName, session)
	pageTo    = rules.Reference(pageName, page)
	union     = rules.TypeShape{
		Form:     symbol.FormUnion,
		Spelling: "string | float64",
		Elems:    []rules.TypeShape{text, float},
	}
	callback = rules.TypeShape{
		Form: symbol.FormFunc, Spelling: "func(string)", Split: 1, Elems: []rules.TypeShape{text},
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

// TestType checks the TypeScript spelling of each canonical form by the
// hub's table, under the policy that the plan resolves. It also checks
// the refusal of each form without a TypeScript spelling, and the child
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
				name: "returns number for an integer of 8 bits",
				give: rules.Scalar("int8", rules.ScalarInt, 8), want: &emit.TypeRef{Spelling: "number"},
			},
			{
				name: "returns number for an integer of 32 bits",
				give: rules.Scalar("int32", rules.ScalarInt, 32), want: &emit.TypeRef{Spelling: "number"},
			},
			{
				name: "returns number for an unsigned integer of 16 bits",
				give: rules.Scalar("uint16", rules.ScalarUint, 16), want: &emit.TypeRef{Spelling: "number"},
			},
			{
				name: "returns bigint for an integer of 64 bits under the default policy",
				give: long, want: &emit.TypeRef{Spelling: "bigint"},
			},
			{
				name: "returns bigint for an unsigned integer of 64 bits under the default policy",
				give: rules.Scalar("uint64", rules.ScalarUint, 64), want: &emit.TypeRef{Spelling: "bigint"},
			},
			{
				name: "returns bigint for an integer of the platform's width under the default policy",
				give: rules.Scalar("int", rules.ScalarInt, 0), want: &emit.TypeRef{Spelling: "bigint"},
			},
			{
				name: "returns number for a float of 32 bits",
				give: rules.Scalar("float32", rules.ScalarFloat, 32), want: &emit.TypeRef{Spelling: "number"},
			},
			{
				name: "returns number for a float of 64 bits",
				give: float, want: &emit.TypeRef{Spelling: "number"},
			},
			{
				name: "returns boolean for Bool",
				give: rules.Leaf(symbol.FormBool, "bool"), want: &emit.TypeRef{Spelling: "boolean"},
			},
			{
				name: "returns string for Text",
				give: text, want: &emit.TypeRef{Spelling: "string"},
			},
			{
				name: "returns Uint8Array for Bytes under the default policy",
				give: rules.Leaf(symbol.FormBytes, "[]byte"), want: &emit.TypeRef{Spelling: "Uint8Array"},
			},
			{
				name: "returns a union with undefined for an optional under the default policy",
				give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "*string", Elems: []rules.TypeShape{text}},
				want: &emit.TypeRef{
					Spelling: "string | undefined", Form: symbol.FormUnion,
					Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: "undefined"}},
				},
			},
			{
				name: "returns a list of a translated reference for a list",
				give: rules.TypeShape{
					Form:     symbol.FormList,
					Spelling: "[]Session",
					Elems:    []rules.TypeShape{sessionTo},
				},
				want: &emit.TypeRef{
					Spelling: "Session[]", Form: symbol.FormList,
					Elems: []*emit.TypeRef{{Spelling: sessionName, Target: session}},
				},
			},
			{
				name: "returns a list of a union in parentheses",
				give: rules.TypeShape{Form: symbol.FormList, Spelling: "[]U", Elems: []rules.TypeShape{union}},
				want: &emit.TypeRef{
					Spelling: "(string | number)[]", Form: symbol.FormList,
					Elems: []*emit.TypeRef{{
						Spelling: "string | number", Form: symbol.FormUnion,
						Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: "number"}},
					}},
				},
			},
			{
				name: "returns a tuple of the length of an array",
				give: rules.TypeShape{
					Form: symbol.FormArray, Spelling: "[2]bool", Length: 2,
					Elems: []rules.TypeShape{rules.Leaf(symbol.FormBool, "bool")},
				},
				want: &emit.TypeRef{
					Spelling: "[boolean, boolean]", Form: symbol.FormTuple,
					Elems: []*emit.TypeRef{{Spelling: "boolean"}, {Spelling: "boolean"}},
				},
			},
			{
				name: "returns a record for a map of a string key",
				give: rules.TypeShape{
					Form:     symbol.FormMap,
					Spelling: "map[string]float64",
					Elems:    []rules.TypeShape{text, float},
				},
				want: &emit.TypeRef{
					Spelling: "Record",
					Args:     []*emit.TypeRef{{Spelling: "string"}, {Spelling: "number"}},
				},
			},
			{
				name: "returns a map for a map of a bigint key",
				give: rules.TypeShape{
					Form:     symbol.FormMap,
					Spelling: "map[int64]string",
					Elems:    []rules.TypeShape{long, text},
				},
				want: &emit.TypeRef{Spelling: "Map", Args: []*emit.TypeRef{{Spelling: "bigint"}, {Spelling: "string"}}},
			},
			{
				name: "returns a partial record for a map of an enum key",
				give: rules.TypeShape{
					Form: symbol.FormMap, Spelling: "map[Status]string",
					Elems: []rules.TypeShape{rules.Reference(statusName, status), text},
				},
				want: &emit.TypeRef{Spelling: "Partial", Args: []*emit.TypeRef{
					{
						Spelling: "Record",
						Args:     []*emit.TypeRef{{Spelling: statusName, Target: status}, {Spelling: "string"}},
					},
				}},
			},
			{
				name: "returns an arrow type of one result",
				give: rules.TypeShape{
					Form:     symbol.FormFunc,
					Spelling: "func(string) float64",
					Split:    1,
					Elems:    []rules.TypeShape{text, float},
				},
				want: &emit.TypeRef{
					Spelling: "(p0: string) => number", Form: symbol.FormFunc, Split: 1,
					Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: "number"}},
				},
			},
			{
				name: "returns an arrow type that returns void without a result",
				give: callback,
				want: &emit.TypeRef{
					Spelling: "(p0: string) => void", Form: symbol.FormFunc, Split: 1,
					Elems: []*emit.TypeRef{{Spelling: "string"}},
				},
			},
			{
				name: "returns an arrow type that returns a tuple of two results",
				give: rules.TypeShape{
					Form: symbol.FormFunc, Spelling: "func() (float64, string)", Split: 0,
					Elems: []rules.TypeShape{float, text},
				},
				want: &emit.TypeRef{
					Spelling: "() => [number, string]", Form: symbol.FormFunc,
					Elems: []*emit.TypeRef{{Spelling: "number"}, {Spelling: "string"}},
				},
			},
			{
				name: "returns a tuple for a tuple",
				give: rules.TypeShape{
					Form:     symbol.FormTuple,
					Spelling: "(String, f64)",
					Elems:    []rules.TypeShape{text, float},
				},
				want: &emit.TypeRef{
					Spelling: "[string, number]", Form: symbol.FormTuple,
					Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: "number"}},
				},
			},
			{
				name: "returns a union for a union",
				give: union,
				want: &emit.TypeRef{
					Spelling: "string | number", Form: symbol.FormUnion,
					Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: "number"}},
				},
			},
			{
				name: "returns a union whose function type is in parentheses",
				give: rules.TypeShape{
					Form:     symbol.FormUnion,
					Spelling: "F | string",
					Elems:    []rules.TypeShape{callback, text},
				},
				want: &emit.TypeRef{
					Spelling: "((p0: string) => void) | string", Form: symbol.FormUnion,
					Elems: []*emit.TypeRef{
						{
							Spelling: "(p0: string) => void",
							Form:     symbol.FormFunc,
							Split:    1,
							Elems:    []*emit.TypeRef{{Spelling: "string"}},
						},
						{Spelling: "string"},
					},
				},
			},
			{
				name: "returns an intersection for an intersection",
				give: rules.TypeShape{
					Form:     symbol.FormIntersection,
					Spelling: "Session & Page",
					Elems:    []rules.TypeShape{sessionTo, pageTo},
				},
				want: &emit.TypeRef{
					Spelling: "Session & Page",
					Form:     symbol.FormIntersection,
					Elems: []*emit.TypeRef{
						{Spelling: sessionName, Target: session},
						{Spelling: pageName, Target: page},
					},
				},
			},
			{
				name: "returns AsyncIterable for an asynchronous stream",
				give: rules.TypeShape{
					Form: symbol.FormStream, Spelling: "Stream<f64>", Async: true, Elems: []rules.TypeShape{float},
				},
				want: &emit.TypeRef{Spelling: "AsyncIterable", Args: []*emit.TypeRef{{Spelling: "number"}}},
			},
			{
				name: "returns the type of a borrow",
				give: rules.TypeShape{Form: symbol.FormBorrow, Spelling: "&str", Elems: []rules.TypeShape{text}},
				want: &emit.TypeRef{Spelling: "string"},
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
				name: "returns unknown for a wildcard without a bound",
				give: rules.TypeShape{Form: symbol.FormWildcard, Spelling: "?"},
				want: &emit.TypeRef{Spelling: "unknown"},
			},
			{
				name: "returns a translated reference with its translated arguments",
				give: rules.Reference(pageName, page, long),
				want: &emit.TypeRef{Spelling: pageName, Target: page, Args: []*emit.TypeRef{{Spelling: "bigint"}}},
			},
			{
				name: "returns a translated reference for a sum",
				give: rules.TypeShape{Form: symbol.FormSum, Spelling: resultName, Ref: result},
				want: &emit.TypeRef{Spelling: resultName, Target: result},
			},
			{
				name: "returns Date for the well-known timestamp under the default policy",
				give: rules.Reference("time.Time", rules.WellKnownTimestamp), want: &emit.TypeRef{Spelling: "Date"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, policy(t, nil))
				assert.NoError(t, err, "TypeScript spells the form")
				assert.Equal(t, got, tt.want, "the reference")
			})
		}

		choices := []struct {
			name   string
			chosen map[plugin.PolicyKey]plugin.Choice
			give   rules.TypeShape
			want   string
		}{
			{
				name: "returns a string for an integer of 64 bits under the string choice",
				chosen: map[plugin.PolicyKey]plugin.Choice{
					typescript.Int64: typescript.String,
				},
				give: long,
				want: "string",
			},
			{
				name: "returns number for an integer of 64 bits under the number choice",
				chosen: map[plugin.PolicyKey]plugin.Choice{
					typescript.Int64: typescript.Number,
				},
				give: long,
				want: "number",
			},
			{
				name:   "returns a union with null for an optional under the null choice",
				chosen: map[plugin.PolicyKey]plugin.Choice{typescript.Absent: typescript.Null},
				give:   rules.TypeShape{Form: symbol.FormOptional, Spelling: "*string", Elems: []rules.TypeShape{text}},
				want:   "string | null",
			},
			{
				name:   "returns a string for the well-known timestamp under the string choice",
				chosen: map[plugin.PolicyKey]plugin.Choice{typescript.Timestamp: typescript.String},
				give:   rules.Reference("time.Time", rules.WellKnownTimestamp), want: "string",
			},
			{
				name:   "returns a string for bytes under the string choice",
				chosen: map[plugin.PolicyKey]plugin.Choice{typescript.Bytes: typescript.String},
				give:   rules.Leaf(symbol.FormBytes, "[]byte"), want: "string",
			},
		}
		for _, tt := range choices {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, policy(t, tt.chosen))
				assert.NoError(t, err, "TypeScript spells the form")
				assert.Equal(t, got.Spelling, tt.want, "the spelling of the choice")
			})
		}

		t.Run("returns a record for a map of a 64-bit integer key under the number choice", func(t *testing.T) {
			t.Parallel()

			got, err := spell.Type(rules.TypeShape{
				Form: symbol.FormMap, Spelling: "map[int64]string", Elems: []rules.TypeShape{long, text},
			}, policy(t, map[plugin.PolicyKey]plugin.Choice{typescript.Int64: typescript.Number}))
			assert.NoError(t, err, "TypeScript spells the map")
			assert.Equal(t, got.Spelling, "Record", "a number key indexes a record")
		})

		refusals := []struct {
			name string
			give rules.TypeShape
			want string
		}{
			{
				name: "returns an error for a synchronous stream",
				give: rules.TypeShape{Form: symbol.FormStream, Spelling: "chan int", Elems: []rules.TypeShape{long}},
				want: "typescript: a synchronous stream has no TypeScript spelling",
			},
			{
				name: "returns an error for the well-known duration",
				give: rules.Reference("time.Duration", rules.WellKnownDuration),
				want: "typescript: the well-known duration has no TypeScript spelling",
			},
			{
				name: "returns an error for a wildcard with a lower bound",
				give: rules.TypeShape{
					Form: symbol.FormWildcard, Spelling: "? super Session", Variance: symbol.VarianceIn,
					Elems: []rules.TypeShape{sessionTo},
				},
				want: "typescript: a wildcard with a lower bound has no TypeScript spelling",
			},
			{
				name: "returns an error for an inline type",
				give: rules.TypeShape{Form: symbol.FormInline, Spelling: "struct{}"},
				want: "typescript: an inline type has no TypeScript spelling",
			},
			{
				name: "returns an error for an opaque type",
				give: rules.Opaque(nil),
				want: "typescript: a type that the rules of its language do not classify has no TypeScript spelling",
			},
			{
				name: "returns an error for an integer of 128 bits",
				give: rules.Scalar("i128", rules.ScalarInt, 128),
				want: "typescript: no TypeScript int type has 128 bits",
			},
			{
				name: "returns an error for an array of an unstated length",
				give: rules.TypeShape{Form: symbol.FormArray, Spelling: "[]string", Elems: []rules.TypeShape{text}},
				want: "typescript: an array of an unstated length has no TypeScript spelling",
			},
			{
				name: "returns an error for an array of two children",
				give: rules.TypeShape{
					Form:     symbol.FormArray,
					Spelling: "[2]X",
					Length:   2,
					Elems:    []rules.TypeShape{text, text},
				},
				want: "typescript: an array takes one child, and the shape has 2",
			},
			{
				name: "returns an error for an asynchronous stream of two children",
				give: rules.TypeShape{
					Form:     symbol.FormStream,
					Spelling: "S",
					Async:    true,
					Elems:    []rules.TypeShape{text, text},
				},
				want: "typescript: an asynchronous stream takes one child, and the shape has 2",
			},
			{
				name: "returns an error for a list of two children",
				give: rules.TypeShape{Form: symbol.FormList, Spelling: "[]X", Elems: []rules.TypeShape{text, text}},
				want: "typescript: a list takes one child, and the shape has 2",
			},
			{
				name: "returns an error for a map of one child",
				give: rules.TypeShape{Form: symbol.FormMap, Spelling: "map[string]", Elems: []rules.TypeShape{text}},
				want: "typescript: a map takes two children, a key and a value, and the shape has 1",
			},
			{
				name: "returns an error for a function type of more parameters than children",
				give: rules.TypeShape{Form: symbol.FormFunc, Spelling: "func()", Split: 1},
				want: "typescript: a function type splits its 0 children at 1",
			},
			{
				name: "returns an error about a child without a TypeScript spelling",
				give: rules.TypeShape{
					Form:     symbol.FormList,
					Spelling: "[]chan int",
					Elems: []rules.TypeShape{
						{Form: symbol.FormStream, Spelling: "chan int", Elems: []rules.TypeShape{long}},
					},
				},
				want: "typescript: chan int: a synchronous stream has no TypeScript spelling",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, policy(t, nil))
				assert.HasError(t, err, "TypeScript has no spelling of the form")
				assert.Equal(t, err.Error(), tt.want, "the error has the reason for the refusal")
				assert.Nil(t, got, "a refusal returns no reference")
			})
		}

		children := []struct {
			name string
			give func(child rules.TypeShape) rules.TypeShape
		}{
			{
				name: "returns an error about the child of an asynchronous stream",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormStream, Async: true, Elems: []rules.TypeShape{c}}
				},
			},
			{
				name: "returns an error about the child of an array",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormArray, Length: 2, Elems: []rules.TypeShape{c}}
				},
			},
			{
				name: "returns an error about a member of a union",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormUnion, Elems: []rules.TypeShape{text, c}}
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

				inline := rules.TypeShape{Form: symbol.FormInline, Spelling: "struct{}"}
				_, err := spell.Type(tt.give(inline), policy(t, nil))
				assert.HasError(t, err, "TypeScript has no spelling of the child")
				assert.Equal(t, err.Error(), "typescript: struct{}: an inline type has no TypeScript spelling",
					"the error starts with the spelling of the child")
			})
		}
	})
}

// Type allocates each reference that it returns, its list of children
// or arguments, and the spelling of a structural reference. The ordinary
// run, which runs no benchmark, checks those ceilings here.
func TestTypeAllocs(t *testing.T) {
	p := policy(t, nil)
	for _, tt := range typeCalls() {
		var (
			got *emit.TypeRef
			err error
		)
		assert.MaxAllocs(t, func() { got, err = spell.Type(tt.give, p) }, tt.allocs,
			"Type of "+tt.name+" allocates within its ceiling")
		assert.NoError(t, err, "Type spells "+tt.name)
		assert.Equal(t, got.Spelling, tt.want, "Type spells "+tt.name+" in TypeScript")
	}
}

// BenchmarkType measures the spoke that every translation into
// TypeScript calls.
func BenchmarkType(b *testing.B) {
	p := policy(b, nil)
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
					got, err = spell.Type(tt.give, p)
				}
				assert.NoError(b, err, "Type spells "+tt.name)
				assert.Equal(b, got.Spelling, tt.want, "Type spells "+tt.name+" in TypeScript")
			})
		}
	})
}

// policy returns the TypeScript target's policy with the choices that
// chosen selects.
func policy(tb testing.TB, chosen map[plugin.PolicyKey]plugin.Choice) plugin.Policy {
	tb.Helper()

	p, err := plugin.NewPolicy(typescript.Target, typescript.Policies(), chosen)
	assert.NoError(tb, err, "the TypeScript policies resolve")
	return p
}

// typeCalls returns the spellings that the allocation test and the
// benchmark measure: a scalar under the policy, a list, an optional and
// a record.
func typeCalls() []typeCall {
	return []typeCall{
		{name: "an integer of 64 bits", give: long, allocs: leafAllocs, want: "bigint"},
		{
			name: "a list of a reference", allocs: listAllocs, want: "Session[]",
			give: rules.TypeShape{Form: symbol.FormList, Spelling: "[]Session", Elems: []rules.TypeShape{sessionTo}},
		},
		{
			name: "an optional", allocs: optionalAllocs, want: "string | undefined",
			give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "*string", Elems: []rules.TypeShape{text}},
		},
		{
			name:   "a map",
			allocs: recordAllocs,
			want:   "Record",
			give: rules.TypeShape{
				Form:     symbol.FormMap,
				Spelling: "map[string]float64",
				Elems:    []rules.TypeShape{text, float},
			},
		},
	}
}
