// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// sourceLang is the language whose declarations the cases translate into
// GraphQL.
const sourceLang symbol.Lang = "golang"

// The spellings of the shapes of the source language, and the names of its
// declarations.
const (
	textSpelling  = "string"
	floatSpelling = "float64"
	int64Spelling = "int64"
	sessionName   = "Session"
	pageName      = "Page"
	resultName    = "Result"
)

// The GraphQL spellings that the cases expect.
const (
	intType     = "Int"
	floatType   = "Float"
	stringType  = "String"
	booleanType = "Boolean"
)

// The allocations of the spellings that the allocation test and the
// benchmark measure.
const (
	// leafAllocs is a scalar: the reference.
	leafAllocs = 1
	// listAllocs is a list of a translated reference: the reference of the
	// list, its list of children, the reference of the child and the
	// spelling of the list.
	listAllocs = 4
	// optionalAllocs is an optional of text: the reference of the optional,
	// its list of children and the reference of the child. The spelling of
	// the optional is the spelling of the child.
	optionalAllocs = 3
	// mapAllocs is a map of text onto a float under the choice of entries:
	// the reference of the map, its list of children and the references of
	// the two children.
	mapAllocs = 4
	// streamAllocs is an asynchronous stream of a translated reference: the
	// reference of the stream, its list of children, the reference of the
	// child and the spelling of the stream.
	streamAllocs = 4
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

// The shapes that the cases compose.
var (
	text      = rules.Leaf(symbol.FormText, textSpelling)
	float     = rules.Scalar(floatSpelling, rules.ScalarFloat, 64)
	long      = rules.Scalar(int64Spelling, rules.ScalarInt, 64)
	sessionTo = rules.Reference(sessionName, session)
	channel   = rules.TypeShape{Form: symbol.FormStream, Spelling: "chan int", Elems: []rules.TypeShape{long}}
	// paramShape is the shape that the fold gives a reference to the type
	// parameter paramName of page.
	paramShape = rules.TypeShape{
		Form:     symbol.FormOpaque,
		Spelling: paramName,
		Ref: symbol.Identity{
			Lang: sourceLang, Package: "svc", Owner: pageName, Name: paramName, Kind: symbol.KindTypeParam,
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

// TestType checks the GraphQL spelling of each canonical form by the table
// of the hub, under the policy that the plan resolves. It also checks the
// refusal of each form without a GraphQL spelling, and the child that each
// refusal is about.
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
				name: "returns Int for an integer of 8 bits",
				give: rules.Scalar("int8", rules.ScalarInt, 8), want: &emit.TypeRef{Spelling: intType},
			},
			{
				name: "returns Int for an integer of 16 bits",
				give: rules.Scalar("int16", rules.ScalarInt, 16), want: &emit.TypeRef{Spelling: intType},
			},
			{
				name: "returns Int for an integer of 32 bits",
				give: rules.Scalar("int32", rules.ScalarInt, 32), want: &emit.TypeRef{Spelling: intType},
			},
			{
				name: "returns Int for an unsigned integer of 8 bits",
				give: rules.Scalar("uint8", rules.ScalarUint, 8), want: &emit.TypeRef{Spelling: intType},
			},
			{
				name: "returns Int for an unsigned integer of 16 bits",
				give: rules.Scalar("uint16", rules.ScalarUint, 16), want: &emit.TypeRef{Spelling: intType},
			},
			{
				name: "returns Float for an unsigned integer of 32 bits under the default policy",
				give: rules.Scalar("uint32", rules.ScalarUint, 32), want: &emit.TypeRef{Spelling: floatType},
			},
			{
				name: "returns String for an integer of 64 bits under the default policy",
				give: long, want: &emit.TypeRef{Spelling: stringType},
			},
			{
				name: "returns String for an integer of the platform's width under the default policy",
				give: rules.Scalar("int", rules.ScalarInt, 0), want: &emit.TypeRef{Spelling: stringType},
			},
			{
				name: "returns String for an unsigned integer of 64 bits under the default policy",
				give: rules.Scalar("uint64", rules.ScalarUint, 64), want: &emit.TypeRef{Spelling: stringType},
			},
			{
				name: "returns String for an unsigned integer of the platform's width under the default policy",
				give: rules.Scalar("uint", rules.ScalarUint, 0), want: &emit.TypeRef{Spelling: stringType},
			},
			{
				name: "returns Float for a float of 32 bits",
				give: rules.Scalar("float32", rules.ScalarFloat, 32), want: &emit.TypeRef{Spelling: floatType},
			},
			{name: "returns Float for a float of 64 bits", give: float, want: &emit.TypeRef{Spelling: floatType}},
			{
				name: "returns Boolean for Bool",
				give: rules.Leaf(symbol.FormBool, "bool"), want: &emit.TypeRef{Spelling: booleanType},
			},
			{name: "returns String for Text", give: text, want: &emit.TypeRef{Spelling: stringType}},
			{
				name: "returns String for Bytes under the default policy",
				give: rules.Leaf(symbol.FormBytes, "[]byte"), want: &emit.TypeRef{Spelling: stringType},
			},
			{
				name: "returns String for Dynamic under the default policy",
				give: rules.Leaf(symbol.FormDynamic, "any"), want: &emit.TypeRef{Spelling: stringType},
			},
			{
				name: "returns String for the well-known timestamp under the default policy",
				give: rules.Reference("time.Time", rules.WellKnownTimestamp), want: &emit.TypeRef{Spelling: stringType},
			},
			{
				name: "returns String for the well-known duration under the default policy",
				give: rules.Reference("time.Duration", rules.WellKnownDuration),
				want: &emit.TypeRef{Spelling: stringType},
			},
			{
				name: "returns the map form of its key and its value for a map under the default policy",
				give: rules.TypeShape{
					Form: symbol.FormMap, Spelling: "map[string]float64", Elems: []rules.TypeShape{text, float},
				},
				want: &emit.TypeRef{
					Form: symbol.FormMap, Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: floatType}},
				},
			},
			{
				name: "returns the nullable form for an optional",
				give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "*string", Elems: []rules.TypeShape{text}},
				want: &emit.TypeRef{
					Spelling: stringType, Form: symbol.FormOptional, Elems: []*emit.TypeRef{{Spelling: stringType}},
				},
			},
			{
				name: "returns the non-null list form for a list",
				give: rules.TypeShape{
					Form: symbol.FormList, Spelling: "[]Session", Elems: []rules.TypeShape{sessionTo},
				},
				want: &emit.TypeRef{
					Spelling: "[Session!]!", Form: symbol.FormList,
					Elems: []*emit.TypeRef{{Spelling: sessionName, Target: session}},
				},
			},
			{
				name: "returns a list of nullable elements for a list of an optional",
				give: rules.TypeShape{
					Form: symbol.FormList, Spelling: "[]*string",
					Elems: []rules.TypeShape{
						{Form: symbol.FormOptional, Spelling: "*string", Elems: []rules.TypeShape{text}},
					},
				},
				want: &emit.TypeRef{
					Spelling: "[String]!", Form: symbol.FormList,
					Elems: []*emit.TypeRef{{
						Spelling: stringType, Form: symbol.FormOptional, Elems: []*emit.TypeRef{{Spelling: stringType}},
					}},
				},
			},
			{
				name: "returns a nullable list for an optional of a list",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "*[]string",
					Elems: []rules.TypeShape{
						{Form: symbol.FormList, Spelling: "[]string", Elems: []rules.TypeShape{text}},
					},
				},
				want: &emit.TypeRef{
					Spelling: "[String!]", Form: symbol.FormOptional,
					Elems: []*emit.TypeRef{{
						Spelling: "[String!]!", Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: stringType}},
					}},
				},
			},
			{
				name: "returns a list of the list form for a list of a list",
				give: rules.TypeShape{
					Form: symbol.FormList, Spelling: "[][]string",
					Elems: []rules.TypeShape{
						{Form: symbol.FormList, Spelling: "[]string", Elems: []rules.TypeShape{text}},
					},
				},
				want: &emit.TypeRef{
					Spelling: "[[String!]!]!", Form: symbol.FormList,
					Elems: []*emit.TypeRef{{
						Spelling: "[String!]!", Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: stringType}},
					}},
				},
			},
			{
				name: "returns a read-only list as a list",
				give: rules.TypeShape{
					Form: symbol.FormList, Spelling: "readonly string[]", Mutability: symbol.MutabilityImmutable,
					Elems: []rules.TypeShape{text},
				},
				want: &emit.TypeRef{
					Spelling: "[String!]!", Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: stringType}},
				},
			},
			{
				name: "returns the stream form of its element for an asynchronous stream",
				give: rules.TypeShape{
					Form: symbol.FormStream, Spelling: "Stream<Session>", Async: true,
					Elems: []rules.TypeShape{sessionTo},
				},
				want: &emit.TypeRef{
					Spelling: "Session!", Form: symbol.FormStream, Async: true,
					Elems: []*emit.TypeRef{{Spelling: sessionName, Target: session}},
				},
			},
			{
				name: "returns a stream of nullable elements for an asynchronous stream of an optional",
				give: rules.TypeShape{
					Form: symbol.FormStream, Spelling: "Stream<*string>", Async: true,
					Elems: []rules.TypeShape{
						{Form: symbol.FormOptional, Spelling: "*string", Elems: []rules.TypeShape{text}},
					},
				},
				want: &emit.TypeRef{
					Spelling: stringType, Form: symbol.FormStream, Async: true,
					Elems: []*emit.TypeRef{{
						Spelling: stringType, Form: symbol.FormOptional, Elems: []*emit.TypeRef{{Spelling: stringType}},
					}},
				},
			},
			{
				name: "returns a translated reference for a reference",
				give: sessionTo, want: &emit.TypeRef{Spelling: sessionName, Target: session},
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
				assert.NoError(t, err, "GraphQL spells the form")
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
				name:   "returns Long for an integer of 64 bits under the choice Long",
				chosen: map[plugin.PolicyKey]plugin.Choice{graphql.Int64Type: graphql.Long},
				give:   long, want: string(graphql.Long),
			},
			{
				name:   "returns Float for an integer of the platform's width under the choice Float",
				chosen: map[plugin.PolicyKey]plugin.Choice{graphql.Int64Type: graphql.Float},
				give:   rules.Scalar("int", rules.ScalarInt, 0), want: floatType,
			},
			{
				name:   "returns UnsignedInt for an unsigned integer of 32 bits under the choice UnsignedInt",
				chosen: map[plugin.PolicyKey]plugin.Choice{graphql.Uint32Type: graphql.UnsignedInt},
				give:   rules.Scalar("uint32", rules.ScalarUint, 32), want: string(graphql.UnsignedInt),
			},
			{
				name:   "returns String for an unsigned integer of 32 bits under the choice String",
				chosen: map[plugin.PolicyKey]plugin.Choice{graphql.Uint32Type: graphql.String},
				give:   rules.Scalar("uint32", rules.ScalarUint, 32), want: stringType,
			},
			{
				name:   "returns UnsignedLong for an unsigned integer of 64 bits under the choice UnsignedLong",
				chosen: map[plugin.PolicyKey]plugin.Choice{graphql.Uint64Type: graphql.UnsignedLong},
				give:   rules.Scalar("uint64", rules.ScalarUint, 64), want: string(graphql.UnsignedLong),
			},
			{
				name:   "returns Base64String for Bytes under the choice Base64String",
				chosen: map[plugin.PolicyKey]plugin.Choice{graphql.BytesType: graphql.Base64String},
				give:   rules.Leaf(symbol.FormBytes, "[]byte"), want: string(graphql.Base64String),
			},
			{
				name:   "returns DateTime for the well-known timestamp under the choice DateTime",
				chosen: map[plugin.PolicyKey]plugin.Choice{graphql.TimestampType: graphql.DateTime},
				give:   rules.Reference("time.Time", rules.WellKnownTimestamp), want: string(graphql.DateTime),
			},
			{
				name:   "returns Duration for the well-known duration under the choice Duration",
				chosen: map[plugin.PolicyKey]plugin.Choice{graphql.DurationType: graphql.Duration},
				give:   rules.Reference("time.Duration", rules.WellKnownDuration), want: string(graphql.Duration),
			},
			{
				name:   "returns Any for Dynamic under the choice Any",
				chosen: map[plugin.PolicyKey]plugin.Choice{graphql.DynamicType: graphql.Any},
				give:   rules.Leaf(symbol.FormDynamic, "any"), want: string(graphql.Any),
			},
			{
				name:   "returns Any for a map under the choice Any",
				chosen: map[plugin.PolicyKey]plugin.Choice{graphql.MapType: graphql.Any},
				give: rules.TypeShape{
					Form: symbol.FormMap, Spelling: "map[string]chan int", Elems: []rules.TypeShape{text, channel},
				},
				want: string(graphql.Any),
			},
			{
				name:   "returns String for an integer of 64 bits under a choice of another policy",
				chosen: map[plugin.PolicyKey]plugin.Choice{graphql.Uint64Type: graphql.UnsignedLong},
				give:   long, want: stringType,
			},
		}
		for _, tt := range choices {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, policy(t, tt.chosen))
				assert.NoError(t, err, "GraphQL spells the form under the choice")
				assert.Equal(t, got, &emit.TypeRef{Spelling: tt.want}, "the named type of the choice")
			})
		}

		refusals := []struct {
			name string
			give rules.TypeShape
			want string
		}{
			{
				name: "returns an error for an integer of 128 bits",
				give: rules.Scalar("i128", rules.ScalarInt, 128), want: "graphql: no GraphQL int type has 128 bits",
			},
			{
				name: "returns an error for an unsigned integer of 128 bits",
				give: rules.Scalar("u128", rules.ScalarUint, 128), want: "graphql: no GraphQL uint type has 128 bits",
			},
			{
				name: "returns an error for a float of 16 bits",
				give: rules.Scalar("f16", rules.ScalarFloat, 16), want: "graphql: no GraphQL float type has 16 bits",
			},
			{
				name: "returns an error for an optional of an optional",
				give: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: "Option<Option<String>>",
					Elems: []rules.TypeShape{{
						Form: symbol.FormOptional, Spelling: "Option<String>", Elems: []rules.TypeShape{text},
					}},
				},
				want: "graphql: an optional of an optional has no GraphQL spelling",
			},
			{
				name: "returns an error for an array",
				give: rules.TypeShape{
					Form: symbol.FormArray, Spelling: "[2]string", Length: 2, Elems: []rules.TypeShape{text},
				},
				want: "graphql: an array has no GraphQL spelling",
			},
			{
				name: "returns an error for a tuple",
				give: rules.TypeShape{
					Form: symbol.FormTuple, Spelling: "(String, f64)", Elems: []rules.TypeShape{text, float},
				},
				want: "graphql: a tuple has no GraphQL spelling",
			},
			{
				name: "returns an error for a union",
				give: rules.TypeShape{
					Form: symbol.FormUnion, Spelling: "string | number", Elems: []rules.TypeShape{text, float},
				},
				want: "graphql: a union has no GraphQL spelling",
			},
			{
				name: "returns an error for an intersection",
				give: rules.TypeShape{
					Form: symbol.FormIntersection, Spelling: "Session & Page",
					Elems: []rules.TypeShape{sessionTo, rules.Reference(pageName, page)},
				},
				want: "graphql: an intersection has no GraphQL spelling",
			},
			{
				name: "returns an error for a function type",
				give: rules.TypeShape{
					Form: symbol.FormFunc, Spelling: "func(string)", Split: 1, Elems: []rules.TypeShape{text},
				},
				want: "graphql: a function type has no GraphQL spelling",
			},
			{
				name: "returns an error for a synchronous stream",
				give: channel, want: "graphql: a synchronous stream has no GraphQL spelling",
			},
			{
				name: "returns an error for a borrow",
				give: rules.TypeShape{Form: symbol.FormBorrow, Spelling: "&str", Elems: []rules.TypeShape{text}},
				want: "graphql: a borrow has no GraphQL spelling",
			},
			{
				name: "returns an error for a wildcard",
				give: rules.TypeShape{
					Form: symbol.FormWildcard, Spelling: "? extends Session", Variance: symbol.VarianceOut,
					Elems: []rules.TypeShape{sessionTo},
				},
				want: "graphql: a wildcard has no GraphQL spelling",
			},
			{
				name: "returns an error for an inline type",
				give: rules.TypeShape{Form: symbol.FormInline, Spelling: "struct{}"},
				want: "graphql: an inline type has no GraphQL spelling",
			},
			{
				name: "returns an error for an opaque type",
				give: rules.Opaque(nil),
				want: "graphql: a type that the rules of its language do not classify has no GraphQL spelling",
			},
			{
				name: "returns an error for the well-known error",
				give: rules.Reference("error", rules.WellKnownError),
				want: "graphql: the well-known error has no GraphQL spelling",
			},
			{
				name: "returns an error for the well-known empty value",
				give: rules.Reference("struct{}", rules.WellKnownEmpty),
				want: "graphql: the well-known empty value has no GraphQL spelling",
			},
			{
				name: "returns an error for the well-known context",
				give: rules.Reference("context", rules.WellKnownContext),
				want: "graphql: the well-known context has no GraphQL spelling, because a schema states no context",
			},
			{
				name: "returns an error for a reference with type arguments",
				give: rules.Reference("Page[int64]", page, long),
				want: "graphql: a reference with type arguments has no GraphQL spelling",
			},
			{
				name: "returns an error for a type parameter", give: paramShape,
				want: "graphql: a type parameter has no GraphQL spelling",
			},
			{
				name: "returns an error about a type parameter inside a list",
				give: rules.TypeShape{Form: symbol.FormList, Spelling: "[]T", Elems: []rules.TypeShape{paramShape}},
				want: "graphql: T: a type parameter has no GraphQL spelling",
			},
			{
				name: "returns an error for an asynchronous stream inside a list",
				give: rules.TypeShape{
					Form: symbol.FormList, Spelling: "[]Stream<Session>",
					Elems: []rules.TypeShape{{
						Form: symbol.FormStream, Spelling: "Stream<Session>", Async: true,
						Elems: []rules.TypeShape{sessionTo},
					}},
				},
				want: "graphql: Stream<Session>: an asynchronous stream inside another form has no GraphQL spelling",
			},
			{
				name: "returns an error for a list of two children",
				give: rules.TypeShape{Form: symbol.FormList, Spelling: "[]X", Elems: []rules.TypeShape{text, text}},
				want: "graphql: a list takes one child, and the shape has 2",
			},
			{
				name: "returns an error for an optional without a child",
				give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "?"},
				want: "graphql: an optional takes one child, and the shape has 0",
			},
			{
				name: "returns an error for a map of one child",
				give: rules.TypeShape{Form: symbol.FormMap, Spelling: "map[string]", Elems: []rules.TypeShape{text}},
				want: "graphql: a map takes two children, and the shape has 1",
			},
			{
				name: "returns an error for an asynchronous stream of two children",
				give: rules.TypeShape{
					Form: symbol.FormStream, Spelling: "S", Async: true, Elems: []rules.TypeShape{text, text},
				},
				want: "graphql: an asynchronous stream takes one child, and the shape has 2",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, policy(t, nil))
				assert.HasError(t, err, "GraphQL has no spelling of the form")
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
				assert.HasError(t, err, "GraphQL has no spelling of the child")
				assert.Equal(t, err.Error(), "graphql: chan int: a synchronous stream has no GraphQL spelling",
					"the error starts with the spelling of the child")
			})
		}
	})

	t.Run("Expression", func(t *testing.T) {
		t.Parallel()

		named := &emit.TypeRef{Spelling: sessionName, Target: session}
		tests := []struct {
			name string
			give *emit.TypeRef
			want string
		}{
			{
				name: "returns the name with the mark of a non-null type for a named reference",
				give: named, want: "Session!",
			},
			{
				name: "returns the spelling of an optional",
				give: &emit.TypeRef{Spelling: sessionName, Form: symbol.FormOptional, Elems: []*emit.TypeRef{named}},
				want: sessionName,
			},
			{
				name: "returns the spelling of a list",
				give: &emit.TypeRef{Spelling: "[Session!]!", Form: symbol.FormList, Elems: []*emit.TypeRef{named}},
				want: "[Session!]!",
			},
			{
				name: "returns the spelling of an asynchronous stream",
				give: &emit.TypeRef{
					Spelling: "Session!", Form: symbol.FormStream, Async: true, Elems: []*emit.TypeRef{named},
				},
				want: "Session!",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, spell.Expression(tt.give), tt.want, "the type expression of the reference")
			})
		}
	})
}

// TestTypeAllocs checks the ceiling of each measured spelling in the
// ordinary run, which does not run benchmarks.
func TestTypeAllocs(t *testing.T) {
	t.Run("Type", func(t *testing.T) {
		p := policy(t, nil)
		for _, tt := range typeCalls() {
			var (
				got *emit.TypeRef
				err error
			)
			assert.MaxAllocs(t, func() { got, err = spell.Type(tt.give, p) }, tt.allocs,
				"Type of "+tt.name+" allocates within its ceiling")
			assert.NoError(t, err, "Type spells "+tt.name)
			assert.Equal(t, got.Spelling, tt.want, "Type spells "+tt.name+" in GraphQL")
		}
	})

	t.Run("Expression", func(t *testing.T) {
		named := &emit.TypeRef{Spelling: sessionName, Target: session}
		listed := &emit.TypeRef{Spelling: "[Session!]!", Form: symbol.FormList, Elems: []*emit.TypeRef{named}}
		var got string
		assert.MaxAllocs(t, func() { got = spell.Expression(named) }, leafAllocs,
			"Expression allocates the expression of a named reference")
		assert.Equal(t, got, "Session!", "Expression writes the mark of a non-null type")
		assert.MaxAllocs(t, func() { got = spell.Expression(listed) }, 0,
			"Expression does not allocate for a structural reference")
		assert.Equal(t, got, "[Session!]!", "Expression returns the spelling of a structural reference")
	})
}

// BenchmarkType measures the spoke that every translation into GraphQL
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
				assert.Equal(b, got.Spelling, tt.want, "Type spells "+tt.name+" in GraphQL")
			})
		}
	})
}

// policy returns the policy of the GraphQL target with the choices that
// chosen selects.
func policy(tb testing.TB, chosen map[plugin.PolicyKey]plugin.Choice) plugin.Policy {
	tb.Helper()

	p, err := plugin.NewPolicy(graphql.Target, graphql.Policies(), chosen)
	assert.NoError(tb, err, "the GraphQL policies resolve")
	return p
}

// typeCalls returns the spellings that the allocation test and the
// benchmark measure: a scalar under the policy, a list, an optional, a map
// and an asynchronous stream.
func typeCalls() []typeCall {
	return []typeCall{
		{name: "an integer of 64 bits", give: long, allocs: leafAllocs, want: stringType},
		{
			name: "a list of a reference", allocs: listAllocs, want: "[Session!]!",
			give: rules.TypeShape{Form: symbol.FormList, Spelling: "[]Session", Elems: []rules.TypeShape{sessionTo}},
		},
		{
			name: "an optional", allocs: optionalAllocs, want: stringType,
			give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "*string", Elems: []rules.TypeShape{text}},
		},
		{
			name: "a map", allocs: mapAllocs, want: "",
			give: rules.TypeShape{
				Form: symbol.FormMap, Spelling: "map[string]float64", Elems: []rules.TypeShape{text, float},
			},
		},
		{
			name: "an asynchronous stream", allocs: streamAllocs, want: "Session!",
			give: rules.TypeShape{
				Form: symbol.FormStream, Spelling: "Stream<Session>", Async: true, Elems: []rules.TypeShape{sessionTo},
			},
		},
	}
}
