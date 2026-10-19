// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// paramName is the type parameter that the fold cases reference.
const paramName = "T"

// The spellings that the cases pass to the shape constructors.
const (
	int32Spelling = "int32"
	int32Bits     = 32
	timeSpelling  = "time.Time"
	unknownName   = "Unknown"
)

// referenceArgsAllocs is a reference with one type argument. It allocates
// the list of arguments that the shape keeps.
const referenceArgsAllocs = 1

// bytesLang classifies "byte" as the eight-bit unsigned scalar, as a
// language with that spelling does.
type bytesLang struct {
	rules.SourceRules
}

// Builtin classifies byte and passes every other spelling to the rules
// that it wraps.
func (b bytesLang) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	if ref.Spelling == "byte" {
		return rules.Scalar(ref.Spelling, rules.ScalarUint, 8)
	}
	return b.SourceRules.Builtin(ref, v)
}

// The library types of genericLang, each of which takes type arguments.
const (
	arraySpelling   = "Array"
	recordSpelling  = "Record"
	pairSpelling    = "Pair"
	fnSpelling      = "Fn"
	wrappedSpelling = "Wrapped"
)

// genericLang classifies library types that take type arguments, as the
// Builtin of TypeScript classifies Array and Record. It returns Array as a
// list, Record as a map, Pair as a tuple and Fn as a function, each
// without children, and Wrapped as an optional with a child of text. It
// passes every other spelling to the rules that it wraps.
type genericLang struct {
	rules.SourceRules
}

// Builtin classifies the library types and passes every other spelling
// on.
func (g genericLang) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	switch ref.Spelling {
	case arraySpelling:
		return rules.TypeShape{Form: symbol.FormList, Spelling: ref.Spelling}
	case recordSpelling:
		return rules.TypeShape{Form: symbol.FormMap, Spelling: ref.Spelling}
	case pairSpelling:
		return rules.TypeShape{Form: symbol.FormTuple, Spelling: ref.Spelling}
	case fnSpelling:
		return rules.TypeShape{Form: symbol.FormFunc, Spelling: ref.Spelling}
	case wrappedSpelling:
		return rules.TypeShape{
			Form: symbol.FormOptional, Spelling: ref.Spelling,
			Elems: []rules.TypeShape{rules.Leaf(symbol.FormText, strSpelling)},
		}
	default:
		return g.SourceRules.Builtin(ref, v)
	}
}

// counting counts the calls of the Builtin of the language.
type counting struct {
	rules.SourceRules
	asked int
}

// Builtin counts the call and returns the shape of the wrapped rules.
func (c *counting) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	c.asked++
	return c.SourceRules.Builtin(ref, v)
}

// blessing is the scripted rules of a language that blesses the
// declaration rowName as the well-known timestamp.
type blessing struct{ rules.SourceRules }

var _ rules.WellKnownRules = blessing{}

// WellKnown blesses every declaration named rowName as the timestamp.
func (blessing) WellKnown(id symbol.Identity) (symbol.Identity, bool) {
	if id.Name != rowName {
		return symbol.Identity{}, false
	}
	return rules.WellKnownTimestamp, true
}

// shaping is the scripted rules of a language that gives the values of the
// struct rowName the shape of a signed integer of 64 bits, as the GraphQL
// rules give a custom scalar of a known specification the shape of its
// specification.
type shaping struct{ rules.SourceRules }

var _ rules.ShapeRules = shaping{}

// ShapeOf gives the struct rowName the shape of a signed integer of 64 bits,
// and gives every other declaration none.
func (shaping) ShapeOf(decl symbol.Symbol, _ rules.View) (rules.TypeShape, bool) {
	if s, is := decl.(*node.Struct); !is || s.Name != rowName {
		return rules.TypeShape{}, false
	}
	return rules.Scalar(int64Spelling, rules.ScalarInt, int64Bits), true
}

// TestShape checks the fold of the kernel. The kernel folds every
// structural form with its children and folds a type parameter to opaque.
// It classifies a resolved name by its declaration, by the well-known
// identity that the rules of its language bless it as, or by the shape that
// those rules give its values. It passes an unresolved name to the Builtin
// of the language, which returns through the constructors.
func TestShape(t *testing.T) {
	t.Parallel()

	t.Run("TypeOf", func(t *testing.T) {
		t.Parallel()

		t.Run("folds a target that its rules bless to the well-known identity", func(t *testing.T) {
			t.Parallel()

			v, _, _ := viewOver(t, coretest.Frozen(t, hierarchy()))
			b := rules.NewBound(blessing{scripted()}, v, nil)
			s := b.TypeOf(named(svcPath, rowName, symbol.KindStruct))
			assert.Equal(t, s.Form, symbol.FormReference, "a blessed target folds to a reference")
			assert.Equal(t, s.Ref, rules.WellKnownTimestamp, "to the well-known identity")
		})

		t.Run("folds a target that its rules shape to the shape that they give it", func(t *testing.T) {
			t.Parallel()

			v, _, _ := viewOver(t, coretest.Frozen(t, hierarchy()))
			b := rules.NewBound(shaping{scripted()}, v, nil)
			assert.Equal(t, b.TypeOf(named(svcPath, rowName, symbol.KindStruct)),
				rules.Scalar(rowName, rules.ScalarInt, int64Bits),
				"the shape that the rules give the values of Row, with the spelling of the reference")
		})

		t.Run("folds a target that its rules do not shape to a reference", func(t *testing.T) {
			t.Parallel()

			v, _, _ := viewOver(t, coretest.Frozen(t, hierarchy()))
			b := rules.NewBound(shaping{scripted()}, v, nil)
			assert.Equal(t, b.TypeOf(named(svcPath, baseName, symbol.KindStruct)),
				rules.Reference(baseName, coretest.ID(svcPath, baseName, symbol.KindStruct)),
				"a target without a shape of its rules folds to a reference to the declaration")
		})

		t.Run("folds every structural form with its children", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			optional := &node.TypeRef{
				Spelling: "*Row", Form: symbol.FormOptional,
				Elems: []*node.TypeRef{named(svcPath, rowName, symbol.KindStruct)},
			}
			s := b.TypeOf(optional)
			assert.Equal(t, s.Form, symbol.FormOptional, "the form is kept")
			assert.Equal(t, s.Spelling, "*Row", "with the spelling")
			assert.Length(t, s.Elems, 1, "and its child")
			assert.Equal(t, s.Elems[0].Form, symbol.FormReference, "folded to a reference")
			assert.Equal(t, s.Elems[0].Ref, coretest.ID(svcPath, rowName, symbol.KindStruct), "to the declaration")

			mapped := &node.TypeRef{
				Spelling: "map[string]int", Form: symbol.FormMap,
				Elems: []*node.TypeRef{builtin(strSpelling), builtin(intSpelling)},
			}
			m := b.TypeOf(mapped)
			assert.Equal(t, m.Elems[0].Form, symbol.FormText, "the key of a map folds")
			assert.Equal(t, m.Elems[1].Form, symbol.FormScalar, "and its value")

			fn := &node.TypeRef{
				Spelling: "func(int) string", Form: symbol.FormFunc, Split: 1,
				Elems: []*node.TypeRef{builtin(intSpelling), builtin(strSpelling)},
			}
			f := b.TypeOf(fn)
			assert.Equal(t, f.Split, 1, "a function type keeps its split")
			arr := &node.TypeRef{
				Spelling: "[4]int", Form: symbol.FormArray, Length: 4,
				Elems: []*node.TypeRef{builtin(intSpelling)},
			}
			assert.Equal(t, b.TypeOf(arr).Length, 4, "an array keeps its length")
			wild := &node.TypeRef{
				Spelling: "? extends Row", Form: symbol.FormWildcard,
				Variance: symbol.VarianceOut, Elems: []*node.TypeRef{named(svcPath, rowName, symbol.KindStruct)},
			}
			assert.Equal(t, b.TypeOf(wild).Variance, symbol.VarianceOut, "a wildcard keeps its variance")
			inline := b.TypeOf(&node.TypeRef{Spelling: "struct{}", Form: symbol.FormInline})
			assert.Equal(t, inline.Form, symbol.FormInline, "the language leaves the inline body inline")
			assert.Empty(t, inline.Elems, "an inline body has no children")
		})

		t.Run("returns the shape that the language gives an inline reference", func(t *testing.T) {
			t.Parallel()

			v, _, _ := viewOver(t, coretest.Frozen(t, hierarchy()))
			b := rules.NewBound(presenceLang{scripted()}, v, nil)
			got := b.TypeOf(&node.TypeRef{Spelling: dynamicSpelling, Form: symbol.FormInline})
			assert.Equal(t, got, rules.Leaf(symbol.FormDynamic, dynamicSpelling),
				"the language folds the inline body to the top type")
		})

		t.Run("keeps the asynchrony of a stream", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			async := &node.TypeRef{
				Spelling: "stream Row", Form: symbol.FormStream, Async: true,
				Elems: []*node.TypeRef{named(svcPath, rowName, symbol.KindStruct)},
			}
			s := b.TypeOf(async)
			assert.True(t, s.Async, "an asynchronous stream folds to an asynchronous stream")
			assert.Equal(t, s.Elems[0].Ref, coretest.ID(svcPath, rowName, symbol.KindStruct), "of its element")
		})

		mutabilities := []struct {
			name string
			form symbol.TypeForm
			give symbol.Mutability
		}{
			{name: "keeps the immutability of a list", form: symbol.FormList, give: symbol.MutabilityImmutable},
			{name: "keeps the mutability of a borrow", form: symbol.FormBorrow, give: symbol.MutabilityMutable},
		}
		for _, tt := range mutabilities {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
				ref := &node.TypeRef{
					Spelling: "T", Form: tt.form, Mutability: tt.give, Elems: []*node.TypeRef{builtin(intSpelling)},
				}
				assert.Equal(t, b.TypeOf(ref).Mutability, tt.give,
					"the shape keeps whether a holder can write through the reference")
			})
		}

		t.Run("folds a stream without the mark to a synchronous stream", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			channel := &node.TypeRef{
				Spelling: "chan int", Form: symbol.FormStream, Elems: []*node.TypeRef{builtin(intSpelling)},
			}
			assert.False(t, b.TypeOf(channel).Async, "a channel is a synchronous stream")
		})

		t.Run("folds a list of eight-bit unsigned scalars to bytes", func(t *testing.T) {
			t.Parallel()

			b := rules.NewBound(bytesLang{scripted()}, viewOnly(t), nil)
			list := &node.TypeRef{
				Spelling: "[]byte", Form: symbol.FormList,
				Elems: []*node.TypeRef{builtin("byte")},
			}
			s := b.TypeOf(list)
			assert.Equal(t, s.Form, symbol.FormBytes, "the one rule beyond structure")
			assert.Equal(t, s.Spelling, "[]byte", "keeps the spelling")
			assert.Empty(t, s.Elems, "and drops the child")
			ints := &node.TypeRef{
				Spelling: "[]int", Form: symbol.FormList,
				Elems: []*node.TypeRef{builtin(intSpelling)},
			}
			assert.Equal(t, b.TypeOf(ints).Form, symbol.FormList, "a list of any other element folds to a list")
		})

		t.Run("classifies a resolved name by its declaration", func(t *testing.T) {
			t.Parallel()

			sum := coretest.Sum(depPath, "Shape", "Circle")
			g := coretest.Frozen(t, hierarchy(), coretest.Package(depPath, sum))
			b, _, _ := boundOver(t, g)
			ref := named(svcPath, rowName, symbol.KindStruct)
			ref.Args = []*node.TypeRef{builtin(intSpelling)}
			s := b.TypeOf(ref)
			assert.Equal(t, s.Form, symbol.FormReference, "a struct is a reference")
			assert.Length(t, s.Args, 1, "with its arguments folded")
			assert.Equal(t, s.Args[0].Form, symbol.FormScalar, "in turn")
			assert.Equal(t, b.TypeOf(named(depPath, "Shape", symbol.KindSum)).Form, symbol.FormSum,
				"and a sum is a sum")
		})

		t.Run("folds a type parameter to opaque without a read", func(t *testing.T) {
			t.Parallel()

			b, reads, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			s := b.TypeOf(named(svcPath, paramName, symbol.KindTypeParam))
			assert.Equal(t, s.Form, symbol.FormOpaque,
				"a use of a parameter states no argument, so no read decides its shape")
			assert.Equal(t, s.Spelling, paramName, "the leaf keeps the spelling of the parameter")
			assert.Empty(t, slices.Collect(reads.Identities()), "the kind of the target alone decides")
		})

		t.Run("sends an unresolved name to the language", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			assert.Equal(t, b.TypeOf(builtin(intSpelling)).Form, symbol.FormScalar, "a builtin classifies")
			assert.Equal(t, b.TypeOf(builtin(unknownName)).Form, symbol.FormOpaque, "an unknown spelling is opaque")
			assert.Equal(t, b.TypeOf(builtin(unknownName)).Spelling, unknownName, "with its spelling")
		})

		generics := []struct {
			name     string
			give     *node.TypeRef
			form     symbol.TypeForm
			children []symbol.TypeForm
		}{
			{
				name: "folds the type argument of a builtin list into its child",
				give: &node.TypeRef{Spelling: arraySpelling, Args: []*node.TypeRef{builtin(intSpelling)}},
				form: symbol.FormList, children: []symbol.TypeForm{symbol.FormScalar},
			},
			{
				name: "folds the type arguments of a builtin map into its key and its value",
				give: &node.TypeRef{
					Spelling: recordSpelling, Args: []*node.TypeRef{builtin(strSpelling), builtin(intSpelling)},
				},
				form: symbol.FormMap, children: []symbol.TypeForm{symbol.FormText, symbol.FormScalar},
			},
			{
				name: "folds every type argument of a builtin tuple into a member",
				give: &node.TypeRef{
					Spelling: pairSpelling,
					Args:     []*node.TypeRef{builtin(intSpelling), builtin(strSpelling), builtin(boolSpelling)},
				},
				form:     symbol.FormTuple,
				children: []symbol.TypeForm{symbol.FormScalar, symbol.FormText, symbol.FormBool},
			},
			{
				name: "folds a builtin list of the eight-bit unsigned scalar to bytes",
				give: &node.TypeRef{Spelling: arraySpelling, Args: []*node.TypeRef{builtin("byte")}},
				form: symbol.FormBytes,
			},
			{
				name: "folds a builtin list with two type arguments to opaque",
				give: &node.TypeRef{
					Spelling: arraySpelling, Args: []*node.TypeRef{builtin(intSpelling), builtin(intSpelling)},
				},
				form: symbol.FormOpaque,
			},
			{
				name: "folds a builtin map with one type argument to opaque",
				give: &node.TypeRef{Spelling: recordSpelling, Args: []*node.TypeRef{builtin(strSpelling)}},
				form: symbol.FormOpaque,
			},
			{
				name: "folds a builtin function with a type argument to opaque",
				give: &node.TypeRef{Spelling: fnSpelling, Args: []*node.TypeRef{builtin(intSpelling)}},
				form: symbol.FormOpaque,
			},
			{
				name: "keeps the children of a builtin shape that has them",
				give: &node.TypeRef{Spelling: wrappedSpelling, Args: []*node.TypeRef{builtin(intSpelling)}},
				form: symbol.FormOptional, children: []symbol.TypeForm{symbol.FormText},
			},
			{
				name: "keeps a builtin leaf for a reference with type arguments",
				give: &node.TypeRef{Spelling: intSpelling, Args: []*node.TypeRef{builtin(strSpelling)}},
				form: symbol.FormScalar,
			},
			{
				name: "returns the builtin shape of a reference without type arguments",
				give: builtin(arraySpelling),
				form: symbol.FormList,
			},
		}
		for _, tt := range generics {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				b := rules.NewBound(genericLang{bytesLang{scripted()}}, viewOnly(t), nil)
				s := b.TypeOf(tt.give)
				var children []symbol.TypeForm
				for _, child := range s.Elems {
					children = append(children, child.Form)
				}
				expect.Equal(t, s.Form, tt.form,
					"the form is the form of the language, or opaque for a count of arguments that it does not take")
				expect.Equal(t, children, tt.children, "the children are the folded arguments")
				expect.Equal(t, s.Spelling, tt.give.Spelling, "the shape keeps the spelling of the reference")
			})
		}

		t.Run("folds a nil reference to opaque", func(t *testing.T) {
			t.Parallel()

			b, _, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			assert.Equal(t, b.TypeOf(nil), rules.Opaque(nil), "nil is opaque without a spelling")
		})

		t.Run("treats a target outside the scope as opaque", func(t *testing.T) {
			t.Parallel()

			g := coretest.Frozen(t, hierarchy(), coretest.Package(depPath, coretest.Struct(depPath, "Far")))
			reads := store.NewReadSet()
			reader, err := g.Reader(reads, func(pkg symbol.Identity) bool { return pkg.Package == svcPath })
			assert.NoError(t, err, "the graph returns a scoped reader")
			b := rules.NewBound(scripted(), rules.NewView(reader, nil, reads, meta.KernelKeys{}), nil)
			s := b.TypeOf(named(depPath, "Far", symbol.KindStruct))
			assert.Equal(t, s.Form, symbol.FormOpaque, "a target that the plan cannot read folds to opaque")
		})

		t.Run("memoises per reference", func(t *testing.T) {
			t.Parallel()

			b, reads, _ := boundOver(t, coretest.Frozen(t, hierarchy()))
			assert.Deterministic(t, func(ref *node.TypeRef) (rules.TypeShape, error) { return b.TypeOf(ref), nil },
				named(svcPath, rowName, symbol.KindStruct), "one reference folds once")
			assert.Length(t, slices.Collect(reads.Identities()), 1, "and the read records once")
		})

		t.Run("folds a reference once per binding", func(t *testing.T) {
			t.Parallel()

			c := &counting{SourceRules: scripted()}
			b := rules.NewBound(c, viewOnly(t), nil)
			ref := builtin(intSpelling)
			first := b.TypeOf(ref)
			second := b.TypeOf(ref)
			assert.Equal(t, c.asked, 1, "the language is asked once")
			assert.Equal(t, first.Form, second.Form, "and the memo returns the same shape")
			other := builtin(intSpelling)
			b.TypeOf(other)
			assert.Equal(t, c.asked, 2, "a distinct reference folds on its own")
			assert.Nil(t, first.Args, "a reference without arguments folds to a shape without any")
		})
	})

	t.Run("Opaque", func(t *testing.T) {
		t.Parallel()

		t.Run("returns FormOpaque with the spelling of the reference", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rules.Opaque(builtin(unknownName)),
				rules.TypeShape{Form: symbol.FormOpaque, Spelling: unknownName}, "the spelling is kept")
		})

		t.Run("returns FormOpaque without a spelling for a nil reference", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rules.Opaque(nil), rules.TypeShape{Form: symbol.FormOpaque},
				"a nil reference has no spelling")
		})
	})

	t.Run("Scalar", func(t *testing.T) {
		t.Parallel()

		t.Run("returns FormScalar of a class at a width", func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				rules.Scalar(int32Spelling, rules.ScalarInt, int32Bits),
				rules.TypeShape{
					Form:     symbol.FormScalar,
					Spelling: int32Spelling,
					Class:    rules.ScalarInt,
					Bits:     int32Bits,
				},
				"a 32-bit signed integer",
			)
		})
	})

	t.Run("Leaf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a shape of the form without children", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rules.Leaf(symbol.FormBool, boolSpelling),
				rules.TypeShape{Form: symbol.FormBool, Spelling: boolSpelling}, "a boolean leaf")
		})
	})

	t.Run("Reference", func(t *testing.T) {
		t.Parallel()

		t.Run("returns FormReference to an identity", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rules.Reference(timeSpelling, rules.WellKnownTimestamp),
				rules.TypeShape{Form: symbol.FormReference, Spelling: timeSpelling, Ref: rules.WellKnownTimestamp},
				"a reference without arguments")
		})

		t.Run("returns the type arguments in order", func(t *testing.T) {
			t.Parallel()

			first, second := rules.Leaf(symbol.FormText, strSpelling), rules.Scalar(intSpelling, rules.ScalarInt, 0)
			row := coretest.ID(svcPath, rowName, symbol.KindStruct)
			assert.Equal(t, rules.Reference(rowName, row, first, second).Args, []rules.TypeShape{first, second},
				"the arguments follow the reference")
		})
	})

	t.Run("IsWellKnown", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Identity
			want bool
		}{
			{name: "reports true for WellKnownTimestamp", give: rules.WellKnownTimestamp, want: true},
			{name: "reports true for WellKnownDuration", give: rules.WellKnownDuration, want: true},
			{name: "reports true for WellKnownEmpty", give: rules.WellKnownEmpty, want: true},
			{name: "reports true for WellKnownError", give: rules.WellKnownError, want: true},
			{
				name: "reports false for a declaration",
				give: coretest.ID(svcPath, rowName, symbol.KindStruct),
				want: false,
			},
			{name: "reports false for the zero identity", give: symbol.Identity{}, want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(
					t,
					rules.IsWellKnown(tt.give),
					tt.want,
					"IsWellKnown reports whether the registry contains the identity",
				)
			})
		}
	})

	t.Run("ScalarClass.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rules.ScalarClass
			want string
		}{
			{name: "returns int for ScalarInt", give: rules.ScalarInt, want: "int"},
			{name: "returns uint for ScalarUint", give: rules.ScalarUint, want: "uint"},
			{name: "returns float for ScalarFloat", give: rules.ScalarFloat, want: "float"},
			{name: "returns the number of the zero class", give: rules.ScalarClass(0), want: "0"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})
}

// TestShapeAllocs checks in the ordinary run that each constructor
// returns its shape by value and allocates only the list of type
// arguments that a reference keeps. The test does not run in parallel,
// because the count includes the allocations of every goroutine.
func TestShapeAllocs(t *testing.T) {
	ref, arg := builtin(unknownName), rules.Scalar(intSpelling, rules.ScalarInt, 0)
	class, id := rules.ScalarFloat, coretest.ID(svcPath, rowName, symbol.KindStruct)
	var shape rules.TypeShape
	assert.MaxAllocs(t, func() { shape = rules.Opaque(ref) }, 0, "Opaque allocates nothing")
	assert.Equal(t, shape.Spelling, unknownName, "Opaque keeps the spelling")
	assert.MaxAllocs(t, func() { shape = rules.Scalar(int32Spelling, rules.ScalarInt, int32Bits) }, 0,
		"Scalar allocates nothing")
	assert.Equal(t, shape.Bits, int32Bits, "Scalar keeps the width")
	assert.MaxAllocs(t, func() { shape = rules.Leaf(symbol.FormBool, boolSpelling) }, 0, "Leaf allocates nothing")
	assert.Equal(t, shape.Form, symbol.FormBool, "Leaf keeps the form")
	assert.MaxAllocs(t, func() { shape = rules.Reference(timeSpelling, rules.WellKnownTimestamp) }, 0,
		"Reference allocates nothing without arguments")
	assert.Equal(t, shape.Ref, rules.WellKnownTimestamp, "Reference keeps the identity")
	assert.MaxAllocs(t, func() { shape = rules.Reference(rowName, id, arg) }, referenceArgsAllocs,
		"Reference allocates the list of its arguments")
	assert.Length(t, shape.Args, 1, "Reference keeps the argument")

	var got bool
	assert.MaxAllocs(t, func() { got = rules.IsWellKnown(rules.WellKnownDuration) }, 0,
		"IsWellKnown allocates nothing")
	assert.True(t, got, "IsWellKnown reports true for WellKnownDuration")
	var spelled string
	assert.MaxAllocs(t, func() { spelled = class.String() }, 0,
		"ScalarClass.String allocates nothing for a declared class")
	assert.Equal(t, spelled, "float", "ScalarClass.String spells ScalarFloat")
}

// BenchmarkShape measures each constructor through which the Builtin of a
// language returns, the registry check, and the spelling of a scalar
// class.
func BenchmarkShape(b *testing.B) {
	b.Run("Opaque", func(b *testing.B) {
		ref := builtin(unknownName)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got rules.TypeShape
		for c.Loop() {
			got = rules.Opaque(ref)
		}
		assert.Equal(b, got.Spelling, unknownName, "Opaque keeps the spelling")
	})

	b.Run("Scalar", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got rules.TypeShape
		for c.Loop() {
			got = rules.Scalar(int32Spelling, rules.ScalarInt, int32Bits)
		}
		assert.Equal(b, got.Bits, int32Bits, "Scalar keeps the width")
	})

	b.Run("Leaf", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got rules.TypeShape
		for c.Loop() {
			got = rules.Leaf(symbol.FormBool, boolSpelling)
		}
		assert.Equal(b, got.Form, symbol.FormBool, "Leaf keeps the form")
	})

	b.Run("Reference", func(b *testing.B) {
		b.Run("without a type argument", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got rules.TypeShape
			for c.Loop() {
				got = rules.Reference(timeSpelling, rules.WellKnownTimestamp)
			}
			assert.Equal(b, got.Ref, rules.WellKnownTimestamp, "Reference keeps the identity")
		})

		b.Run("with a type argument", func(b *testing.B) {
			id, arg := coretest.ID(svcPath, rowName, symbol.KindStruct), rules.Scalar(intSpelling, rules.ScalarInt, 0)
			c := bench.Start(b).MaxAllocs(referenceArgsAllocs)
			defer c.End()
			var got rules.TypeShape
			for c.Loop() {
				got = rules.Reference(rowName, id, arg)
			}
			assert.Length(b, got.Args, 1, "Reference keeps the argument")
		})
	})

	b.Run("IsWellKnown", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		got := false
		for c.Loop() {
			got = rules.IsWellKnown(rules.WellKnownDuration)
		}
		assert.True(b, got, "IsWellKnown reports true for WellKnownDuration")
	})

	b.Run("ScalarClass.String", func(b *testing.B) {
		class := rules.ScalarFloat
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = class.String()
		}
		assert.Equal(b, got, "float", "ScalarClass.String spells ScalarFloat")
	})
}
