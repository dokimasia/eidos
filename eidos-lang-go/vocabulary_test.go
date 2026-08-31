// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	golang "go.dokimi.dev/eidos/lang-go"
)

// ref is the fixture type reference.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// The vocabulary is what every kind template spells through, so
// each helper's output is pinned byte for byte.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Docs", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, golang.Docs(nil), "", "no lines, no comment")
		assert.Equal(t, golang.Docs([]string{"One.", "Two."}), "// One.\n// Two.\n",
			"one line comment per line")
		assert.Equal(t, golang.Docs([]string{"Inner."}, "\t"), "\t// Inner.\n",
			"a member's doc indents to the member's depth")
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, golang.Spell(ref("[]store.Row")), "[]store.Row",
			"the source spelling rides through verbatim")
		assert.Equal(t, golang.Spell(nil), "any",
			"a declaration stating no type spells the empty interface")
		assert.Equal(t, golang.Spell(&emit.TypeRef{}), "any",
			"and so does a reference spelling nothing")
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, golang.Params(nil), "", "no parameters, no list")
		assert.Equal(t, golang.Params([]*emit.Param{
			{Name: "ctx", Type: ref("context.Context")},
			{Name: "keys", Type: ref("string"), Variadic: symbol.VariadicPositional},
		}), "ctx context.Context, keys ...string",
			"names, types and the variadic marker")
		assert.Equal(t, golang.Params([]*emit.Param{{Type: ref("int")}}), "int",
			"an unnamed parameter is its type alone")
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, golang.Results(nil), "", "no results, no list")
		assert.Equal(t, golang.Results([]*emit.Return{{Type: ref("error")}}), " error",
			"one bare result stands alone")
		assert.Equal(t, golang.Results([]*emit.Return{
			{Type: ref("Row")}, {Type: ref("error")},
		}), " (Row, error)", "several parenthesise")
		assert.Equal(t, golang.Results([]*emit.Return{{Name: "n", Type: ref("int")}}),
			" (n int)", "one named result parenthesises too, because Go requires it")
	})

	t.Run("Receiver", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, golang.Receiver(&emit.Method{
			Receiver: &emit.Param{Name: "s", Type: ref("*Store")},
		}), "s *Store", "a declared receiver spells name and type")
		assert.Equal(t, golang.Receiver(&emit.Method{Receives: ref("*Store")}),
			"*Store", "a method attached from outside carries the type alone")
		assert.Equal(t, golang.Receiver(&emit.Method{}), "",
			"no receiver, no spelling")
	})

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, golang.Package(symbol.Identity{Name: "store"}), "store",
			"the identity's own name wins")
		assert.Equal(t, golang.Package(symbol.Identity{Package: "svc/api"}), "api",
			"and the path's last element stands in")
		assert.Equal(t, golang.Package(symbol.Identity{}), "",
			"an identity naming nothing spells nothing, and the formatter refuses")
	})
}
