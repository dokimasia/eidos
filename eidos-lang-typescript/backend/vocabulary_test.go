// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-typescript/backend"
)

// ref is the fixture type reference.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// The vocabulary is what every kind template spells through, so
// each helper's output is pinned byte for byte.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Docs", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Docs(nil), "", "no lines, no block")
		assert.Equal(t, backend.Docs([]string{"One.", "Two."}),
			"/**\n * One.\n * Two.\n */\n", "a TSDoc block with the star gutter")
		assert.Equal(t, backend.Docs([]string{"Inner."}, "  "),
			"  /**\n   * Inner.\n   */\n",
			"a member's doc indents whole to the member's depth")
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Spell(ref("Row[]")), "Row[]",
			"the source spelling passes through verbatim")
		assert.Equal(t, backend.Spell(nil), "unknown",
			"a declaration stating no type spells unknown, not any")
		assert.Equal(t, backend.Spell(&emit.TypeRef{
			Spelling: "Map",
			Args: []*emit.TypeRef{
				ref("string"),
				{Spelling: "Set", Args: []*emit.TypeRef{ref("Row")}},
			},
		}), "Map<string, Set<Row>>",
			"an argument list spells in angle brackets, arguments recursing")
	})

	t.Run("TypeParams", func(t *testing.T) {
		t.Parallel()

		got, err := backend.TypeParams(nil)
		assert.NoError(t, err, "no parameters spell")
		assert.Equal(t, got, "", "as nothing")

		got, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "T", Bounds: []*emit.TypeRef{ref("Codec"), ref("Closer")}},
			{Name: "U", Default: ref("string")},
		})
		assert.NoError(t, err, "bounds and defaults spell")
		assert.Equal(t, got, "<T extends Codec & Closer, U = string>",
			"bounds intersect behind extends, the default behind equals")

		got, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "T", Variance: symbol.VarianceIn},
			{Name: "U", Variance: symbol.VarianceOut},
		})
		assert.NoError(t, err, "variance spells")
		assert.Equal(t, got, "<in T, out U>",
			"the declaration-site keyword before the name")

		_, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "N", Const: true, Type: ref("number")},
		})
		assert.HasError(t, err,
			"a value parameter refuses, because TypeScript's const "+
				"modifier narrows a type parameter instead")
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Params([]*emit.Param{
			{Name: "key", Type: ref("string")},
			{Name: "rest", Type: ref("number"), Variadic: symbol.VariadicPositional},
		}), "key: string, ...rest: number[]",
			"colon-typed names and the rest marker with its array type")
		assert.Equal(t, backend.Params([]*emit.Param{{Type: ref("string")}}),
			"_: string", "an unnamed parameter still needs a name")
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Results(nil), ": void", "no results spell void")
		assert.Equal(t, backend.Results([]*emit.Return{{Type: ref("Row")}}),
			": Row", "one result is the annotation")
		assert.Equal(t, backend.Results([]*emit.Return{
			{Type: ref("Row")}, {Type: ref("Error")},
		}), ": [Row, Error]",
			"several become a tuple, because one value returns")
	})

	t.Run("Module", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Module(symbol.Identity{Package: "svc/api"}), "api",
			"the path's last element")
		assert.Equal(t, backend.Module(symbol.Identity{}), "",
			"an identity naming nothing spells nothing")
	})
}
