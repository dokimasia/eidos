// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ref is the fixture type reference.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// generic is the fixture instantiation of a bare name.
func generic(name string, args ...*emit.TypeRef) *emit.TypeRef {
	return &emit.TypeRef{Spelling: name, Args: args}
}

// variantOf is the fixture variant with one payload entry.
func variantOf(f *emit.Field) *emit.SumVariant {
	v := &emit.SumVariant{Name: "Write"}
	v.Fields.Append(f)
	return v
}

// payload spells one entry's payload and fails the test where the
// payload is refused.
func payload(t *testing.T, f *emit.Field) string {
	t.Helper()

	got, err := backend.SumPayload(variantOf(f))
	assert.NoError(t, err, "the payload spells")
	return got
}

// mustSpell returns a check over a helper's spelling and refusal: it
// fails the test where the helper refuses and returns the spelling
// otherwise.
func mustSpell(t *testing.T) func(string, error) string {
	t.Helper()

	return func(got string, err error) string {
		t.Helper()

		assert.NoError(t, err, "the helper spells")
		return got
	}
}

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

		assert.Equal(t, mustSpell(t)(backend.Spell(ref("Vec<Row>"))), "Vec<Row>",
			"the source spelling passes through verbatim")
		assert.Equal(t, mustSpell(t)(backend.Spell(generic("Map",
			ref("String"), generic("Vec", ref("Row")),
		))), "Map<String, Vec<Row>>",
			"an argument list spells in angle brackets, arguments recursing")

		_, err := backend.Spell(nil)
		assert.HasError(t, err,
			"a declaration stating no type refuses, because the unit type in its "+
				"place compiles and changes the declaration")
		assert.Contains(t, err.Error(), string(rust.Lang)+": ", "under the language's identity")
		_, err = backend.Spell(ref(""))
		assert.HasError(t, err, "an empty spelling refuses the same way")
		_, err = backend.Spell(generic("Map", ref("String"), generic("Vec", nil)))
		assert.HasError(t, err, "and so does an unstated argument at any depth")
	})

	t.Run("TypeParams", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, mustSpell(t)(backend.TypeParams(nil)), "", "no parameters spell nothing")
		assert.Equal(t, mustSpell(t)(backend.TypeParams([]*emit.TypeParam{
			{Name: "T", Bounds: []*emit.TypeRef{ref("Codec"), ref("Send")}},
			{Name: "U", Default: ref("String")},
		})), "<T: Codec + Send, U = String>",
			"bounds joined by plus signs, the default behind equals")
		assert.Equal(t, mustSpell(t)(backend.TypeParams([]*emit.TypeParam{
			{Name: "N", Const: true, Type: ref("usize"), DefaultValue: "4"},
		})), "<const N: usize = 4>",
			"the const form with its value's type and default")

		_, err := backend.TypeParams([]*emit.TypeParam{
			{Name: "T", Variance: symbol.VarianceOut},
		})
		assert.HasError(t, err,
			"variance refuses, because Rust infers it from use")
		_, err = backend.TypeParams([]*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{nil}}})
		assert.HasError(t, err, "a bound that states no type refuses")
		_, err = backend.TypeParams([]*emit.TypeParam{{Name: "N", Const: true}})
		assert.HasError(t, err, "a const parameter without its value's type refuses")
	})

	t.Run("ImplParams", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, mustSpell(t)(backend.ImplParams([]*emit.TypeParam{
			{Name: "T", Bounds: []*emit.TypeRef{ref("Codec")}, Default: ref("String")},
			{Name: "N", Const: true, Type: ref("usize"), DefaultValue: "4"},
		})), "<T: Codec, const N: usize>",
			"the bounds restate and the defaults drop, because the definition "+
				"states them and rustc refuses one on an impl")
	})

	t.Run("FnParams", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, mustSpell(t)(backend.FnParams([]*emit.TypeParam{
			{Name: "U", Bounds: []*emit.TypeRef{ref("Codec")}},
		})), "<U: Codec>", "a callable's parameter list spells its bounds")

		_, err := backend.FnParams([]*emit.TypeParam{{Name: "U", Default: ref("String")}})
		assert.HasError(t, err,
			"a default refuses, because Rust takes one on a type definition alone")
		_, err = backend.FnParams([]*emit.TypeParam{
			{Name: "N", Const: true, Type: ref("usize"), DefaultValue: "4"},
		})
		assert.HasError(t, err, "and so does a const parameter's default")
	})

	t.Run("TypeNames", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.TypeNames(nil), "", "no parameters, no list")
		assert.Equal(t, backend.TypeNames([]*emit.TypeParam{
			{Name: "T", Bounds: []*emit.TypeRef{ref("Codec")}},
			{Name: "N", Const: true, Type: ref("usize")},
		}), "<T, N>", "the names alone, the form an impl block's target repeats")
	})

	t.Run("Binder", func(t *testing.T) {
		t.Parallel()

		param := symbol.Identity{Lang: rust.Lang, Package: "store", Name: "T", Kind: symbol.KindTypeParam}
		declared := symbol.Identity{Lang: rust.Lang, Package: "store", Name: "Row", Kind: symbol.KindStruct}
		tests := []struct {
			name     string
			receiver *emit.TypeRef
			want     string
		}{
			{"a receiver taking no arguments binds nothing", ref("Row"), ""},
			{"a bare identifier binds as a parameter", generic("Box", ref("T")), "<T>"},
			{"String is concrete", generic("Wrapper", ref("String")), ""},
			{"a primitive is concrete", generic("Pair", ref("K"), ref("i32")), "<K>"},
			{"a target naming a type parameter binds", generic("Box",
				&emit.TypeRef{Spelling: "T", Target: param}), "<T>"},
			{"a target naming a declaration is concrete", generic("Box",
				&emit.TypeRef{Spelling: "Row", Target: declared}), ""},
			{"a path is concrete", generic("Box", ref("std::path::PathBuf")), ""},
			{"an instantiation is concrete", generic("Box", generic("Vec", ref("T"))), ""},
			{"a structural form is concrete", generic("Box",
				&emit.TypeRef{Spelling: "&str", Form: symbol.FormBorrow}), ""},
			{"the unit type is concrete", generic("Box", ref("()")), ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, mustSpell(t)(backend.Binder(tt.receiver)), tt.want, tt.name)
			})
		}

		_, err := backend.Binder(generic("Box", nil))
		assert.HasError(t, err, "an argument that states no type refuses")
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, mustSpell(t)(backend.Params([]*emit.Param{
			{Name: "key", Type: ref("String")},
			{Type: ref("u32")},
		})), "key: String, _: u32",
			"colon-typed names, the discard pattern where none is stated")

		_, err := backend.Params([]*emit.Param{{Name: "key"}})
		assert.HasError(t, err, "a parameter that states no type refuses")
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, mustSpell(t)(backend.Results(nil)), "", "no results, no annotation")
		assert.Equal(t, mustSpell(t)(backend.Results([]*emit.Return{{Type: ref("Row")}})),
			" -> Row", "one result is the arrow annotation")
		assert.Equal(t, mustSpell(t)(backend.Results([]*emit.Return{
			{Type: ref("Row")}, {Type: ref("Error")},
		})), " -> (Row, Error)", "several return as one tuple")

		_, err := backend.Results([]*emit.Return{{Name: "row"}})
		assert.HasError(t, err, "a result that states no type refuses")
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

		got, err := backend.StructMods(&emit.Struct{Name: "Row", Final: true})
		assert.NoError(t, err, "a final struct passes, because nothing subclasses")
		assert.Equal(t, got, "pub ", "and spells its visibility alone")

		_, err = backend.StructMods(&emit.Struct{Name: "Row", Abstract: true})
		assert.HasError(t, err,
			"an abstract struct refuses, because every struct can be made")
		_, err = backend.StructMods(&emit.Struct{
			Name: "Row", Extends: []*emit.TypeRef{ref("Base")},
		})
		assert.HasError(t, err,
			"a supertype refuses, because a struct neither inherits nor promotes")
	})

	t.Run("Supertraits", func(t *testing.T) {
		t.Parallel()

		got, err := backend.Supertraits(&emit.Interface{Name: "Store"})
		assert.NoError(t, err, "no supertraits spell")
		assert.Equal(t, got, "", "as nothing")

		got, err = backend.Supertraits(&emit.Interface{
			Name:    "Store",
			Extends: []*emit.TypeRef{ref("Keyed"), ref("Ord")},
		})
		assert.NoError(t, err, "supertraits spell")
		assert.Equal(t, got, ": Keyed + Ord",
			"joined by plus signs behind the colon")

		_, err = backend.Supertraits(&emit.Interface{
			Name:   "Store",
			Embeds: []*emit.Embed{{Ref: ref("Base")}},
		})
		assert.HasError(t, err, "a trait widens through supertraits alone")
		_, err = backend.Supertraits(&emit.Interface{
			Name: "Store", Extends: []*emit.TypeRef{nil},
		})
		assert.HasError(t, err, "a supertrait that states no type refuses")
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
		assert.HasError(t, err, "a trait item takes the trait's visibility")
		_, err = backend.TraitFn(&emit.Method{Name: "load", Final: true})
		assert.HasError(t, err,
			"final refuses, because every implementation may override a trait method")
		_, err = backend.TraitFn(&emit.Method{Name: "load", Override: true})
		assert.HasError(t, err, "override refuses, because a trait method overrides nothing")
		_, err = backend.TraitFn(&emit.Method{Name: "load", Body: emit.Body{Verbatim: "0"}})
		assert.HasError(t, err, "a body without a default refuses, because the signature drops it")
		got, err = backend.TraitFn(&emit.Method{
			Name: "load", HasDefault: true, Body: emit.Body{Verbatim: "0"},
		})
		assert.NoError(t, err, "a default body passes")
		assert.Equal(t, got, "", "and the template places it")
	})

	t.Run("ConstType", func(t *testing.T) {
		t.Parallel()

		got, err := backend.ConstType(&emit.Constant{Name: "MAX", Type: ref("u32"), Value: "8"})
		assert.NoError(t, err, "a typed constant with a value spells")
		assert.Equal(t, got, "u32", "its stated type")

		_, err = backend.ConstType(&emit.Constant{Name: "MAX", Value: "8"})
		assert.HasError(t, err, "a constant without a type refuses, because rustc requires one")
		_, err = backend.ConstType(&emit.Constant{Name: "MAX", Type: ref("u32")})
		assert.HasError(t, err, "a constant without a value refuses, because const MAX: u32 = declares nothing")
	})

	t.Run("ImplFn", func(t *testing.T) {
		t.Parallel()

		got, err := backend.ImplFn(&emit.Method{Name: "load", Async: true})
		assert.NoError(t, err, "an async impl method spells")
		assert.Equal(t, got, "pub async ", "async behind the visibility")

		got, err = backend.ImplFn(&emit.Method{Name: "load", Final: true})
		assert.NoError(t, err, "a final impl method passes, because nothing overrides an inherent method")
		assert.Equal(t, got, "pub ", "and spells its visibility alone")

		_, err = backend.ImplFn(&emit.Method{Name: "load", Abstract: true})
		assert.HasError(t, err, "an impl method states its body outright")
		_, err = backend.ImplFn(&emit.Method{Name: "load", HasDefault: true})
		assert.HasError(t, err, "default bodies belong to traits")
		_, err = backend.ImplFn(&emit.Method{Name: "load", Override: true})
		assert.HasError(t, err, "an inherent method overrides nothing")
	})

	t.Run("SelfParams", func(t *testing.T) {
		t.Parallel()

		key := []*emit.Param{{Name: "key", Type: ref("String")}}
		tests := []struct {
			name   string
			method *emit.Method
			want   string
		}{
			{
				"the shared borrow where no receiver is stated",
				&emit.Method{Name: "close"}, "&self",
			},
			{
				"the receiver before the parameters",
				&emit.Method{Name: "load", Params: key}, "&self, key: String",
			},
			{
				"an associated function takes no receiver",
				&emit.Method{Name: "make", Level: symbol.LevelType, Params: key}, "key: String",
			},
			{
				"Self by value spells self",
				&emit.Method{Name: "into", Receiver: &emit.Param{Type: ref("Self")}}, "self",
			},
			{
				"&Self spells &self",
				&emit.Method{Name: "get", Receiver: &emit.Param{Name: "self", Type: ref("&Self")}}, "&self",
			},
			{
				"&mut Self spells &mut self",
				&emit.Method{Name: "set", Receiver: &emit.Param{Type: ref("&mut Self")}, Params: key},
				"&mut self, key: String",
			},
			{
				"any other receiver type spells the typed form",
				&emit.Method{Name: "pin", Receiver: &emit.Param{Type: generic("Box", ref("Self"))}},
				"self: Box<Self>",
			},
			{
				"a receiver's comment follows as a block comment",
				&emit.Method{Name: "get", Receiver: &emit.Param{Type: ref("&Self"), Comment: "shared"}},
				"&self /* shared */",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, mustSpell(t)(backend.SelfParams(tt.method)), tt.want, tt.name)
			})
		}

		refused := []struct {
			name   string
			method *emit.Method
		}{
			{"a receiver on an associated function", &emit.Method{
				Name: "make", Level: symbol.LevelType, Receiver: &emit.Param{Type: ref("Self")},
			}},
			{"a receiver named other than self", &emit.Method{
				Name: "get", Receiver: &emit.Param{Name: "this", Type: ref("&Self")},
			}},
			{"a receiver that states no type", &emit.Method{
				Name: "get", Receiver: &emit.Param{Name: "self"},
			}},
			{"a parameter that states no type", &emit.Method{
				Name: "get", Params: []*emit.Param{{Name: "key"}},
			}},
		}
		for _, tt := range refused {
			t.Run(tt.name+" refuses", func(t *testing.T) {
				t.Parallel()
				_, err := backend.SelfParams(tt.method)
				assert.HasError(t, err, tt.name)
			})
		}
	})

	t.Run("FieldMods", func(t *testing.T) {
		t.Parallel()

		got, err := backend.FieldMods(&emit.Field{
			Name: "key", Visibility: symbol.VisibilityInternal,
		})
		assert.NoError(t, err, "a crate-scoped field spells")
		assert.Equal(t, got, "pub(crate) ", "with its scope")

		_, err = backend.FieldMods(&emit.Field{Name: "key", Level: symbol.LevelType})
		assert.HasError(t, err, "a struct has no statics")
		_, err = backend.FieldMods(&emit.Field{Name: "key", Mutability: symbol.MutabilityImmutable})
		assert.HasError(t, err, "a field's mutability follows its owning binding")
		_, err = backend.FieldMods(&emit.Field{Name: "key", Value: "1"})
		assert.HasError(t, err, "a struct declares no field defaults")
	})

	t.Run("AssocType", func(t *testing.T) {
		t.Parallel()

		got, err := backend.AssocType(&emit.Alias{Name: "Item"})
		assert.NoError(t, err, "a bare alias spells")
		assert.Equal(t, got, "type Item;", "as the associated type")

		_, err = backend.AssocType(&emit.Struct{Name: "Inner"})
		assert.HasError(t, err, "a trait nests associated types alone")
		_, err = backend.AssocType(&emit.Alias{Name: "Item", Target: ref("Row")})
		assert.HasError(t, err, "the implementation supplies the target")
		_, err = backend.AssocType(&emit.Alias{Name: "Item", Defined: true})
		assert.HasError(t, err, "an associated type defines nothing itself")
		got, err = backend.AssocType(&emit.Alias{
			Name: "Item", TypeParams: []*emit.TypeParam{{Name: "T"}},
		})
		assert.NoError(t, err, "a generic associated type spells")
		assert.Equal(t, got, "type Item<T>;",
			"its parameter list in angle brackets")
		_, err = backend.AssocType(&emit.Alias{
			Name: "Item", TypeParams: []*emit.TypeParam{{Name: "T", Default: ref("String")}},
		})
		assert.HasError(t, err, "a default on its parameter refuses")
		_, err = backend.AssocType(&emit.Alias{
			Name: "Item", Visibility: symbol.VisibilityInternal,
		})
		assert.HasError(t, err, "a trait item takes the trait's visibility")
	})

	t.Run("SumMods", func(t *testing.T) {
		t.Parallel()

		got, err := backend.SumMods(&emit.Sum{Name: "Shape"})
		assert.NoError(t, err, "a plain sum spells")
		assert.Equal(t, got, "pub ", "public by default")

		withMethods := &emit.Sum{Name: "Shape"}
		withMethods.Methods.Append(&emit.Method{Name: "area"})
		_, err = backend.SumMods(withMethods)
		assert.HasError(t, err, "behaviour goes in impl blocks")
	})

	t.Run("SumPayload", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, payload(t, &emit.Field{Name: "radius", Type: ref("f64")}),
			" { radius: f64 }", "named entries brace")
		assert.Equal(t, payload(t, &emit.Field{Type: ref("String")}),
			"(String)", "unnamed entries parenthesise")
		got, err := backend.SumPayload(&emit.SumVariant{Name: "Quit"})
		assert.NoError(t, err, "an empty payload spells")
		assert.Equal(t, got, "", "as nothing")

		mixed := &emit.SumVariant{Name: "Write"}
		mixed.Fields.Append(
			&emit.Field{Name: "at", Type: ref("u32")},
			&emit.Field{Type: ref("String")},
		)
		_, err = backend.SumPayload(mixed)
		assert.HasError(t, err, "a payload spells one way")

		refused := []*emit.Field{
			{Name: "at", Doc: []string{"documented"}},
			{Name: "at", Visibility: symbol.VisibilityPublic},
			{Name: "at", Level: symbol.LevelType},
			{Name: "at", Mutability: symbol.MutabilityImmutable},
			{Name: "at", Value: "1"},
			{Name: "at"},
		}
		for _, f := range refused {
			_, err := backend.SumPayload(variantOf(f))
			assert.HasError(t, err,
				"an inline entry states a name and a type alone")
		}
	})

	t.Run("Attrs", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, backend.Attrs(nil), "", "no annotations, no lines")
		assert.Equal(t, backend.Attrs(symbol.Annotations{
			{Name: "derive", Args: []string{"Debug", "Clone"}},
			{Name: "non_exhaustive"},
		}, "    "),
			"    #[derive(Debug, Clone)]\n    #[non_exhaustive]\n",
			"one outer attribute per annotation at the member's depth")
	})
}
