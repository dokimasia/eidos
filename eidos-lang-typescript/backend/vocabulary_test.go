// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spelling fixture: the modules a reference imports from and the
// declarations it names.
const (
	// storeModule and legacyModule both export rowName.
	storeModule  = "./store"
	legacyModule = "./legacy"
	// rowName and baseName name exported declarations, and rowName2
	// is the name rowName binds under beside another import of it.
	rowName  = "Row"
	rowName2 = rowName + "2"
	baseName = "Base"
)

// ref returns an unresolved reference spelled s.
func ref(s string) *emit.TypeRef { return &emit.TypeRef{Spelling: s} }

// imported returns a reference to name exported by module.
func imported(module, name string) *emit.TypeRef {
	return &emit.TypeRef{Spelling: name, Package: module}
}

// speller returns a speller over a fresh set, and the set.
func speller() (backend.Speller, *render.ImportSet) {
	set := &render.ImportSet{}
	return backend.NewSpeller(set), set
}

// spelled spells one reference in a type position and asserts it
// spells.
func spelled(tb assert.TB, s backend.Speller, t *emit.TypeRef) string {
	tb.Helper()

	out, err := s.Spell(t)
	assert.NoError(tb, err, "the reference spells")
	return out
}

// unspellable binds rowName of storeModule on s and returns a list of
// legacyModule's rowName, which s refuses: the element imports under
// another name, and a composite's spelling does not follow from its
// structure.
func unspellable(tb assert.TB, s backend.Speller) *emit.TypeRef {
	tb.Helper()

	spelled(tb, s, imported(storeModule, rowName))
	return &emit.TypeRef{
		Form: symbol.FormList, Spelling: "Row[]", Elems: []*emit.TypeRef{imported(legacyModule, rowName)},
	}
}

// The vocabulary is what every kind template spells through, so
// each helper's output is pinned byte for byte, and every import a
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
			assert.Equal(t, set.Paths(), []string{storeModule}, "the import is the file's")
		})
	})

	t.Run("Docs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no lines", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Docs(nil), "", "no block")
		})

		t.Run("writes a TSDoc block with the star gutter", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Docs([]string{"One.", "Two."}), "/**\n * One.\n * Two.\n */\n",
				"one gutter line per line")
		})

		t.Run("writes at the given depth", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Docs([]string{"Inner."}, "  "), "  /**\n   * Inner.\n   */\n",
				"the whole block indented to the member's depth")
		})
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the spelling of a reference that names no module", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, spelled(t, s, ref("Row[]")), "Row[]", "the spelling as written")
			assert.Equal(t, set.Len(), 0, "and no import")
		})

		t.Run("returns unknown for a missing reference", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, nil), backend.Anonymous, "unknown, not any")
		})

		t.Run("writes an argument list in angle brackets", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			assert.Equal(t, spelled(t, s, &emit.TypeRef{
				Spelling: "Map",
				Args: []*emit.TypeRef{
					ref("string"),
					{Spelling: "Set", Args: []*emit.TypeRef{ref(rowName)}},
				},
			}), "Map<string, Set<Row>>", "the arguments recurse")
		})

		t.Run("imports what a type position names for the type checker alone", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, spelled(t, s, imported(storeModule, rowName)), rowName, "the exported name")
			assert.Equal(t, set.Entries(), []render.Entry{
				{Path: storeModule, Name: rowName, TypeOnly: true},
			}, "a type-only import")
		})

		t.Run("imports a target of the language from its module", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			target := &emit.TypeRef{
				Spelling: rowName,
				Target:   symbol.Identity{Lang: typescript.Lang, Package: storeModule, Name: rowName},
			}
			assert.Equal(t, spelled(t, s, target), rowName, "the exported name")
			assert.Equal(t, set.Paths(), []string{storeModule}, "imported from the declaring module")
		})

		t.Run("renames a declaration another import binds", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			spelled(t, s, imported(storeModule, rowName))
			assert.Equal(t, spelled(t, s, imported(legacyModule, rowName)), rowName2,
				"the second declaration binds the next free name")
		})

		t.Run("writes a member of an imported declaration behind its bound name", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			spelled(t, s, imported(storeModule, rowName))
			assert.Equal(t, spelled(t, s, imported(legacyModule, rowName+".Key")), rowName2+".Key",
				"the first segment is the import")
		})

		t.Run("keeps a composite's spelling where every child spells as written", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			list := &emit.TypeRef{
				Form: symbol.FormList, Spelling: "Row[]", Elems: []*emit.TypeRef{imported(storeModule, rowName)},
			}
			assert.Equal(t, spelled(t, s, list), "Row[]", "the written spelling")
			assert.Equal(t, set.Paths(), []string{storeModule}, "and the child's import")
		})

		t.Run("returns an error for a composite whose child imports under another name", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			spelled(t, s, imported(storeModule, rowName))
			list := &emit.TypeRef{
				Form: symbol.FormList, Spelling: "Row[]", Elems: []*emit.TypeRef{imported(legacyModule, rowName)},
			}
			_, err := s.Spell(list)
			assert.HasError(t, err, "the spelling does not follow from the structure")
			assert.Contains(t, err.Error(), string(typescript.Lang)+": ", "under the language's prefix")
		})
	})

	t.Run("TypeParams", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a bound that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.TypeParams([]*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{unspellable(t, s)}}})
			assert.HasError(t, err, "the bound's refusal is the list's")
		})

		t.Run("returns an error for a default that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.TypeParams([]*emit.TypeParam{{Name: "T", Default: unspellable(t, s)}})
			assert.HasError(t, err, "the default's refusal is the list's")
		})

		typeParams := func(t *testing.T, ps ...*emit.TypeParam) (string, error) {
			t.Helper()

			s, _ := speller()
			return s.TypeParams(ps)
		}

		t.Run("returns nothing for no parameters", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t)
			assert.NoError(t, err, "no parameters spell")
			assert.Equal(t, got, "", "as nothing")
		})

		t.Run("intersects the bounds behind extends", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t, &emit.TypeParam{Name: "T", Bounds: []*emit.TypeRef{ref("Codec"), ref("Closer")}})
			assert.NoError(t, err, "the bounds spell")
			assert.Equal(t, got, "<T extends Codec & Closer>", "an intersection")
		})

		t.Run("writes a default behind an equals sign", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t, &emit.TypeParam{Name: "U", Default: ref("string")})
			assert.NoError(t, err, "the default spells")
			assert.Equal(t, got, "<U = string>", "the default type")
		})

		t.Run("writes the declaration-site variance before the name", func(t *testing.T) {
			t.Parallel()

			got, err := typeParams(t,
				&emit.TypeParam{Name: "T", Variance: symbol.VarianceIn},
				&emit.TypeParam{Name: "U", Variance: symbol.VarianceOut})
			assert.NoError(t, err, "the variance spells")
			assert.Equal(t, got, "<in T, out U>", "in and out")
		})

		t.Run("returns an error for a value parameter", func(t *testing.T) {
			t.Parallel()

			_, err := typeParams(t, &emit.TypeParam{Name: "N", Const: true, Type: ref("number")})
			assert.HasError(t, err, "TypeScript's const narrows a type parameter instead")
		})

		t.Run("binds the import a bound names", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			bound := imported(storeModule, rowName)
			got, err := s.TypeParams([]*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{bound}}})
			assert.NoError(t, err, "the bound spells")
			assert.Equal(t, got, "<T extends Row>", "the imported name")
			assert.Equal(t, set.Paths(), []string{storeModule}, "and its import")
		})
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a type that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Params([]*emit.Param{{Name: "rows", Type: unspellable(t, s)}})
			assert.HasError(t, err, "the type's refusal is the list's")
		})

		params := func(t *testing.T, ps ...*emit.Param) (string, error) {
			t.Helper()

			s, _ := speller()
			return s.Params(ps)
		}

		t.Run("writes each name before its colon-typed type", func(t *testing.T) {
			t.Parallel()

			got, err := params(t, &emit.Param{Name: "key", Type: ref("string")})
			assert.NoError(t, err, "the list spells")
			assert.Equal(t, got, "key: string", "the name typed behind a colon")
		})

		t.Run("writes a rest parameter as an array behind three dots", func(t *testing.T) {
			t.Parallel()

			got, err := params(t, &emit.Param{Name: "rest", Type: ref("number"), Variadic: symbol.VariadicPositional})
			assert.NoError(t, err, "the list spells")
			assert.Equal(t, got, "...rest: number[]", "the rest marker and its array type")
		})

		t.Run("writes an unnamed parameter under the discard name", func(t *testing.T) {
			t.Parallel()

			got, err := params(t, &emit.Param{Type: ref("string")})
			assert.NoError(t, err, "the list spells")
			assert.Equal(t, got, "_: string", "the discard name")
		})

		t.Run("writes a default behind an equals sign", func(t *testing.T) {
			t.Parallel()

			got, err := params(t, &emit.Param{Name: "limit", Type: ref("number"), Default: "8"})
			assert.NoError(t, err, "the list spells")
			assert.Equal(t, got, "limit: number = 8", "the default verbatim")
		})

		t.Run("returns an error for a rest parameter with a default", func(t *testing.T) {
			t.Parallel()

			_, err := params(t, &emit.Param{
				Name: "rest", Type: ref("number"), Variadic: symbol.VariadicPositional, Default: "8",
			})
			assert.HasError(t, err, "TypeScript initializes no rest")
		})
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a result that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Results([]*emit.Return{{Type: unspellable(t, s)}})
			assert.HasError(t, err, "the result's refusal is the annotation's")
		})

		results := func(t *testing.T, rs ...*emit.Return) string {
			t.Helper()

			s, _ := speller()
			got, err := s.Results(rs)
			assert.NoError(t, err, "the results spell")
			return got
		}

		t.Run("writes void for no results", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, results(t), ": void", "the empty annotation")
		})

		t.Run("writes one result as the annotation", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, results(t, &emit.Return{Type: ref(rowName)}), ": Row", "the type")
		})

		t.Run("writes several results as a tuple", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, results(t, &emit.Return{Type: ref(rowName)}, &emit.Return{Type: ref("Error")}),
				": [Row, Error]", "one value returns")
		})
	})

	t.Run("Returns", func(t *testing.T) {
		t.Parallel()

		returns := func(t *testing.T, d symbol.Symbol) string {
			t.Helper()

			s, _ := speller()
			got, err := s.Returns(d)
			assert.NoError(t, err, "the annotation spells")
			return got
		}

		t.Run("wraps an async callable's result in a promise", func(t *testing.T) {
			t.Parallel()

			load := &emit.Function{Name: "load", Async: true, Returns: []*emit.Return{{Type: ref(rowName)}}}
			assert.Equal(t, returns(t, load), ": Promise<Row>", "the promise of the result")
		})

		t.Run("writes nothing for a setter", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, returns(t, &emit.Method{Name: "size", Accessor: symbol.AccessorSet}), "",
				"TypeScript forbids a setter's annotation")
		})
	})

	t.Run("Mods", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    symbol.Symbol
			want    string
			wantErr bool
		}{
			{name: "writes export for an unstated visibility", give: &emit.Struct{Name: rowName}, want: "export "},
			{
				name: "writes export abstract for an abstract class",
				give: &emit.Struct{Name: rowName, Abstract: true}, want: "export abstract ",
			},
			{
				name: "writes nothing for a package scope",
				give: &emit.Variable{Name: "count", Visibility: symbol.VisibilityPackage}, want: "",
			},
			{
				name: "writes export async for an async function",
				give: &emit.Function{Name: "load", Async: true}, want: "export async ",
			},
			{
				name: "writes export for a constant with a value",
				give: &emit.Constant{Name: "MAX", Value: "8"}, want: "export ",
			},
			{
				name: "returns an error for a final class",
				give: &emit.Struct{Name: rowName, Final: true}, wantErr: true,
			},
			{
				name: "returns an error for a protected scope",
				give: &emit.Alias{Name: "ID", Visibility: symbol.VisibilityProtected}, wantErr: true,
			},
			{
				name:    "returns an error for a decorated function",
				give:    &emit.Function{Name: "load", Annotations: symbol.Annotations{{Name: "log"}}},
				wantErr: true,
			},
			{
				name: "returns an error for a constant without a value",
				give: &emit.Constant{Name: "MAX"}, wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Mods(tt.give)
				if tt.wantErr {
					assert.HasError(t, err, "a module-level declaration cannot state it")
					return
				}
				assert.NoError(t, err, "the keywords spell")
				assert.Equal(t, got, tt.want, "in TypeScript's order")
			})
		}
	})

	t.Run("MemberMods", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a field's keywords in TypeScript's order", func(t *testing.T) {
			t.Parallel()

			got, err := backend.MemberMods(&emit.Field{
				Name: "key", Visibility: symbol.VisibilityPrivate,
				Level: symbol.LevelType, Mutability: symbol.MutabilityImmutable,
			})
			assert.NoError(t, err, "the keywords spell")
			assert.Equal(t, got, "private static readonly ", "accessibility, static, readonly")
		})

		t.Run("writes override before async", func(t *testing.T) {
			t.Parallel()

			got, err := backend.MemberMods(&emit.Method{Name: "load", Override: true, Async: true})
			assert.NoError(t, err, "the keywords spell")
			assert.Equal(t, got, "override async ", "public is implicit")
		})

		t.Run("writes abstract for a method without a body", func(t *testing.T) {
			t.Parallel()

			got, err := backend.MemberMods(&emit.Method{Name: "load", Abstract: true})
			assert.NoError(t, err, "the keyword spells")
			assert.Equal(t, got, "abstract ", "a signature")
		})

		tests := []struct {
			name string
			give symbol.Symbol
		}{
			{name: "returns an error for a final method", give: &emit.Method{Name: "load", Final: true}},
			{
				name: "returns an error for an abstract method with a body",
				give: &emit.Method{Name: "load", Abstract: true, Body: emit.Body{Verbatim: "return 1;"}},
			},
			{
				name: "returns an error for a package scope",
				give: &emit.Field{Name: "key", Visibility: symbol.VisibilityPackage},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.MemberMods(tt.give)
				assert.HasError(t, err, "a class member cannot state it")
			})
		}
	})

	t.Run("IndexMods", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for an instance index signature", func(t *testing.T) {
			t.Parallel()

			got, err := backend.IndexMods(&emit.Method{Name: "index", Indexer: true})
			assert.NoError(t, err, "the signature spells")
			assert.Equal(t, got, "", "no keyword")
		})

		t.Run("writes static for a type-level index signature", func(t *testing.T) {
			t.Parallel()

			got, err := backend.IndexMods(&emit.Method{Name: "index", Indexer: true, Level: symbol.LevelType})
			assert.NoError(t, err, "the signature spells")
			assert.Equal(t, got, "static ", "the one keyword it takes")
		})

		tests := []struct {
			name string
			give *emit.Method
		}{
			{name: "returns an error for an accessibility", give: &emit.Method{Visibility: symbol.VisibilityPrivate}},
			{name: "returns an error for abstract", give: &emit.Method{Abstract: true}},
			{name: "returns an error for an override marker", give: &emit.Method{Override: true}},
			{name: "returns an error for final", give: &emit.Method{Final: true}},
			{name: "returns an error for a default", give: &emit.Method{HasDefault: true}},
			{name: "returns an error for asynchrony", give: &emit.Method{Async: true}},
			{name: "returns an error for a hard-private name", give: &emit.Method{Hard: true}},
			{name: "returns an error for a throws clause", give: &emit.Method{Throws: []*emit.TypeRef{ref("Error")}}},
			{
				name: "returns an error for decorators",
				give: &emit.Method{Annotations: symbol.Annotations{{Name: "log"}}},
			},
			{name: "returns an error for a body", give: &emit.Method{Body: emit.Body{Verbatim: "return 1;"}}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tt.give.Name, tt.give.Indexer = "index", true
				_, err := backend.IndexMods(tt.give)
				assert.HasError(t, err, "an index signature takes static alone")
			})
		}
	})

	t.Run("CtorMods", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a public constructor", func(t *testing.T) {
			t.Parallel()

			got, err := backend.CtorMods(&emit.Method{Name: "make", Constructs: true})
			assert.NoError(t, err, "the constructor spells")
			assert.Equal(t, got, "", "public is implicit")
		})

		t.Run("writes a stated accessibility", func(t *testing.T) {
			t.Parallel()

			ctor := &emit.Method{Name: "make", Constructs: true, Visibility: symbol.VisibilityPrivate}
			got, err := backend.CtorMods(ctor)
			assert.NoError(t, err, "the constructor spells")
			assert.Equal(t, got, "private ", "its accessibility alone")
		})

		tests := []struct {
			name string
			give *emit.Method
		}{
			{name: "returns an error for type level", give: &emit.Method{Level: symbol.LevelType}},
			{
				name: "returns an error for type parameters",
				give: &emit.Method{TypeParams: []*emit.TypeParam{{Name: "T"}}},
			},
			{name: "returns an error for abstract", give: &emit.Method{Abstract: true}},
			{name: "returns an error for an override marker", give: &emit.Method{Override: true}},
			{name: "returns an error for final", give: &emit.Method{Final: true}},
			{name: "returns an error for a default", give: &emit.Method{HasDefault: true}},
			{name: "returns an error for asynchrony", give: &emit.Method{Async: true}},
			{name: "returns an error for a hard-private name", give: &emit.Method{Hard: true}},
			{name: "returns an error for an accessor", give: &emit.Method{Accessor: symbol.AccessorGet}},
			{name: "returns an error for a throws clause", give: &emit.Method{Throws: []*emit.TypeRef{ref("Error")}}},
			{
				name: "returns an error for decorators",
				give: &emit.Method{Annotations: symbol.Annotations{{Name: "log"}}},
			},
			{name: "returns an error for a package scope", give: &emit.Method{Visibility: symbol.VisibilityPackage}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tt.give.Name, tt.give.Constructs = "make", true
				_, err := backend.CtorMods(tt.give)
				assert.HasError(t, err, "a constructor takes an accessibility alone")
			})
		}
	})

	t.Run("PropMods", func(t *testing.T) {
		t.Parallel()

		t.Run("writes readonly for an immutable property", func(t *testing.T) {
			t.Parallel()

			got, err := backend.PropMods(&emit.Field{Name: "key", Mutability: symbol.MutabilityImmutable})
			assert.NoError(t, err, "the property spells")
			assert.Equal(t, got, "readonly ", "the one keyword")
		})

		t.Run("returns an error for a static property", func(t *testing.T) {
			t.Parallel()

			_, err := backend.PropMods(&emit.Field{Name: "key", Level: symbol.LevelType})
			assert.HasError(t, err, "an interface property has no static level")
		})
	})

	t.Run("SigMods", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a bare signature", func(t *testing.T) {
			t.Parallel()

			got, err := backend.SigMods(&emit.Method{Name: "load"})
			assert.NoError(t, err, "the signature passes")
			assert.Equal(t, got, "", "and spells nothing")
		})

		t.Run("returns an error for a stated modifier", func(t *testing.T) {
			t.Parallel()

			_, err := backend.SigMods(&emit.Method{Name: "load", Async: true})
			assert.HasError(t, err, "an interface method is a bare signature")
		})

		t.Run("returns an error for a body", func(t *testing.T) {
			t.Parallel()

			_, err := backend.SigMods(&emit.Method{Name: "load", Body: emit.Body{Verbatim: "return 1;"}})
			assert.HasError(t, err, "the signature drops it")
		})
	})

	t.Run("EnumKey", func(t *testing.T) {
		t.Parallel()

		t.Run("writes an identifier bare", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.EnumKey(&emit.EnumVariant{Name: "Open"}), "Open", "unquoted")
		})

		t.Run("quotes any other name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.EnumKey(&emit.EnumVariant{Name: "content-type"}), "'content-type'",
				"an enum member admits the quoted form")
		})
	})

	t.Run("AccessorKw", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for an ordinary method", func(t *testing.T) {
			t.Parallel()

			got, err := backend.AccessorKw(&emit.Method{Name: "load"})
			assert.NoError(t, err, "the method passes")
			assert.Equal(t, got, "", "no keyword")
		})

		t.Run("writes get for a getter", func(t *testing.T) {
			t.Parallel()

			got, err := backend.AccessorKw(&emit.Method{
				Name: "size", Accessor: symbol.AccessorGet, Returns: []*emit.Return{{Type: ref("number")}},
			})
			assert.NoError(t, err, "the getter spells")
			assert.Equal(t, got, "get ", "its keyword")
		})

		t.Run("writes set for a setter", func(t *testing.T) {
			t.Parallel()

			got, err := backend.AccessorKw(&emit.Method{
				Name: "size", Accessor: symbol.AccessorSet, Params: []*emit.Param{{Name: "v", Type: ref("number")}},
			})
			assert.NoError(t, err, "the setter spells")
			assert.Equal(t, got, "set ", "its keyword")
		})

		t.Run("returns an error for a getter taking parameters", func(t *testing.T) {
			t.Parallel()

			_, err := backend.AccessorKw(&emit.Method{
				Name: "size", Accessor: symbol.AccessorGet,
				Params:  []*emit.Param{{Name: "v", Type: ref("number")}},
				Returns: []*emit.Return{{Type: ref("number")}},
			})
			assert.HasError(t, err, "a getter takes nothing")
		})

		t.Run("returns an error for a setter that returns", func(t *testing.T) {
			t.Parallel()

			_, err := backend.AccessorKw(&emit.Method{
				Name: "size", Accessor: symbol.AccessorSet,
				Params:  []*emit.Param{{Name: "v", Type: ref("number")}},
				Returns: []*emit.Return{{Type: ref("number")}},
			})
			assert.HasError(t, err, "a setter returns nothing")
		})
	})

	t.Run("Hard", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the prefix for a hard-private member", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Hard(&emit.Field{Name: "key", Hard: true})
			assert.NoError(t, err, "the member spells")
			assert.Equal(t, got, "#", "the privacy is in the name")
		})

		t.Run("returns nothing for an ordinary member", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Hard(&emit.Method{Name: "load"})
			assert.NoError(t, err, "the member spells")
			assert.Equal(t, got, "", "unprefixed")
		})

		t.Run("returns an error for a visibility beside the hard name", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Hard(&emit.Field{Name: "key", Hard: true, Visibility: symbol.VisibilityPrivate})
			assert.HasError(t, err, "the two mechanisms cannot combine")
		})
	})

	t.Run("IndexSig", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a key that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.IndexSig(&emit.Method{
				Name: "index", Params: []*emit.Param{{Name: "key", Type: unspellable(t, s)}},
				Returns: []*emit.Return{{Type: ref("string")}},
			})
			assert.HasError(t, err, "the key's refusal is the signature's")
		})

		t.Run("returns an error for an element that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.IndexSig(&emit.Method{
				Name: "index", Params: []*emit.Param{{Name: "key", Type: ref("string")}},
				Returns: []*emit.Return{{Type: unspellable(t, s)}},
			})
			assert.HasError(t, err, "the element's refusal is the signature's")
		})

		t.Run("writes the signature with its key and element typed", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.IndexSig(&emit.Method{
				Name: "index", Indexer: true,
				Params:  []*emit.Param{{Name: "key", Type: ref("string")}},
				Returns: []*emit.Return{{Type: ref(rowName)}},
			})
			assert.NoError(t, err, "the signature spells")
			assert.Equal(t, got, "[key: string]: Row;", "whole")
		})

		t.Run("returns an error for a signature without one named typed key", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.IndexSig(&emit.Method{Name: "index", Indexer: true})
			assert.HasError(t, err, "one key is required")
		})
	})

	t.Run("Heritage", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a base that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Heritage(&emit.Struct{Name: "Cache", Extends: []*emit.TypeRef{unspellable(t, s)}})
			assert.HasError(t, err, "the base's refusal is the clause's")
		})

		t.Run("returns an error for a contract that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Heritage(&emit.Struct{Name: "Cache", Implements: []*emit.TypeRef{unspellable(t, s)}})
			assert.HasError(t, err, "the contract's refusal is the clause's")
		})

		t.Run("returns an error for a widened contract that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Heritage(&emit.Interface{Name: "Store", Extends: []*emit.TypeRef{unspellable(t, s)}})
			assert.HasError(t, err, "the widened contract's refusal is the clause's")
		})

		t.Run("writes a class's base behind extends", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.Heritage(&emit.Struct{Name: rowName, Extends: []*emit.TypeRef{ref(baseName)}})
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, got, " extends Base", "one base")
		})

		t.Run("writes a class's contracts behind implements after its base", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.Heritage(&emit.Struct{
				Name:       rowName,
				Extends:    []*emit.TypeRef{ref(baseName)},
				Implements: []*emit.TypeRef{ref("Keyed"), ref("Closer")},
			})
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, got, " extends Base implements Keyed, Closer", "comma-joined contracts")
		})

		t.Run("writes an interface's widened contracts behind extends", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			store := &emit.Interface{Name: "Store", Extends: []*emit.TypeRef{ref("Keyed"), ref("Closer")}}
			got, err := s.Heritage(store)
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, got, " extends Keyed, Closer", "comma-joined")
		})

		t.Run("imports a class's base as a value", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			_, err := s.Heritage(&emit.Struct{Name: rowName, Extends: []*emit.TypeRef{imported(storeModule, baseName)}})
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: storeModule, Name: baseName}},
				"the base exists at run time")
		})

		t.Run("imports a contract for the type checker alone", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			row := &emit.Struct{Name: rowName, Implements: []*emit.TypeRef{imported(storeModule, "Keyed")}}
			_, err := s.Heritage(row)
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: storeModule, Name: "Keyed", TypeOnly: true}},
				"a type-only import")
		})

		t.Run("returns an error for a second class base", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Heritage(&emit.Struct{Name: rowName, Extends: []*emit.TypeRef{ref("A"), ref("B")}})
			assert.HasError(t, err, "TypeScript extends one base")
		})

		t.Run("returns an error for an embed", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Heritage(&emit.Struct{Name: rowName, Embeds: []*emit.Embed{{Ref: ref(baseName)}}})
			assert.HasError(t, err, "nothing promotes members")
		})
	})

	t.Run("Binding", func(t *testing.T) {
		t.Parallel()

		t.Run("writes let for a mutable binding", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Binding(&emit.Variable{Name: "count"})
			assert.NoError(t, err, "the binding spells")
			assert.Equal(t, got, "let", "the mutable keyword")
		})

		t.Run("writes const for an immutable binding with a value", func(t *testing.T) {
			t.Parallel()

			count := &emit.Variable{Name: "count", Mutability: symbol.MutabilityImmutable, Value: "0"}
			got, err := backend.Binding(count)
			assert.NoError(t, err, "the binding spells")
			assert.Equal(t, got, "const", "the immutable keyword")
		})

		t.Run("returns an error for an immutable binding without a value", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Binding(&emit.Variable{Name: "count", Mutability: symbol.MutabilityImmutable})
			assert.HasError(t, err, "TypeScript initializes a const where it is declared")
		})
	})

	t.Run("Decorators", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for no annotations", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Decorators(nil), "", "no lines")
		})

		t.Run("writes one decorator line per annotation at the given depth", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Decorators(symbol.Annotations{
				{Name: "injectable"},
				{Name: "route", Args: []string{`"/rows"`, "true"}},
			}, "  "), "  @injectable\n  @route(\"/rows\", true)\n", "the arguments verbatim")
		})
	})
}
