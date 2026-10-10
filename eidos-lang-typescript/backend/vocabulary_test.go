// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spelling cases name the modules a reference imports from and the
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
	// memberPath is a member inside rowName.
	memberPath = ".Key"
	// storePath is the module path of a module of the workspace, and
	// reactModule is the bare specifier of an ambient module.
	storePath   = "svc/store"
	reactModule = "react"
	// goLang and goPackage are the language and the package of the
	// declaration that a translated reference refers to.
	goLang    = symbol.Lang("go")
	goPackage = "svc"
	// promiseType and dateType are global types of TypeScript.
	promiseType = "Promise"
	dateType    = "Date"
)

// The declarations, the spellings and the keywords the allocation cases
// name.
const (
	keyParam    = "K"
	valueParam  = "V"
	eqTrait     = "Eq"
	hashTrait   = "Hash"
	closerName  = "Closer"
	idName      = "id"
	nameParam   = "name"
	countParam  = "count"
	sizeName    = "size"
	indexName   = "index"
	keyName     = "key"
	wireName    = "content-type"
	phaseName   = "Phase"
	openName    = "Open"
	makeName    = "make"
	rowDoc      = "Row is one record."
	exportWord  = "export "
	logDecorate = "log"
)

// The allocations of the vocabulary.
const (
	// textAllocs is one spelling, sized once or joined once.
	textAllocs = 1
	// typeParamsAllocs is a parameter of two bounds and an unbounded one:
	// the joined bounds, the bounded parameter's spelling, the joined
	// list, and its brackets.
	typeParamsAllocs = 1 + 1 + 1 + 1
	// heritageAllocs is a base and two contracts: the base behind extends,
	// the joined contracts, and the contracts behind implements.
	heritageAllocs = 1 + 1 + 1
	// paramsAllocs is two parameters: each one's spelling, and the joined
	// list.
	paramsAllocs = 2 + 1
	// partsAllocs is three parameters: each one's spelling, the list of
	// parts, which Go places on the heap past two, and the joined list.
	partsAllocs = 3 + 1 + 1
	// resultTupleAllocs is two results: the joined tuple, and the
	// annotation.
	resultTupleAllocs = 1 + 1
	// funcsAllocs is the vocabulary of a file: the map of twenty-one
	// helpers, four allocations, and the speller's seven bound helpers.
	funcsAllocs = 4 + 7
	// joinedKeywordsAllocs is two keywords joined behind an accessibility,
	// one join each.
	joinedKeywordsAllocs = 1 + 1
)

// The vocabulary is what every kind template spells through, so
// each helper's output is pinned byte for byte, and every import a
// spelling needs is pinned beside it.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("NewSpeller", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a speller that records its imports in the given set", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			spelled(t, backend.NewSpeller(&set), imported(storeModule, rowName))
			assert.Equal(t, set.Paths(), []string{storeModule}, "the import is the set's")
		})
	})

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the spelling of a reference that names no module", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, spelled(t, s, ref("Row[]")), "Row[]", "the spelling as written")
			assert.Equal(t, set.Len(), 0, "no import")
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

		relatives := []struct {
			name   string
			home   string
			module string
			want   string
		}{
			{
				name: "imports a target of the language from a module beside the file",
				home: "svc/client", module: storePath, want: "./store",
			},
			{
				name: "imports a target of the language from a module below the directory of the file",
				home: "svc/client", module: "svc/store/v2", want: "./store/v2",
			},
			{
				name: "imports a target of the language from a module of another directory",
				home: "api/client", module: storePath, want: "../svc/store",
			},
			{
				name: "imports a target of the language from a module one directory up",
				home: "a/b/client", module: "a/store", want: "../store",
			},
			{
				name: "imports a target of the language from a module two directories up",
				home: "a/b/client", module: "store", want: "../../store",
			},
			{
				name: "imports a target of the language from a module named as the directory of the file",
				home: "svc/client", module: "svc", want: "../svc",
			},
			{
				name: "imports a target of the language from a directory that shares the start of a name",
				home: "svc/client", module: "svcx/store", want: "../svcx/store",
			},
			{
				name: "imports a target of the language for a file at the root",
				home: "client", module: storePath, want: "./svc/store",
			},
		}
		for _, tt := range relatives {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, set := speller()
				set.SetHome(tt.home)
				target := &emit.TypeRef{
					Spelling: rowName,
					Target:   symbol.Identity{Lang: typescript.Lang, Package: tt.module, Name: rowName},
				}
				assert.Equal(t, spelled(t, s, target), rowName, "the exported name")
				assert.Equal(t, set.Paths(), []string{tt.want}, "the specifier is relative to the file")
			})
		}

		t.Run("imports a target of the language through the bare specifier that its source wrote", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			set.SetHome(storePath)
			target := &emit.TypeRef{
				Spelling: rowName, Package: reactModule,
				Target: symbol.Identity{Lang: typescript.Lang, Package: reactModule, Name: rowName},
			}
			assert.Equal(t, spelled(t, s, target), rowName, "the exported name")
			assert.Equal(t, set.Paths(), []string{reactModule}, "the specifier as the source wrote it")
		})

		t.Run("imports a target of the language through a relative specifier of the file's own", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			set.SetHome("api/client")
			target := &emit.TypeRef{
				Spelling: rowName, Package: storeModule,
				Target: symbol.Identity{Lang: typescript.Lang, Package: storePath, Name: rowName},
			}
			assert.Equal(t, spelled(t, s, target), rowName, "the exported name")
			assert.Equal(t, set.Paths(), []string{"../svc/store"},
				"the specifier is relative to the file and not to the source")
		})

		t.Run("imports a translated reference from the module path that the layout recorded", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			set.SetHome("svc/client")
			translated := &emit.TypeRef{
				Spelling: rowName, Package: storePath,
				Target: symbol.Identity{Lang: goLang, Package: goPackage, Name: rowName, Kind: symbol.KindStruct},
			}
			assert.Equal(t, spelled(t, s, translated), rowName, "the translated name")
			assert.Equal(t, set.Paths(), []string{"./store"}, "the specifier is relative to the file")
		})

		t.Run("imports nothing for a target of the file's own module", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			set.SetHome(storePath)
			target := &emit.TypeRef{
				Spelling: rowName + memberPath,
				Target:   symbol.Identity{Lang: typescript.Lang, Package: storePath, Name: rowName},
			}
			assert.Equal(t, spelled(t, s, target), rowName+memberPath, "the spelling as written")
			assert.Equal(t, set.Len(), 0, "the file imports nothing of its own")
		})

		t.Run("returns a global type as written", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			assert.Equal(t, spelled(t, s, ref(dateType)), dateType, "the global type needs no import")
			assert.Equal(t, set.Len(), 0, "no import")
		})

		t.Run("returns an error for a global type that a declaration of the file hides", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			set.Reserve(dateType)
			_, err := s.Spell(ref(dateType))
			assert.HasError(t, err, "the file's Date hides the global one")
			assert.Contains(t, err.Error(), "hides the global type "+dateType, "the refusal contains the hidden type")
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
			assert.Equal(t, spelled(t, s, imported(legacyModule, rowName+memberPath)), rowName2+memberPath,
				"the first segment is the import")
		})

		t.Run("keeps a composite's spelling where every child spells as written", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			list := &emit.TypeRef{
				Form: symbol.FormList, Spelling: "Row[]", Elems: []*emit.TypeRef{imported(storeModule, rowName)},
			}
			assert.Equal(t, spelled(t, s, list), "Row[]", "the written spelling")
			assert.Equal(t, set.Paths(), []string{storeModule}, "the child's import")
		})

		t.Run("returns an error for a composite whose child imports under another name", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Spell(unspellable(t, s))
			assert.HasError(t, err, "the spelling does not follow from the structure")
			assert.Contains(t, err.Error(), string(typescript.Lang)+": ", "under the language's prefix")
		})
	})

	t.Run("IndexSig", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the signature whole", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.IndexSig(indexer())
			assert.NoError(t, err, "the signature spells")
			assert.Equal(t, got, "[key: string]: Row;", "whole")
		})

		t.Run("returns an error for a key that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			m := indexer()
			m.Params[0].Type = unspellable(t, s)
			_, err := s.IndexSig(m)
			assert.HasError(t, err, "the key's refusal is the signature's")
		})

		t.Run("returns an error for an element that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			m := indexer()
			m.Returns[0].Type = unspellable(t, s)
			_, err := s.IndexSig(m)
			assert.HasError(t, err, "the element's refusal is the signature's")
		})

		noElement := indexer()
		noElement.Returns = nil
		generic := indexer()
		generic.TypeParams = []*emit.TypeParam{{Name: keyParam}}
		tests := []struct {
			name string
			give *emit.Method
		}{
			{
				name: "returns an error for a signature without one named typed key",
				give: &emit.Method{Name: indexName, Indexer: true},
			},
			{name: "returns an error for a signature without an element type", give: noElement},
			{name: "returns an error for a signature with type parameters", give: generic},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				_, err := s.IndexSig(tt.give)
				assert.HasError(t, err, "an index signature cannot state it")
			})
		}
	})

	t.Run("TypeParams", func(t *testing.T) {
		t.Parallel()

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

			got, err := typeParams(
				t,
				&emit.TypeParam{Name: "T", Bounds: []*emit.TypeRef{ref("Codec"), ref(closerName)}},
			)
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
			assert.Equal(t, got, "<in T, out U>", "in before one, out before the other")
		})

		t.Run("binds the import a bound names", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			got, err := s.TypeParams(
				[]*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{imported(storeModule, rowName)}}},
			)
			assert.NoError(t, err, "the bound spells")
			assert.Equal(t, got, "<T extends Row>", "the imported name")
			assert.Equal(t, set.Paths(), []string{storeModule}, "the bound's import")
		})

		t.Run("returns an error for a value parameter", func(t *testing.T) {
			t.Parallel()

			_, err := typeParams(t, &emit.TypeParam{Name: "N", Const: true, Type: ref("number")})
			assert.HasError(t, err, "TypeScript's const narrows a type parameter instead")
		})

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
	})

	t.Run("Heritage", func(t *testing.T) {
		t.Parallel()

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
				Implements: []*emit.TypeRef{ref(keyedName), ref(closerName)},
			})
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, got, " extends Base implements Keyed, Closer", "comma-joined contracts")
		})

		t.Run("writes an interface's widened contracts behind extends", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			store := &emit.Interface{Name: "Store", Extends: []*emit.TypeRef{ref(keyedName), ref(closerName)}}
			got, err := s.Heritage(store)
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, got, " extends Keyed, Closer", "comma-joined")
		})

		t.Run("returns nothing for an interface without contracts", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			got, err := s.Heritage(&emit.Interface{Name: "Store"})
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, got, "", "no clause")
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
			row := &emit.Struct{Name: rowName, Implements: []*emit.TypeRef{imported(storeModule, keyedName)}}
			_, err := s.Heritage(row)
			assert.NoError(t, err, "the heritage spells")
			assert.Equal(t, set.Entries(), []render.Entry{{Path: storeModule, Name: keyedName, TypeOnly: true}},
				"a type-only import")
		})

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

		tests := []struct {
			name string
			give symbol.Symbol
		}{
			{
				name: "returns an error for a second class base",
				give: &emit.Struct{Name: rowName, Extends: []*emit.TypeRef{ref("A"), ref("B")}},
			},
			{
				name: "returns an error for a class's embed",
				give: &emit.Struct{Name: rowName, Embeds: []*emit.Embed{{Ref: ref(baseName)}}},
			},
			{
				name: "returns an error for an interface's embed",
				give: &emit.Interface{Name: "Store", Embeds: []*emit.Embed{{Ref: ref(baseName)}}},
			},
			{name: "returns an error for an enum", give: &emit.Enum{Name: phaseName}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, _ := speller()
				_, err := s.Heritage(tt.give)
				assert.HasError(t, err, "TypeScript cannot state the heritage")
			})
		}
	})

	t.Run("Params", func(t *testing.T) {
		t.Parallel()

		params := func(t *testing.T, ps ...*emit.Param) (string, error) {
			t.Helper()

			s, _ := speller()
			return s.Params(ps)
		}

		tests := []struct {
			name string
			give []*emit.Param
			want string
		}{
			{
				name: "writes each name before its colon-typed type",
				give: []*emit.Param{{Name: keyName, Type: ref("string")}},
				want: "key: string",
			},
			{
				name: "writes a rest parameter as an array behind three dots",
				give: []*emit.Param{{Name: "rest", Type: ref("number"), Variadic: symbol.VariadicPositional}},
				want: "...rest: number[]",
			},
			{
				name: "writes an unnamed parameter under the discard name",
				give: []*emit.Param{{Type: ref("string")}},
				want: "_: string",
			},
			{
				name: "numbers the discard name from the second unnamed parameter",
				give: []*emit.Param{{Type: ref("string")}, {Type: ref("number")}},
				want: "_: string, _1: number",
			},
			{
				name: "writes a default behind an equals sign",
				give: []*emit.Param{{Name: "limit", Type: ref("number"), Default: "8"}},
				want: "limit: number = 8",
			},
			{
				name: "writes an optional parameter behind its marker",
				give: []*emit.Param{{Name: "limit", Type: ref("number"), Optional: true}},
				want: "limit?: number",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := params(t, tt.give...)
				assert.NoError(t, err, "the list spells")
				assert.Equal(t, got, tt.want, "the TypeScript list")
			})
		}

		t.Run("returns an error for a type that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Params([]*emit.Param{{Name: "rows", Type: unspellable(t, s)}})
			assert.HasError(t, err, "the type's refusal is the list's")
		})

		refusals := []struct {
			name string
			give *emit.Param
		}{
			{
				name: "returns an error for a rest parameter with a default",
				give: &emit.Param{Name: "rest", Type: ref("number"), Variadic: symbol.VariadicPositional, Default: "8"},
			},
			{
				name: "returns an error for an optional rest parameter",
				give: &emit.Param{
					Name:     "rest",
					Type:     ref("number"),
					Variadic: symbol.VariadicPositional,
					Optional: true,
				},
			},
			{
				name: "returns an error for an optional parameter with a default",
				give: &emit.Param{Name: "limit", Type: ref("number"), Optional: true, Default: "8"},
			},
			{
				name: "returns an error for a name that is no identifier",
				give: &emit.Param{Name: wireName, Type: ref("string")},
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := params(t, tt.give)
				assert.HasError(t, err, "TypeScript cannot state the parameter")
			})
		}
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

		t.Run("writes a synchronous callable's results", func(t *testing.T) {
			t.Parallel()

			load := &emit.Function{Name: "load", Returns: []*emit.Return{{Type: ref(rowName)}}}
			assert.Equal(t, returns(t, load), ": Row", "the result's type")
		})

		t.Run("wraps an async callable's result in a promise", func(t *testing.T) {
			t.Parallel()

			load := &emit.Function{Name: "load", Async: true, Returns: []*emit.Return{{Type: ref(rowName)}}}
			assert.Equal(t, returns(t, load), ": Promise<Row>", "the promise of the result")
		})

		t.Run("wraps an async method returning nothing in a promise of void", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, returns(t, &emit.Method{Name: "flush", Async: true}), ": Promise<void>",
				"void inside the promise")
		})

		t.Run("writes nothing for a setter", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, returns(t, &emit.Method{Name: sizeName, Accessor: symbol.AccessorSet}), "",
				"TypeScript forbids a setter's annotation")
		})

		t.Run("returns an error for an async result that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			load := &emit.Function{Name: "load", Async: true, Returns: []*emit.Return{{Type: unspellable(t, s)}}}
			_, err := s.Returns(load)
			assert.HasError(t, err, "the result's refusal is the annotation's")
		})

		t.Run("returns an error for an async callable in a file that declares Promise", func(t *testing.T) {
			t.Parallel()

			s, set := speller()
			set.Reserve(promiseType)
			_, err := s.Returns(&emit.Function{Name: "load", Async: true})
			assert.HasError(t, err, "the file's Promise hides the global one")
			assert.Contains(t, err.Error(), "hides the global type "+promiseType,
				"the refusal contains the hidden type")
		})
	})

	t.Run("Results", func(t *testing.T) {
		t.Parallel()

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

		t.Run("returns an error for a result that does not spell", func(t *testing.T) {
			t.Parallel()

			s, _ := speller()
			_, err := s.Results([]*emit.Return{{Type: unspellable(t, s)}})
			assert.HasError(t, err, "the result's refusal is the annotation's")
		})
	})

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

	t.Run("PropKey", func(t *testing.T) {
		t.Parallel()

		t.Run("writes an identifier bare", func(t *testing.T) {
			t.Parallel()

			got, err := backend.PropKey(&emit.Field{Name: idName})
			assert.NoError(t, err, "the key spells")
			assert.Equal(t, got, idName, "unquoted")
		})

		t.Run("quotes a name that is no identifier", func(t *testing.T) {
			t.Parallel()

			got, err := backend.PropKey(&emit.Field{Name: wireName})
			assert.NoError(t, err, "the key spells")
			assert.Equal(t, got, "'content-type'", "the wire name whole")
		})

		t.Run("returns an error for a hard-private name that is no identifier", func(t *testing.T) {
			t.Parallel()

			_, err := backend.PropKey(&emit.Field{Name: wireName, Hard: true})
			assert.HasError(t, err, "# admits no quoted form")
		})
	})

	t.Run("MethodKey", func(t *testing.T) {
		t.Parallel()

		t.Run("writes an identifier bare", func(t *testing.T) {
			t.Parallel()

			got, err := backend.MethodKey(&emit.Method{Name: makeName})
			assert.NoError(t, err, "the key spells")
			assert.Equal(t, got, makeName, "unquoted")
		})

		t.Run("quotes a name that is no identifier", func(t *testing.T) {
			t.Parallel()

			got, err := backend.MethodKey(&emit.Method{Name: wireName})
			assert.NoError(t, err, "the key spells")
			assert.Equal(t, got, "'content-type'", "the wire name whole")
		})

		t.Run("returns an error for a hard-private name that is no identifier", func(t *testing.T) {
			t.Parallel()

			_, err := backend.MethodKey(&emit.Method{Name: wireName, Hard: true})
			assert.HasError(t, err, "# admits no quoted form")
		})
	})

	t.Run("EnumKey", func(t *testing.T) {
		t.Parallel()

		t.Run("writes an identifier bare", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.EnumKey(&emit.EnumVariant{Name: openName}), openName, "unquoted")
		})

		t.Run("quotes any other name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.EnumKey(&emit.EnumVariant{Name: wireName}), "'content-type'",
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

			got, err := backend.AccessorKw(getter())
			assert.NoError(t, err, "the getter spells")
			assert.Equal(t, got, "get ", "its keyword")
		})

		t.Run("writes set for a setter", func(t *testing.T) {
			t.Parallel()

			got, err := backend.AccessorKw(setter())
			assert.NoError(t, err, "the setter spells")
			assert.Equal(t, got, "set ", "its keyword")
		})

		takingGetter := getter()
		takingGetter.Params = []*emit.Param{{Name: "v", Type: ref("number")}}
		emptyGetter := getter()
		emptyGetter.Returns = nil
		genericGetter := getter()
		genericGetter.TypeParams = []*emit.TypeParam{{Name: "T"}}
		bareSetter := setter()
		bareSetter.Params = nil
		returningSetter := setter()
		returningSetter.Returns = []*emit.Return{{Type: ref("number")}}
		genericSetter := setter()
		genericSetter.TypeParams = []*emit.TypeParam{{Name: "T"}}
		tests := []struct {
			name string
			give *emit.Method
		}{
			{name: "returns an error for a getter taking parameters", give: takingGetter},
			{name: "returns an error for a getter returning nothing", give: emptyGetter},
			{name: "returns an error for a getter with type parameters", give: genericGetter},
			{name: "returns an error for a setter without its value", give: bareSetter},
			{name: "returns an error for a setter that returns", give: returningSetter},
			{name: "returns an error for a setter with type parameters", give: genericSetter},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.AccessorKw(tt.give)
				assert.HasError(t, err, "the accessor's shape refuses it")
			})
		}
	})

	t.Run("Hard", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the prefix for a hard-private member", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Hard(&emit.Field{Name: keyName, Hard: true})
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

			_, err := backend.Hard(&emit.Field{Name: keyName, Hard: true, Visibility: symbol.VisibilityPrivate})
			assert.HasError(t, err, "the two mechanisms cannot combine")
		})
	})

	t.Run("Mods", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Symbol
			want string
		}{
			{name: "writes export for an unstated visibility", give: &emit.Struct{Name: rowName}, want: exportWord},
			{
				name: "writes export abstract for an abstract class",
				give: &emit.Struct{Name: rowName, Abstract: true}, want: "export abstract ",
			},
			{
				name: "writes nothing for a package scope",
				give: &emit.Variable{Name: countParam, Visibility: symbol.VisibilityPackage}, want: "",
			},
			{
				name: "writes export async for an async function",
				give: &emit.Function{Name: "load", Async: true}, want: "export async ",
			},
			{
				name: "writes export for a constant with a value",
				give: &emit.Constant{Name: "MAX", Value: "8"}, want: exportWord,
			},
			{name: "writes export for an interface", give: &emit.Interface{Name: "Store"}, want: exportWord},
			{name: "writes export for an alias", give: &emit.Alias{Name: "ID"}, want: exportWord},
			{name: "writes export for an enum", give: &emit.Enum{Name: phaseName}, want: exportWord},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Mods(tt.give)
				assert.NoError(t, err, "the keywords spell")
				assert.Equal(t, got, tt.want, "in TypeScript's order")
			})
		}

		decorated := symbol.Annotations{{Name: logDecorate}}
		withMembers := &emit.Enum{Name: phaseName}
		withMembers.Methods.Append(&emit.Method{Name: "describe"})
		refusals := []struct {
			name string
			give symbol.Symbol
		}{
			{name: "returns an error for a final class", give: &emit.Struct{Name: rowName, Final: true}},
			{
				name: "returns an error for a protected class",
				give: &emit.Struct{Name: rowName, Visibility: symbol.VisibilityProtected},
			},
			{
				name: "returns an error for a protected alias",
				give: &emit.Alias{Name: "ID", Visibility: symbol.VisibilityProtected},
			},
			{
				name: "returns an error for a decorated interface",
				give: &emit.Interface{Name: "Store", Annotations: decorated},
			},
			{
				name: "returns an error for a decorated function",
				give: &emit.Function{Name: "load", Annotations: decorated},
			},
			{
				name: "returns an error for a function stating throws",
				give: &emit.Function{Name: "load", Throws: []*emit.TypeRef{ref("Error")}},
			},
			{
				name: "returns an error for a private function",
				give: &emit.Function{Name: "load", Visibility: symbol.VisibilityPrivate},
			},
			{name: "returns an error for a decorated alias", give: &emit.Alias{Name: "ID", Annotations: decorated}},
			{name: "returns an error for a defined alias", give: &emit.Alias{Name: "ID", Defined: true}},
			{name: "returns an error for a decorated enum", give: &emit.Enum{Name: phaseName, Annotations: decorated}},
			{name: "returns an error for an enum with members", give: withMembers},
			{
				name: "returns an error for a decorated constant",
				give: &emit.Constant{Name: "MAX", Value: "8", Annotations: decorated},
			},
			{name: "returns an error for a constant without a value", give: &emit.Constant{Name: "MAX"}},
			{
				name: "returns an error for a decorated variable",
				give: &emit.Variable{Name: countParam, Annotations: decorated},
			},
			{name: "returns an error for a method", give: &emit.Method{Name: "load"}},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.Mods(tt.give)
				assert.HasError(t, err, "a module-level declaration cannot state it")
			})
		}
	})

	t.Run("MemberMods", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give symbol.Symbol
			want string
		}{
			{
				name: "writes a field's keywords in TypeScript's order",
				give: &emit.Field{
					Name: keyName, Visibility: symbol.VisibilityPrivate,
					Level: symbol.LevelType, Mutability: symbol.MutabilityImmutable,
				},
				want: "private static readonly ",
			},
			{
				name: "writes override before async",
				give: &emit.Method{Name: "load", Override: true, Async: true},
				want: "override async ",
			},
			{
				name: "writes abstract for a method without a body",
				give: &emit.Method{Name: "load", Abstract: true},
				want: "abstract ",
			},
			{
				name: "leaves out async for an async abstract method",
				give: &emit.Method{Name: "load", Abstract: true, Async: true},
				want: "abstract ",
			},
			{
				name: "leaves out async for an async getter",
				give: &emit.Method{Name: sizeName, Accessor: symbol.AccessorGet, Async: true},
				want: "",
			},
			{
				name: "writes static for a type-level method",
				give: &emit.Method{Name: makeName, Level: symbol.LevelType},
				want: "static ",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.MemberMods(tt.give)
				assert.NoError(t, err, "the keywords spell")
				assert.Equal(t, got, tt.want, "in TypeScript's order")
			})
		}

		refusals := []struct {
			name string
			give symbol.Symbol
		}{
			{name: "returns an error for a final method", give: &emit.Method{Name: "load", Final: true}},
			{name: "returns an error for a method's default", give: &emit.Method{Name: "load", HasDefault: true}},
			{
				name: "returns an error for an abstract method with a body",
				give: &emit.Method{Name: "load", Abstract: true, Body: emit.Body{Verbatim: "return 1;"}},
			},
			{
				name: "returns an error for a method stating throws",
				give: &emit.Method{Name: "load", Throws: []*emit.TypeRef{ref("Error")}},
			},
			{
				name: "returns an error for a field's package scope",
				give: &emit.Field{Name: keyName, Visibility: symbol.VisibilityPackage},
			},
			{
				name: "returns an error for a method's package scope",
				give: &emit.Method{Name: "load", Visibility: symbol.VisibilityPackage},
			},
			{name: "returns an error for an alias", give: &emit.Alias{Name: "ID"}},
		}
		for _, tt := range refusals {
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

			got, err := backend.IndexMods(&emit.Method{Name: indexName, Indexer: true})
			assert.NoError(t, err, "the signature spells")
			assert.Equal(t, got, "", "no keyword")
		})

		t.Run("writes static for a type-level index signature", func(t *testing.T) {
			t.Parallel()

			got, err := backend.IndexMods(&emit.Method{Name: indexName, Indexer: true, Level: symbol.LevelType})
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
				give: &emit.Method{Annotations: symbol.Annotations{{Name: logDecorate}}},
			},
			{name: "returns an error for a body", give: &emit.Method{Body: emit.Body{Verbatim: "return 1;"}}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tt.give.Name, tt.give.Indexer = indexName, true
				_, err := backend.IndexMods(tt.give)
				assert.HasError(t, err, "an index signature takes static alone")
			})
		}
	})

	t.Run("CtorMods", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a public constructor", func(t *testing.T) {
			t.Parallel()

			got, err := backend.CtorMods(&emit.Method{Name: makeName, Constructs: true})
			assert.NoError(t, err, "the constructor spells")
			assert.Equal(t, got, "", "public is implicit")
		})

		t.Run("writes a stated accessibility", func(t *testing.T) {
			t.Parallel()

			ctor := &emit.Method{Name: makeName, Constructs: true, Visibility: symbol.VisibilityPrivate}
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
				give: &emit.Method{Annotations: symbol.Annotations{{Name: logDecorate}}},
			},
			{name: "returns an error for a package scope", give: &emit.Method{Visibility: symbol.VisibilityPackage}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				tt.give.Name, tt.give.Constructs = makeName, true
				_, err := backend.CtorMods(tt.give)
				assert.HasError(t, err, "a constructor takes an accessibility alone")
			})
		}
	})

	t.Run("PropMods", func(t *testing.T) {
		t.Parallel()

		t.Run("writes readonly for an immutable property", func(t *testing.T) {
			t.Parallel()

			got, err := backend.PropMods(&emit.Field{Name: keyName, Mutability: symbol.MutabilityImmutable})
			assert.NoError(t, err, "the property spells")
			assert.Equal(t, got, "readonly ", "the one keyword")
		})

		t.Run("returns nothing for a mutable property", func(t *testing.T) {
			t.Parallel()

			got, err := backend.PropMods(&emit.Field{Name: keyName})
			assert.NoError(t, err, "the property spells")
			assert.Equal(t, got, "", "no keyword")
		})

		tests := []struct {
			name string
			give *emit.Field
		}{
			{name: "returns an error for a hard-private name", give: &emit.Field{Name: keyName, Hard: true}},
			{
				name: "returns an error for decorators",
				give: &emit.Field{Name: keyName, Annotations: symbol.Annotations{{Name: logDecorate}}},
			},
			{
				name: "returns an error for a stated scope",
				give: &emit.Field{Name: keyName, Visibility: symbol.VisibilityPrivate},
			},
			{name: "returns an error for a static property", give: &emit.Field{Name: keyName, Level: symbol.LevelType}},
			{name: "returns an error for an initializer", give: &emit.Field{Name: keyName, Value: "1"}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.PropMods(tt.give)
				assert.HasError(t, err, "an interface property cannot state it")
			})
		}
	})

	t.Run("SigMods", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a bare signature", func(t *testing.T) {
			t.Parallel()

			got, err := backend.SigMods(&emit.Method{Name: "load"})
			assert.NoError(t, err, "the signature passes")
			assert.Equal(t, got, "", "no keyword")
		})

		t.Run("returns nothing for an async method signature", func(t *testing.T) {
			t.Parallel()

			got, err := backend.SigMods(&emit.Method{Name: "load", Async: true})
			assert.NoError(t, err, "the promise of the annotation has the asynchrony")
			assert.Equal(t, got, "", "no keyword")
		})

		tests := []struct {
			name string
			give *emit.Method
		}{
			{name: "returns an error for a stated modifier", give: &emit.Method{Name: "load", Override: true}},
			{
				name: "returns an error for an async index signature",
				give: &emit.Method{Name: indexName, Indexer: true, Async: true},
			},
			{
				name: "returns an error for an async construct signature",
				give: &emit.Method{Name: makeName, Constructs: true, Async: true},
			},
			{
				name: "returns an error for a body",
				give: &emit.Method{Name: "load", Body: emit.Body{Verbatim: "return 1;"}},
			},
			{
				name: "returns an error for decorators",
				give: &emit.Method{Name: "load", Annotations: symbol.Annotations{{Name: logDecorate}}},
			},
			{
				name: "returns an error for a stated scope",
				give: &emit.Method{Name: "load", Visibility: symbol.VisibilityPrivate},
			},
			{
				name: "returns an error for an accessor",
				give: &emit.Method{Name: sizeName, Accessor: symbol.AccessorGet},
			},
			{name: "returns an error for a hard-private name", give: &emit.Method{Name: "load", Hard: true}},
			{
				name: "returns an error for a throws clause",
				give: &emit.Method{Name: "load", Throws: []*emit.TypeRef{ref("Error")}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.SigMods(tt.give)
				assert.HasError(t, err, "an interface method is a bare signature")
			})
		}
	})

	t.Run("Binding", func(t *testing.T) {
		t.Parallel()

		t.Run("writes let for a mutable binding", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Binding(&emit.Variable{Name: countParam})
			assert.NoError(t, err, "the binding spells")
			assert.Equal(t, got, "let", "the mutable keyword")
		})

		t.Run("writes const for an immutable binding with a value", func(t *testing.T) {
			t.Parallel()

			count := &emit.Variable{Name: countParam, Mutability: symbol.MutabilityImmutable, Value: "0"}
			got, err := backend.Binding(count)
			assert.NoError(t, err, "the binding spells")
			assert.Equal(t, got, "const", "the immutable keyword")
		})

		t.Run("returns an error for an immutable binding without a value", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Binding(&emit.Variable{Name: countParam, Mutability: symbol.MutabilityImmutable})
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

// A keyword helper, a guard and a name spelled as written allocate
// nothing, and every other helper allocates the text it writes. The
// ordinary run, which runs no benchmark, checks those ceilings here.
func TestVocabularyAllocs(t *testing.T) {
	checkAllocs(t, vocabularyCalls())
}

// BenchmarkVocabulary measures each helper a kind template calls per
// declaration, and the vocabulary the render binds per file.
func BenchmarkVocabulary(b *testing.B) {
	benchCalls(b, vocabularyCalls())
}

// vocabularyCalls returns a call of every function and method of
// vocabulary.go.
func vocabularyCalls() []allocCall {
	s, set := speller()
	bare, row := ref(numberType), imported(storeModule, rowName)
	beside, besideSet := speller()
	besideSet.SetHome("svc/client")
	module := &emit.TypeRef{
		Spelling: rowName, Target: symbol.Identity{Lang: typescript.Lang, Package: storePath, Name: rowName},
	}
	instance := &emit.TypeRef{Spelling: "Map", Args: []*emit.TypeRef{ref(stringType), bare}}
	index := indexer()
	params := []*emit.TypeParam{
		{Name: keyParam, Bounds: []*emit.TypeRef{ref(eqTrait), ref(hashTrait)}},
		{Name: valueParam},
	}
	heir := &emit.Struct{
		Name: rowName, Extends: []*emit.TypeRef{ref(baseName)},
		Implements: []*emit.TypeRef{ref(keyedName), ref(closerName)},
	}
	args := []*emit.Param{{Name: idName, Type: bare}, {Name: nameParam, Type: ref(stringType)}}
	three := []*emit.Param{{Name: idName, Type: bare}, {Name: nameParam, Type: bare}, {Name: countParam, Type: bare}}
	one := []*emit.Return{{Type: bare}}
	pair := []*emit.Return{{Type: bare}, {Type: ref(stringType)}}
	async := &emit.Function{Name: "load", Async: true, Returns: one}
	flush := &emit.Function{Name: "flush", Async: true}
	doc := []string{rowDoc}
	field, wire := &emit.Field{Name: idName}, &emit.Field{Name: wireName}
	method, variant := &emit.Method{Name: makeName}, &emit.EnumVariant{Name: openName}
	get, hard := getter(), &emit.Field{Name: keyName, Hard: true}
	class, abstract := &emit.Struct{Name: rowName}, &emit.Struct{Name: rowName, Abstract: true}
	member := &emit.Field{
		Name: keyName, Visibility: symbol.VisibilityPrivate,
		Level: symbol.LevelType, Mutability: symbol.MutabilityImmutable,
	}
	static := &emit.Method{Name: indexName, Indexer: true, Level: symbol.LevelType}
	ctor := &emit.Method{Name: makeName, Constructs: true, Visibility: symbol.VisibilityPrivate}
	prop := &emit.Field{Name: keyName, Mutability: symbol.MutabilityImmutable}
	sig, binding := &emit.Method{Name: "load"}, &emit.Variable{Name: countParam}
	annotations := symbol.Annotations{{Name: logDecorate}}
	var (
		spellerOut backend.Speller
		out        string
		err        error
		funcs      template.FuncMap
	)
	spells := func(want string) func(tb assert.TB) {
		return func(tb assert.TB) {
			assert.NoError(tb, err, "the helper spells")
			assert.Equal(tb, out, want, "the helper writes the TypeScript spelling")
		}
	}
	return []allocCall{
		{
			name: "NewSpeller", call: func() { spellerOut = backend.NewSpeller(set) },
			check: func(tb assert.TB) { assert.Equal(tb, spellerOut, s, "NewSpeller returns the set's speller") },
		},
		{
			name:     "Spell",
			caseName: "a builtin type",
			call:     func() { out, err = s.Spell(bare) },
			check:    spells(numberType),
		},
		{
			name:     "Spell",
			caseName: "an imported declaration",
			call:     func() { out, err = s.Spell(row) },
			check:    spells(rowName),
		},
		{
			name: "Spell", caseName: "a declaration of a module beside the file", allocs: textAllocs,
			call:  func() { out, err = beside.Spell(module) },
			check: spells(rowName),
		},
		{
			name: "Spell", caseName: "an instantiation", allocs: textAllocs,
			call: func() { out, err = s.Spell(instance) }, check: spells("Map<string, number>"),
		},
		{
			name: "IndexSig", allocs: textAllocs,
			call: func() { out, err = s.IndexSig(index) }, check: spells("[key: string]: Row;"),
		},
		{
			name: "TypeParams", allocs: typeParamsAllocs,
			call: func() { out, err = s.TypeParams(params) }, check: spells("<K extends Eq & Hash, V>"),
		},
		{
			name: "Heritage", allocs: heritageAllocs,
			call: func() { out, err = s.Heritage(heir) }, check: spells(" extends Base implements Keyed, Closer"),
		},
		{
			name: "Params", caseName: "two parameters", allocs: paramsAllocs,
			call: func() { out, err = s.Params(args) }, check: spells("id: number, name: string"),
		},
		{
			name: "Params", caseName: "three parameters", allocs: partsAllocs,
			call: func() { out, err = s.Params(three) }, check: spells("id: number, name: number, count: number"),
		},
		{
			name: "Returns", caseName: "an async function returning a value", allocs: textAllocs,
			call: func() { out, err = s.Returns(async) }, check: spells(": Promise<number>"),
		},
		{
			name: "Returns", caseName: "an async function returning nothing",
			call: func() { out, err = s.Returns(flush) }, check: spells(": Promise<void>"),
		},
		{
			name: "Results", caseName: "one result", allocs: textAllocs,
			call: func() { out, err = s.Results(one) }, check: spells(": number"),
		},
		{
			name: "Results", caseName: "a tuple of two", allocs: resultTupleAllocs,
			call: func() { out, err = s.Results(pair) }, check: spells(": [number, string]"),
		},
		{
			name: "Funcs", allocs: funcsAllocs,
			call:  func() { funcs = backend.Funcs(set) },
			check: func(tb assert.TB) { assert.Length(tb, funcs, 21, "Funcs returns the twenty-one helpers") },
		},
		{
			name: "Docs", allocs: textAllocs,
			call: func() { out = backend.Docs(doc) }, check: spells("/**\n * " + rowDoc + "\n */\n"),
		},
		{
			name:     "PropKey",
			caseName: "a plain key",
			call:     func() { out, err = backend.PropKey(field) },
			check:    spells(idName),
		},
		{
			name: "PropKey", caseName: "a quoted key", allocs: textAllocs,
			call: func() { out, err = backend.PropKey(wire) }, check: spells("'content-type'"),
		},
		{name: "MethodKey", call: func() { out, err = backend.MethodKey(method) }, check: spells(makeName)},
		{name: "EnumKey", call: func() { out = backend.EnumKey(variant) }, check: spells(openName)},
		{name: "AccessorKw", call: func() { out, err = backend.AccessorKw(get) }, check: spells("get ")},
		{name: "Hard", call: func() { out, err = backend.Hard(hard) }, check: spells("#")},
		{
			name:     "Mods",
			caseName: "an exported class",
			call:     func() { out, err = backend.Mods(class) },
			check:    spells(exportWord),
		},
		{
			name: "Mods", caseName: "an abstract class", allocs: textAllocs,
			call: func() { out, err = backend.Mods(abstract) }, check: spells("export abstract "),
		},
		{
			name: "MemberMods", allocs: joinedKeywordsAllocs,
			call: func() { out, err = backend.MemberMods(member) }, check: spells("private static readonly "),
		},
		{name: "IndexMods", call: func() { out, err = backend.IndexMods(static) }, check: spells("static ")},
		{name: "CtorMods", call: func() { out, err = backend.CtorMods(ctor) }, check: spells("private ")},
		{name: "PropMods", call: func() { out, err = backend.PropMods(prop) }, check: spells("readonly ")},
		{name: "SigMods", call: func() { out, err = backend.SigMods(sig) }, check: spells("")},
		{name: "Binding", call: func() { out, err = backend.Binding(binding) }, check: spells("let")},
		{
			name: "Decorators", allocs: textAllocs,
			call: func() { out = backend.Decorators(annotations) }, check: spells("@log\n"),
		},
	}
}

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

// getter returns a getter of one number.
func getter() *emit.Method {
	return &emit.Method{Name: sizeName, Accessor: symbol.AccessorGet, Returns: []*emit.Return{{Type: ref(numberType)}}}
}

// setter returns a setter of one number.
func setter() *emit.Method {
	return &emit.Method{
		Name: sizeName, Accessor: symbol.AccessorSet, Params: []*emit.Param{{Name: "v", Type: ref(numberType)}},
	}
}
