// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-go/backend"
)

// ref is the fixture type reference.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// The vocabulary is what every kind template spells through, so
// each helper's output is pinned byte for byte.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Docs", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Docs(nil), "", "no lines, no comment")
		assert.Equal(t, backend.Docs([]string{"One.", "Two."}), "// One.\n// Two.\n",
			"one line comment per line")
		assert.Equal(t, backend.Docs([]string{"Inner."}, "\t"), "\t// Inner.\n",
			"a member's doc indents to the member's depth")
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Spell(ref("[]store.Row")), "[]store.Row",
			"the source spelling passes through verbatim")
		assert.Equal(t, backend.Spell(nil), "any",
			"a declaration stating no type spells the empty interface")
		assert.Equal(t, backend.Spell(&emit.TypeRef{}), "any",
			"and so does a reference spelling nothing")
		assert.Equal(t, backend.Spell(&emit.TypeRef{
			Spelling: "Map",
			Args: []*emit.TypeRef{
				ref("string"),
				{Spelling: "List", Args: []*emit.TypeRef{ref("User")}},
			},
		}), "Map[string, List[User]]",
			"an argument list spells in brackets, arguments recursing")
	})

	t.Run("TypeParams", func(t *testing.T) {
		t.Parallel()

		got, err := backend.TypeParams(nil)
		assert.NoError(t, err, "no parameters spell")
		assert.Equal(t, got, "", "as nothing")

		got, err = backend.TypeParams([]*emit.TypeParam{{Name: "T"}})
		assert.NoError(t, err, "an unbounded parameter spells")
		assert.Equal(t, got, "[T any]", "under the any constraint")

		got, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "K", Bounds: []*emit.TypeRef{ref("comparable")}},
			{Name: "V", Bounds: []*emit.TypeRef{ref("Codec"), ref("Closer")}},
		})
		assert.NoError(t, err, "bounded parameters spell")
		assert.Equal(t, got, "[K comparable, V interface{ Codec; Closer }]",
			"one bound stands alone, several fold into a constraint interface")

		_, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "T", Variance: symbol.VarianceOut},
		})
		assert.HasError(t, err, "variance refuses, because Go states none")
		_, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "N", Const: true, Type: ref("int")},
		})
		assert.HasError(t, err, "a value parameter refuses")
		_, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "T", Default: ref("string")},
		})
		assert.HasError(t, err, "a default refuses")
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Params(nil), "", "no parameters, no list")
		assert.Equal(t, backend.Params([]*emit.Param{
			{Name: "ctx", Type: ref("context.Context")},
			{Name: "keys", Type: ref("string"), Variadic: symbol.VariadicPositional},
		}), "ctx context.Context, keys ...string",
			"names, types and the variadic marker")
		assert.Equal(t, backend.Params([]*emit.Param{{Type: ref("int")}}), "int",
			"an unnamed parameter is its type alone")
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Results(nil), "", "no results, no list")
		assert.Equal(t, backend.Results([]*emit.Return{{Type: ref("error")}}), " error",
			"one bare result stands alone")
		assert.Equal(t, backend.Results([]*emit.Return{
			{Type: ref("Row")}, {Type: ref("error")},
		}), " (Row, error)", "several parenthesise")
		assert.Equal(t, backend.Results([]*emit.Return{{Name: "n", Type: ref("int")}}),
			" (n int)", "one named result parenthesises too, because Go requires it")
	})

	t.Run("Receiver", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Receiver(&emit.Method{
			Receiver: &emit.Param{Name: "s", Type: ref("*Store")},
		}), "s *Store", "a declared receiver spells name and type")
		assert.Equal(t, backend.Receiver(&emit.Method{Receives: ref("*Store")}),
			"*Store", "a method attached from outside carries the type alone")
		assert.Equal(t, backend.Receiver(&emit.Method{}), "",
			"no receiver, no spelling")
	})

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Package(symbol.Identity{Name: "store"}), "store",
			"the identity's own name wins")
		assert.Equal(t, backend.Package(symbol.Identity{Package: "svc/api"}), "api",
			"and the path's last element stands in")
		assert.Equal(t, backend.Package(symbol.Identity{}), "",
			"an identity naming nothing spells nothing, and the formatter refuses")
	})
}
