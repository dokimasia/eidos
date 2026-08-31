// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package java_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-java"
)

// ref is the fixture type reference.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// The vocabulary is what both kind templates spell through, so
// each helper's output is pinned byte for byte.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Docs", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, java.Docs(nil), "", "no lines, no block")
		assert.Equal(t, java.Docs([]string{"One."}, "    "),
			"    /**\n     * One.\n     */\n",
			"a Javadoc block indented whole to the member's depth")
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, java.Spell(ref("List<Row>")), "List<Row>",
			"the source spelling rides through verbatim")
		assert.Equal(t, java.Spell(nil), "Object",
			"a declaration stating no type spells the root type")
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, java.Params([]*emit.Param{
			{Name: "key", Type: ref("String")},
			{Name: "rest", Type: ref("int"), Variadic: symbol.VariadicPositional},
		}), "String key, int... rest", "type before name, dots on the variadic")
		assert.Equal(t, java.Params([]*emit.Param{{Type: ref("int")}}), "int arg0",
			"an unnamed parameter is named by position, because Java requires one")
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		got, err := java.Results(nil)
		assert.NoError(t, err, "no results spell")
		assert.Equal(t, got, "void", "as void")
		got, err = java.Results([]*emit.Return{{Type: ref("Row")}})
		assert.NoError(t, err, "one result spells")
		assert.Equal(t, got, "Row", "as itself")
		_, err = java.Results([]*emit.Return{{Type: ref("Row")}, {Type: ref("Err")}})
		assert.HasError(t, err, "a second result arrives thrown, not returned")
	})

	t.Run("PackageClause", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, java.PackageClause(symbol.Identity{Package: "svc/api"}),
			"package svc.api;\n\n", "dots for slashes, a blank line after")
		assert.Equal(t, java.PackageClause(symbol.Identity{}), "",
			"no package is the default package")
	})
}
