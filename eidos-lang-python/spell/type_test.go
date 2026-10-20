// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/spell"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// sourceLang is the language whose declarations the cases translate into
// Python.
const sourceLang symbol.Lang = "golang"

// The spellings of the shapes of the source language.
const (
	textSpelling  = "string"
	floatSpelling = "float64"
	int64Spelling = "int64"
	sessionName   = "Session"
	pageName      = "Page"
	namedName     = "Named"
	sizedName     = "Sized"
	resultName    = "Result"
)

// The package paths of the modules of the standard library that the
// spellings of Python name.
const (
	builtinsModule = "builtins"
	abcModule      = "collections/abc"
	typingModule   = "typing"
	datetimeModule = "datetime"
)

// The builtins that the cases expect, each a reference of the module
// builtins. Type returns references of its own, so the cases share them.
var (
	intRef    = &emit.TypeRef{Spelling: "int", Package: builtinsModule}
	floatRef  = &emit.TypeRef{Spelling: "float", Package: builtinsModule}
	boolRef   = &emit.TypeRef{Spelling: "bool", Package: builtinsModule}
	strRef    = &emit.TypeRef{Spelling: "str", Package: builtinsModule}
	bytesRef  = &emit.TypeRef{Spelling: "bytes", Package: builtinsModule}
	objectRef = &emit.TypeRef{Spelling: "object", Package: builtinsModule}
)

// inlineRefusal is the refusal of an inline child, after the spelling of
// the child.
const inlineRefusal = "python: struct{}: an inline type has no Python spelling"

// The allocations of the spellings that the allocation test and the
// benchmark measure.
const (
	// leafAllocs is a builtin type: the reference.
	leafAllocs = 1
	// listAllocs is a list of a translated reference: the reference of the
	// list, its list of arguments and the reference of the argument.
	listAllocs = 3
	// optionalAllocs is an optional of text: the reference of the optional,
	// its list of children, the reference of the child and the spelling.
	optionalAllocs = 4
	// mapAllocs is a map of text onto a float: the reference of the map, its
	// list of arguments and the references of the two arguments.
	mapAllocs = 4
)

// The declarations of the source language that the cases reference. state
// is a type that session declares.
var (
	session = symbol.Identity{Lang: sourceLang, Package: "svc", Name: sessionName, Kind: symbol.KindStruct}
	page    = symbol.Identity{Lang: sourceLang, Package: "svc", Name: pageName, Kind: symbol.KindStruct}
	named   = symbol.Identity{Lang: sourceLang, Package: "svc", Name: namedName, Kind: symbol.KindInterface}
	sized   = symbol.Identity{Lang: sourceLang, Package: "svc", Name: sizedName, Kind: symbol.KindInterface}
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
	flag      = rules.Leaf(symbol.FormBool, "bool")
	sessionTo = rules.Reference(sessionName, session)
	namedTo   = rules.Reference(namedName, named)
	sizedTo   = rules.Reference(sizedName, sized)
	inline    = rules.TypeShape{Form: symbol.FormInline, Spelling: "struct{}"}
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

// TestType checks the Python spelling of each canonical form by the table of
// the hub, under the policy that the plan resolves. It also checks the
// refusal of each form without a Python spelling, and the child that each
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
				name: "returns int for an integer of 8 bits",
				give: rules.Scalar("int8", rules.ScalarInt, 8), want: intRef,
			},
			{
				name: "returns int for an unsigned integer of 64 bits",
				give: rules.Scalar("uint64", rules.ScalarUint, 64), want: intRef,
			},
			{
				name: "returns int for an integer of the platform's width",
				give: rules.Scalar("int", rules.ScalarInt, 0), want: intRef,
			},
			{
				name: "returns int for an integer of 128 bits",
				give: rules.Scalar("i128", rules.ScalarInt, 128), want: intRef,
			},
			{
				name: "returns float for a float of 16 bits",
				give: rules.Scalar("f16", rules.ScalarFloat, 16), want: floatRef,
			},
			{
				name: "returns float for a float of 32 bits",
				give: rules.Scalar("float32", rules.ScalarFloat, 32), want: floatRef,
			},
			{
				name: "returns float for a float of 64 bits",
				give: float, want: floatRef,
			},
			{name: "returns bool for Bool", give: flag, want: boolRef},
			{name: "returns str for Text", give: text, want: strRef},
			{
				name: "returns bytes for Bytes",
				give: rules.Leaf(symbol.FormBytes, "[]byte"), want: bytesRef,
			},
			{
				name: "returns object for the top type under the default policy",
				give: rules.Leaf(symbol.FormDynamic, "any"), want: objectRef,
			},
			{
				name: "returns the union with None for an optional",
				give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "*string", Elems: []rules.TypeShape{text}},
				want: &emit.TypeRef{
					Spelling: "str | None", Form: symbol.FormOptional, Elems: []*emit.TypeRef{strRef},
				},
			},
			{
				name: "returns an optional of a list with the argument of the list in brackets",
				give: rules.TypeShape{
					Form:     symbol.FormOptional,
					Spelling: "*[]string",
					Elems: []rules.TypeShape{
						{Form: symbol.FormList, Spelling: "[]string", Elems: []rules.TypeShape{text}},
					},
				},
				want: &emit.TypeRef{
					Spelling: "list[str] | None", Form: symbol.FormOptional,
					Elems: []*emit.TypeRef{{Spelling: "list", Package: builtinsModule, Args: []*emit.TypeRef{strRef}}},
				},
			},
			{
				name: "returns list for a list of a translated reference",
				give: rules.TypeShape{
					Form:     symbol.FormList,
					Spelling: "[]Session",
					Elems:    []rules.TypeShape{sessionTo},
				},
				want: &emit.TypeRef{
					Spelling: "list",
					Package:  builtinsModule,
					Args:     []*emit.TypeRef{{Spelling: sessionName, Target: session}},
				},
			},
			{name: "returns the name of a type parameter", give: paramShape, want: &emit.TypeRef{Spelling: paramName}},
			{
				name: "returns list for a list of a type parameter",
				give: rules.TypeShape{Form: symbol.FormList, Spelling: "[]T", Elems: []rules.TypeShape{paramShape}},
				want: &emit.TypeRef{
					Spelling: "list",
					Package:  builtinsModule,
					Args:     []*emit.TypeRef{{Spelling: paramName}},
				},
			},
			{
				name: "returns Sequence for an immutable list",
				give: rules.TypeShape{
					Form: symbol.FormList, Spelling: "ReadonlyArray", Mutability: symbol.MutabilityImmutable,
					Elems: []rules.TypeShape{text},
				},
				want: &emit.TypeRef{Spelling: "Sequence", Package: abcModule, Args: []*emit.TypeRef{strRef}},
			},
			{
				name: "returns a tuple of the length of an array",
				give: rules.TypeShape{
					Form:     symbol.FormArray,
					Spelling: "[2]bool",
					Length:   2,
					Elems:    []rules.TypeShape{flag},
				},
				want: &emit.TypeRef{
					Spelling: "tuple[bool, bool]", Form: symbol.FormTuple, Mutability: symbol.MutabilityImmutable,
					Elems: []*emit.TypeRef{boolRef, boolRef},
				},
			},
			{
				name: "returns bytes for an array of 8-bit unsigned integers",
				give: rules.TypeShape{
					Form: symbol.FormArray, Spelling: "[16]byte", Length: 16,
					Elems: []rules.TypeShape{rules.Scalar("byte", rules.ScalarUint, 8)},
				},
				want: bytesRef,
			},
			{
				name: "returns dict for a map",
				give: rules.TypeShape{
					Form:     symbol.FormMap,
					Spelling: "map[string]float64",
					Elems:    []rules.TypeShape{text, float},
				},
				want: &emit.TypeRef{Spelling: "dict", Package: builtinsModule, Args: []*emit.TypeRef{strRef, floatRef}},
			},
			{
				name: "returns Mapping for an immutable map",
				give: rules.TypeShape{
					Form: symbol.FormMap, Spelling: "ReadonlyMap", Mutability: symbol.MutabilityImmutable,
					Elems: []rules.TypeShape{text, long},
				},
				want: &emit.TypeRef{
					Spelling: "Mapping",
					Package:  abcModule,
					Args:     []*emit.TypeRef{strRef, intRef},
				},
			},
			{
				name: "returns an immutable tuple for a tuple",
				give: rules.TypeShape{
					Form:     symbol.FormTuple,
					Spelling: "[string, number]",
					Elems:    []rules.TypeShape{text, float},
				},
				want: &emit.TypeRef{
					Spelling: "tuple[str, float]", Form: symbol.FormTuple, Mutability: symbol.MutabilityImmutable,
					Elems: []*emit.TypeRef{strRef, floatRef},
				},
			},
			{
				name: "returns the empty tuple for a tuple without members",
				give: rules.TypeShape{Form: symbol.FormTuple, Spelling: "()"},
				want: &emit.TypeRef{
					Spelling:   "tuple[()]",
					Form:       symbol.FormTuple,
					Mutability: symbol.MutabilityImmutable,
				},
			},
			{
				name: "returns a union for a union",
				give: rules.TypeShape{
					Form:     symbol.FormUnion,
					Spelling: "string | number",
					Elems:    []rules.TypeShape{text, float},
				},
				want: &emit.TypeRef{
					Spelling: "str | float", Form: symbol.FormUnion,
					Elems: []*emit.TypeRef{strRef, floatRef},
				},
			},
			{
				name: "returns Never for a union without members",
				give: rules.TypeShape{Form: symbol.FormUnion, Spelling: "never"},
				want: &emit.TypeRef{Spelling: "Never", Package: typingModule},
			},
			{
				name: "returns an intersection without a spelling for an intersection of interfaces",
				give: rules.TypeShape{
					Form:     symbol.FormIntersection,
					Spelling: "Named & Sized",
					Elems:    []rules.TypeShape{namedTo, sizedTo},
				},
				want: &emit.TypeRef{
					Form: symbol.FormIntersection,
					Elems: []*emit.TypeRef{
						{Spelling: namedName, Target: named}, {Spelling: sizedName, Target: sized},
					},
				},
			},
			{
				name: "returns Callable for a function type of one result",
				give: rules.TypeShape{
					Form:     symbol.FormFunc,
					Spelling: "func(string) float64",
					Split:    1,
					Elems:    []rules.TypeShape{text, float},
				},
				want: &emit.TypeRef{
					Spelling: "Callable[[str], float]", Form: symbol.FormFunc, Split: 1,
					Elems: []*emit.TypeRef{strRef, floatRef},
				},
			},
			{
				name: "returns Callable with the result None for a function type without a result",
				give: rules.TypeShape{
					Form:     symbol.FormFunc,
					Spelling: "func(string, bool)",
					Split:    2,
					Elems:    []rules.TypeShape{text, flag},
				},
				want: &emit.TypeRef{
					Spelling: "Callable[[str, bool], None]", Form: symbol.FormFunc, Split: 2,
					Elems: []*emit.TypeRef{strRef, boolRef},
				},
			},
			{
				name: "returns Callable with a tuple for a function type of two results",
				give: rules.TypeShape{
					Form: symbol.FormFunc, Spelling: "func() (float64, string)", Elems: []rules.TypeShape{float, text},
				},
				want: &emit.TypeRef{
					Spelling: "Callable[[], tuple[float, str]]", Form: symbol.FormFunc,
					Elems: []*emit.TypeRef{floatRef, strRef},
				},
			},
			{
				name: "returns Iterator for a synchronous stream",
				give: rules.TypeShape{
					Form:     symbol.FormStream,
					Spelling: "chan float64",
					Elems:    []rules.TypeShape{float},
				},
				want: &emit.TypeRef{
					Spelling: "Iterator",
					Package:  abcModule,
					Args:     []*emit.TypeRef{floatRef},
				},
			},
			{
				name: "returns AsyncIterator for an asynchronous stream",
				give: rules.TypeShape{
					Form:     symbol.FormStream,
					Spelling: "AsyncIterable<number>",
					Async:    true,
					Elems:    []rules.TypeShape{float},
				},
				want: &emit.TypeRef{
					Spelling: "AsyncIterator", Package: abcModule, Args: []*emit.TypeRef{floatRef},
				},
			},
			{
				name: "returns the type of a borrow",
				give: rules.TypeShape{Form: symbol.FormBorrow, Spelling: "&str", Elems: []rules.TypeShape{text}},
				want: strRef,
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
				name: "returns object for a wildcard without a bound",
				give: rules.TypeShape{Form: symbol.FormWildcard, Spelling: "?"},
				want: objectRef,
			},
			{
				name: "returns a translated reference with its translated arguments",
				give: rules.Reference(pageName, page, long),
				want: &emit.TypeRef{Spelling: pageName, Target: page, Args: []*emit.TypeRef{intRef}},
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
			{
				name: "returns datetime for the well-known timestamp",
				give: rules.Reference("time.Time", rules.WellKnownTimestamp),
				want: &emit.TypeRef{Spelling: "datetime", Package: datetimeModule},
			},
			{
				name: "returns timedelta for the well-known duration",
				give: rules.Reference("time.Duration", rules.WellKnownDuration),
				want: &emit.TypeRef{Spelling: "timedelta", Package: datetimeModule},
			},
			{
				name: "returns None for the well-known empty value",
				give: rules.Reference("google.protobuf.Empty", rules.WellKnownEmpty),
				want: &emit.TypeRef{Spelling: "None"},
			},
			{
				name: "returns Exception for the well-known error",
				give: rules.Reference("error", rules.WellKnownError),
				want: &emit.TypeRef{Spelling: "Exception", Package: builtinsModule},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, policy(t, nil))
				assert.NoError(t, err, "Python spells the form")
				assert.Equal(t, got, tt.want, "the reference")
			})
		}

		t.Run("returns Any for the top type under the choice Any", func(t *testing.T) {
			t.Parallel()

			chosen := map[plugin.PolicyKey]plugin.Choice{python.DynamicPolicy: python.Any}
			got, err := spell.Type(rules.Leaf(symbol.FormDynamic, "any"), policy(t, chosen))
			assert.NoError(t, err, "Python spells the top type")
			assert.Equal(t, got, &emit.TypeRef{Spelling: "Any", Package: typingModule}, "the type of the choice")
		})

		refusals := []struct {
			name string
			give rules.TypeShape
			want string
		}{
			{
				name: "returns an error for an inline type",
				give: inline,
				want: "python: an inline type has no Python spelling",
			},
			{
				name: "returns an error for the well-known context",
				give: rules.Reference("context", rules.WellKnownContext),
				want: "python: the well-known context has no Python spelling, because Python has no standard type of " +
					"the context of a call",
			},
			{
				name: "returns an error for an opaque type",
				give: rules.Opaque(nil),
				want: "python: a type that the rules of its language do not classify has no Python spelling",
			},
			{
				name: "returns an error for a float of 128 bits",
				give: rules.Scalar("f128", rules.ScalarFloat, 128),
				want: "python: no Python float type has 128 bits",
			},
			{
				name: "returns an error for an array of an unstated length",
				give: rules.TypeShape{Form: symbol.FormArray, Spelling: "[N]string", Elems: []rules.TypeShape{text}},
				want: "python: an array of an unstated length has no Python spelling",
			},
			{
				name: "returns an error for an array of two children",
				give: rules.TypeShape{
					Form:     symbol.FormArray,
					Spelling: "[2]X",
					Length:   2,
					Elems:    []rules.TypeShape{text, text},
				},
				want: "python: an array takes one child, and the shape has 2",
			},
			{
				name: "returns an error for a list of two children",
				give: rules.TypeShape{Form: symbol.FormList, Spelling: "[]X", Elems: []rules.TypeShape{text, text}},
				want: "python: a list takes one child, and the shape has 2",
			},
			{
				name: "returns an error for a map of one child",
				give: rules.TypeShape{Form: symbol.FormMap, Spelling: "map[string]", Elems: []rules.TypeShape{text}},
				want: "python: a map takes two children, and the shape has 1",
			},
			{
				name: "returns an error for a borrow of two children",
				give: rules.TypeShape{Form: symbol.FormBorrow, Spelling: "&X", Elems: []rules.TypeShape{text, text}},
				want: "python: a borrow takes one child, and the shape has 2",
			},
			{
				name: "returns an error for a function type of more parameters than children",
				give: rules.TypeShape{Form: symbol.FormFunc, Spelling: "func()", Split: 1},
				want: "python: a function type splits its 0 children at 1",
			},
			{
				name: "returns an error for a wildcard with a lower bound",
				give: rules.TypeShape{
					Form: symbol.FormWildcard, Spelling: "? super Session", Variance: symbol.VarianceIn,
					Elems: []rules.TypeShape{sessionTo},
				},
				want: "python: a wildcard with a lower bound has no Python spelling",
			},
			{
				name: "returns an error for an intersection without members",
				give: rules.TypeShape{Form: symbol.FormIntersection, Spelling: "unknown"},
				want: "python: an intersection without members has no Python spelling",
			},
			{
				name: "returns an error for an intersection of a struct",
				give: rules.TypeShape{
					Form:     symbol.FormIntersection,
					Spelling: "Named & Session",
					Elems:    []rules.TypeShape{namedTo, sessionTo},
				},
				want: "python: Session: a Protocol extends interfaces alone, and the member is no interface",
			},
			{
				name: "returns an error about an argument of a member of an intersection",
				give: rules.TypeShape{
					Form: symbol.FormIntersection, Spelling: "Named[struct{}]",
					Elems: []rules.TypeShape{rules.Reference(namedName, named, inline)},
				},
				want: "python: Named: struct{}: an inline type has no Python spelling",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Type(tt.give, policy(t, nil))
				assert.HasError(t, err, "Python has no spelling of the form")
				assert.Equal(t, err.Error(), tt.want, "the error has the reason for the refusal")
				assert.Nil(t, got, "a refusal returns no reference")
			})
		}

		children := []struct {
			name string
			give rules.TypeShape
		}{
			{
				name: "returns an error about the child of an optional",
				give: rules.TypeShape{Form: symbol.FormOptional, Elems: []rules.TypeShape{inline}},
			},
			{
				name: "returns an error about the child of a borrow",
				give: rules.TypeShape{Form: symbol.FormBorrow, Elems: []rules.TypeShape{inline}},
			},
			{
				name: "returns an error about the child of an array",
				give: rules.TypeShape{Form: symbol.FormArray, Length: 2, Elems: []rules.TypeShape{inline}},
			},
			{
				name: "returns an error about a member of a tuple",
				give: rules.TypeShape{Form: symbol.FormTuple, Elems: []rules.TypeShape{text, inline}},
			},
			{
				name: "returns an error about the parameter of a function type",
				give: rules.TypeShape{Form: symbol.FormFunc, Split: 1, Elems: []rules.TypeShape{inline}},
			},
			{
				name: "returns an error about the bound of a wildcard",
				give: rules.TypeShape{
					Form:     symbol.FormWildcard,
					Variance: symbol.VarianceOut,
					Elems:    []rules.TypeShape{inline},
				},
			},
			{
				name: "returns an error about an argument of a reference",
				give: rules.Reference(pageName, page, inline),
			},
		}
		for _, tt := range children {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spell.Type(tt.give, policy(t, nil))
				assert.HasError(t, err, "Python has no spelling of the child")
				assert.Equal(t, err.Error(), inlineRefusal, "the error starts with the spelling of the child")
			})
		}
	})
}

// Type allocates each reference that it returns, its list of children or
// arguments, and the spelling of a structural reference. The ordinary run,
// which runs no benchmark, checks those ceilings here.
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
		assert.Equal(t, got.Spelling, tt.want, "Type spells "+tt.name+" in Python")
	}
}

// BenchmarkType measures the spoke that every translation into Python calls.
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
				assert.Equal(b, got.Spelling, tt.want, "Type spells "+tt.name+" in Python")
			})
		}
	})
}

// policy returns the policy of the Python target with the choices that
// chosen selects.
func policy(tb testing.TB, chosen map[plugin.PolicyKey]plugin.Choice) plugin.Policy {
	tb.Helper()

	p, err := plugin.NewPolicy(python.Target, python.Policies(), chosen)
	assert.NoError(tb, err, "the Python policies resolve")
	return p
}

// typeCalls returns the spellings that the allocation test and the benchmark
// measure: a scalar, a list, an optional and a map.
func typeCalls() []typeCall {
	return []typeCall{
		{name: "an integer of 64 bits", give: long, allocs: leafAllocs, want: "int"},
		{
			name: "a list of a reference", allocs: listAllocs, want: "list",
			give: rules.TypeShape{Form: symbol.FormList, Spelling: "[]Session", Elems: []rules.TypeShape{sessionTo}},
		},
		{
			name: "an optional", allocs: optionalAllocs, want: "str | None",
			give: rules.TypeShape{Form: symbol.FormOptional, Spelling: "*string", Elems: []rules.TypeShape{text}},
		},
		{
			name:   "a map",
			allocs: mapAllocs,
			want:   "dict",
			give: rules.TypeShape{
				Form:     symbol.FormMap,
				Spelling: "map[string]float64",
				Elems:    []rules.TypeShape{text, float},
			},
		},
	}
}
