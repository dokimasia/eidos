// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package typescript_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-typescript"
)

// ref is the fixture type reference.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// The vocabulary is what every kind template spells through, so
// each helper's output is pinned byte for byte.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Docs", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, typescript.Docs(nil), "", "no lines, no block")
		assert.Equal(t, typescript.Docs([]string{"One.", "Two."}),
			"/**\n * One.\n * Two.\n */\n", "a TSDoc block with the star gutter")
		assert.Equal(t, typescript.Docs([]string{"Inner."}, "  "),
			"  /**\n   * Inner.\n   */\n",
			"a member's doc indents whole to the member's depth")
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, typescript.Spell(ref("Row[]")), "Row[]",
			"the source spelling rides through verbatim")
		assert.Equal(t, typescript.Spell(nil), "unknown",
			"a declaration stating no type spells unknown, not any")
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, typescript.Params([]*emit.Param{
			{Name: "key", Type: ref("string")},
			{Name: "rest", Type: ref("number"), Variadic: symbol.VariadicPositional},
		}), "key: string, ...rest: number[]",
			"colon-typed names and the rest marker with its array type")
		assert.Equal(t, typescript.Params([]*emit.Param{{Type: ref("string")}}),
			"_: string", "an unnamed parameter still needs a name")
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, typescript.Results(nil), ": void", "no results spell void")
		assert.Equal(t, typescript.Results([]*emit.Return{{Type: ref("Row")}}),
			": Row", "one result is the annotation")
		assert.Equal(t, typescript.Results([]*emit.Return{
			{Type: ref("Row")}, {Type: ref("Error")},
		}), ": [Row, Error]",
			"several become a tuple, because one value returns")
	})

	t.Run("Module", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, typescript.Module(symbol.Identity{Package: "svc/api"}), "api",
			"the path's last element")
		assert.Equal(t, typescript.Module(symbol.Identity{}), "",
			"an identity naming nothing spells nothing")
	})
}
