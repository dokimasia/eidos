// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
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

		got, err := backend.Params([]*emit.Param{
			{Name: "key", Type: ref("string")},
			{Name: "rest", Type: ref("number"), Variadic: symbol.VariadicPositional},
		})
		assert.NoError(t, err, "the list spells")
		assert.Equal(t, got, "key: string, ...rest: number[]",
			"colon-typed names and the rest marker with its array type")

		got, err = backend.Params([]*emit.Param{{Type: ref("string")}})
		assert.NoError(t, err, "an unnamed parameter spells")
		assert.Equal(t, got, "_: string", "under the discard name")

		got, err = backend.Params([]*emit.Param{
			{Name: "limit", Type: ref("number"), Default: "8"},
		})
		assert.NoError(t, err, "a defaulted parameter spells")
		assert.Equal(t, got, "limit: number = 8",
			"its default verbatim behind the equals sign")

		_, err = backend.Params([]*emit.Param{{
			Name: "rest", Type: ref("number"),
			Variadic: symbol.VariadicPositional, Default: "8",
		}})
		assert.HasError(t, err, "a rest parameter takes no default")
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

	t.Run("Mods", func(t *testing.T) {
		t.Parallel()

		got, err := backend.Mods(&emit.Struct{Name: "Row"})
		assert.NoError(t, err, "an unstated visibility spells")
		assert.Equal(t, got, "export ", "as export, because a generated API is consumed")

		got, err = backend.Mods(&emit.Struct{Name: "Row", Abstract: true})
		assert.NoError(t, err, "an abstract class spells")
		assert.Equal(t, got, "export abstract ", "abstract behind export")

		got, err = backend.Mods(&emit.Variable{
			Name: "count", Visibility: symbol.VisibilityPackage,
		})
		assert.NoError(t, err, "package scope spells")
		assert.Equal(t, got, "", "by omitting export, because the module is the scope")

		got, err = backend.Mods(&emit.Function{Name: "load", Async: true})
		assert.NoError(t, err, "an async function spells")
		assert.Equal(t, got, "export async ", "async behind export")

		_, err = backend.Mods(&emit.Struct{Name: "Row", Final: true})
		assert.HasError(t, err, "a final class refuses, because TypeScript seals nothing")
		_, err = backend.Mods(&emit.Alias{
			Name: "ID", Visibility: symbol.VisibilityProtected,
		})
		assert.HasError(t, err, "a protected module-level scope refuses")
		_, err = backend.Mods(&emit.Function{
			Name:        "load",
			Annotations: symbol.Annotations{{Name: "log"}},
		})
		assert.HasError(t, err, "decorators mark classes and members alone")
	})

	t.Run("MemberMods", func(t *testing.T) {
		t.Parallel()

		got, err := backend.MemberMods(&emit.Field{
			Name:       "key",
			Visibility: symbol.VisibilityPrivate,
			Level:      symbol.LevelType,
			Mutability: symbol.MutabilityImmutable,
		})
		assert.NoError(t, err, "a guarded field spells")
		assert.Equal(t, got, "private static readonly ",
			"accessibility, static, readonly, in TypeScript's stated order")

		got, err = backend.MemberMods(&emit.Method{
			Name: "load", Override: true, Async: true,
		})
		assert.NoError(t, err, "an overriding async method spells")
		assert.Equal(t, got, "override async ",
			"public stays implicit, override before async")

		_, err = backend.MemberMods(&emit.Method{Name: "load", Final: true})
		assert.HasError(t, err, "a final method refuses")
		_, err = backend.MemberMods(&emit.Field{
			Name: "key", Visibility: symbol.VisibilityPackage,
		})
		assert.HasError(t, err, "a package scope refuses on a class member")
	})

	t.Run("PropMods", func(t *testing.T) {
		t.Parallel()

		got, err := backend.PropMods(&emit.Field{
			Name: "key", Mutability: symbol.MutabilityImmutable,
		})
		assert.NoError(t, err, "an immutable property spells")
		assert.Equal(t, got, "readonly ", "as readonly")

		_, err = backend.PropMods(&emit.Field{
			Name: "key", Level: symbol.LevelType,
		})
		assert.HasError(t, err, "a static interface property refuses")
	})

	t.Run("SigMods", func(t *testing.T) {
		t.Parallel()

		got, err := backend.SigMods(&emit.Method{Name: "load"})
		assert.NoError(t, err, "a bare signature passes")
		assert.Equal(t, got, "", "and spells nothing")

		_, err = backend.SigMods(&emit.Method{Name: "load", Async: true})
		assert.HasError(t, err, "a stated modifier refuses on an interface method")
	})

	t.Run("AccessorKw", func(t *testing.T) {
		t.Parallel()

		got, err := backend.AccessorKw(&emit.Method{Name: "load"})
		assert.NoError(t, err, "an ordinary method passes")
		assert.Equal(t, got, "", "and spells nothing")

		got, err = backend.AccessorKw(&emit.Method{
			Name: "size", Accessor: symbol.AccessorGet,
			Returns: []*emit.Return{{Type: ref("number")}},
		})
		assert.NoError(t, err, "a getter spells")
		assert.Equal(t, got, "get ", "its keyword before the name")

		got, err = backend.AccessorKw(&emit.Method{
			Name: "size", Accessor: symbol.AccessorSet,
			Params: []*emit.Param{{Name: "v", Type: ref("number")}},
		})
		assert.NoError(t, err, "a setter spells")
		assert.Equal(t, got, "set ", "its keyword before the name")

		_, err = backend.AccessorKw(&emit.Method{
			Name: "size", Accessor: symbol.AccessorGet,
			Params:  []*emit.Param{{Name: "v", Type: ref("number")}},
			Returns: []*emit.Return{{Type: ref("number")}},
		})
		assert.HasError(t, err, "a getter taking parameters refuses")
		_, err = backend.AccessorKw(&emit.Method{
			Name: "size", Accessor: symbol.AccessorSet,
			Params:  []*emit.Param{{Name: "v", Type: ref("number")}},
			Returns: []*emit.Return{{Type: ref("number")}},
		})
		assert.HasError(t, err, "a setter returning refuses")
	})

	t.Run("Hard", func(t *testing.T) {
		t.Parallel()

		got, err := backend.Hard(&emit.Field{Name: "key", Hard: true})
		assert.NoError(t, err, "a hard-private field spells")
		assert.Equal(t, got, "#", "the prefix carrying the privacy")

		got, err = backend.Hard(&emit.Method{Name: "load"})
		assert.NoError(t, err, "an ordinary member passes")
		assert.Equal(t, got, "", "unprefixed")

		_, err = backend.Hard(&emit.Field{
			Name: "key", Hard: true, Visibility: symbol.VisibilityPrivate,
		})
		assert.HasError(t, err,
			"a stated visibility beside the hard name refuses, because "+
				"the privacy lives in the name")
	})

	t.Run("IndexSig", func(t *testing.T) {
		t.Parallel()

		got, err := backend.IndexSig(&emit.Method{
			Name:    "index",
			Indexer: true,
			Params:  []*emit.Param{{Name: "key", Type: ref("string")}},
			Returns: []*emit.Return{{Type: ref("Row")}},
		})
		assert.NoError(t, err, "an index signature spells")
		assert.Equal(t, got, "[key: string]: Row;", "whole, key and element typed")

		_, err = backend.IndexSig(&emit.Method{Name: "index", Indexer: true})
		assert.HasError(t, err, "one named typed key is required")
	})

	t.Run("Heritage", func(t *testing.T) {
		t.Parallel()

		got, err := backend.Heritage(&emit.Struct{
			Name:       "Row",
			Extends:    []*emit.TypeRef{ref("Base")},
			Implements: []*emit.TypeRef{ref("Keyed"), ref("Closer")},
		})
		assert.NoError(t, err, "a class heritage spells")
		assert.Equal(t, got, " extends Base implements Keyed, Closer",
			"one base behind extends, the contracts behind implements")

		got, err = backend.Heritage(&emit.Interface{
			Name:    "Store",
			Extends: []*emit.TypeRef{ref("Keyed"), ref("Closer")},
		})
		assert.NoError(t, err, "an interface heritage spells")
		assert.Equal(t, got, " extends Keyed, Closer",
			"the widened contracts joined behind extends")

		_, err = backend.Heritage(&emit.Struct{
			Name:    "Row",
			Extends: []*emit.TypeRef{ref("A"), ref("B")},
		})
		assert.HasError(t, err, "a second class base refuses")
		_, err = backend.Heritage(&emit.Struct{
			Name:   "Row",
			Embeds: []*emit.Embed{{Ref: ref("Base")}},
		})
		assert.HasError(t, err, "an embed refuses, because nothing promotes")
	})

	t.Run("Binding", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Binding(&emit.Variable{Name: "count"}), "let",
			"a mutable binding is let")
		assert.Equal(t, backend.Binding(&emit.Variable{
			Name: "count", Mutability: symbol.MutabilityImmutable,
		}), "const", "an immutable one is const")
	})

	t.Run("Decorators", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Decorators(nil), "", "no annotations, no lines")
		assert.Equal(t, backend.Decorators(symbol.Annotations{
			{Name: "injectable"},
			{Name: "route", Args: []string{`"/rows"`, "true"}},
		}, "  "),
			"  @injectable\n  @route(\"/rows\", true)\n",
			"one line per annotation at the member's depth, arguments verbatim")
	})
}
