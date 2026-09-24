// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ref is the fixture type reference.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// The vocabulary is what the kind templates spell through, so each
// helper's output is pinned byte for byte.
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
			Name: "Inner", Visibility: symbol.VisibilityPrivate, Level: symbol.LevelType,
			Abstract: true, Sealed: true,
		})
		assert.NoError(t, err, "a member class spells a member's keywords")
		assert.Equal(t, got, "private abstract static sealed ",
			"access, abstract, static and sealed, in Java's stated order")

		got, err = backend.TypeMods(&emit.Struct{
			Name: "Inner", Visibility: symbol.VisibilityProtected, Level: symbol.LevelType, Final: true,
		})
		assert.NoError(t, err, "a protected static final member class spells")
		assert.Equal(t, got, "protected static final ", "static before final")

		got, err = backend.TypeMods(&emit.Interface{Name: "Shape", Sealed: true})
		assert.NoError(t, err, "a sealed interface spells")
		assert.Equal(t, got, "public sealed ", "sealed behind the access")

		got, err = backend.TypeMods(&emit.Enum{Name: "Phase", Visibility: symbol.VisibilityPrivate})
		assert.NoError(t, err, "a member enum spells")
		assert.Equal(t, got, "private ", "its access alone")

		_, err = backend.TypeMods(&emit.Struct{Name: "Row", Abstract: true, Final: true})
		assert.HasError(t, err, "a class both abstract and final refuses, because javac rejects the pair")
		_, err = backend.TypeMods(&emit.Struct{Name: "Row", Final: true, Sealed: true})
		assert.HasError(t, err, "a class both final and sealed refuses, because javac rejects the pair")
		_, err = backend.TypeMods(&emit.Struct{Name: "Row", Visibility: symbol.VisibilityInternal})
		assert.HasError(t, err, "an internal scope refuses, because no Java keyword spells it")
		_, err = backend.TypeMods(&emit.Alias{Name: "Id"})
		assert.HasError(t, err, "an alias has no type keywords")
	})

	t.Run("MemberType", func(t *testing.T) {
		t.Parallel()

		for _, s := range []symbol.Symbol{
			&emit.Struct{Name: "Inner"},
			&emit.Interface{Name: "Inner", Visibility: symbol.VisibilityPublic},
			&emit.Enum{Name: "Phase"},
			&emit.Alias{Name: "Id", Visibility: symbol.VisibilityPrivate},
		} {
			got, err := backend.MemberType(s)
			assert.NoError(t, err, "a public or unstated member type passes, and so does "+
				"a kind the nested template reports")
			assert.Equal(t, got, "", "and the helper writes nothing")
		}
		for _, s := range []symbol.Symbol{
			&emit.Struct{Name: "Inner", Visibility: symbol.VisibilityPrivate},
			&emit.Interface{Name: "Inner", Visibility: symbol.VisibilityProtected},
			&emit.Enum{Name: "Phase", Visibility: symbol.VisibilityPackage},
		} {
			_, err := backend.MemberType(s)
			assert.HasError(t, err,
				"a narrower scope refuses, because every member type of an interface is public")
		}
	})

	t.Run("ConstantMods", func(t *testing.T) {
		t.Parallel()

		got, err := backend.ConstantMods(&emit.Field{Name: "MAX", Type: ref("int"), Value: "8"})
		assert.NoError(t, err, "an interface constant spells")
		assert.Equal(t, got, "", "with no keyword, because Java reads it as public static final")

		_, err = backend.ConstantMods(&emit.Field{Name: "MAX", Type: ref("int")})
		assert.HasError(t, err, "a field without an initializer refuses")
		_, err = backend.ConstantMods(&emit.Field{
			Name: "MAX", Value: "8", Visibility: symbol.VisibilityPrivate,
		})
		assert.HasError(t, err, "a scope other than public refuses")
		_, err = backend.ConstantMods(&emit.Field{
			Name: "MAX", Value: "8", Mutability: symbol.MutabilityMutable,
		})
		assert.HasError(t, err, "a mutable field refuses, because an interface field is final")
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
		got, err = backend.MethodMods(&emit.Method{Name: "load", Abstract: true})
		assert.NoError(t, err, "an abstract method without a body spells")
		assert.Equal(t, got, "public abstract ", "access before abstract")
		_, err = backend.MethodMods(&emit.Method{
			Name: "load", Abstract: true, Body: emit.Body{Verbatim: "return 1;"},
		})
		assert.HasError(t, err, "an abstract method with a body refuses, because the signature drops it")
	})

	t.Run("SigMods", func(t *testing.T) {
		t.Parallel()

		got, err := backend.SigMods(&emit.Method{Name: "load"})
		assert.NoError(t, err, "a bare signature passes")
		assert.Equal(t, got, "", "implicitly public")

		got, err = backend.SigMods(&emit.Method{Name: "load", HasDefault: true})
		assert.NoError(t, err, "a method with a default body spells")
		assert.Equal(t, got, "default ", "as default at instance level")

		got, err = backend.SigMods(&emit.Method{
			Name: "make", HasDefault: true, Level: symbol.LevelType,
		})
		assert.NoError(t, err, "a type-level body spells")
		assert.Equal(t, got, "static ", "as static, which excludes default")

		_, err = backend.SigMods(&emit.Method{Name: "load", Override: true})
		assert.HasError(t, err, "an interface method overrides nothing")
		_, err = backend.SigMods(&emit.Method{Name: "load", Body: emit.Body{Verbatim: "return 1;"}})
		assert.HasError(t, err, "a body without a default refuses, because the signature drops it")
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
			Sealed:  true,
			Extends: []*emit.TypeRef{ref("Figure")},
			Permits: []*emit.TypeRef{ref("Circle"), ref("Square")},
		})
		assert.NoError(t, err, "a sealed heritage spells")
		assert.Equal(t, got, " extends Figure permits Circle, Square",
			"the enumerated subtypes last, behind permits")

		got, err = backend.Heritage(&emit.Struct{
			Name: "Shape", Sealed: true, Permits: []*emit.TypeRef{ref("Circle")},
		})
		assert.NoError(t, err, "a sealed class heritage spells")
		assert.Equal(t, got, " permits Circle", "its permits clause alone")

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
		_, err = backend.Heritage(&emit.Struct{Name: "Shape", Sealed: true})
		assert.HasError(t, err,
			"a sealed class without a permits clause refuses, because javac finds no "+
				"subclass in a file of its own")
		_, err = backend.Heritage(&emit.Interface{
			Name: "Shape", Permits: []*emit.TypeRef{ref("Circle")},
		})
		assert.HasError(t, err, "a permits clause on a type that is not sealed refuses")
		_, err = backend.Heritage(&emit.Enum{Name: "Phase"})
		assert.HasError(t, err, "an enum has no heritage clause here")
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
		assert.Equal(t, backend.Annotate(symbol.Annotations{
			{Name: "Deprecated"},
			{Name: "SuppressWarnings", Args: []string{`"unchecked"`}},
		}, "    "),
			"    @Deprecated\n    @SuppressWarnings(\"unchecked\")\n",
			"one line per annotation at the member's depth, arguments verbatim")
	})
}
