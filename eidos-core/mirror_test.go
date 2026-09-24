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

// methodID returns a fixture method identity.
func methodID(name string) symbol.Identity {
	return symbol.Identity{
		Lang: "golang", Package: "svc/store", Name: name,
		Kind: symbol.KindMethod,
	}
}

// Mirror owns the signature-copying lessons, so they are contract:
// the receiver never collides with a parameter, spellings copy over
// verbatim, and the mirrored method names its origin.
func TestMirror(t *testing.T) {
	t.Parallel()

	t.Run("names the receiver against the parameters", func(t *testing.T) {
		t.Parallel()

		m := &node.Method{
			ID:   methodID("Put"),
			Name: "Put",
			Params: []*node.Param{{
				Name: "s",
				Type: &node.TypeRef{Spelling: "Session"},
			}},
		}
		got := eidos.Mirror("Store", m)
		assert.Equal(t, got.Name, "Put", "the name mirrors")
		assert.Equal(t, got.Receiver.Name, "st",
			"a method declaring Put(s Session) must not bind its receiver to s")
		assert.Equal(t, got.Receiver.Type.Spelling, "*Store",
			"the receiver points at the host")
		assert.Equal(t, got.Origin, m.ID,
			"the mirrored method names its origin")
		assert.Length(t, got.Params, 1, "the parameters mirror")
		assert.Equal(t, got.Params[0].Type.Spelling, "Session",
			"type spellings copy over verbatim")
	})

	t.Run("mirrors returns and generic spellings", func(t *testing.T) {
		t.Parallel()

		m := &node.Method{
			Name: "Get",
			Returns: []*node.Return{{
				Type: &node.TypeRef{
					Spelling: "Map[string, User]",
					Args: []*node.TypeRef{
						{Spelling: "string"},
						{Spelling: "User"},
					},
				},
			}},
		}
		got := eidos.Mirror("Cache", m)
		assert.Equal(t, got.Receiver.Name, "c",
			"an untaken first letter is the receiver")
		assert.Length(t, got.Returns, 1, "the results mirror")
		assert.Equal(t, got.Returns[0].Type.Spelling, "Map[string, User]",
			"the instantiation spelling is carried whole")
		assert.Length(t, got.Returns[0].Type.Args, 2,
			"and its arguments copy over as a tree")
	})

	t.Run("mirrors the whole signature", func(t *testing.T) {
		t.Parallel()

		m := &node.Method{
			ID:         methodID("fetch"),
			Name:       "fetch",
			Visibility: symbol.VisibilityPackage,
			Level:      symbol.LevelType,
			Async:      true,
			TypeParams: []*node.TypeParam{{
				Name:     "K",
				Variance: symbol.VarianceOut,
				Bounds:   []*node.TypeRef{{Spelling: "comparable"}},
			}},
			Params: []*node.Param{{
				Name:     "keys",
				Label:    "for",
				Default:  "nil",
				Optional: true,
				Variadic: symbol.VariadicPositional,
				Type: &node.TypeRef{
					Spelling: "[]K",
					Form:     symbol.FormList,
					Elems:    []*node.TypeRef{{Spelling: "K"}},
				},
			}},
			Returns: []*node.Return{{Name: "found", Type: &node.TypeRef{Spelling: "int"}}},
			Throws:  []*node.TypeRef{{Spelling: "NotFound"}},
		}
		assert.Equal(t, eidos.Mirror("Store", m), &emit.Method{
			Origin:     m.ID,
			Name:       "fetch",
			Visibility: symbol.VisibilityPackage,
			Level:      symbol.LevelType,
			Async:      true,
			Receiver:   &emit.Param{Name: "s", Type: &emit.TypeRef{Spelling: "*Store"}},
			Receives:   &emit.TypeRef{Spelling: "Store"},
			TypeParams: []*emit.TypeParam{{
				Name:     "K",
				Variance: symbol.VarianceOut,
				Bounds:   []*emit.TypeRef{{Spelling: "comparable"}},
			}},
			Params: []*emit.Param{{
				Name:     "keys",
				Label:    "for",
				Default:  "nil",
				Optional: true,
				Variadic: symbol.VariadicPositional,
				Type: &emit.TypeRef{
					Spelling: "[]K",
					Form:     symbol.FormList,
					Elems:    []*emit.TypeRef{{Spelling: "K"}},
				},
			}},
			Returns: []*emit.Return{{Name: "found", Type: &emit.TypeRef{Spelling: "int"}}},
			Throws:  []*emit.TypeRef{{Spelling: "NotFound"}},
		}, "every signature field copies over, and Receives names the host")
	})

	t.Run("names the receiver against results and type parameters", func(t *testing.T) {
		t.Parallel()

		m := &node.Method{
			Name:       "Get",
			TypeParams: []*node.TypeParam{{Name: "s"}},
			Returns:    []*node.Return{{Name: "st", Type: &node.TypeRef{Spelling: "int"}}},
		}
		assert.Equal(t, eidos.Mirror("Store", m).Receiver.Name, "recv",
			"a receiver sharing a result's or a type parameter's name does not compile")
	})

	t.Run("names the receiver by the host's first character", func(t *testing.T) {
		t.Parallel()

		got := eidos.Mirror("Übung", &node.Method{Name: "Run"})
		assert.Equal(t, got.Receiver.Name, "ü",
			"a host spelled outside ASCII yields a whole character, not its first byte")
	})

	t.Run("numbers the receiver when every short name is taken", func(t *testing.T) {
		t.Parallel()

		m := &node.Method{
			Name: "Put",
			Params: []*node.Param{
				{Name: "s"}, {Name: "st"}, {Name: "recv"}, {Name: "recv0"},
			},
		}
		assert.Equal(t, eidos.Mirror("Store", m).Receiver.Name, "recv1",
			"the numbered names run until one is free")
	})
}
