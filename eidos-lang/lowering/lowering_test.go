// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package lowering_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/lowering"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The language and host the member check names in its error.
const (
	rustLang  = "rust"
	shapeHost = "Shape"
)

// stackMethods is the most methods the member check's set holds on the
// stack.
const stackMethods = 8

// The allocations of the copies and of the member check.
const (
	// copyParamsAllocs is the copy of one parameter with one bound of
	// one argument: the list, the parameter, the list of bounds, the
	// bound, its list of arguments and the argument.
	copyParamsAllocs = 6
	// copyRefsAllocs is the copy of two references without children: the
	// list and the two references.
	copyRefsAllocs = 3
	// copyRefAllocs is the copy of a reference without children.
	copyRefAllocs = 1
	// uniqueAllocs is the member check of more methods than the stack
	// holds: the set at its final size.
	uniqueAllocs = 3
)

// The copies restate shapes without sharing, and the member check
// names its language, so both contracts are pinned here.
func TestLowering(t *testing.T) {
	t.Parallel()

	t.Run("CopyTypeParams", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a fresh parameter for each parameter", func(t *testing.T) {
			t.Parallel()

			in := boundParams()
			out := lowering.CopyTypeParams(in)
			assert.Length(t, out, 1, "one parameter copies")
			assert.NotEqual(t, out[0], in[0], "as a fresh parameter", assert.ByIdentity())
			assert.Equal(t, out[0].Name, in[0].Name, "under its name")
		})

		t.Run("returns fresh bounds down to the argument tree", func(t *testing.T) {
			t.Parallel()

			in := boundParams()
			out := lowering.CopyTypeParams(in)
			bound := in[0].Bounds[0]
			assert.NotEqual(t, out[0].Bounds[0], bound, "with a fresh bound", assert.ByIdentity())
			assert.NotEqual(
				t,
				out[0].Bounds[0].Args[0],
				bound.Args[0],
				"down to the argument tree",
				assert.ByIdentity(),
			)
			assert.Equal(t, out[0].Bounds[0].Args[0].Spelling, "K", "spelled the same")
		})

		t.Run("returns nil for no parameters", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, lowering.CopyTypeParams(nil), "emptiness copies to nil")
		})
	})

	t.Run("CopyTypeRefs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a fresh reference for each reference in order", func(t *testing.T) {
			t.Parallel()

			in := leaves()
			out := lowering.CopyTypeRefs(in)
			assert.Equal(t, out, in, "the copies spell the references in order")
			assert.NotEqual(t, out[0], in[0], "and share no node with them", assert.ByIdentity())
		})

		t.Run("returns nil for no references", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, lowering.CopyTypeRefs(nil), "a nil list copies to nil")
		})
	})

	t.Run("CopyTypeRef", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a tree equal to its input", func(t *testing.T) {
			t.Parallel()

			in := mapOfUsers()
			assert.Equal(t, lowering.CopyTypeRef(in), in, "the copy restates the whole tree")
		})

		t.Run("returns a tree that shares no node with its input", func(t *testing.T) {
			t.Parallel()

			in := mapOfUsers()
			out := lowering.CopyTypeRef(in)
			expect.NotEqual(t, out.Elems[1], in.Elems[1], "a child is fresh", assert.ByIdentity())
			expect.NotEqual(t, out.Elems[1].Elems[0], in.Elems[1].Elems[0], "at any depth", assert.ByIdentity())
		})

		t.Run("returns nil for a nil reference", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, lowering.CopyTypeRef(nil), "a nil reference copies to nil")
		})
	})

	t.Run("UniqueMethods", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for distinct names", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, lowering.UniqueMethods(rustLang, shapeHost, methods(stackMethods+1)),
				"every name once passes")
		})

		t.Run("returns an error naming the duplicate under the language", func(t *testing.T) {
			t.Parallel()

			err := lowering.UniqueMethods(rustLang, shapeHost, []*emit.Method{{Name: "area"}, {Name: "area"}})
			assert.HasError(t, err, "a name declared twice fails")
			assert.Contains(t, err.Error(), "rust: method area", "under the language's prefix")
		})
	})
}

// Each copy allocates what it returns, and the member check allocates
// its set only past the stack's room, in the ordinary run, which runs
// no benchmark.
func TestLoweringAllocs(t *testing.T) {
	params, refs, leaf := boundParams(), leaves(), &emit.TypeRef{Spelling: "string"}
	few, many := methods(stackMethods), methods(stackMethods+1)
	var (
		copiedParams []*emit.TypeParam
		copiedRefs   []*emit.TypeRef
		copied       *emit.TypeRef
		err          error
	)
	assert.MaxAllocs(t, func() { copiedParams = lowering.CopyTypeParams(params) }, copyParamsAllocs,
		"CopyTypeParams allocates the copies down to the argument")
	assert.Length(t, copiedParams, 1, "CopyTypeParams copies the parameter")
	assert.MaxAllocs(t, func() { copiedRefs = lowering.CopyTypeRefs(refs) }, copyRefsAllocs,
		"CopyTypeRefs allocates the list and the copies")
	assert.Length(t, copiedRefs, 2, "CopyTypeRefs copies both references")
	assert.MaxAllocs(t, func() { copied = lowering.CopyTypeRef(leaf) }, copyRefAllocs,
		"CopyTypeRef allocates the copy")
	assert.Equal(t, copied.Spelling, leaf.Spelling, "CopyTypeRef copies the spelling")
	assert.MaxAllocs(t, func() { err = lowering.UniqueMethods(rustLang, shapeHost, few) }, 0,
		"UniqueMethods allocates nothing for methods the stack's set holds")
	assert.NoError(t, err, "UniqueMethods passes distinct names")
	assert.MaxAllocs(t, func() { err = lowering.UniqueMethods(rustLang, shapeHost, many) }, uniqueAllocs,
		"UniqueMethods allocates the set past the stack's room")
	assert.NoError(t, err, "UniqueMethods passes distinct names")
}

// BenchmarkLowering measures each copy a backend's lowering makes of a
// shape, and the member check over a host's methods.
func BenchmarkLowering(b *testing.B) {
	b.Run("CopyTypeParams", func(b *testing.B) {
		params := boundParams()
		c := bench.Start(b).MaxAllocs(copyParamsAllocs)
		defer c.End()
		var got []*emit.TypeParam
		for c.Loop() {
			got = lowering.CopyTypeParams(params)
		}
		assert.Length(b, got, 1, "CopyTypeParams copies the parameter")
	})

	b.Run("CopyTypeRefs", func(b *testing.B) {
		refs := leaves()
		c := bench.Start(b).MaxAllocs(copyRefsAllocs)
		defer c.End()
		var got []*emit.TypeRef
		for c.Loop() {
			got = lowering.CopyTypeRefs(refs)
		}
		assert.Length(b, got, 2, "CopyTypeRefs copies both references")
	})

	b.Run("CopyTypeRef", func(b *testing.B) {
		leaf := &emit.TypeRef{Spelling: "string"}
		c := bench.Start(b).MaxAllocs(copyRefAllocs)
		defer c.End()
		var got *emit.TypeRef
		for c.Loop() {
			got = lowering.CopyTypeRef(leaf)
		}
		assert.Equal(b, got.Spelling, leaf.Spelling, "CopyTypeRef copies the spelling")
	})

	tests := []struct {
		name    string
		methods []*emit.Method
		allocs  uint64
	}{
		{name: "methods that fit on the stack", methods: methods(stackMethods)},
		{
			name:    "more methods than fit on the stack",
			methods: methods(stackMethods + 1),
			allocs:  uniqueAllocs,
		},
	}
	b.Run("UniqueMethods", func(b *testing.B) {
		for _, tt := range tests {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var err error
				for c.Loop() {
					err = lowering.UniqueMethods(rustLang, shapeHost, tt.methods)
				}
				assert.NoError(b, err, "UniqueMethods passes distinct names")
			})
		}
	})
}

// boundParams returns one parameter T with one bound Ord of one
// argument K.
func boundParams() []*emit.TypeParam {
	bound := &emit.TypeRef{Spelling: "Ord", Args: []*emit.TypeRef{{Spelling: "K"}}}
	return []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{bound}}}
}

// leaves returns two references without children.
func leaves() []*emit.TypeRef {
	return []*emit.TypeRef{{Spelling: "string"}, {Spelling: "int"}}
}

// mapOfUsers returns a map from strings to optional users: a reference
// whose children have children.
func mapOfUsers() *emit.TypeRef {
	return &emit.TypeRef{
		Spelling: "map[string]*User", Form: symbol.FormMap,
		Elems: []*emit.TypeRef{
			{Spelling: "string"},
			{Spelling: "*User", Form: symbol.FormOptional, Elems: []*emit.TypeRef{{Spelling: "User"}}},
		},
	}
}

// methods returns n methods of distinct names.
func methods(n int) []*emit.Method {
	out := make([]*emit.Method, 0, n)
	for i := range n {
		out = append(out, &emit.Method{Name: "m" + strconv.Itoa(i)})
	}
	return out
}
