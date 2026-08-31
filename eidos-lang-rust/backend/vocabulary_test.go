// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-rust/backend"
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
		assert.Equal(t, backend.Docs([]string{"One.", "Two."}), "/// One.\n/// Two.\n",
			"one outer doc comment per line")
		assert.Equal(t, backend.Docs([]string{"Inner."}, "    "), "    /// Inner.\n",
			"a member's doc indents to the member's depth")
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Spell(ref("Vec<Row>")), "Vec<Row>",
			"the source spelling passes through verbatim")
		assert.Equal(t, backend.Spell(nil), "()",
			"a declaration stating no type reaches the unit type, and the "+
				"compiler's refusal names the file")
		assert.Equal(t, backend.Spell(&emit.TypeRef{
			Spelling: "Map",
			Args: []*emit.TypeRef{
				ref("String"),
				{Spelling: "Vec", Args: []*emit.TypeRef{ref("Row")}},
			},
		}), "Map<String, Vec<Row>>",
			"an argument list spells in angle brackets, arguments recursing")
	})

	t.Run("TypeParams", func(t *testing.T) {
		t.Parallel()

		got, err := backend.TypeParams(nil)
		assert.NoError(t, err, "no parameters spell")
		assert.Equal(t, got, "", "as nothing")

		got, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "T", Bounds: []*emit.TypeRef{ref("Codec"), ref("Send")}},
			{Name: "U", Default: ref("String")},
		})
		assert.NoError(t, err, "bounds and defaults spell")
		assert.Equal(t, got, "<T: Codec + Send, U = String>",
			"bounds joined by plus signs, the default behind equals")

		got, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "N", Const: true, Type: ref("usize"), DefaultValue: "4"},
		})
		assert.NoError(t, err, "a value parameter spells")
		assert.Equal(t, got, "<const N: usize = 4>",
			"the const form with its value's type and default")

		_, err = backend.TypeParams([]*emit.TypeParam{
			{Name: "T", Variance: symbol.VarianceOut},
		})
		assert.HasError(t, err,
			"variance refuses, because Rust infers it from use")
	})

	t.Run("Binder", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Binder(ref("Row")), "",
			"a receiver taking no arguments binds nothing")
		assert.Equal(t, backend.Binder(&emit.TypeRef{
			Spelling: "Box", Args: []*emit.TypeRef{ref("T")},
		}), "<T>", "a generic receiver's arguments restate as the binder")
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Params([]*emit.Param{
			{Name: "key", Type: ref("String")},
			{Type: ref("u32")},
		}), "key: String, _: u32",
			"colon-typed names, the discard pattern where none was stated")
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Results(nil), "", "no results, no annotation")
		assert.Equal(t, backend.Results([]*emit.Return{{Type: ref("Row")}}),
			" -> Row", "one result is the arrow annotation")
		assert.Equal(t, backend.Results([]*emit.Return{
			{Type: ref("Row")}, {Type: ref("Error")},
		}), " -> (Row, Error)", "several return as one tuple")
	})

	t.Run("Vis", func(t *testing.T) {
		t.Parallel()

		got, err := backend.Vis(symbol.VisibilityUnknown, "Row")
		assert.NoError(t, err, "an unstated visibility spells")
		assert.Equal(t, got, "pub ", "as pub, because a generated API is consumed")

		got, err = backend.Vis(symbol.VisibilityInternal, "Row")
		assert.NoError(t, err, "internal spells")
		assert.Equal(t, got, "pub(crate) ", "as the crate scope")

		got, err = backend.Vis(symbol.VisibilityPackage, "Row")
		assert.NoError(t, err, "package scope spells")
		assert.Equal(t, got, "", "as nothing, which is module-private")

		_, err = backend.Vis(symbol.VisibilityPrivate, "Row")
		assert.HasError(t, err, "a type scope refuses, because Rust scopes by module")
	})

	t.Run("StructMods", func(t *testing.T) {
		t.Parallel()

		_, err := backend.StructMods(&emit.Struct{Name: "Row", Abstract: true})
		assert.HasError(t, err,
			"an abstract struct refuses, because every struct can be made")
	})

	t.Run("FnMods", func(t *testing.T) {
		t.Parallel()

		got, err := backend.FnMods(&emit.Function{Name: "load", Async: true})
		assert.NoError(t, err, "an async function spells")
		assert.Equal(t, got, "pub async ", "async behind the visibility")
	})

	t.Run("TraitFn", func(t *testing.T) {
		t.Parallel()

		got, err := backend.TraitFn(&emit.Method{Name: "load", Async: true})
		assert.NoError(t, err, "an async trait method spells")
		assert.Equal(t, got, "async ",
			"bare async, because a trait item takes no visibility")

		got, err = backend.TraitFn(&emit.Method{Name: "load", Abstract: true})
		assert.NoError(t, err, "an abstract trait method passes")
		assert.Equal(t, got, "", "a bodiless signature is the trait's shape")

		_, err = backend.TraitFn(&emit.Method{
			Name: "load", Visibility: symbol.VisibilityInternal,
		})
		assert.HasError(t, err, "a trait item carries the trait's visibility")
		_, err = backend.TraitFn(&emit.Method{Name: "load", Final: true})
		assert.HasError(t, err, "final refuses")
	})

	t.Run("ImplFn", func(t *testing.T) {
		t.Parallel()

		got, err := backend.ImplFn(&emit.Method{Name: "load", Async: true})
		assert.NoError(t, err, "an async impl method spells")
		assert.Equal(t, got, "pub async ", "async behind the visibility")

		_, err = backend.ImplFn(&emit.Method{Name: "load", Abstract: true})
		assert.HasError(t, err, "an impl method carries its body outright")
		_, err = backend.ImplFn(&emit.Method{Name: "load", HasDefault: true})
		assert.HasError(t, err, "default bodies belong to traits")
	})

	t.Run("SelfParams", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.SelfParams(&emit.Method{Name: "close"}),
			"&self", "the reference receiver alone")
		assert.Equal(t, backend.SelfParams(&emit.Method{
			Name:   "load",
			Params: []*emit.Param{{Name: "key", Type: ref("String")}},
		}), "&self, key: String", "the receiver before the parameters")
		assert.Equal(t, backend.SelfParams(&emit.Method{
			Name:   "make",
			Level:  symbol.LevelType,
			Params: []*emit.Param{{Name: "key", Type: ref("String")}},
		}), "key: String", "an associated function stands without a receiver")
	})

	t.Run("FieldMods", func(t *testing.T) {
		t.Parallel()

		got, err := backend.FieldMods(&emit.Field{
			Name: "key", Visibility: symbol.VisibilityInternal,
		})
		assert.NoError(t, err, "a crate-scoped field spells")
		assert.Equal(t, got, "pub(crate) ", "with its scope")

		_, err = backend.FieldMods(&emit.Field{Name: "key", Level: symbol.LevelType})
		assert.HasError(t, err, "a struct holds no statics")
		_, err = backend.FieldMods(&emit.Field{Name: "key", Value: "1"})
		assert.HasError(t, err, "a struct declares no field defaults")
	})

	t.Run("Attrs", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Attrs(nil), "", "no annotations, no lines")
		assert.Equal(t, backend.Attrs(emit.Annotations{
			{Name: "derive", Args: []string{"Debug", "Clone"}},
			{Name: "non_exhaustive"},
		}, "    "),
			"    #[derive(Debug, Clone)]\n    #[non_exhaustive]\n",
			"one outer attribute per annotation at the member's depth")
	})
}
