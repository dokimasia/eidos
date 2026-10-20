// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python"
	pythonrules "go.dokimi.dev/eidos/lang/python/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings that the specs of builtin.go fold more than once.
const (
	anySpelling       = "Any"
	generatorSpelling = "Generator"
	sequenceSpelling  = "Sequence"
)

// The ceilings of builtin.go: a shape and a well-known identity allocate
// nothing, and an optional Literal allocates the list of its child.
const (
	builtinAllocs         = 0
	optionalLiteralAllocs = 1
)

// produceID is the generator function produce of the fixture.
var produceID = symbol.Identity{Lang: python.Lang, Package: modelsPkg, Name: "produce", Kind: symbol.KindFunction}

// anyChild is the child of a generic class without type arguments: the top
// type, as a type checker reads a missing argument as Any.
var anyChild = []rules.TypeShape{rules.Leaf(symbol.FormDynamic, anySpelling)}

// statusRef is a resolved member of the enum Status of the fixture, as a
// value of Literal.
var statusRef = &node.TypeRef{
	Spelling: "Status.NEW",
	Target: symbol.Identity{
		Lang: python.Lang, Package: modelsPkg, Owner: "Status", Name: "NEW", Kind: symbol.KindEnumVariant,
	},
}

// TestBuiltin pins the shapes of the builtins and of the classes and forms of
// typeshed, and the well-known classes.
func TestBuiltin(t *testing.T) {
	t.Parallel()

	t.Run("Builtin", func(t *testing.T) {
		t.Parallel()

		intArg := []*node.TypeRef{{Spelling: "int"}}
		strInt := []*node.TypeRef{{Spelling: "str"}, {Spelling: "int"}}
		mutable, immutable := symbol.MutabilityMutable, symbol.MutabilityImmutable
		tests := []struct {
			name string
			give *node.TypeRef
			want rules.TypeShape
		}{
			{
				name: "folds int to a signed integer of the platform's width",
				give: &node.TypeRef{Spelling: "int"},
				want: rules.Scalar("int", rules.ScalarInt, 0),
			},
			{
				name: "folds float to a float of 64 bits",
				give: &node.TypeRef{Spelling: "float"},
				want: rules.Scalar("float", rules.ScalarFloat, 64),
			},
			{
				name: "folds bool to a bool",
				give: &node.TypeRef{Spelling: "bool"},
				want: rules.Leaf(symbol.FormBool, "bool"),
			},
			{
				name: "folds str to text",
				give: &node.TypeRef{Spelling: "str"},
				want: rules.Leaf(symbol.FormText, "str"),
			},
			{
				name: "folds bytes to bytes",
				give: &node.TypeRef{Spelling: "bytes"},
				want: rules.Leaf(symbol.FormBytes, "bytes"),
			},
			{
				name: "folds bytearray to bytes",
				give: &node.TypeRef{Spelling: "bytearray"},
				want: rules.Leaf(symbol.FormBytes, "bytearray"),
			},
			{
				name: "folds object to the top type",
				give: &node.TypeRef{Spelling: "object"},
				want: rules.Leaf(symbol.FormDynamic, "object"),
			},
			{
				name: "folds list without arguments to a mutable list of the top type",
				give: &node.TypeRef{Spelling: "list"},
				want: rules.TypeShape{Form: symbol.FormList, Spelling: "list", Mutability: mutable, Elems: anyChild},
			},
			{
				name: "folds list with an argument to a mutable list whose child the fold takes",
				give: &node.TypeRef{Spelling: "list", Args: intArg},
				want: rules.TypeShape{Form: symbol.FormList, Spelling: "list", Mutability: mutable},
			},
			{
				name: "folds tuple without arguments to an immutable list of the top type",
				give: &node.TypeRef{Spelling: "tuple"},
				want: rules.TypeShape{Form: symbol.FormList, Spelling: "tuple", Mutability: immutable, Elems: anyChild},
			},
			{
				name: "folds dict without arguments to a mutable map of the top type",
				give: &node.TypeRef{Spelling: "dict"},
				want: rules.TypeShape{
					Form: symbol.FormMap, Spelling: "dict", Mutability: mutable,
					Elems: []rules.TypeShape{
						rules.Leaf(symbol.FormDynamic, anySpelling),
						rules.Leaf(symbol.FormDynamic, anySpelling),
					},
				},
			},
			{
				name: "folds frozendict with arguments to an immutable map",
				give: &node.TypeRef{Spelling: "frozendict", Args: strInt},
				want: rules.TypeShape{Form: symbol.FormMap, Spelling: "frozendict", Mutability: immutable},
			},
			{
				name: "folds set to Opaque",
				give: &node.TypeRef{Spelling: "set"},
				want: rules.TypeShape{Form: symbol.FormOpaque, Spelling: "set"},
			},
			{
				name: "folds None to the well-known empty value",
				give: &node.TypeRef{Spelling: "None"},
				want: rules.Reference("None", rules.WellKnownEmpty),
			},
			{
				name: "folds Exception to the well-known error",
				give: &node.TypeRef{Spelling: "Exception"},
				want: rules.Reference("Exception", rules.WellKnownError),
			},
			{
				name: "folds BaseException to the well-known error",
				give: &node.TypeRef{Spelling: "BaseException"},
				want: rules.Reference("BaseException", rules.WellKnownError),
			},
			{
				name: "folds Any of typing to the top type",
				give: &node.TypeRef{Spelling: anySpelling, Package: typingModule},
				want: rules.Leaf(symbol.FormDynamic, anySpelling),
			},
			{
				name: "folds Any of typing_extensions to the top type",
				give: &node.TypeRef{Spelling: anySpelling, Package: typingExtensions},
				want: rules.Leaf(symbol.FormDynamic, anySpelling),
			},
			{
				name: "folds Text to text",
				give: &node.TypeRef{Spelling: "Text", Package: typingModule},
				want: rules.Leaf(symbol.FormText, "Text"),
			},
			{
				name: "folds LiteralString to text",
				give: &node.TypeRef{Spelling: "LiteralString", Package: typingModule},
				want: rules.Leaf(symbol.FormText, "LiteralString"),
			},
			{
				name: "folds TypeGuard to a bool",
				give: &node.TypeRef{Spelling: "TypeGuard", Package: typingModule},
				want: rules.Leaf(symbol.FormBool, "TypeGuard"),
			},
			{
				name: "folds TypeIs to a bool",
				give: &node.TypeRef{Spelling: "TypeIs", Package: typingModule},
				want: rules.Leaf(symbol.FormBool, "TypeIs"),
			},
			{
				name: "folds Sequence of collections.abc to an immutable list",
				give: &node.TypeRef{Spelling: sequenceSpelling, Package: abcModule},
				want: rules.TypeShape{
					Form: symbol.FormList, Spelling: sequenceSpelling, Mutability: immutable, Elems: anyChild,
				},
			},
			{
				name: "folds a dotted name of collections.abc by its last name",
				give: &node.TypeRef{Spelling: "abc.Iterable", Package: abcModule},
				want: rules.TypeShape{
					Form: symbol.FormList, Spelling: "abc.Iterable", Mutability: immutable, Elems: anyChild,
				},
			},
			{
				name: "folds Collection to an immutable list",
				give: &node.TypeRef{Spelling: "Collection", Package: abcModule},
				want: rules.TypeShape{
					Form: symbol.FormList, Spelling: "Collection", Mutability: immutable, Elems: anyChild,
				},
			},
			{
				name: "folds MutableSequence to a mutable list",
				give: &node.TypeRef{Spelling: "MutableSequence", Package: abcModule},
				want: rules.TypeShape{
					Form: symbol.FormList, Spelling: "MutableSequence", Mutability: mutable, Elems: anyChild,
				},
			},
			{
				name: "folds List of typing to a mutable list",
				give: &node.TypeRef{Spelling: "List", Package: typingModule},
				want: rules.TypeShape{Form: symbol.FormList, Spelling: "List", Mutability: mutable, Elems: anyChild},
			},
			{
				name: "folds Deque of typing to a mutable list",
				give: &node.TypeRef{Spelling: "Deque", Package: typingModule},
				want: rules.TypeShape{Form: symbol.FormList, Spelling: "Deque", Mutability: mutable, Elems: anyChild},
			},
			{
				name: "folds deque of collections to a mutable list",
				give: &node.TypeRef{Spelling: "deque", Package: collectionsMod},
				want: rules.TypeShape{Form: symbol.FormList, Spelling: "deque", Mutability: mutable, Elems: anyChild},
			},
			{
				name: "folds Mapping to an immutable map",
				give: &node.TypeRef{Spelling: "Mapping", Package: abcModule, Args: strInt},
				want: rules.TypeShape{Form: symbol.FormMap, Spelling: "Mapping", Mutability: immutable},
			},
			{
				name: "folds MutableMapping to a mutable map",
				give: &node.TypeRef{Spelling: "MutableMapping", Package: abcModule, Args: strInt},
				want: rules.TypeShape{Form: symbol.FormMap, Spelling: "MutableMapping", Mutability: mutable},
			},
			{
				name: "folds Dict of typing to a mutable map",
				give: &node.TypeRef{Spelling: "Dict", Package: typingModule, Args: strInt},
				want: rules.TypeShape{Form: symbol.FormMap, Spelling: "Dict", Mutability: mutable},
			},
			{
				name: "folds DefaultDict of typing to a mutable map",
				give: &node.TypeRef{Spelling: "DefaultDict", Package: typingModule, Args: strInt},
				want: rules.TypeShape{Form: symbol.FormMap, Spelling: "DefaultDict", Mutability: mutable},
			},
			{
				name: "folds defaultdict of collections to a mutable map",
				give: &node.TypeRef{Spelling: "defaultdict", Package: collectionsMod, Args: strInt},
				want: rules.TypeShape{Form: symbol.FormMap, Spelling: "defaultdict", Mutability: mutable},
			},
			{
				name: "folds OrderedDict of collections to a mutable map",
				give: &node.TypeRef{Spelling: "OrderedDict", Package: collectionsMod, Args: strInt},
				want: rules.TypeShape{Form: symbol.FormMap, Spelling: "OrderedDict", Mutability: mutable},
			},
			{
				name: "folds ChainMap of collections to a mutable map",
				give: &node.TypeRef{Spelling: "ChainMap", Package: collectionsMod, Args: strInt},
				want: rules.TypeShape{Form: symbol.FormMap, Spelling: "ChainMap", Mutability: mutable},
			},
			{
				name: "folds Iterator without arguments to a stream of the top type",
				give: &node.TypeRef{Spelling: "Iterator", Package: abcModule},
				want: rules.TypeShape{Form: symbol.FormStream, Spelling: "Iterator", Elems: anyChild},
			},
			{
				name: "folds Iterator with an argument to a stream whose child the fold takes",
				give: &node.TypeRef{Spelling: "Iterator", Package: abcModule, Args: intArg},
				want: rules.TypeShape{Form: symbol.FormStream, Spelling: "Iterator"},
			},
			{
				name: "folds Generator of three arguments to a stream whose child the fold takes",
				give: &node.TypeRef{Spelling: generatorSpelling, Package: typingModule, Args: []*node.TypeRef{
					{Spelling: "str"}, {Spelling: "None"}, {Spelling: "None"},
				}},
				want: rules.TypeShape{Form: symbol.FormStream, Spelling: generatorSpelling},
			},
			{
				name: "folds AsyncIterator to an asynchronous stream",
				give: &node.TypeRef{Spelling: "AsyncIterator", Package: abcModule, Args: intArg},
				want: rules.TypeShape{Form: symbol.FormStream, Spelling: "AsyncIterator", Async: true},
			},
			{
				name: "folds AsyncIterable to an asynchronous stream",
				give: &node.TypeRef{Spelling: "AsyncIterable", Package: abcModule},
				want: rules.TypeShape{Form: symbol.FormStream, Spelling: "AsyncIterable", Async: true, Elems: anyChild},
			},
			{
				name: "folds AsyncGenerator to an asynchronous stream",
				give: &node.TypeRef{Spelling: "AsyncGenerator", Package: abcModule},
				want: rules.TypeShape{
					Form: symbol.FormStream, Spelling: "AsyncGenerator", Async: true, Elems: anyChild,
				},
			},
			{
				name: "folds another form of typing to Opaque",
				give: &node.TypeRef{Spelling: "Callable", Package: typingModule},
				want: rules.TypeShape{Form: symbol.FormOpaque, Spelling: "Callable"},
			},
			{
				name: "folds datetime to the well-known timestamp",
				give: &node.TypeRef{Spelling: "datetime", Package: datetimeModule},
				want: rules.Reference("datetime", rules.WellKnownTimestamp),
			},
			{
				name: "folds a dotted datetime by its last name",
				give: &node.TypeRef{Spelling: "datetime.datetime", Package: datetimeModule},
				want: rules.Reference("datetime.datetime", rules.WellKnownTimestamp),
			},
			{
				name: "folds timedelta to the well-known duration",
				give: &node.TypeRef{Spelling: "timedelta", Package: datetimeModule},
				want: rules.Reference("timedelta", rules.WellKnownDuration),
			},
			{
				name: "folds another class of datetime to Opaque",
				give: &node.TypeRef{Spelling: "date", Package: datetimeModule},
				want: rules.TypeShape{Form: symbol.FormOpaque, Spelling: "date"},
			},
			{
				name: "folds a name of another module to Opaque",
				give: &node.TypeRef{Spelling: "Thing", Package: modelsPkg},
				want: rules.TypeShape{Form: symbol.FormOpaque, Spelling: "Thing"},
			},
			{
				name: "folds a dotted name whose first name nothing binds to Opaque",
				give: &node.TypeRef{Spelling: "lib.Thing"},
				want: rules.TypeShape{Form: symbol.FormOpaque, Spelling: "lib.Thing"},
			},
			{
				name: "folds a reference with a target to Opaque",
				give: &node.TypeRef{Spelling: "Order", Target: orderID},
				want: rules.TypeShape{Form: symbol.FormOpaque, Spelling: "Order"},
			},
			{
				name: "folds a structural reference to Opaque",
				give: &node.TypeRef{Spelling: "int|None", Form: symbol.FormOptional},
				want: rules.TypeShape{Form: symbol.FormOpaque, Spelling: "int|None"},
			},
			{
				name: "folds a nil reference to Opaque",
				want: rules.TypeShape{Form: symbol.FormOpaque},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := pythonrules.Rules{}.Builtin(tt.give, rules.View{})
				assert.Equal(t, got, tt.want, "the shape of the reference")
			})
		}

		paid := &node.TypeRef{Spelling: "Status.PAID", Target: symbol.Identity{
			Lang: python.Lang, Package: modelsPkg, Owner: "Status", Name: "PAID", Kind: symbol.KindEnumVariant,
		}}
		red := &node.TypeRef{Spelling: "Color.RED", Target: symbol.Identity{
			Lang: python.Lang, Package: modelsPkg, Owner: "Color", Name: "RED", Kind: symbol.KindEnumVariant,
		}}
		nestedRed := &node.TypeRef{Spelling: "Outer.Color.RED", Target: symbol.Identity{
			Lang: python.Lang, Package: modelsPkg, Owner: "Outer.Color", Name: "RED", Kind: symbol.KindEnumVariant,
		}}
		nestedColor := symbol.Identity{
			Lang: python.Lang, Package: modelsPkg, Owner: "Outer", Name: "Color", Kind: symbol.KindEnum,
		}
		opaque := rules.TypeShape{Form: symbol.FormOpaque, Spelling: literalSpelling}
		literals := []struct {
			name string
			give []*node.TypeRef
			want rules.TypeShape
		}{
			{
				name: "folds a Literal of strings to text",
				give: []*node.TypeRef{{Spelling: `"a"`}, {Spelling: `'b'`}},
				want: rules.Leaf(symbol.FormText, literalSpelling),
			},
			{
				name: "folds a Literal of integers to a signed integer",
				give: []*node.TypeRef{{Spelling: "1"}, {Spelling: "-2"}},
				want: rules.Scalar(literalSpelling, rules.ScalarInt, 0),
			},
			{
				name: "folds a Literal of truth values to a bool",
				give: []*node.TypeRef{{Spelling: "True"}, {Spelling: "False"}},
				want: rules.Leaf(symbol.FormBool, literalSpelling),
			},
			{
				name: "folds a Literal of bytes to bytes",
				give: []*node.TypeRef{{Spelling: `b"a"`}},
				want: rules.Leaf(symbol.FormBytes, literalSpelling),
			},
			{
				name: "folds a Literal of the members of one enum to a reference to the enum",
				give: []*node.TypeRef{statusRef, paid},
				want: rules.Reference(literalSpelling, statusID),
			},
			{
				name: "folds a Literal of a member of a nested enum to a reference to the enum",
				give: []*node.TypeRef{nestedRed},
				want: rules.Reference(literalSpelling, nestedColor),
			},
			{
				name: "folds a Literal of None alone to the well-known empty value",
				give: []*node.TypeRef{{Spelling: "None"}},
				want: rules.Reference(literalSpelling, rules.WellKnownEmpty),
			},
			{
				name: "folds a Literal of strings and None to an optional of text",
				give: []*node.TypeRef{{Spelling: `"a"`}, {Spelling: "None"}},
				want: rules.TypeShape{
					Form: symbol.FormOptional, Spelling: literalSpelling,
					Elems: []rules.TypeShape{rules.Leaf(symbol.FormText, literalSpelling)},
				},
			},
			{
				name: "folds a Literal of two kinds to Opaque",
				give: []*node.TypeRef{{Spelling: `"a"`}, {Spelling: "1"}},
				want: opaque,
			},
			{
				name: "folds a Literal of the members of two enums to Opaque",
				give: []*node.TypeRef{statusRef, red},
				want: opaque,
			},
			{name: "folds a Literal of a float to Opaque", give: []*node.TypeRef{{Spelling: "1.5"}}, want: opaque},
			{
				name: "folds a Literal of a name without a target to Opaque",
				give: []*node.TypeRef{{Spelling: "Status.NEW"}},
				want: opaque,
			},
			{
				name: "folds a Literal of an expression to Opaque",
				give: []*node.TypeRef{{Spelling: "1 + 2"}},
				want: opaque,
			},
			{name: "folds a Literal with a nil value to Opaque", give: []*node.TypeRef{nil}, want: opaque},
			{name: "folds a Literal without values to Opaque", want: opaque},
		}
		for _, tt := range literals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ref := &node.TypeRef{Spelling: literalSpelling, Package: typingModule, Args: tt.give}
				assert.Equal(t, pythonrules.Rules{}.Builtin(ref, rules.View{}), tt.want, "the shape of the values")
			})
		}
	})

	t.Run("ShapeOf", func(t *testing.T) {
		t.Parallel()

		ref := &node.TypeRef{Spelling: sequenceSpelling}
		sequence := symbol.Identity{
			Lang:    python.Lang,
			Package: typingModule,
			Name:    sequenceSpelling,
			Kind:    symbol.KindStruct,
		}
		set := symbol.Identity{Lang: python.Lang, Package: builtinsModule, Name: "set", Kind: symbol.KindStruct}
		member := symbol.Identity{
			Lang: python.Lang, Package: typingModule, Owner: "Outer", Name: sequenceSpelling, Kind: symbol.KindStruct,
		}
		foreign := symbol.Identity{Lang: "go", Package: typingModule, Name: sequenceSpelling, Kind: symbol.KindStruct}
		tests := []struct {
			name      string
			give      symbol.Symbol
			ref       *node.TypeRef
			wantShape rules.TypeShape
			want      bool
		}{
			{
				name: "shapes a class of the table of typeshed",
				give: &node.Struct{ID: sequence},
				ref:  ref,
				want: true,
				wantShape: rules.TypeShape{
					Form: symbol.FormList, Spelling: sequenceSpelling, Mutability: symbol.MutabilityImmutable,
					Elems: anyChild,
				},
			},
			{
				name:      "shapes another class of builtins as Opaque",
				give:      &node.Struct{ID: set},
				ref:       &node.TypeRef{Spelling: "set"},
				want:      true,
				wantShape: rules.TypeShape{Form: symbol.FormOpaque, Spelling: "set"},
			},
			{name: "reports false for a class of the workspace", give: &node.Struct{ID: orderID}, ref: ref},
			{name: "reports false for a member of a class", give: &node.Struct{ID: member}, ref: ref},
			{name: "reports false for a class of another language", give: &node.Struct{ID: foreign}, ref: ref},
			{name: "reports false for a nil declaration", ref: ref},
			{name: "reports false for a nil reference", give: &node.Struct{ID: sequence}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, shaped := pythonrules.Rules{}.ShapeOf(tt.ref, tt.give, rules.View{})
				assert.Equal(t, shaped, tt.want, "whether the rules shape the declaration")
				assert.Equal(t, got, tt.wantShape, "the shape of the values")
			})
		}

		folds := []struct {
			name string
			give string
			want rules.TypeShape
		}{
			{
				name: "folds a reference to Sequence of typeshed to an immutable list",
				give: "weights",
				want: rules.TypeShape{
					Form: symbol.FormList, Spelling: sequenceSpelling, Mutability: symbol.MutabilityImmutable,
					Elems: []rules.TypeShape{rules.Scalar("float", rules.ScalarFloat, 64)},
				},
			},
			{
				name: "folds a reference to dict of typeshed to a mutable map",
				give: "counts",
				want: rules.TypeShape{
					Form:       symbol.FormMap,
					Spelling:   "dict",
					Mutability: symbol.MutabilityMutable,
					Elems: []rules.TypeShape{
						rules.Leaf(symbol.FormText, "str"),
						rules.Scalar("int", rules.ScalarInt, 0),
					},
				},
			},
			{
				name: "folds a reference to set of typeshed to Opaque",
				give: "seen",
				want: rules.TypeShape{Form: symbol.FormOpaque, Spelling: "set"},
			},
			{
				name: "folds a reference to datetime of typeshed to the well-known timestamp",
				give: "when",
				want: rules.Reference("datetime", rules.WellKnownTimestamp),
			},
			{
				name: "folds a reference to object of typeshed to the top type",
				give: "anything",
				want: rules.Leaf(symbol.FormDynamic, "object"),
			},
			{
				name: "folds a Literal of the members of an enum of the fixture to a reference to the enum",
				give: "favourite",
				want: rules.Reference(literalSpelling, statusID),
			},
		}
		for _, tt := range folds {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				field := f.member(t, catalogID, tt.give)
				got := rules.NewBound(pythonrules.New(), f.view, nil).TypeOf(field.Type)
				assert.Equal(t, got, tt.want, "the fold of the field "+tt.give)
			})
		}

		t.Run("folds a reference to Generator of typeshed to a stream of its first argument", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			fn, is := f.decl(t, produceID).(*node.Function)
			assert.True(t, is, "produce is a function")
			assert.Length(t, fn.Returns, 1, "produce has one result")
			want := rules.TypeShape{
				Form: symbol.FormStream, Spelling: generatorSpelling,
				Elems: []rules.TypeShape{rules.Leaf(symbol.FormText, "str")},
			}
			assert.Equal(t, rules.NewBound(pythonrules.New(), f.view, nil).TypeOf(fn.Returns[0].Type), want,
				"the stream of the values that the generator yields")
		})
	})

	t.Run("WellKnown", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			give   symbol.Identity
			want   symbol.Identity
			wantOK bool
		}{
			{
				name: "blesses datetime as the timestamp",
				give: symbol.Identity{
					Lang:    python.Lang,
					Package: datetimeModule,
					Name:    "datetime",
					Kind:    symbol.KindStruct,
				},
				want:   rules.WellKnownTimestamp,
				wantOK: true,
			},
			{
				name: "blesses timedelta as the duration",
				give: symbol.Identity{
					Lang:    python.Lang,
					Package: datetimeModule,
					Name:    "timedelta",
					Kind:    symbol.KindStruct,
				},
				want:   rules.WellKnownDuration,
				wantOK: true,
			},
			{
				name: "blesses Exception as the error",
				give: symbol.Identity{
					Lang:    python.Lang,
					Package: builtinsModule,
					Name:    "Exception",
					Kind:    symbol.KindStruct,
				},
				want:   rules.WellKnownError,
				wantOK: true,
			},
			{
				name: "blesses BaseException as the error",
				give: symbol.Identity{
					Lang: python.Lang, Package: builtinsModule, Name: "BaseException", Kind: symbol.KindStruct,
				},
				want:   rules.WellKnownError,
				wantOK: true,
			},
			{
				name: "reports false for another exception class",
				give: symbol.Identity{
					Lang:    python.Lang,
					Package: builtinsModule,
					Name:    "ValueError",
					Kind:    symbol.KindStruct,
				},
			},
			{
				name: "reports false for a member of a class",
				give: symbol.Identity{
					Lang: python.Lang, Package: datetimeModule, Owner: "datetime", Name: "now", Kind: symbol.KindMethod,
				},
			},
			{
				name: "reports false for a declaration of another language",
				give: symbol.Identity{Lang: "go", Package: datetimeModule, Name: "datetime", Kind: symbol.KindStruct},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, blessed := pythonrules.Rules{}.WellKnown(tt.give)
				expect.Equal(t, blessed, tt.wantOK, "whether the class is well-known")
				expect.Equal(t, got, tt.want, "the well-known identity")
			})
		}
	})
}

// TestBuiltinAllocs checks the ceilings of the methods of builtin.go in the
// ordinary run, which does not run benchmarks.
func TestBuiltinAllocs(t *testing.T) {
	checkAllocs(t, builtinCalls())
}

// BenchmarkBuiltin measures each method of builtin.go under its ceiling.
func BenchmarkBuiltin(b *testing.B) {
	benchCalls(b, builtinCalls())
}

// builtinCalls returns a call of every method of builtin.go, and of the
// costly cases of Builtin.
func builtinCalls() []allocCall {
	r := pythonrules.Rules{}
	str := &node.TypeRef{Spelling: "str"}
	optional := &node.TypeRef{Spelling: literalSpelling, Package: typingModule, Args: []*node.TypeRef{
		{Spelling: `"a"`}, {Spelling: "None"},
	}}
	generator := &node.TypeRef{Spelling: generatorSpelling, Package: typingModule, Args: []*node.TypeRef{
		{Spelling: "str"}, {Spelling: "None"}, {Spelling: "None"},
	}}
	sequence := &node.Struct{ID: symbol.Identity{
		Lang: python.Lang, Package: typingModule, Name: sequenceSpelling, Kind: symbol.KindStruct,
	}}
	ref := &node.TypeRef{Spelling: sequenceSpelling}
	timestamp := symbol.Identity{Lang: python.Lang, Package: datetimeModule, Name: "datetime", Kind: symbol.KindStruct}
	var (
		shape   rules.TypeShape
		shaped  bool
		blessed symbol.Identity
	)
	return []allocCall{
		{
			name: "Builtin", caseName: "a class of builtins", allocs: builtinAllocs,
			call:  func() { shape = r.Builtin(str, rules.View{}) },
			check: func(tb assert.TB) { assert.Equal(tb, shape.Form, symbol.FormText, "str folds to text") },
		},
		{
			name: "Builtin", caseName: "an optional Literal", allocs: optionalLiteralAllocs,
			call:  func() { shape = r.Builtin(optional, rules.View{}) },
			check: func(tb assert.TB) { assert.Equal(tb, shape.Form, symbol.FormOptional, "the Literal is optional") },
		},
		{
			name: "Builtin", caseName: "a Generator of three arguments", allocs: builtinAllocs,
			call:  func() { shape = r.Builtin(generator, rules.View{}) },
			check: func(tb assert.TB) { assert.Equal(tb, shape.Form, symbol.FormStream, "Generator is a stream") },
		},
		{
			name: "ShapeOf", allocs: builtinAllocs,
			call:  func() { shape, shaped = r.ShapeOf(ref, sequence, rules.View{}) },
			check: func(tb assert.TB) { assert.True(tb, shaped, "Sequence has a shape") },
		},
		{
			name: "WellKnown", allocs: builtinAllocs,
			call: func() { blessed, _ = r.WellKnown(timestamp) },
			check: func(tb assert.TB) {
				assert.Equal(tb, blessed, rules.WellKnownTimestamp, "datetime is the timestamp")
			},
		},
	}
}
