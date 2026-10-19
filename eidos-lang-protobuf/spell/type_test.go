// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// sourceLang is the language whose declarations the cases translate into
// protobuf.
const sourceLang symbol.Lang = "golang"

// The spellings of the source language's shapes.
const (
	textSpelling  = "string"
	floatSpelling = "float64"
	int64Spelling = "int64"
	sessionName   = "Session"
	pageName      = "Page"
	resultName    = "Result"
)

// The allocations of the spellings that the allocation test and the
// benchmark measure.
const (
	// leafAllocs is a scalar: the reference.
	leafAllocs = 1
	// listAllocs is a list of a translated reference: the list's
	// reference, its list of children, the child's reference and the
	// list's spelling.
	listAllocs = 4
	// optionalAllocs is an optional of text: the optional's reference, its
	// list of children, the child's reference and the optional's spelling.
	optionalAllocs = 4
	// mapAllocs is a map of text onto a float: the map's reference, its
	// list of children, the two children's references and the map's
	// spelling.
	mapAllocs = 5
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
	float     = rules.Scalar(floatSpelling, rules.ScalarFloat, 64)
	long      = rules.Scalar(int64Spelling, rules.ScalarInt, 64)
	sessionTo = rules.Reference(sessionName, session)
	channel   = rules.TypeShape{Form: symbol.FormStream, Spelling: "chan int", Elems: []rules.TypeShape{long}}
)

// typeCall is one spelling that the allocation test and the benchmark
// measure: the shape, its ceiling and the spelling that it returns.
type typeCall struct {
	name   string
	give   rules.TypeShape
	allocs uint64
	want   string
}

// TestType checks the protobuf spelling of each canonical form by the
// hub's table, under the policy that the plan resolves. It also checks
// the refusal of each form without a protobuf spelling, and the child
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
				name: "returns int32 for an integer of 8 bits under the default policy",
				give: rules.Scalar("int8", rules.ScalarInt, 8), want: &emit.TypeRef{Spelling: "int32"},
			},
			{
				name: "returns int32 for an integer of 16 bits under the default policy",
				give: rules.Scalar("int16", rules.ScalarInt, 16), want: &emit.TypeRef{Spelling: "int32"},
			},
			{
				name: "returns int32 for an integer of 32 bits under the default policy",
				give: rules.Scalar("int32", rules.ScalarInt, 32), want: &emit.TypeRef{Spelling: "int32"},
			},
			{
				name: "returns int64 for an integer of 64 bits under the default policy",
				give: long, want: &emit.TypeRef{Spelling: "int64"},
			},
			{
				name: "returns int64 for an integer of the platform's width under the default policy",
				give: rules.Scalar("int", rules.ScalarInt, 0), want: &emit.TypeRef{Spelling: "int64"},
			},
			{
				name: "returns uint32 for an unsigned integer of 8 bits under the default policy",
				give: rules.Scalar("uint8", rules.ScalarUint, 8), want: &emit.TypeRef{Spelling: "uint32"},
			},
			{
				name: "returns uint32 for an unsigned integer of 32 bits under the default policy",
				give: rules.Scalar("uint32", rules.ScalarUint, 32), want: &emit.TypeRef{Spelling: "uint32"},
			},
			{
				name: "returns uint64 for an unsigned integer of 64 bits under the default policy",
				give: rules.Scalar("uint64", rules.ScalarUint, 64), want: &emit.TypeRef{Spelling: "uint64"},
			},
			{
				name: "returns uint64 for an unsigned integer of the platform's width under the default policy",
				give: rules.Scalar("uint", rules.ScalarUint, 0), want: &emit.TypeRef{Spelling: "uint64"},
			},
			{
				name: "returns float for a float of 32 bits",
				give: rules.Scalar("float32", rules.ScalarFloat, 32), want: &emit.TypeRef{Spelling: "float"},
			},
			{
				name: "returns double for a float of 64 bits",
				give: float, want: &emit.TypeRef{Spelling: "double"},
			},
			{
				name: "returns bool for Bool",
				give: rules.Leaf(symbol.FormBool, "bool"), want: &emit.TypeRef{Spelling: "bool"},
			},
			{
				name: "returns string for Text",
				give: text, want: &emit.TypeRef{Spelling: "string"},
			},
			{
				name: "returns bytes for Bytes",
				give: rules.Leaf(symbol.FormBytes, "[]byte"), want: &emit.TypeRef{Spelling: "bytes"},
			},
			{
				name: "returns the optional form for an optional",
				give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "*string", Elems: []rules.TypeShape{text}},
				want: &emit.TypeRef{
					Spelling: "optional string", Form: symbol.FormOptional,
					Elems: []*emit.TypeRef{{Spelling: "string"}},
				},
			},
			{
				name: "returns the list form for a list",
				give: rules.TypeShape{
					Form:     symbol.FormList,
					Spelling: "[]Session",
					Elems:    []rules.TypeShape{sessionTo},
				},
				want: &emit.TypeRef{
					Spelling: "repeated Session", Form: symbol.FormList,
					Elems: []*emit.TypeRef{{Spelling: sessionName, Target: session}},
				},
			},
			{
				name: "returns a list of the list form for a list of a list",
				give: rules.TypeShape{
					Form:     symbol.FormList,
					Spelling: "[][]string",
					Elems: []rules.TypeShape{
						{Form: symbol.FormList, Spelling: "[]string", Elems: []rules.TypeShape{text}},
					},
				},
				want: &emit.TypeRef{
					Spelling: "repeated repeated string", //nolint:dupword // each list writes its own label
					Form:     symbol.FormList,
					Elems: []*emit.TypeRef{{
						Spelling: "repeated string", Form: symbol.FormList,
						Elems: []*emit.TypeRef{{Spelling: "string"}},
					}},
				},
			},
			{
				name: "returns the map form for a map",
				give: rules.TypeShape{
					Form:     symbol.FormMap,
					Spelling: "map[string]float64",
					Elems:    []rules.TypeShape{text, float},
				},
				want: &emit.TypeRef{
					Spelling: "map<string, double>", Form: symbol.FormMap,
					Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: "double"}},
				},
			},
			{
				name: "returns the stream form for an asynchronous stream",
				give: rules.TypeShape{
					Form: symbol.FormStream, Spelling: "Stream<Session>", Async: true,
					Elems: []rules.TypeShape{sessionTo},
				},
				want: &emit.TypeRef{
					Spelling: "stream Session", Form: symbol.FormStream, Async: true,
					Elems: []*emit.TypeRef{{Spelling: sessionName, Target: session}},
				},
			},
			{
				name: "returns Value with the file that declares it for the top type",
				give: rules.Leaf(symbol.FormDynamic, "any"),
				want: &emit.TypeRef{Spelling: "google.protobuf.Value", Package: "google/protobuf/struct.proto"},
			},
			{
				name: "returns Timestamp with the file that declares it for the well-known timestamp",
				give: rules.Reference("time.Time", rules.WellKnownTimestamp),
				want: &emit.TypeRef{Spelling: "google.protobuf.Timestamp", Package: "google/protobuf/timestamp.proto"},
			},
			{
				name: "returns Duration with the file that declares it for the well-known duration",
				give: rules.Reference("time.Duration", rules.WellKnownDuration),
				want: &emit.TypeRef{Spelling: "google.protobuf.Duration", Package: "google/protobuf/duration.proto"},
			},
			{
				name: "returns Empty with the file that declares it for the well-known empty value",
				give: rules.Reference("struct{}", rules.WellKnownEmpty),
				want: &emit.TypeRef{Spelling: "google.protobuf.Empty", Package: "google/protobuf/empty.proto"},
			},
			{
				name: "returns a translated reference for a reference",
				give: sessionTo,
				want: &emit.TypeRef{Spelling: sessionName, Target: session},
			},
			{
				name: "returns a translated reference for a sum",
				give: rules.TypeShape{Form: symbol.FormSum, Spelling: resultName, Ref: result},
				want: &emit.TypeRef{Spelling: resultName, Target: result},
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

				got, err := spell.Type(tt.give, policy(t, nil))
				assert.NoError(t, err, "protobuf spells the form")
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
				name:   "returns sint32 for an integer of 16 bits under the sint32 choice",
				chosen: map[plugin.PolicyKey]plugin.Choice{protobuf.Int32Encoding: protobuf.Sint32},
				give:   rules.Scalar("int16", rules.ScalarInt, 16), want: "sint32",
			},
			{
				name:   "returns sfixed32 for an integer of 32 bits under the sfixed32 choice",
				chosen: map[plugin.PolicyKey]plugin.Choice{protobuf.Int32Encoding: protobuf.Sfixed32},
				give:   rules.Scalar("int32", rules.ScalarInt, 32), want: "sfixed32",
			},
			{
				name:   "returns sint64 for an integer of 64 bits under the sint64 choice",
				chosen: map[plugin.PolicyKey]plugin.Choice{protobuf.Int64Encoding: protobuf.Sint64},
				give:   long, want: "sint64",
			},
			{
				name:   "returns sfixed64 for an integer of the platform's width under the sfixed64 choice",
				chosen: map[plugin.PolicyKey]plugin.Choice{protobuf.Int64Encoding: protobuf.Sfixed64},
				give:   rules.Scalar("int", rules.ScalarInt, 0), want: "sfixed64",
			},
			{
				name:   "returns fixed32 for an unsigned integer of 32 bits under the fixed32 choice",
				chosen: map[plugin.PolicyKey]plugin.Choice{protobuf.Uint32Encoding: protobuf.Fixed32},
				give:   rules.Scalar("uint32", rules.ScalarUint, 32), want: "fixed32",
			},
			{
				name:   "returns fixed64 for an unsigned integer of 64 bits under the fixed64 choice",
				chosen: map[plugin.PolicyKey]plugin.Choice{protobuf.Uint64Encoding: protobuf.Fixed64},
				give:   rules.Scalar("uint64", rules.ScalarUint, 64), want: "fixed64",
			},
			{
				name:   "returns int64 for an integer of 64 bits under a choice of the 32-bit policy",
				chosen: map[plugin.PolicyKey]plugin.Choice{protobuf.Int32Encoding: protobuf.Sint32},
				give:   long, want: "int64",
			},
		}
		for _, tt := range choices {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, policy(t, tt.chosen))
				assert.NoError(t, err, "protobuf spells the integer")
				assert.Equal(t, got.Spelling, tt.want, "the spelling of the choice")
			})
		}

		t.Run("returns a map of a key under its choice", func(t *testing.T) {
			t.Parallel()

			got, err := spell.Type(rules.TypeShape{
				Form: symbol.FormMap, Spelling: "map[int64]string", Elems: []rules.TypeShape{long, text},
			}, policy(t, map[plugin.PolicyKey]plugin.Choice{protobuf.Int64Encoding: protobuf.Sint64}))
			assert.NoError(t, err, "protobuf spells the map")
			assert.Equal(t, got.Spelling, "map<sint64, string>", "the key takes the encoding of its policy")
		})

		refusals := []struct {
			name string
			give rules.TypeShape
			want string
		}{
			{
				name: "returns an error for an integer of 128 bits",
				give: rules.Scalar("i128", rules.ScalarInt, 128),
				want: "protobuf: no protobuf int type has 128 bits",
			},
			{
				name: "returns an error for an unsigned integer of 128 bits",
				give: rules.Scalar("u128", rules.ScalarUint, 128),
				want: "protobuf: no protobuf uint type has 128 bits",
			},
			{
				name: "returns an error for a float of 16 bits",
				give: rules.Scalar("f16", rules.ScalarFloat, 16),
				want: "protobuf: no protobuf float type has 16 bits",
			},
			{
				name: "returns an error for an optional of an optional",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "Option<Option<String>>",
					Elems: []rules.TypeShape{{
						Form: symbol.FormOptional, Spelling: "Option<String>", Elems: []rules.TypeShape{text},
					}},
				},
				want: "protobuf: an optional of an optional has no protobuf spelling",
			},
			{
				name: "returns an error for a read-only list",
				give: rules.TypeShape{
					Form: symbol.FormList, Spelling: "readonly string[]", Mutability: symbol.MutabilityImmutable,
					Elems: []rules.TypeShape{text},
				},
				want: "protobuf: a read-only list has no protobuf spelling",
			},
			{
				name: "returns an error for a read-only map",
				give: rules.TypeShape{
					Form:       symbol.FormMap,
					Spelling:   "ReadonlyMap<string, number>",
					Mutability: symbol.MutabilityImmutable,
					Elems:      []rules.TypeShape{text, float},
				},
				want: "protobuf: a read-only map has no protobuf spelling",
			},
			{
				name: "returns an error for an array",
				give: rules.TypeShape{
					Form: symbol.FormArray, Spelling: "[2]string", Length: 2, Elems: []rules.TypeShape{text},
				},
				want: "protobuf: an array has no protobuf spelling",
			},
			{
				name: "returns an error for a tuple",
				give: rules.TypeShape{
					Form:     symbol.FormTuple,
					Spelling: "(String, f64)",
					Elems:    []rules.TypeShape{text, float},
				},
				want: "protobuf: a tuple has no protobuf spelling",
			},
			{
				name: "returns an error for a union",
				give: rules.TypeShape{
					Form:     symbol.FormUnion,
					Spelling: "string | number",
					Elems:    []rules.TypeShape{text, float},
				},
				want: "protobuf: a union has no protobuf spelling",
			},
			{
				name: "returns an error for an intersection",
				give: rules.TypeShape{
					Form: symbol.FormIntersection, Spelling: "Session & Page",
					Elems: []rules.TypeShape{sessionTo, rules.Reference(pageName, page)},
				},
				want: "protobuf: an intersection has no protobuf spelling",
			},
			{
				name: "returns an error for a function type",
				give: rules.TypeShape{
					Form:     symbol.FormFunc,
					Spelling: "func(string)",
					Split:    1,
					Elems:    []rules.TypeShape{text},
				},
				want: "protobuf: a function type has no protobuf spelling",
			},
			{
				name: "returns an error for a synchronous stream",
				give: channel,
				want: "protobuf: a synchronous stream has no protobuf spelling",
			},
			{
				name: "returns an error for a borrow",
				give: rules.TypeShape{Form: symbol.FormBorrow, Spelling: "&str", Elems: []rules.TypeShape{text}},
				want: "protobuf: a borrow has no protobuf spelling",
			},
			{
				name: "returns an error for a wildcard",
				give: rules.TypeShape{
					Form: symbol.FormWildcard, Spelling: "? extends Session", Variance: symbol.VarianceOut,
					Elems: []rules.TypeShape{sessionTo},
				},
				want: "protobuf: a wildcard has no protobuf spelling",
			},
			{
				name: "returns an error for an inline type",
				give: rules.TypeShape{Form: symbol.FormInline, Spelling: "struct{}"},
				want: "protobuf: an inline type has no protobuf spelling",
			},
			{
				name: "returns an error for an opaque type",
				give: rules.Opaque(nil),
				want: "protobuf: a type that the rules of its language do not classify has no protobuf spelling",
			},
			{
				name: "returns an error for the well-known error",
				give: rules.Reference("error", rules.WellKnownError),
				want: "protobuf: the well-known error has no protobuf spelling",
			},
			{
				name: "returns an error for a reference with type arguments",
				give: rules.Reference("Page[int64]", page, long),
				want: "protobuf: a reference with type arguments has no protobuf spelling",
			},
			{
				name: "returns an error for a list of two children",
				give: rules.TypeShape{Form: symbol.FormList, Spelling: "[]X", Elems: []rules.TypeShape{text, text}},
				want: "protobuf: a list takes one child, and the shape has 2",
			},
			{
				name: "returns an error for an optional without a child",
				give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "?"},
				want: "protobuf: an optional takes one child, and the shape has 0",
			},
			{
				name: "returns an error for a map of one child",
				give: rules.TypeShape{Form: symbol.FormMap, Spelling: "map[string]", Elems: []rules.TypeShape{text}},
				want: "protobuf: a map takes two children, and the shape has 1",
			},
			{
				name: "returns an error for an asynchronous stream of two children",
				give: rules.TypeShape{
					Form: symbol.FormStream, Spelling: "S", Async: true, Elems: []rules.TypeShape{text, text},
				},
				want: "protobuf: an asynchronous stream takes one child, and the shape has 2",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, policy(t, nil))
				assert.HasError(t, err, "protobuf has no spelling of the form")
				assert.Equal(t, err.Error(), tt.want, "the error has the reason for the refusal")
				assert.Nil(t, got, "a refusal returns no reference")
			})
		}

		children := []struct {
			name string
			give func(child rules.TypeShape) rules.TypeShape
		}{
			{
				name: "returns an error about the child of an optional",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormOptional, Elems: []rules.TypeShape{c}}
				},
			},
			{
				name: "returns an error about the child of a list",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormList, Elems: []rules.TypeShape{c}}
				},
			},
			{
				name: "returns an error about the key of a map",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormMap, Elems: []rules.TypeShape{c, text}}
				},
			},
			{
				name: "returns an error about the value of a map",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormMap, Elems: []rules.TypeShape{text, c}}
				},
			},
			{
				name: "returns an error about the child of an asynchronous stream",
				give: func(c rules.TypeShape) rules.TypeShape {
					return rules.TypeShape{Form: symbol.FormStream, Async: true, Elems: []rules.TypeShape{c}}
				},
			},
		}
		for _, tt := range children {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Type(tt.give(channel), policy(t, nil))
				assert.HasError(t, err, "protobuf has no spelling of the child")
				assert.Equal(t, err.Error(), "protobuf: chan int: a synchronous stream has no protobuf spelling",
					"the error starts with the spelling of the child")
			})
		}
	})
}

// Type allocates each reference that it returns, its list of children
// and the spelling of a structural reference. The ordinary run, which
// runs no benchmark, checks those ceilings here.
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
		assert.Equal(t, got.Spelling, tt.want, "Type spells "+tt.name+" in protobuf")
	}
}

// BenchmarkType measures the spoke that every translation into protobuf
// calls.
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
				assert.Equal(b, got.Spelling, tt.want, "Type spells "+tt.name+" in protobuf")
			})
		}
	})
}

// policy returns the protobuf target's policy with the choices that
// chosen selects.
func policy(tb testing.TB, chosen map[plugin.PolicyKey]plugin.Choice) plugin.Policy {
	tb.Helper()

	p, err := plugin.NewPolicy(protobuf.Target, protobuf.Policies(), chosen)
	assert.NoError(tb, err, "the protobuf policies resolve")
	return p
}

// typeCalls returns the spellings that the allocation test and the
// benchmark measure: a scalar under the policy, a well-known type, a list,
// an optional and a map.
func typeCalls() []typeCall {
	return []typeCall{
		{name: "an integer of 64 bits", give: long, allocs: leafAllocs, want: "int64"},
		{
			name: "the well-known timestamp", allocs: leafAllocs, want: "google.protobuf.Timestamp",
			give: rules.Reference("time.Time", rules.WellKnownTimestamp),
		},
		{
			name: "a list of a reference", allocs: listAllocs, want: "repeated Session",
			give: rules.TypeShape{Form: symbol.FormList, Spelling: "[]Session", Elems: []rules.TypeShape{sessionTo}},
		},
		{
			name: "an optional", allocs: optionalAllocs, want: "optional string",
			give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "*string", Elems: []rules.TypeShape{text}},
		},
		{
			name:   "a map",
			allocs: mapAllocs,
			want:   "map<string, double>",
			give: rules.TypeShape{
				Form:     symbol.FormMap,
				Spelling: "map[string]float64",
				Elems:    []rules.TypeShape{text, float},
			},
		},
	}
}
