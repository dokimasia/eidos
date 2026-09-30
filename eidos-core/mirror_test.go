// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// mirrorLang is the language the fixture's identities are in.
const mirrorLang symbol.Lang = "golang"

// The mirror fixture: the host a method is mirrored onto, and the
// declarations its signature names.
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

// methodID returns a fixture method identity.
func methodID(name string) symbol.Identity {
	return symbol.Identity{
		Lang: mirrorLang, Package: mirrorPackage, Name: name,
		Kind: symbol.KindMethod,
	}
}

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
