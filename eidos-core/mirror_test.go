// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// mirrorLang is the language the fixture's identities are in.
const mirrorLang symbol.Lang = "golang"

// The host the mirror cases mirror a method onto, and the declarations
// the method's signature names.
const (
	mirrorPackage = "svc/store"
	mirrorHost    = "Store"
	fetchMethod   = "fetch"
	getMethod     = "Get"
	keysParam     = "keys"
	keysLabel     = "for"
	keysDefault   = "nil"
	keyParam      = "K"
	keyBound      = "comparable"
	keysSpelling  = "[]K"
	foundResult   = "found"
	intType       = "int"
	notFound      = "NotFound"
	userMap       = "Map[string, User]"
	stringType    = "string"
	userType      = "User"
	sizeParam     = "N"
	sizeDefault   = "8"
)

// The allocations of a mirror.
const (
	// mirrorAllocs is the mirror of a method with one parameter and one
	// return: the method, its receiving type, and for the parameter and
	// the return each the list, the view and the type.
	mirrorAllocs = 2 + 2*3
	// bareMirrorAllocs is the mirror of a method without a signature: the
	// method and its receiving type.
	bareMirrorAllocs = 2
	// typeParamAllocs is the lift of one type parameter with one bound:
	// the list, the parameter, the list of its bounds and the bound.
	typeParamAllocs = 4
	// typeArgsAllocs is the arguments of two type parameters: the list
	// and each argument.
	typeArgsAllocs = 1 + 2
)

// TestMirror pins that Mirror copies every fact of a signature and the
// spellings of its types verbatim, and that it leaves the facts of an
// implementation and the receiver to the target.
func TestMirror(t *testing.T) {
	t.Parallel()

	t.Run("Mirror", func(t *testing.T) {
		t.Parallel()

		t.Run("copies every signature field", func(t *testing.T) {
			t.Parallel()

			m := &node.Method{
				ID:         methodID(fetchMethod),
				Name:       fetchMethod,
				Visibility: symbol.VisibilityPackage,
				Level:      symbol.LevelType,
				Async:      true,
				Accessor:   symbol.AccessorGet,
				Indexer:    true,
				Constructs: true,
				Hard:       true,
				Optional:   true,
				Mutability: symbol.MutabilityImmutable,
				Computed:   true,
				TypeParams: []*node.TypeParam{{
					Name:     keyParam,
					Variance: symbol.VarianceOut,
					Bounds:   []*node.TypeRef{{Spelling: keyBound}},
				}},
				Params: []*node.Param{{
					Name:     keysParam,
					Label:    keysLabel,
					Default:  keysDefault,
					Optional: true,
					Variadic: symbol.VariadicPositional,
					Passing:  symbol.PassingKeyword,
					Type: &node.TypeRef{
						Spelling: keysSpelling,
						Form:     symbol.FormList,
						Elems:    []*node.TypeRef{{Spelling: keyParam}},
					},
				}},
				Returns: []*node.Return{{Name: foundResult, Type: &node.TypeRef{Spelling: intType}}},
				Throws:  []*node.TypeRef{{Spelling: notFound}},
			}
			assert.Equal(t, eidos.Mirror(mirrorHost, m), &emit.Method{
				Origin:     m.ID,
				Name:       fetchMethod,
				Visibility: symbol.VisibilityPackage,
				Level:      symbol.LevelType,
				Async:      true,
				Accessor:   symbol.AccessorGet,
				Indexer:    true,
				Constructs: true,
				Hard:       true,
				Optional:   true,
				Mutability: symbol.MutabilityImmutable,
				Computed:   true,
				Receives:   &emit.TypeRef{Spelling: mirrorHost},
				TypeParams: []*emit.TypeParam{{
					Name:     keyParam,
					Variance: symbol.VarianceOut,
					Bounds:   []*emit.TypeRef{{Spelling: keyBound}},
				}},
				Params: emit.SlotOf(&emit.Param{
					Name:     keysParam,
					Label:    keysLabel,
					Default:  keysDefault,
					Optional: true,
					Variadic: symbol.VariadicPositional,
					Passing:  symbol.PassingKeyword,
					Type: &emit.TypeRef{
						Spelling: keysSpelling,
						Form:     symbol.FormList,
						Elems:    []*emit.TypeRef{{Spelling: keyParam}},
					},
				}),
				Returns: emit.SlotOf(&emit.Return{Name: foundResult, Type: &emit.TypeRef{Spelling: intType}}),
				Throws:  []*emit.TypeRef{{Spelling: notFound}},
			}, "every signature field copies over, and Receives names the host")
		})

		t.Run("copies an instantiation's spelling with its argument tree", func(t *testing.T) {
			t.Parallel()

			m := &node.Method{
				Name: getMethod,
				Returns: []*node.Return{{
					Type: &node.TypeRef{
						Spelling: userMap,
						Args:     []*node.TypeRef{{Spelling: stringType}, {Spelling: userType}},
					},
				}},
			}
			got := eidos.Mirror(mirrorHost, m)
			assert.Length(t, got.Returns.Items(), 1, "the results mirror")
			assert.Equal(t, got.Returns.Items()[0].Type,
				&emit.TypeRef{
					Spelling: userMap,
					Args:     []*emit.TypeRef{{Spelling: stringType}, {Spelling: userType}},
				}, "the instantiation's spelling and its arguments copy over as a tree")
		})

		t.Run("leaves the receiver to the target", func(t *testing.T) {
			t.Parallel()

			got := eidos.Mirror(mirrorHost, &node.Method{Name: getMethod})
			assert.Nil(t, got.Receiver, "a pointer receiver is Go's spelling, and Rust spells self")
		})

		t.Run("leaves out the facts of an implementation", func(t *testing.T) {
			t.Parallel()

			got := eidos.Mirror(mirrorHost, &node.Method{
				Name: getMethod, Abstract: true, Final: true, Override: true, HasDefault: true, Overload: true,
			})
			expect.False(t, got.Abstract, "the generator writes the implementation of the mirror")
			expect.False(t, got.Final, "the mirror states no final")
			expect.False(t, got.Override, "the mirror overrides nothing")
			expect.False(t, got.HasDefault, "the mirror states no default body")
			expect.False(t, got.Overload, "the generator writes a body for the mirror")
		})
	})

	t.Run("MirrorTypeParams", func(t *testing.T) {
		t.Parallel()

		t.Run("copies each type parameter whole", func(t *testing.T) {
			t.Parallel()

			tps := []*node.TypeParam{
				{
					Name:     keyParam,
					Variance: symbol.VarianceOut,
					Bounds:   []*node.TypeRef{{Spelling: keyBound}},
					Default:  &node.TypeRef{Spelling: stringType},
				},
				{Name: sizeParam, Const: true, Type: &node.TypeRef{Spelling: intType}, DefaultValue: sizeDefault},
			}
			assert.Equal(t, eidos.MirrorTypeParams(tps), []*emit.TypeParam{
				{
					Name:     keyParam,
					Variance: symbol.VarianceOut,
					Bounds:   []*emit.TypeRef{{Spelling: keyBound}},
					Default:  &emit.TypeRef{Spelling: stringType},
				},
				{Name: sizeParam, Const: true, Type: &emit.TypeRef{Spelling: intType}, DefaultValue: sizeDefault},
			}, "each parameter copies over with its references restated")
		})

		t.Run("returns nil for no type parameters", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, eidos.MirrorTypeParams(nil), "a declaration without type parameters lifts none")
		})
	})

	t.Run("TypeArgs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an argument named after each type parameter", func(t *testing.T) {
			t.Parallel()

			tps := []*emit.TypeParam{
				{Name: keyParam, Bounds: []*emit.TypeRef{{Spelling: keyBound}}},
				{Name: sizeParam, Const: true},
			}
			assert.Equal(t, eidos.TypeArgs(tps), []*emit.TypeRef{{Spelling: keyParam}, {Spelling: sizeParam}},
				"each argument has the name of its parameter and no target")
		})

		t.Run("returns nil for no type parameters", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, eidos.TypeArgs(nil), "a declaration without type parameters has no arguments")
		})
	})
}

// A mirror allocates the method it returns and its parts in the
// ordinary run, which runs no benchmark. The check runs alone, because
// the count includes every goroutine's allocations.
func TestMirrorAllocs(t *testing.T) {
	signed, bare := getter(), &node.Method{ID: methodID(getMethod), Name: getMethod}
	var got *emit.Method
	assert.MaxAllocs(t, func() { got = eidos.Mirror(mirrorHost, signed) }, mirrorAllocs,
		"Mirror allocates the method and the parts of its signature")
	assert.Length(t, got.Params.Items(), 1, "Mirror returns the parameter")
	assert.MaxAllocs(t, func() { got = eidos.Mirror(mirrorHost, bare) }, bareMirrorAllocs,
		"Mirror allocates the method and its receiving type for a method without a signature")
	assert.Equal(t, got.Name, getMethod, "Mirror returns the method's name")
	tps := []*node.TypeParam{{Name: keyParam, Bounds: []*node.TypeRef{{Spelling: keyBound}}}}
	var lifted []*emit.TypeParam
	assert.MaxAllocs(t, func() { lifted = eidos.MirrorTypeParams(tps) }, typeParamAllocs,
		"MirrorTypeParams allocates the list, the parameter, the list of its bounds and the bound")
	assert.Length(t, lifted, 1, "MirrorTypeParams returns the parameter")
	params := []*emit.TypeParam{{Name: keyParam}, {Name: sizeParam}}
	var args []*emit.TypeRef
	assert.MaxAllocs(t, func() { args = eidos.TypeArgs(params) }, typeArgsAllocs,
		"TypeArgs allocates the list and each argument")
	assert.Length(t, args, 2, "TypeArgs returns an argument for each parameter")
}

// BenchmarkMirror measures the mirror of a getter of one parameter and
// one return, and of a method without a signature.
func BenchmarkMirror(b *testing.B) {
	b.Run("Mirror", func(b *testing.B) {
		b.Run("a method with a signature", func(b *testing.B) {
			m := getter()
			c := bench.Start(b).MaxAllocs(mirrorAllocs)
			defer c.End()
			var got *emit.Method
			for c.Loop() {
				got = eidos.Mirror(mirrorHost, m)
			}
			assert.Length(b, got.Returns.Items(), 1, "Mirror returns the return")
		})

		b.Run("a method without a signature", func(b *testing.B) {
			m := &node.Method{ID: methodID(getMethod), Name: getMethod}
			c := bench.Start(b).MaxAllocs(bareMirrorAllocs)
			defer c.End()
			var got *emit.Method
			for c.Loop() {
				got = eidos.Mirror(mirrorHost, m)
			}
			assert.Equal(b, got.Receives.Spelling, mirrorHost, "Mirror names the host")
		})
	})

	b.Run("MirrorTypeParams", func(b *testing.B) {
		b.Run("a parameter with a bound", func(b *testing.B) {
			tps := []*node.TypeParam{{Name: keyParam, Bounds: []*node.TypeRef{{Spelling: keyBound}}}}
			c := bench.Start(b).MaxAllocs(typeParamAllocs)
			defer c.End()
			var got []*emit.TypeParam
			for c.Loop() {
				got = eidos.MirrorTypeParams(tps)
			}
			assert.Length(b, got, 1, "MirrorTypeParams returns the parameter")
		})
	})

	b.Run("TypeArgs", func(b *testing.B) {
		b.Run("two parameters", func(b *testing.B) {
			tps := []*emit.TypeParam{{Name: keyParam}, {Name: sizeParam}}
			c := bench.Start(b).MaxAllocs(typeArgsAllocs)
			defer c.End()
			var got []*emit.TypeRef
			for c.Loop() {
				got = eidos.TypeArgs(tps)
			}
			assert.Length(b, got, 2, "TypeArgs returns an argument for each parameter")
		})
	})
}

// methodID returns a fixture method identity.
func methodID(name string) symbol.Identity {
	return symbol.Identity{
		Lang: mirrorLang, Package: mirrorPackage, Name: name,
		Kind: symbol.KindMethod,
	}
}

// getter returns a method of one string parameter and one int return.
func getter() *node.Method {
	return &node.Method{
		ID: methodID(getMethod), Name: getMethod,
		Params:  []*node.Param{{Name: keysParam, Type: &node.TypeRef{Spelling: stringType}}},
		Returns: []*node.Return{{Type: &node.TypeRef{Spelling: intType}}},
	}
}
