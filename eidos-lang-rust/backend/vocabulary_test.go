// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/rust/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spelling fixture: the modules a reference uses items from and
// the items it names.
const (
	// storeModule and legacyModule both declare rowName.
	storeModule  = "crate/store"
	legacyModule = "crate/legacy"
	rowName      = "Row"
	rowName2     = rowName + "2"
	baseName     = "Base"
	// keyPath is a path inside rowName.
	keyPath = "::Key"
)

// ref returns an unresolved reference spelled s.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// generic returns the instantiation of a bare name.
func generic(name string, args ...*emit.TypeRef) *emit.TypeRef {
	return &emit.TypeRef{Spelling: name, Args: args}
}

// paramRef returns a reference to the type parameter name, targeted
// at the parameter the way Link targets one.
func paramRef(name string) *emit.TypeRef {
	return &emit.TypeRef{
		Spelling: name,
		Target:   symbol.Identity{Lang: rust.Lang, Package: storeModule, Name: name, Kind: symbol.KindTypeParam},
	}
}

// imported returns a reference to an item of module, spelled s.
func imported(module, s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s, Package: module} }

// speller returns a speller over a fresh set, and the set.
func speller() (backend.Speller, *render.ImportSet) {
	set := &render.ImportSet{}
	return backend.NewSpeller(set), set
}

// variantOf returns a variant with one payload entry.
func variantOf(f *emit.Field) *emit.SumVariant {
	v := &emit.SumVariant{Name: "Write"}
	v.Fields.Append(f)
	return v
}

// must returns a check over a helper's spelling and error. The check
// fails the test where the helper returns an error, and returns the
// spelling otherwise.
func must(tb assert.TB) func(string, error) string {
	tb.Helper()

	return func(got string, err error) string {
		tb.Helper()

		assert.NoError(tb, err, "the helper spells")
		return got
	}
}

// The vocabulary is what every kind template spells through, so
// each helper's output is pinned byte for byte, and every use a
// spelling needs is pinned beside it.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Funcs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a spell helper bound to the file's import set", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			spell, is := backend.Funcs(&set)[backend.FuncSpell].(func(*emit.TypeRef) (string, error))
			assert.True(t, is, "the helper spells a reference")
			_, err := spell(imported(storeModule, rowName))
			assert.NoError(t, err, "the reference spells")
			assert.Equal(t, set.Paths(), []string{storeModule}, "the use is the file's")
		})
	})

	t.Run("Docs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no lines", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Docs(nil), "", "no comment")
		})

		t.Run("writes one outer doc comment per line", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Docs([]string{"One.", "Two."}), "/// One.\n/// Two.\n", "at the top level")
		})

		t.Run("writes at the given depth", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Docs([]string{"Inner."}, "    "), "    /// Inner.\n", "at the member's depth")
		})
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the spelling of a reference that names no module", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, must(t)(s.Spell(ref("Vec<Row>"))), "Vec<Row>", "the spelling as written")
			assert.Equal(t, set.Len(), 0, "no use")
		})

		t.Run("writes an argument list in angle brackets", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, must(t)(s.Spell(generic("Map", ref("String"), generic("Vec", ref(rowName))))),
				"Map<String, Vec<Row>>", "the arguments recurse")
		})

		t.Run("uses the item a reference names", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, must(t)(s.Spell(imported(storeModule, rowName))), rowName, "the item's name")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: storeModule, Name: rowName}}, "the item's use")
		})

		t.Run("uses a target of the language from its module", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			target := ref(rowName)
			target.Target = symbol.Identity{Lang: rust.Lang, Package: storeModule, Name: rowName}
			assert.Equal(t, must(t)(s.Spell(target)), rowName, "the item's name")
			assert.Equal(t, set.Paths(), []string{storeModule}, "used from the declaring module")
		})

		t.Run("renames an item another use binds", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			must(t)(s.Spell(imported(storeModule, rowName)))
			assert.Equal(t, must(t)(s.Spell(imported(legacyModule, rowName))), rowName2,
				"the second item binds the next free name")
		})

		t.Run("writes a path inside a used item behind its bound name", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			must(t)(s.Spell(imported(storeModule, rowName)))
			assert.Equal(t, must(t)(s.Spell(imported(legacyModule, rowName+keyPath))), rowName2+keyPath,
				"the first segment is the use")
		})

		t.Run("returns an error for a composite whose child is used under another name", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			must(t)(s.Spell(imported(storeModule, rowName)))
			borrow := &emit.TypeRef{
				Form: symbol.FormBorrow, Spelling: "&Row", Elems: []*emit.TypeRef{imported(legacyModule, rowName)},
			}
			_, err := s.Spell(borrow)
			assert.HasError(t, err, "the spelling does not follow from the structure")
		})

		tests := []struct {
			name string
			give *emit.TypeRef
		}{
			{name: "returns an error for a missing reference", give: nil},
			{name: "returns an error for an empty spelling", give: ref("")},
			{
				name: "returns an error for an unstated argument at depth",
				give: generic("Map", ref("String"), generic("Vec", nil)),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				_, err := s.Spell(tt.give)
				assert.HasError(t, err, "the unit type in its place changes the declaration")
				assert.Contains(t, err.Error(), string(rust.Lang)+": ", "under the language's prefix")
			})
		}
	})

	t.Run("TypeParams", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no parameters", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, must(t)(s.TypeParams(nil)), "", "no list")
		})

		t.Run("joins the bounds with plus signs behind a colon", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			ps := []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{ref("Codec"), ref("Send")}}}
			assert.Equal(t, must(t)(s.TypeParams(ps)), "<T: Codec + Send>", "the bounds joined")
		})

		t.Run("writes a default behind an equals sign", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, must(t)(s.TypeParams([]*emit.TypeParam{{Name: "U", Default: ref("String")}})),
				"<U = String>", "a type definition's default")
		})

		t.Run("writes a value parameter in the const form", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			ps := []*emit.TypeParam{{Name: "N", Const: true, Type: ref("usize"), DefaultValue: "4"}}
			assert.Equal(t, must(t)(s.TypeParams(ps)), "<const N: usize = 4>", "its value's type and default")
		})

		tests := []struct {
			name string
			give *emit.TypeParam
		}{
			{name: "returns an error for a variance", give: &emit.TypeParam{Name: "T", Variance: symbol.VarianceOut}},
			{
				name: "returns an error for a bound that states no type",
				give: &emit.TypeParam{Name: "T", Bounds: []*emit.TypeRef{nil}},
			},
			{
				name: "returns an error for a const parameter without its value's type",
				give: &emit.TypeParam{Name: "N", Const: true},
			},
			{
				name: "returns an error for a default that states no type",
				give: &emit.TypeParam{Name: "T", Default: ref("")},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				_, err := s.TypeParams([]*emit.TypeParam{tt.give})
				assert.HasError(t, err, "Rust cannot state the parameter")
			})
		}
	})

	t.Run("ImplParams", func(t *testing.T) {
		t.Parallel()

		t.Run("restates the bounds without the defaults", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, must(t)(s.ImplParams([]*emit.TypeParam{
				{Name: "T", Bounds: []*emit.TypeRef{ref("Codec")}, Default: ref("String")},
				{Name: "N", Const: true, Type: ref("usize"), DefaultValue: "4"},
			})), "<T: Codec, const N: usize>", "rustc refuses a default on an impl")
		})
	})

	t.Run("FnParams", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a callable's bounds", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, must(t)(s.FnParams([]*emit.TypeParam{{Name: "U", Bounds: []*emit.TypeRef{ref("Codec")}}})),
				"<U: Codec>", "the bound behind the colon")
		})

		tests := []struct {
			name string
			give *emit.TypeParam
		}{
			{
				name: "returns an error for a type parameter's default",
				give: &emit.TypeParam{Name: "U", Default: ref("String")},
			},
			{
				name: "returns an error for a const parameter's default",
				give: &emit.TypeParam{Name: "N", Const: true, Type: ref("usize"), DefaultValue: "4"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				_, err := s.FnParams([]*emit.TypeParam{tt.give})
				assert.HasError(t, err, "Rust takes a default on a type definition alone")
			})
		}
	})

	t.Run("TypeNames", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no parameters", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.TypeNames(nil), "", "no list")
		})

		t.Run("writes the names alone", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.TypeNames([]*emit.TypeParam{
				{Name: "T", Bounds: []*emit.TypeRef{ref("Codec")}},
				{Name: "N", Const: true, Type: ref("usize")},
			}), "<T, N>", "the form an impl block's target repeats")
		})
	})

	t.Run("Binder", func(t *testing.T) {
		t.Parallel()

		declared := symbol.Identity{Lang: rust.Lang, Package: storeModule, Name: rowName, Kind: symbol.KindStruct}
		tests := []struct {
			name string
			give *emit.TypeRef
			want string
		}{
			{name: "writes nothing for a receiver taking no arguments", give: ref(rowName), want: ""},
			{name: "binds a target naming a type parameter", give: generic("Box", paramRef("T")), want: "<T>"},
			{name: "binds nothing for a bare name without a target", give: generic("Box", ref("T")), want: ""},
			{
				name: "binds nothing for a standard type the resolution left open",
				give: generic("Wrapper", ref("PathBuf")), want: "",
			},
			{name: "binds nothing for String", give: generic("Wrapper", ref("String")), want: ""},
			{
				name: "binds a parameter beside a primitive",
				give: generic("Pair", paramRef("K"), ref("i32")), want: "<K>",
			},
			{
				name: "binds nothing for a target naming a declaration",
				give: generic("Box", &emit.TypeRef{Spelling: rowName, Target: declared}), want: "",
			},
			{name: "binds nothing for a path", give: generic("Box", ref("std::path::PathBuf")), want: ""},
			{name: "binds nothing for an instantiation", give: generic("Box", generic("Vec", ref("T"))), want: ""},
			{
				name: "binds nothing for a structural form",
				give: generic("Box", &emit.TypeRef{Spelling: "&str", Form: symbol.FormBorrow}), want: "",
			},
			{name: "binds nothing for the unit type", give: generic("Box", ref("()")), want: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, must(t)(backend.Binder(tt.give)), tt.want, "the binder")
			})
		}

		t.Run("returns an error for an argument that states no type", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Binder(generic("Box", nil))
			assert.HasError(t, err, "the binder is refused")
		})
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		t.Run("writes each name before its colon-typed type", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, must(t)(s.Params([]*emit.Param{{Name: "key", Type: ref("String")}})), "key: String",
				"the Rust order")
		})

		t.Run("binds an unnamed parameter to the discard pattern", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, must(t)(s.Params([]*emit.Param{{Type: ref("u32")}})), "_: u32", "the discard pattern")
		})

		t.Run("returns an error for a parameter that states no type", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Params([]*emit.Param{{Name: "key"}})
			assert.HasError(t, err, "the list is refused")
		})
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no results", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, must(t)(s.Results(nil)), "", "no annotation")
		})

		t.Run("writes one result behind the arrow", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, must(t)(s.Results([]*emit.Return{{Type: ref(rowName)}})), " -> Row", "the annotation")
		})

		t.Run("writes several results as one tuple", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, must(t)(s.Results([]*emit.Return{{Type: ref(rowName)}, {Type: ref("Error")}})),
				" -> (Row, Error)", "a tuple")
		})

		t.Run("returns an error for a result that states no type", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Results([]*emit.Return{{Name: "row"}})
			assert.HasError(t, err, "the annotation is refused")
		})
	})

	t.Run("Vis", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Visibility
			want string
		}{
			{name: "writes pub for an unstated visibility", give: symbol.VisibilityUnknown, want: "pub "},
			{name: "writes pub(crate) for internal", give: symbol.VisibilityInternal, want: "pub(crate) "},
			{name: "writes nothing for package scope", give: symbol.VisibilityPackage, want: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, must(t)(backend.Vis(tt.give, rowName)), tt.want, "the keyword")
			})
		}

		t.Run("returns an error for a type scope", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Vis(symbol.VisibilityPrivate, rowName)
			assert.HasError(t, err, "Rust scopes by module")
		})
	})

	t.Run("StructMods", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a final struct's visibility alone", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, must(t)(backend.StructMods(&emit.Struct{Name: rowName, Final: true})), "pub ",
				"nothing subclasses")
		})

		tests := []struct {
			name string
			give *emit.Struct
		}{
			{name: "returns an error for an abstract struct", give: &emit.Struct{Name: rowName, Abstract: true}},
			{
				name: "returns an error for a struct stating supertypes",
				give: &emit.Struct{Name: rowName, Extends: []*emit.TypeRef{ref(baseName)}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.StructMods(tt.give)
				assert.HasError(t, err, "a struct cannot state it")
			})
		}
	})

	t.Run("Supertraits", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no supertraits", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, must(t)(s.Supertraits(&emit.Interface{Name: "Store"})), "", "no bounds")
		})

		t.Run("joins the supertraits with plus signs behind a colon", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			store := &emit.Interface{Name: "Store", Extends: []*emit.TypeRef{ref("Keyed"), ref("Ord")}}
			assert.Equal(t, must(t)(s.Supertraits(store)), ": Keyed + Ord", "the bounds joined")
		})

		tests := []struct {
			name string
			give *emit.Interface
		}{
			{
				name: "returns an error for an embed",
				give: &emit.Interface{Name: "Store", Embeds: []*emit.Embed{{Ref: ref(baseName)}}},
			},
			{
				name: "returns an error for a supertrait that states no type",
				give: &emit.Interface{Name: "Store", Extends: []*emit.TypeRef{nil}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				_, err := s.Supertraits(tt.give)
				assert.HasError(t, err, "a trait cannot state it")
			})
		}
	})

	t.Run("FnMods", func(t *testing.T) {
		t.Parallel()

		t.Run("writes async behind the visibility", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, must(t)(backend.FnMods(&emit.Function{Name: "load", Async: true})), "pub async ",
				"the keywords")
		})
	})

	t.Run("TraitFn", func(t *testing.T) {
		t.Parallel()

		t.Run("writes async bare for an async trait method", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, must(t)(backend.TraitFn(&emit.Method{Name: "load", Async: true})), "async ",
				"a trait item takes no visibility")
		})

		t.Run("returns nothing for an abstract trait method", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, must(t)(backend.TraitFn(&emit.Method{Name: "load", Abstract: true})), "",
				"the trait's shape")
		})

		t.Run("returns nothing for a method with a default body", func(t *testing.T) {
			t.Parallel()

			load := &emit.Method{Name: "load", HasDefault: true, Body: emit.Body{Verbatim: "0"}}
			assert.Equal(t, must(t)(backend.TraitFn(load)), "", "the template places the body")
		})

		tests := []struct {
			name string
			give *emit.Method
		}{
			{
				name: "returns an error for a stated scope",
				give: &emit.Method{Name: "load", Visibility: symbol.VisibilityInternal},
			},
			{name: "returns an error for a final method", give: &emit.Method{Name: "load", Final: true}},
			{name: "returns an error for an override marker", give: &emit.Method{Name: "load", Override: true}},
			{
				name: "returns an error for a body without a default",
				give: &emit.Method{Name: "load", Body: emit.Body{Verbatim: "0"}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.TraitFn(tt.give)
				assert.HasError(t, err, "a trait method cannot state it")
			})
		}
	})

	t.Run("ConstType", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a constant's stated type", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, must(t)(s.ConstType(&emit.Constant{Name: "MAX", Type: ref("u32"), Value: "8"})), "u32",
				"the type")
		})

		tests := []struct {
			name string
			give *emit.Constant
		}{
			{name: "returns an error for a constant without a type", give: &emit.Constant{Name: "MAX", Value: "8"}},
			{
				name: "returns an error for a constant without a value",
				give: &emit.Constant{Name: "MAX", Type: ref("u32")},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				_, err := s.ConstType(tt.give)
				assert.HasError(t, err, "rustc requires both")
			})
		}
	})

	t.Run("ImplFn", func(t *testing.T) {
		t.Parallel()

		t.Run("writes async behind the visibility", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, must(t)(backend.ImplFn(&emit.Method{Name: "load", Async: true})), "pub async ",
				"the keywords")
		})

		t.Run("writes a final method's visibility alone", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, must(t)(backend.ImplFn(&emit.Method{Name: "load", Final: true})), "pub ",
				"nothing overrides an inherent method")
		})

		tests := []struct {
			name string
			give *emit.Method
		}{
			{name: "returns an error for an abstract method", give: &emit.Method{Name: "load", Abstract: true}},
			{name: "returns an error for a default body", give: &emit.Method{Name: "load", HasDefault: true}},
			{name: "returns an error for an override marker", give: &emit.Method{Name: "load", Override: true}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.ImplFn(tt.give)
				assert.HasError(t, err, "an impl method cannot state it")
			})
		}
	})

	t.Run("SelfParams", func(t *testing.T) {
		t.Parallel()

		key := []*emit.Param{{Name: "key", Type: ref("String")}}
		tests := []struct {
			name    string
			give    *emit.Method
			want    string
			wantErr bool
		}{
			{
				name: "writes the shared borrow where no receiver is stated",
				give: &emit.Method{Name: "close"}, want: "&self",
			},
			{
				name: "writes the receiver before the parameters",
				give: &emit.Method{Name: "load", Params: key}, want: "&self, key: String",
			},
			{
				name: "writes no receiver for an associated function",
				give: &emit.Method{Name: "make", Level: symbol.LevelType, Params: key}, want: "key: String",
			},
			{
				name: "writes self for Self by value",
				give: &emit.Method{Name: "into", Receiver: &emit.Param{Type: ref("Self")}}, want: "self",
			},
			{
				name: "writes &self for &Self",
				give: &emit.Method{Name: "get", Receiver: &emit.Param{Name: "self", Type: ref("&Self")}},
				want: "&self",
			},
			{
				name: "writes &mut self for &mut Self",
				give: &emit.Method{Name: "set", Receiver: &emit.Param{Type: ref("&mut Self")}, Params: key},
				want: "&mut self, key: String",
			},
			{
				name: "writes the typed form for any other receiver type",
				give: &emit.Method{Name: "pin", Receiver: &emit.Param{Type: generic("Box", ref("Self"))}},
				want: "self: Box<Self>",
			},
			{
				name: "writes a receiver's comment as a block comment",
				give: &emit.Method{Name: "get", Receiver: &emit.Param{Type: ref("&Self"), Comment: "shared"}},
				want: "&self /* shared */",
			},
			{
				name: "returns an error for a receiver on an associated function",
				give: &emit.Method{
					Name: "make", Level: symbol.LevelType, Receiver: &emit.Param{Type: ref("Self")},
				},
				wantErr: true,
			},
			{
				name:    "returns an error for a receiver named other than self",
				give:    &emit.Method{Name: "get", Receiver: &emit.Param{Name: "this", Type: ref("&Self")}},
				wantErr: true,
			},
			{
				name:    "returns an error for a receiver that states no type",
				give:    &emit.Method{Name: "get", Receiver: &emit.Param{Name: "self"}},
				wantErr: true,
			},
			{
				name:    "returns an error for a parameter that states no type",
				give:    &emit.Method{Name: "get", Params: []*emit.Param{{Name: "key"}}},
				wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				got, err := s.SelfParams(tt.give)
				if tt.wantErr {
					assert.HasError(t, err, "Rust cannot state the receiver")
					return
				}
				assert.NoError(t, err, "the list spells")
				assert.Equal(t, got, tt.want, "the parameter list")
			})
		}
	})

	t.Run("FieldMods", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a crate-scoped field's scope", func(t *testing.T) {
			t.Parallel()

			key := &emit.Field{Name: "key", Visibility: symbol.VisibilityInternal}
			assert.Equal(t, must(t)(backend.FieldMods(key)), "pub(crate) ", "the crate scope")
		})

		tests := []struct {
			name string
			give *emit.Field
		}{
			{name: "returns an error for a type-level field", give: &emit.Field{Name: "key", Level: symbol.LevelType}},
			{
				name: "returns an error for an immutable field",
				give: &emit.Field{Name: "key", Mutability: symbol.MutabilityImmutable},
			},
			{name: "returns an error for a field initializer", give: &emit.Field{Name: "key", Value: "1"}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.FieldMods(tt.give)
				assert.HasError(t, err, "a field cannot state it")
			})
		}
	})

	t.Run("AssocType", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a bare alias as the associated type", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, must(t)(s.AssocType(&emit.Alias{Name: "Item"})), "type Item;", "the declaration")
		})

		t.Run("writes a generic associated type's parameters", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			item := &emit.Alias{Name: "Item", TypeParams: []*emit.TypeParam{{Name: "T"}}}
			assert.Equal(t, must(t)(s.AssocType(item)), "type Item<T>;", "in angle brackets")
		})

		tests := []struct {
			name string
			give symbol.Symbol
		}{
			{name: "returns an error for a nested struct", give: &emit.Struct{Name: "Inner"}},
			{
				name: "returns an error for an alias with a target",
				give: &emit.Alias{Name: "Item", Target: ref(rowName)},
			},
			{name: "returns an error for a defined alias", give: &emit.Alias{Name: "Item", Defined: true}},
			{
				name: "returns an error for a parameter default",
				give: &emit.Alias{Name: "Item", TypeParams: []*emit.TypeParam{{Name: "T", Default: ref("String")}}},
			},
			{
				name: "returns an error for a stated visibility",
				give: &emit.Alias{Name: "Item", Visibility: symbol.VisibilityInternal},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				_, err := s.AssocType(tt.give)
				assert.HasError(t, err, "an associated type cannot state it")
			})
		}
	})

	t.Run("SumMods", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a plain sum public", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, must(t)(backend.SumMods(&emit.Sum{Name: "Shape"})), "pub ", "public by default")
		})

		t.Run("returns an error for a sum with methods", func(t *testing.T) {
			t.Parallel()

			withMethods := &emit.Sum{Name: "Shape"}
			withMethods.Methods.Append(&emit.Method{Name: "area"})
			_, err := backend.SumMods(withMethods)
			assert.HasError(t, err, "behaviour goes in impl blocks")
		})
	})

	t.Run("SumPayload", func(t *testing.T) {
		t.Parallel()

		payload := func(t *testing.T, v *emit.SumVariant) string {
			t.Helper()

			s, _ := speller()
			return must(t)(s.SumPayload(v))
		}

		t.Run("braces named entries", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, payload(t, variantOf(&emit.Field{Name: "radius", Type: ref("f64")})), " { radius: f64 }",
				"a struct variant")
		})

		t.Run("parenthesises unnamed entries", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, payload(t, variantOf(&emit.Field{Type: ref("String")})), "(String)", "a tuple variant")
		})

		t.Run("returns nothing for an empty payload", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, payload(t, &emit.SumVariant{Name: "Quit"}), "", "a unit variant")
		})

		t.Run("returns an error for a payload of mixed entries", func(t *testing.T) {
			t.Parallel()

			mixed := &emit.SumVariant{Name: "Write"}
			mixed.Fields.Append(&emit.Field{Name: "at", Type: ref("u32")}, &emit.Field{Type: ref("String")})
			s, _ := speller()
			_, err := s.SumPayload(mixed)
			assert.HasError(t, err, "a payload spells one way")
		})

		tests := []struct {
			name string
			give *emit.Field
		}{
			{
				name: "returns an error for a documented entry",
				give: &emit.Field{Name: "at", Doc: []string{"documented"}},
			},
			{
				name: "returns an error for an entry with a visibility",
				give: &emit.Field{Name: "at", Visibility: symbol.VisibilityPublic},
			},
			{name: "returns an error for a type-level entry", give: &emit.Field{Name: "at", Level: symbol.LevelType}},
			{
				name: "returns an error for an immutable entry",
				give: &emit.Field{Name: "at", Mutability: symbol.MutabilityImmutable},
			},
			{name: "returns an error for an entry with a default", give: &emit.Field{Name: "at", Value: "1"}},
			{name: "returns an error for an entry without a type", give: &emit.Field{Name: "at"}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				_, err := s.SumPayload(variantOf(tt.give))
				assert.HasError(t, err, "an inline entry states a name and a type alone")
			})
		}
	})

	t.Run("Attrs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no annotations", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Attrs(nil), "", "no lines")
		})

		t.Run("writes one outer attribute per annotation at the given depth", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Attrs(symbol.Annotations{
				{Name: "derive", Args: []string{"Debug", "Clone"}},
				{Name: "non_exhaustive"},
			}, "    "), "    #[derive(Debug, Clone)]\n    #[non_exhaustive]\n", "the arguments verbatim")
		})
	})
}
