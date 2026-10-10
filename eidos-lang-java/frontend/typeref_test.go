// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// A type reference keeps its spelling and the structure its syntax
// states, so the form, the spelling and the children of every Java type
// form are pinned.
func TestTyperef(t *testing.T) {
	t.Parallel()

	t.Run("typeRef", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a primitive type as Named with its spelling", func(t *testing.T) {
			t.Parallel()

			ref := fieldType(t, "int f;")
			assert.Equal(t, ref.Form, symbol.FormNamed, "a name")
			assert.Equal(t, ref.Spelling, "int", "spelled as written")
		})

		t.Run("lowers a qualified name without the whitespace between its names", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldType(t, "java.util . List f;").Spelling, "java.util.List", "dots between the names")
		})

		t.Run("lowers a generic type with its bare name as its spelling", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldType(t, "java.util.List<String> f;").Spelling, "java.util.List", "the bare name")
		})

		t.Run("lowers a generic type's arguments as its Args", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, spellingsOf(fieldType(t, "Map<String, Integer> f;").Args), []string{"String", "Integer"},
				"both arguments in order")
		})

		t.Run("lowers an array type as a List of its element", func(t *testing.T) {
			t.Parallel()

			ref := fieldType(t, "String[] f;")
			assert.Equal(t, ref.Form, symbol.FormList, "an array")
			assert.Equal(t, spellingsOf(ref.Elems), []string{"String"}, "of its element")
		})

		t.Run("lowers each dimension of an array type as one List", func(t *testing.T) {
			t.Parallel()

			ref := fieldType(t, "int[][] f;")
			assert.Equal(t, ref.Elems[0].Form, symbol.FormList, "an array of arrays")
			assert.Equal(t, ref.Elems[0].Elems[0].Spelling, "int", "of int")
		})

		t.Run("lowers an annotated type without its annotation", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, spellingsOf(fieldType(t, "List<@NonNull String> f;").Args), []string{"String"},
				"a type annotation is no part of the signature")
		})

		t.Run("lowers an unbounded wildcard as a Wildcard without a child", func(t *testing.T) {
			t.Parallel()

			arg := fieldType(t, "List<?> f;").Args[0]
			assert.Equal(t, arg.Form, symbol.FormWildcard, "a wildcard")
			assert.Empty(t, arg.Elems, "without a bound")
		})

		t.Run("lowers an extends wildcard with Variance Out", func(t *testing.T) {
			t.Parallel()

			arg := fieldType(t, "List<? extends Number> f;").Args[0]
			assert.Equal(t, arg.Variance, symbol.VarianceOut, "an upper bound")
			assert.Equal(t, spellingsOf(arg.Elems), []string{"Number"}, "of its bound")
		})

		t.Run("lowers a super wildcard with Variance In", func(t *testing.T) {
			t.Parallel()

			arg := fieldType(t, "List<? super Integer> f;").Args[0]
			assert.Equal(t, arg.Variance, symbol.VarianceIn, "a lower bound")
			assert.Equal(t, spellingsOf(arg.Elems), []string{"Integer"}, "of its bound")
		})
	})

	t.Run("spelling", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a qualified name without its type annotations", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldType(t, "java.util.@NonNull List f;").Spelling, "java.util.List",
				"the annotation is left out")
		})

		t.Run("spells a generic segment of a qualified name whole", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldType(t, "Outer<String>.Inner f;").Spelling, "Outer<String>.Inner",
				"the segment's arguments are part of the name")
		})
	})

	t.Run("dimensioned", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a declarator's dimensions into its field's type", func(t *testing.T) {
			t.Parallel()

			ref := fieldType(t, "int f[][];")
			assert.Equal(t, ref.Form, symbol.FormList, "an array")
			assert.Equal(t, ref.Elems[0].Form, symbol.FormList, "of arrays")
		})

		t.Run("lowers dimensions on an array type and its declarator together", func(t *testing.T) {
			t.Parallel()

			ref := fieldType(t, "int[] f[];")
			assert.Equal(t, ref.Elems[0].Elems[0].Spelling, "int", "two dimensions of int")
		})
	})

	t.Run("wildcardOf", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an annotated wildcard without its annotation", func(t *testing.T) {
			t.Parallel()

			arg := fieldType(t, "List<@A ?> f;").Args[0]
			assert.Empty(t, arg.Elems, "the annotation is no bound")
			assert.Equal(t, arg.Variance, symbol.VarianceInvariant, "an unbounded wildcard")
		})
	})

	t.Run("listOf", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a List of a generic type with the type's arguments", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldType(t, "Map<String, List<Integer>>[] f;").Spelling, "Map<String,List<Integer>>[]",
				"the element's arguments and one dimension")
		})

		t.Run("spells a List of a List with one dimension each", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldType(t, "int[][] f;").Spelling, "int[][]", "two dimensions")
		})
	})
}

// fieldType parses a public class A of one field declaration and returns
// the field's type.
func fieldType(tb testing.TB, decl string) *node.TypeRef {
	tb.Helper()

	st := classOf(tb, "public class A {\n    "+decl+"\n}\n", "A")
	assert.NotEmpty(tb, st.Fields, "the class declares the field")
	return st.Fields[0].Type
}
