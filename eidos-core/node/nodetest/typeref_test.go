// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package nodetest_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/node/nodetest"
	"go.dokimi.dev/eidos/core/symbol"
)

// The import path and the spelling of the external type of the cases.
const (
	externalPath = "database/sql"
	externalName = "sql.DB"
)

func TestTypeRef(t *testing.T) {
	t.Parallel()

	t.Run("Named", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a named reference with its spelling and its arguments", func(t *testing.T) {
			t.Parallel()

			arg := nodetest.Named(intName)
			ref := nodetest.Named(mapName, arg)
			expect.Equal(t, ref.Form, symbol.FormNamed, "the reference is named")
			expect.Equal(t, ref.Spelling, mapName, "the reference has the spelling")
			expect.Empty(t, ref.Package, "the reference has no package")
			sameRefs(t, ref.Args, []*node.TypeRef{arg}, "the reference keeps its arguments")
		})
	})

	t.Run("External", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a named reference with its package", func(t *testing.T) {
			t.Parallel()

			ref := nodetest.External(externalPath, externalName)
			expect.Equal(t, ref.Form, symbol.FormNamed, "the reference is named")
			expect.Equal(t, ref.Spelling, externalName, "the reference has the spelling")
			expect.Equal(t, ref.Package, externalPath, "the reference has the package")
		})
	})

	t.Run("Optional", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an optional of one child without a spelling", func(t *testing.T) {
			t.Parallel()

			elem := nodetest.Named(intName)
			ref := nodetest.Optional(elem)
			expect.Equal(t, ref.Form, symbol.FormOptional, "the reference is an optional")
			expect.Empty(t, ref.Spelling, "the reference has no spelling")
			sameRefs(t, ref.Elems, []*node.TypeRef{elem}, "the reference has its child")
		})

		t.Run("returns an empty child for a nil child", func(t *testing.T) {
			t.Parallel()

			ref := nodetest.Optional(nil)
			assert.Length(t, ref.Elems, 1, "the optional keeps its arity")
			assert.Equal(t, ref.Elems[0], &node.TypeRef{}, "the child is an empty reference")
		})
	})

	t.Run("List", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a list of one child", func(t *testing.T) {
			t.Parallel()

			elem := nodetest.Named(intName)
			ref := nodetest.List(elem)
			expect.Equal(t, ref.Form, symbol.FormList, "the reference is a list")
			sameRefs(t, ref.Elems, []*node.TypeRef{elem}, "the reference has its child")
		})
	})

	t.Run("Array", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an array of one child with its length", func(t *testing.T) {
			t.Parallel()

			elem := nodetest.Named(intName)
			ref := nodetest.Array(3, elem)
			expect.Equal(t, ref.Form, symbol.FormArray, "the reference is an array")
			expect.Equal(t, ref.Length, 3, "the reference has its length")
			sameRefs(t, ref.Elems, []*node.TypeRef{elem}, "the reference has its child")
		})
	})

	t.Run("Map", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a map of its key and its value", func(t *testing.T) {
			t.Parallel()

			key, value := nodetest.Named(keyName), nodetest.Named(valueName)
			ref := nodetest.Map(key, value)
			expect.Equal(t, ref.Form, symbol.FormMap, "the reference is a map")
			sameRefs(t, ref.Elems, []*node.TypeRef{key, value}, "the key precedes the value")
		})
	})

	t.Run("Stream", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a stream of one child", func(t *testing.T) {
			t.Parallel()

			elem := nodetest.Named(intName)
			ref := nodetest.Stream(elem)
			expect.Equal(t, ref.Form, symbol.FormStream, "the reference is a stream")
			expect.False(t, ref.Async, "the stream is not asynchronous")
			sameRefs(t, ref.Elems, []*node.TypeRef{elem}, "the reference has its child")
		})
	})

	t.Run("Borrow", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a borrow of one child", func(t *testing.T) {
			t.Parallel()

			elem := nodetest.Named(intName)
			ref := nodetest.Borrow(elem)
			expect.Equal(t, ref.Form, symbol.FormBorrow, "the reference is a borrow")
			sameRefs(t, ref.Elems, []*node.TypeRef{elem}, "the reference has its child")
		})
	})

	t.Run("Func", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a function type with its parameters before its results", func(t *testing.T) {
			t.Parallel()

			param, result := nodetest.Named(intName), nodetest.Named(stringName)
			ref := nodetest.Func([]*node.TypeRef{param}, []*node.TypeRef{result})
			expect.Equal(t, ref.Form, symbol.FormFunc, "the reference is a function type")
			expect.Equal(t, ref.Split, 1, "the results begin after the one parameter")
			sameRefs(t, ref.Elems, []*node.TypeRef{param, result}, "the parameter precedes the result")
		})
	})

	t.Run("Tuple", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a tuple of its members in order", func(t *testing.T) {
			t.Parallel()

			first, second := nodetest.Named(firstName), nodetest.Named(secondName)
			ref := nodetest.Tuple(first, second)
			expect.Equal(t, ref.Form, symbol.FormTuple, "the reference is a tuple")
			sameRefs(t, ref.Elems, []*node.TypeRef{first, second}, "the members are in order")
		})
	})

	t.Run("Union", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a union of its members in order", func(t *testing.T) {
			t.Parallel()

			first, second := nodetest.Named(firstName), nodetest.Named(secondName)
			ref := nodetest.Union(first, second)
			expect.Equal(t, ref.Form, symbol.FormUnion, "the reference is a union")
			sameRefs(t, ref.Elems, []*node.TypeRef{first, second}, "the members are in order")
		})
	})

	t.Run("Intersection", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an intersection of its members in order", func(t *testing.T) {
			t.Parallel()

			first, second := nodetest.Named(firstName), nodetest.Named(secondName)
			ref := nodetest.Intersection(first, second)
			expect.Equal(t, ref.Form, symbol.FormIntersection, "the reference is an intersection")
			sameRefs(t, ref.Elems, []*node.TypeRef{first, second}, "the members are in order")
		})
	})

	t.Run("Wildcard", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a wildcard of its bound under its variance", func(t *testing.T) {
			t.Parallel()

			bound := nodetest.Named(elemName)
			ref := nodetest.Wildcard(symbol.VarianceOut, bound)
			expect.Equal(t, ref.Form, symbol.FormWildcard, "the reference is a wildcard")
			expect.Equal(t, ref.Variance, symbol.VarianceOut, "the wildcard has its variance")
			sameRefs(t, ref.Elems, []*node.TypeRef{bound}, "the wildcard has its bound")
		})

		t.Run("returns a wildcard without a child for a nil bound", func(t *testing.T) {
			t.Parallel()

			ref := nodetest.Wildcard(symbol.VarianceInvariant, nil)
			assert.Empty(t, ref.Elems, "an unbounded wildcard has no child")
		})
	})

	t.Run("Inline", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an inline body with its fields and methods", func(t *testing.T) {
			t.Parallel()

			fields, methods := []*node.Field{{Name: fieldName}}, []*node.Method{{Name: methodName}}
			ref := nodetest.Inline(fields, methods)
			expect.Equal(t, ref.Form, symbol.FormInline, "the reference is an inline body")
			expect.Equal(t, ref.Fields, fields, "the body keeps the list of its fields", assert.ByIdentity())
			expect.Equal(t, ref.Methods, methods, "the body keeps the list of its methods", assert.ByIdentity())
		})
	})
}

// sameRefs checks that got has the references of want, each the same
// object as its counterpart, in order.
func sameRefs(tb assert.TB, got, want []*node.TypeRef, msg string) {
	tb.Helper()

	assert.Length(tb, got, len(want), msg)
	for i := range want {
		expect.Equal(tb, got[i], want[i], msg, assert.ByIdentity())
	}
}
