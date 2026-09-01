// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
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
}
