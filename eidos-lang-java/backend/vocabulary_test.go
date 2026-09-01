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

	t.Run("TypeMods", func(t *testing.T) {
		t.Parallel()

		got, err := backend.TypeMods(&emit.Struct{Name: "Row"})
		assert.NoError(t, err, "an unstated visibility spells")
		assert.Equal(t, got, "public ", "as public, because a generated API is consumed")

		got, err = backend.TypeMods(&emit.Struct{
			Name: "Row", Visibility: symbol.VisibilityPackage, Final: true,
		})
		assert.NoError(t, err, "a package-scoped final class spells")
		assert.Equal(t, got, "final ", "default access is no keyword at all")

		got, err = backend.TypeMods(&emit.Struct{Name: "Row", Abstract: true})
		assert.NoError(t, err, "an abstract class spells")
		assert.Equal(t, got, "public abstract ", "abstract behind the access")

		got, err = backend.TypeMods(&emit.Struct{
			Name: "Inner", Level: symbol.LevelType, Final: true, Sealed: true,
		})
		assert.NoError(t, err, "a static sealed final class spells")
		assert.Equal(t, got, "public static final sealed ",
			"static, final and sealed in Java's stated order")

		got, err = backend.TypeMods(&emit.Interface{Name: "Shape", Sealed: true})
		assert.NoError(t, err, "a sealed interface spells")
		assert.Equal(t, got, "public sealed ", "sealed behind the access")

		_, err = backend.TypeMods(&emit.Struct{
			Name: "Row", Visibility: symbol.VisibilityPrivate,
		})
		assert.HasError(t, err,
			"a private file-level type refuses, because Java takes public "+
				"or default access alone")
	})

	t.Run("FieldMods", func(t *testing.T) {
		t.Parallel()

		got, err := backend.FieldMods(&emit.Field{
			Name:       "key",
			Level:      symbol.LevelType,
			Mutability: symbol.MutabilityImmutable,
		})
		assert.NoError(t, err, "a guarded field spells")
		assert.Equal(t, got, "public static final ",
			"access, static, final, in Java's stated order")

		got, err = backend.FieldMods(&emit.Field{
			Name: "key", Visibility: symbol.VisibilityProtected,
		})
		assert.NoError(t, err, "a protected member spells")
		assert.Equal(t, got, "protected ", "with its keyword")
	})

	t.Run("MethodMods", func(t *testing.T) {
		t.Parallel()

		got, err := backend.MethodMods(&emit.Method{
			Name: "load", Visibility: symbol.VisibilityPrivate,
			Level: symbol.LevelType,
		})
		assert.NoError(t, err, "a private static method spells")
		assert.Equal(t, got, "private static ", "access before static")

		_, err = backend.MethodMods(&emit.Method{Name: "load", Async: true})
		assert.HasError(t, err,
			"asynchrony refuses, because Java marks no signature")
		_, err = backend.MethodMods(&emit.Method{Name: "load", HasDefault: true})
		assert.HasError(t, err, "default belongs to interface methods")
	})

	t.Run("SigMods", func(t *testing.T) {
		t.Parallel()

		got, err := backend.SigMods(&emit.Method{Name: "load"})
		assert.NoError(t, err, "a bare signature passes")
		assert.Equal(t, got, "", "implicitly public")

		got, err = backend.SigMods(&emit.Method{Name: "load", HasDefault: true})
		assert.NoError(t, err, "a body-carrying method spells")
		assert.Equal(t, got, "default ", "as default at instance level")

		got, err = backend.SigMods(&emit.Method{
			Name: "make", HasDefault: true, Level: symbol.LevelType,
		})
		assert.NoError(t, err, "a type-level body spells")
		assert.Equal(t, got, "static ", "as static, which excludes default")

		_, err = backend.SigMods(&emit.Method{Name: "load", Override: true})
		assert.HasError(t, err, "an interface method overrides nothing")
	})

	t.Run("Heritage", func(t *testing.T) {
		t.Parallel()

		got, err := backend.Heritage(&emit.Struct{
			Name:       "Row",
			Extends:    []*emit.TypeRef{ref("Base")},
			Implements: []*emit.TypeRef{ref("Keyed"), ref("Closeable")},
		})
		assert.NoError(t, err, "a class heritage spells")
		assert.Equal(t, got, " extends Base implements Keyed, Closeable",
			"one superclass behind extends, the contracts behind implements")

		got, err = backend.Heritage(&emit.Interface{
			Name:    "Store",
			Extends: []*emit.TypeRef{ref("Keyed"), ref("Closeable")},
		})
		assert.NoError(t, err, "an interface heritage spells")
		assert.Equal(t, got, " extends Keyed, Closeable",
			"the widened contracts joined behind extends")

		got, err = backend.Heritage(&emit.Interface{
			Name:    "Shape",
			Extends: []*emit.TypeRef{ref("Figure")},
			Permits: []*emit.TypeRef{ref("Circle"), ref("Square")},
		})
		assert.NoError(t, err, "a sealed heritage spells")
		assert.Equal(t, got, " extends Figure permits Circle, Square",
			"the enumerated subtypes last, behind permits")

		_, err = backend.Heritage(&emit.Struct{
			Name:    "Row",
			Extends: []*emit.TypeRef{ref("A"), ref("B")},
		})
		assert.HasError(t, err, "a second superclass refuses")
		_, err = backend.Heritage(&emit.Interface{
			Name:   "Store",
			Embeds: []*emit.Embed{{Ref: ref("Base")}},
		})
		assert.HasError(t, err, "an embed refuses, because nothing promotes")
	})

	t.Run("Throws", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Throws(nil), "", "no failures, no clause")
		assert.Equal(t,
			backend.Throws([]*emit.TypeRef{ref("IOException"), ref("SQLException")}),
			" throws IOException, SQLException",
			"the declared failure types joined behind the keyword")
	})

	t.Run("Annotate", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Annotate(nil), "", "no annotations, no lines")
		assert.Equal(t, backend.Annotate(emit.Annotations{
			{Name: "Deprecated"},
			{Name: "SuppressWarnings", Args: []string{`"unchecked"`}},
		}, "    "),
			"    @Deprecated\n    @SuppressWarnings(\"unchecked\")\n",
			"one line per annotation at the member's depth, arguments verbatim")
	})
}
