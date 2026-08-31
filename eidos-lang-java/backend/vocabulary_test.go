// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-java/backend"
)

// ref is the fixture type reference.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// The vocabulary is what both kind templates spell through, so
// each helper's output is pinned byte for byte.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Docs", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Docs(nil), "", "no lines, no block")
		assert.Equal(t, backend.Docs([]string{"One."}, "    "),
			"    /**\n     * One.\n     */\n",
			"a Javadoc block indented whole to the member's depth")
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Spell(ref("List<Row>")), "List<Row>",
			"the source spelling passes through verbatim")
		assert.Equal(t, backend.Spell(nil), "Object",
			"a declaration stating no type spells the root type")
		assert.Equal(t, backend.Spell(&emit.TypeRef{
			Spelling: "Map",
			Args: []*emit.TypeRef{
				ref("String"),
				{Spelling: "List", Args: []*emit.TypeRef{ref("Row")}},
			},
		}), "Map<String, List<Row>>",
			"an argument list spells in angle brackets, arguments recursing")
	})

	t.Run("TypeParams", func(t *testing.T) {
		t.Parallel()

		got, err := backend.TypeParams(nil)
		assert.NoError(t, err, "no parameters spell")
		assert.Equal(t, got, "", "as nothing")

		got, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "K"},
			{Name: "V", Bounds: []*emit.TypeRef{ref("Codec"), ref("Closeable")}},
		})
		assert.NoError(t, err, "bounded parameters spell")
		assert.Equal(t, got, "<K, V extends Codec & Closeable>",
			"bounds joined by ampersands behind extends")

		_, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "T", Variance: symbol.VarianceOut},
		})
		assert.HasError(t, err,
			"variance refuses, because Java's wildcard is use-site")
		_, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "N", Const: true, Type: ref("int")},
		})
		assert.HasError(t, err, "a value parameter refuses")
		_, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "T", Default: ref("String")},
		})
		assert.HasError(t, err, "a default refuses")
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Params([]*emit.Param{
			{Name: "key", Type: ref("String")},
			{Name: "rest", Type: ref("int"), Variadic: symbol.VariadicPositional},
		}), "String key, int... rest", "type before name, dots on the variadic")
		assert.Equal(t, backend.Params([]*emit.Param{{Type: ref("int")}}), "int arg0",
			"an unnamed parameter is named by position, because Java requires one")
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		got, err := backend.Results(nil)
		assert.NoError(t, err, "no results spell")
		assert.Equal(t, got, "void", "as void")
		got, err = backend.Results([]*emit.Return{{Type: ref("Row")}})
		assert.NoError(t, err, "one result spells")
		assert.Equal(t, got, "Row", "as itself")
		_, err = backend.Results([]*emit.Return{{Type: ref("Row")}, {Type: ref("Err")}})
		assert.HasError(t, err, "a second result arrives thrown, not returned")
	})

	t.Run("PackageClause", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.PackageClause(symbol.Identity{Package: "svc/api"}),
			"package svc.api;\n\n", "dots for slashes, a blank line after")
		assert.Equal(t, backend.PackageClause(symbol.Identity{}), "",
			"no package is the default package")
	})
}
