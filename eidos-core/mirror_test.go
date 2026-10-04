// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

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
)

// Mirror copies a signature once, in the framework, so the copy is
// contract: every field arrives, spellings copy over verbatim, and
// the receiver is the target's to state.
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
				Receives:   &emit.TypeRef{Spelling: mirrorHost},
				TypeParams: []*emit.TypeParam{{
					Name:     keyParam,
					Variance: symbol.VarianceOut,
					Bounds:   []*emit.TypeRef{{Spelling: keyBound}},
				}},
				Params: []*emit.Param{{
					Name:     keysParam,
					Label:    keysLabel,
					Default:  keysDefault,
					Optional: true,
					Variadic: symbol.VariadicPositional,
					Type: &emit.TypeRef{
						Spelling: keysSpelling,
						Form:     symbol.FormList,
						Elems:    []*emit.TypeRef{{Spelling: keyParam}},
					},
				}},
				Returns: []*emit.Return{{Name: foundResult, Type: &emit.TypeRef{Spelling: intType}}},
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
			assert.Length(t, got.Returns, 1, "the results mirror")
			assert.Equal(t, got.Returns[0].Type,
				&emit.TypeRef{
					Spelling: userMap,
					Args:     []*emit.TypeRef{{Spelling: stringType}, {Spelling: userType}},
				}, "the instantiation's spelling and its arguments copy over as a tree")
		})

		t.Run("leaves the receiver to the target", func(t *testing.T) {
			t.Parallel()

			got := eidos.Mirror(mirrorHost, &node.Method{Name: getMethod})
			assert.True(t, got.Receiver == nil,
				"a pointer receiver is Go's spelling, and Rust spells self")
		})
	})
}

// A mirror allocates the method it returns and its parts in the
// ordinary run, which runs no benchmark. The check runs alone, because
// AllocsPerRun counts every goroutine's allocations and refuses to run
// beside parallel tests.
func TestMirrorAllocs(t *testing.T) {
	signed, bare := getter(), &node.Method{ID: methodID(getMethod), Name: getMethod}
	var got *emit.Method
	assert.MaxAllocs(t, func() { got = eidos.Mirror(mirrorHost, signed) }, mirrorAllocs,
		"Mirror allocates the method and the parts of its signature")
	assert.Length(t, got.Params, 1, "Mirror returns the parameter")
	assert.MaxAllocs(t, func() { got = eidos.Mirror(mirrorHost, bare) }, bareMirrorAllocs,
		"Mirror allocates the method and its receiving type for a method without a signature")
	assert.Equal(t, got.Name, getMethod, "Mirror returns the method's name")
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
			assert.Length(b, got.Returns, 1, "Mirror returns the return")
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
