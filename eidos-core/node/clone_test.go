// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package node_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// cloneAllocs is the copy of the inline body of [inlineBody]: the body, its
// two children, its argument and its absent member with their three
// lists, the field with its type and its list, and the method with its
// list. The method adds its receiver and the type of the receiver, its
// receiving type, its type parameter with a bound and a default, its
// parameter, its result and their types, its thrown type, and the five
// lists of its type parameters, bounds, parameters, results and thrown
// types: 1 + 6 + 3 + 2 + 18.
const cloneAllocs = 1 + 6 + 3 + 2 + 18

// CloneRef copies a reference and every node below it, and the copy shares
// no node with the reference.
func TestClone(t *testing.T) {
	t.Parallel()

	t.Run("CloneRef", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a nil reference", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, node.CloneRef(nil), "nothing to copy")
		})

		t.Run("returns a reference equal to the reference", func(t *testing.T) {
			t.Parallel()

			body := inlineBody()
			assert.Equal(t, node.CloneRef(body), body, "the copy has every fact of the reference")
		})

		t.Run("returns a reference whose nodes are not the nodes of the reference", func(t *testing.T) {
			t.Parallel()

			body := inlineBody()
			original := nodesBelow(body)
			for _, n := range nodesBelow(node.CloneRef(body)) {
				expect.NotContains(t, original, n, "the copy shares no node with the reference", assert.ByIdentity())
			}
		})

		t.Run("returns a copy of each absent member", func(t *testing.T) {
			t.Parallel()

			body := inlineBody()
			got := node.CloneRef(body)
			assert.Length(t, got.Absent, 1, "the copy has the absent member")
			assert.NotEqual(t, got.Absent[0], body.Absent[0], "the absent member is a copy", assert.ByIdentity())
		})

		t.Run("keeps a nil member of a list as nil", func(t *testing.T) {
			t.Parallel()

			body := &node.TypeRef{
				Form:    symbol.FormInline,
				Fields:  []*node.Field{nil},
				Methods: []*node.Method{nil, {Params: []*node.Param{nil}, Returns: []*node.Return{nil}}},
			}
			got := node.CloneRef(body)
			assert.Equal(t, got, body, "a nil member stays nil in its place")
		})
	})
}

// BenchmarkClone measures the copy of [inlineBody].
func BenchmarkClone(b *testing.B) {
	b.Run("CloneRef", func(b *testing.B) {
		body := inlineBody()
		var got *node.TypeRef
		c := bench.Start(b).MaxAllocs(cloneAllocs)
		defer c.End()
		for c.Loop() {
			got = node.CloneRef(body)
		}
		assert.Equal(b, got, body, "the copy has every fact of the reference")
	})
}

// TestCloneAllocs checks the allocation ceiling of the copy.
func TestCloneAllocs(t *testing.T) {
	body := inlineBody()
	expect.MaxAllocs(t, func() { node.CloneRef(body) }, cloneAllocs,
		"CloneRef allocates each node and list that it copies")
}

// inlineBody returns an inline body with a node in every place that a
// reference can have one: children, an argument, an absent member, a field
// and a method with a receiver, a receiving type, a type parameter, a
// parameter, a result and a thrown type.
func inlineBody() *node.TypeRef {
	return &node.TypeRef{
		Spelling: "{a: A; m<T extends B = C>(this: D, p: E): F}",
		Pos:      position.Pos{File: "a.ts", Line: 1},
		Form:     symbol.FormInline,
		Elems:    []*node.TypeRef{{Spelling: "A"}, {Spelling: "B"}},
		Args:     []*node.TypeRef{{Spelling: "G"}},
		Absent:   []*node.TypeRef{{Spelling: "undefined"}},
		Fields:   []*node.Field{{Name: "a", Doc: []string{"A field."}, Type: &node.TypeRef{Spelling: "A"}}},
		Methods: []*node.Method{{
			Name:     "m",
			Receiver: &node.Param{Name: "this", Type: &node.TypeRef{Spelling: "D"}},
			Receives: &node.TypeRef{Spelling: "H"},
			TypeParams: []*node.TypeParam{{
				Name: "T", Bounds: []*node.TypeRef{{Spelling: "B"}}, Default: &node.TypeRef{Spelling: "C"},
			}},
			Params:  []*node.Param{{Name: "p", Type: &node.TypeRef{Spelling: "E"}}},
			Returns: []*node.Return{{Type: &node.TypeRef{Spelling: "F"}}},
			Throws:  []*node.TypeRef{{Spelling: "I"}},
		}},
	}
}

// nodesBelow returns every node that the walk of r visits, r included.
func nodesBelow(r *node.TypeRef) []symbol.Symbol {
	var out []symbol.Symbol
	node.Walk(r, func(s symbol.Symbol) bool {
		out = append(out, s)
		return true
	})
	return out
}
