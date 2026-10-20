// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The allocations of a spelling: none for a name, and the text of a
// structural type.
const (
	nameSpellAllocs = 0
	formSpellAllocs = 1
)

// npRefusal is the refusal of a reference through the module numpy under the
// name np, which a declaration of the file binds.
const npRefusal = "python: np.ndarray names the module numpy as np, and the file binds np otherwise"

// TestTypes pins the spelling of every form of a type reference in an
// annotation and where Python evaluates it, and the imports that each
// spelling records.
func TestTypes(t *testing.T) {
	t.Parallel()

	t.Run("Writer", func(t *testing.T) {
		t.Parallel()

		t.Run("Spell", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name        string
				give        *emit.TypeRef
				giveVersion plugin.Choice
				want        string
				wantImports []render.Entry
			}{
				{name: "returns a builtin by its name", give: intRef, want: "int"},
				{
					name: "returns a class of another module under the name that its import binds",
					give: decimalRef, want: "Decimal",
					wantImports: []render.Entry{{Path: "decimal", Name: "Decimal"}},
				},
				{
					name: "returns a nested class of another module after the class that its import binds",
					give: &emit.TypeRef{Spelling: "Store.Key", Package: "shop/stores"},
					want: "Store.Key", wantImports: []render.Entry{{Path: "shop/stores", Name: "Store"}},
				},
				{
					name: "returns a nested class of a target after the class that its import binds",
					give: &emit.TypeRef{
						Spelling: "Store.Key", Package: "shop/stores",
						Target: symbol.Identity{Lang: python.Lang, Package: "shop/stores", Owner: "Store", Name: "Key"},
					},
					want: "Store.Key", wantImports: []render.Entry{{Path: "shop/stores", Name: "Store"}},
				},
				{
					name: "returns a reference through a module after the import of the module",
					give: &emit.TypeRef{
						Spelling: "json.JSONDecoder", Package: "json",
						Target: symbol.Identity{Lang: python.Lang, Package: "json/decoder", Name: "JSONDecoder"},
					},
					want: "json.JSONDecoder", wantImports: []render.Entry{{Path: "json"}},
				},
				{
					name: "returns a reference through a module without a target after the import of the module",
					give: &emit.TypeRef{Spelling: "json.JSONDecoder", Package: "json"},
					want: "json.JSONDecoder", wantImports: []render.Entry{{Path: "json"}},
				},
				{
					name: "returns a reference through a module without a package after the import of its names",
					give: &emit.TypeRef{
						Spelling: "os.path.PathLike",
						Target:   symbol.Identity{Lang: python.Lang, Package: "os", Name: "PathLike"},
					},
					want: "os.path.PathLike", wantImports: []render.Entry{{Path: "os/path"}},
				},
				{
					name: "returns a reference through a renamed module after the import of the module as its name",
					give: &emit.TypeRef{
						Spelling: "np.ndarray", Package: "numpy",
						Target: symbol.Identity{Lang: python.Lang, Package: "numpy", Name: "ndarray"},
					},
					want: "np.ndarray", wantImports: []render.Entry{{Path: "numpy\x00", Name: "np"}},
				},
				{
					name: "returns a class of the module by its name",
					give: &emit.TypeRef{Spelling: rowName, Package: home}, want: rowName,
				},
				{
					name: "returns the arguments of a generic class in brackets",
					give: &emit.TypeRef{Spelling: "Store", Args: []*emit.TypeRef{intRef, strRef}},
					want: "Store[int, str]",
				},
				{
					name: "returns a name of typing that the version adds from typing_extensions before",
					give: &emit.TypeRef{Spelling: "ReadOnly", Package: "typing"}, giveVersion: python.Version312,
					want: "ReadOnly", wantImports: []render.Entry{{Path: "typing_extensions", Name: "ReadOnly"}},
				},
				{
					name: "returns a name of typing that the version has from typing",
					give: &emit.TypeRef{Spelling: "ReadOnly", Package: "typing"}, giveVersion: python.Version313,
					want: "ReadOnly", wantImports: []render.Entry{{Path: "typing", Name: "ReadOnly"}},
				},
				{
					name: "returns an optional as the union of its type and None",
					give: &emit.TypeRef{Form: symbol.FormOptional, Elems: []*emit.TypeRef{intRef}},
					want: "int | None",
				},
				{
					name: "returns a union with bars between its members",
					give: &emit.TypeRef{Form: symbol.FormUnion, Elems: []*emit.TypeRef{intRef, strRef}},
					want: "int | str",
				},
				{
					name: "returns a union without members as Never",
					give: &emit.TypeRef{Form: symbol.FormUnion}, want: "Never",
					wantImports: []render.Entry{{Path: "typing", Name: "Never"}},
				},
				{
					name: "returns a tuple with its members",
					give: &emit.TypeRef{Form: symbol.FormTuple, Elems: []*emit.TypeRef{intRef, strRef}},
					want: "tuple[int, str]",
				},
				{
					name: "returns a tuple without members as the empty tuple",
					give: &emit.TypeRef{Form: symbol.FormTuple}, want: "tuple[()]",
				},
				{
					name: "returns a list as list",
					give: &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{intRef}},
					want: "list[int]",
				},
				{
					name: "returns an immutable list as a tuple of any length",
					give: &emit.TypeRef{
						Form: symbol.FormList, Mutability: symbol.MutabilityImmutable, Elems: []*emit.TypeRef{intRef},
					},
					want: "tuple[int, ...]",
				},
				{
					name: "returns an array as a tuple of its length",
					give: &emit.TypeRef{Form: symbol.FormArray, Length: 2, Elems: []*emit.TypeRef{intRef}},
					want: "tuple[int, int]",
				},
				{
					name: "returns a map as dict",
					give: &emit.TypeRef{Form: symbol.FormMap, Elems: []*emit.TypeRef{strRef, intRef}},
					want: "dict[str, int]",
				},
				{
					name: "returns an immutable map as a Mapping of collections.abc",
					give: &emit.TypeRef{
						Form: symbol.FormMap, Mutability: symbol.MutabilityImmutable,
						Elems: []*emit.TypeRef{strRef, intRef},
					},
					want: "Mapping[str, int]", wantImports: []render.Entry{{Path: "collections/abc", Name: "Mapping"}},
				},
				{
					name:        "returns a function type without a result as a Callable that returns None",
					give:        &emit.TypeRef{Form: symbol.FormFunc, Split: 1, Elems: []*emit.TypeRef{intRef}},
					want:        "Callable[[int], None]",
					wantImports: []render.Entry{{Path: "collections/abc", Name: "Callable"}},
				},
				{
					name:        "returns a function type with one result as a Callable that returns it",
					give:        &emit.TypeRef{Form: symbol.FormFunc, Split: 1, Elems: []*emit.TypeRef{intRef, strRef}},
					want:        "Callable[[int], str]",
					wantImports: []render.Entry{{Path: "collections/abc", Name: "Callable"}},
				},
				{
					name:        "returns a function type with two results as a Callable that returns a tuple",
					give:        &emit.TypeRef{Form: symbol.FormFunc, Elems: []*emit.TypeRef{intRef, strRef}},
					want:        "Callable[[], tuple[int, str]]",
					wantImports: []render.Entry{{Path: "collections/abc", Name: "Callable"}},
				},
				{
					name: "returns a stream as an Iterator",
					give: &emit.TypeRef{Form: symbol.FormStream, Elems: []*emit.TypeRef{intRef}},
					want: "Iterator[int]", wantImports: []render.Entry{{Path: "collections/abc", Name: "Iterator"}},
				},
				{
					name:        "returns an asynchronous stream as an AsyncIterator",
					give:        &emit.TypeRef{Form: symbol.FormStream, Async: true, Elems: []*emit.TypeRef{intRef}},
					want:        "AsyncIterator[int]",
					wantImports: []render.Entry{{Path: "collections/abc", Name: "AsyncIterator"}},
				},
				{
					name: "returns a borrow as its type",
					give: &emit.TypeRef{Form: symbol.FormBorrow, Elems: []*emit.TypeRef{intRef}}, want: "int",
				},
				{
					name: "returns a wildcard as its bound",
					give: &emit.TypeRef{Form: symbol.FormWildcard, Elems: []*emit.TypeRef{intRef}}, want: "int",
				},
				{
					name: "returns a wildcard without a bound as object",
					give: &emit.TypeRef{Form: symbol.FormWildcard}, want: "object",
				},
				{name: "returns no reference as object", want: "object"},
				{name: "returns a reference without a spelling as object", give: &emit.TypeRef{}, want: "object"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, cmp.Or(tt.giveVersion, python.Version311), false)
					got, err := w.Spell(tt.give)
					assert.NoError(t, err, "Python spells the form")
					assert.Equal(t, got, tt.want, "the spelling")
					assert.Equal(t, set.Entries(), tt.wantImports, "the imports of the spelling", assert.EquateEmpty())
				})
			}

			t.Run("returns a qualified reference whose first name the header binds", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				_, err := w.Bind(&emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{"import numpy as np"},
				}}})
				assert.NoError(t, err, "the header binds the module")
				got, err := w.Spell(&emit.TypeRef{Spelling: "np.ndarray", Package: "numpy"})
				assert.NoError(t, err, "Python spells the reference")
				assert.Equal(t, got, "np.ndarray", "the spelling of the reference")
				assert.Equal(t, set.Entries(), []render.Entry{{Path: "numpy\x00", Name: "np"}},
					"the import of the header alone")
			})

			starred := []struct {
				name        string
				give        string
				wantImports []render.Entry
			}{
				{
					name: "returns the name of a declaration of a module that the header imports with a star",
					give: "Money", wantImports: []render.Entry{{Path: "shop/models", Name: "*"}},
				},
				{
					name: "returns an import of a name with an underscore of a module that the header imports with a star",
					give: "_Money",
					wantImports: []render.Entry{
						{Path: "shop/models", Name: "*"}, {Path: "shop/models", Name: "_Money"},
					},
				},
			}
			for _, tt := range starred {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, python.Version311, false)
					_, err := w.Bind(&emit.File{Annotations: symbol.Annotations{{
						Name: string(python.ImportMarker), Args: []string{"from shop.models import *"},
					}}})
					assert.NoError(t, err, "the header binds the star import")
					got, err := w.Spell(&emit.TypeRef{Spelling: tt.give, Package: "shop/models"})
					assert.NoError(t, err, "Python spells the reference")
					assert.Equal(t, got, tt.give, "the name of the declaration")
					assert.Equal(t, set.Entries(), tt.wantImports, "the imports of the file")
				})
			}

			t.Run("returns an error for the module of a reference under a name of the file", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				set.Reserve("np")
				_, err := w.Spell(&emit.TypeRef{
					Spelling: "np.ndarray", Package: "numpy",
					Target: symbol.Identity{Lang: python.Lang, Package: "numpy", Name: "ndarray"},
				})
				assert.HasError(t, err, "a declaration of the file binds the name of the module")
				assert.Equal(t, err.Error(), npRefusal, "the refusal names the reference and the module")
			})

			t.Run("returns an error for a module whose name a declaration of the file takes", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				set.Reserve("json")
				_, err := w.Spell(&emit.TypeRef{Spelling: "json.JSONDecoder", Package: "json"})
				assert.HasError(t, err, "a declaration of the file takes the name of the module")
				assert.Equal(t, err.Error(),
					"python: json.JSONDecoder names the module json, and a declaration of the file takes the name json",
					"the refusal names the reference and the module")
			})

			t.Run("returns builtins.int for a builtin that a declaration of the file hides", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				set.Reserve("int")
				got, err := w.Spell(&emit.TypeRef{Spelling: "int", Package: "builtins"})
				assert.NoError(t, err, "Python spells the builtin")
				assert.Equal(t, got, "builtins.int", "the qualified builtin")
				assert.Equal(t, set.Entries(), []render.Entry{{Path: "builtins"}}, "the import of builtins")
			})

			t.Run("returns a builtin by its name where no declaration of the file hides it", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				got, err := w.Spell(&emit.TypeRef{Spelling: "int", Package: "builtins"})
				assert.NoError(t, err, "Python spells the builtin")
				assert.Equal(t, got, "int", "the builtin")
				assert.Empty(t, set.Entries(), "a builtin needs no import")
			})

			hidden := []struct {
				name     string
				giveName string
				give     *emit.TypeRef
				want     string
			}{
				{
					name:     "returns builtins.tuple for a tuple where a declaration of the file hides tuple",
					giveName: "tuple", give: &emit.TypeRef{Form: symbol.FormTuple, Elems: []*emit.TypeRef{intRef}},
					want: "builtins.tuple[int]",
				},
				{
					name:     "returns builtins.tuple for the empty tuple where a declaration of the file hides tuple",
					giveName: "tuple", give: &emit.TypeRef{Form: symbol.FormTuple}, want: "builtins.tuple[()]",
				},
				{
					name:     "returns builtins.tuple for an immutable list where a declaration of the file hides tuple",
					giveName: "tuple",
					give: &emit.TypeRef{
						Form: symbol.FormList, Mutability: symbol.MutabilityImmutable, Elems: []*emit.TypeRef{intRef},
					},
					want: "builtins.tuple[int, ...]",
				},
				{
					name:     "returns builtins.tuple for an array where a declaration of the file hides tuple",
					giveName: "tuple",
					give:     &emit.TypeRef{Form: symbol.FormArray, Length: 2, Elems: []*emit.TypeRef{intRef}},
					want:     "builtins.tuple[int, int]",
				},
				{
					name:     "returns builtins.tuple for the results of a function type where a declaration hides tuple",
					giveName: "tuple",
					give:     &emit.TypeRef{Form: symbol.FormFunc, Elems: []*emit.TypeRef{intRef, strRef}},
					want:     "Callable[[], builtins.tuple[int, str]]",
				},
				{
					name:     "returns builtins.list for a list where a declaration of the file hides list",
					giveName: "list", give: &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{intRef}},
					want: "builtins.list[int]",
				},
				{
					name:     "returns builtins.dict for a map where a declaration of the file hides dict",
					giveName: "dict", give: &emit.TypeRef{Form: symbol.FormMap, Elems: []*emit.TypeRef{strRef, intRef}},
					want: "builtins.dict[str, int]",
				},
				{
					name:     "returns builtins.object for a wildcard without a bound where a declaration hides object",
					giveName: "object", give: &emit.TypeRef{Form: symbol.FormWildcard}, want: "builtins.object",
				},
				{
					name:     "returns builtins.object for no reference where a declaration of the file hides object",
					giveName: "object", want: "builtins.object",
				},
			}
			for _, tt := range hidden {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, python.Version311, false)
					set.Reserve(tt.giveName)
					got, err := w.Spell(tt.give)
					assert.NoError(t, err, "Python spells the form")
					assert.Equal(t, got, tt.want, "the qualified builtin")
					assert.Contains(t, set.Entries(), render.Entry{Path: "builtins"}, "the import of builtins")
				})
			}

			t.Run("returns Any of typing for no reference under the policy python.dynamic Any", func(t *testing.T) {
				t.Parallel()

				set := &render.ImportSet{}
				p, err := plugin.NewPolicy(python.Target, python.Policies(), map[plugin.PolicyKey]plugin.Choice{
					python.DynamicPolicy: python.Any,
				})
				assert.NoError(t, err, "the policy chooses Any")
				set.SetPolicy(p)
				got, err := backend.NewWriter(set, false).Spell(nil)
				assert.NoError(t, err, "Python spells a dynamic value")
				assert.Equal(t, got, "Any", "the dynamic type")
				assert.Equal(t, set.Entries(), []render.Entry{{Path: "typing", Name: "Any"}}, "the import of Any")
			})

			pending := []struct {
				name        string
				giveVersion plugin.Choice
				giveStub    bool
				give        *emit.TypeRef
				want        string
			}{
				{
					name: "returns a string for a declaration of the file that has not finished rendering",
					give: rowRef, giveVersion: python.Version313, want: `"Row"`,
				},
				{
					name:        "returns a string in single quotes for a pending spelling with a double quote",
					give:        &emit.TypeRef{Spelling: rowName, Args: []*emit.TypeRef{{Spelling: `Literal["a"]`}}},
					giveVersion: python.Version311, want: `'Row[Literal["a"]]'`,
				},
				{
					name: "returns an escaped string for a pending spelling with both quotes",
					give: &emit.TypeRef{
						Spelling: rowName, Args: []*emit.TypeRef{{Spelling: `Literal["a", 'b']`}},
					},
					giveVersion: python.Version311, want: `"Row[Literal[\"a\", 'b']]"`,
				},
				{
					name:        "returns a string that escapes the backslash of a pending spelling",
					give:        &emit.TypeRef{Spelling: rowName, Args: []*emit.TypeRef{{Spelling: `Literal["a\\b"]`}}},
					giveVersion: python.Version311, want: `'Row[Literal["a\\\\b"]]'`,
				},
				{
					name: "returns the name of a pending declaration from Python 3.14",
					give: rowRef, giveVersion: python.Version314, want: rowName,
				},
				{
					name: "returns the name of a pending declaration in a stub file",
					give: rowRef, giveVersion: python.Version311, giveStub: true, want: rowName,
				},
			}
			for _, tt := range pending {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, tt.giveVersion, tt.giveStub)
					set.Reserve(rowName)
					set.Enter(rowName)
					got, err := w.Spell(tt.give)
					assert.NoError(t, err, "Python spells the reference")
					assert.Equal(t, got, tt.want, "the annotation")
				})
			}

			refusals := []struct {
				name string
				give *emit.TypeRef
				want string
			}{
				{
					name: "returns an error for an intersection",
					give: &emit.TypeRef{
						Spelling: "A & B", Form: symbol.FormIntersection, Elems: []*emit.TypeRef{intRef, strRef},
					},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns an error for an inline type",
					give: &emit.TypeRef{Spelling: "{ id: string }", Form: symbol.FormInline},
					want: "python: { id: string } is an inline type, which Python declares as a class",
				},
				{
					name: "returns an error for a form of the projection",
					give: &emit.TypeRef{Spelling: "i64", Form: symbol.FormScalar},
					want: "python: i64 has a form of the projection, which no backend writes",
				},
				{
					name: "returns an error for an optional without its type",
					give: &emit.TypeRef{Spelling: "?", Form: symbol.FormOptional},
					want: "python: ? is a structural type without the type that it contains",
				},
				{
					name: "returns an error for an array of an unstated length",
					give: &emit.TypeRef{Spelling: "[]int", Form: symbol.FormArray, Elems: []*emit.TypeRef{intRef}},
					want: "python: []int is an array of an unstated length, which Python does not spell",
				},
				{
					name: "returns an error for an array without the type that it contains",
					give: &emit.TypeRef{Spelling: "[2]", Form: symbol.FormArray, Length: 2},
					want: "python: [2] is a structural type without the type that it contains",
				},
				{
					name: "returns an error for a function type whose results start outside its types",
					give: &emit.TypeRef{
						Spelling: "fn",
						Form:     symbol.FormFunc,
						Split:    2,
						Elems:    []*emit.TypeRef{intRef},
					},
					want: "python: fn is a function type whose results start at 2 of 1 types",
				},
				{
					name: "returns the error of a parameter of a function type",
					give: &emit.TypeRef{
						Form: symbol.FormFunc, Split: 1,
						Elems: []*emit.TypeRef{{Spelling: "A & B", Form: symbol.FormIntersection}},
					},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of a result of a function type",
					give: &emit.TypeRef{
						Form:  symbol.FormFunc,
						Elems: []*emit.TypeRef{{Spelling: "A & B", Form: symbol.FormIntersection}},
					},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of a member of a structural type",
					give: &emit.TypeRef{
						Form:  symbol.FormList,
						Elems: []*emit.TypeRef{{Spelling: "A & B", Form: symbol.FormIntersection}},
					},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version311, false)
					_, err := w.Spell(tt.give)
					assert.HasError(t, err, "Python refuses the reference")
					assert.Equal(t, err.Error(), tt.want, "the refusal names the reference")
				})
			}
		})

		t.Run("SpellValue", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the spelling of a declaration that has finished rendering", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				set.Reserve(nodeName, rowName)
				set.Enter(rowName)
				got, err := w.SpellValue(&emit.TypeRef{Spelling: nodeName})
				assert.NoError(t, err, "a module evaluates a declaration that it has defined")
				assert.Equal(t, got, nodeName, "the base")
			})

			t.Run("returns an error for a declaration of a module that has not finished rendering", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version314, false)
				set.Reserve(rowName)
				set.Enter(rowName)
				_, err := w.SpellValue(rowRef)
				assert.HasError(t, err, "a module refuses a value of a declaration that it defines later")
				assert.Equal(t, err.Error(),
					"python: Python evaluates Row when the module runs, and the module declares it later",
					"the refusal names the declaration")
			})

			t.Run("returns the name of a pending declaration in a stub file", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, true)
				set.Reserve(rowName)
				set.Enter(rowName)
				got, err := w.SpellValue(rowRef)
				assert.NoError(t, err, "a type checker reads a stub file without running it")
				assert.Equal(t, got, rowName, "the base")
			})

			t.Run("returns the error of the spelling", func(t *testing.T) {
				t.Parallel()

				w, _ := newWriter(t, python.Version311, false)
				_, err := w.SpellValue(&emit.TypeRef{Spelling: "A & B", Form: symbol.FormIntersection})
				assert.HasError(t, err, "Python refuses the reference")
			})
		})
	})
}

// Spell allocates nothing for a name, and the text of a structural type.
// The ordinary run, which runs no benchmark, checks those ceilings here.
func TestTypesAllocs(t *testing.T) {
	checkAllocs(t, typesCalls(t))
}

// BenchmarkTypes measures the spellings of the annotations of a file.
func BenchmarkTypes(b *testing.B) {
	b.Run("Writer", func(b *testing.B) {
		benchCalls(b, typesCalls(b))
	})
}

// typesCalls returns the calls of Spell for a builtin, for a class of the
// module builtins, for an imported class and for a list, and of SpellValue
// for a base.
func typesCalls(tb assert.TB) []allocCall {
	w, _ := newWriter(tb, python.Version311, false)
	list := &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{intRef}}
	var got string
	return []allocCall{
		{
			name: "Spell", caseName: "a builtin", allocs: nameSpellAllocs,
			call:  func() { got, _ = w.Spell(intRef) },
			check: func(tb assert.TB) { assert.Equal(tb, got, "int", "Spell writes the builtin") },
		},
		{
			name: "Spell", caseName: "a class of the module builtins", allocs: nameSpellAllocs,
			call:  func() { got, _ = w.Spell(bytesRef) },
			check: func(tb assert.TB) { assert.Equal(tb, got, "bytes", "Spell writes the builtin") },
		},
		{
			name: "Spell", caseName: "an imported class", allocs: nameSpellAllocs,
			call:  func() { got, _ = w.Spell(decimalRef) },
			check: func(tb assert.TB) { assert.Equal(tb, got, "Decimal", "Spell writes the class") },
		},
		{
			name: "Spell", caseName: "a list", allocs: formSpellAllocs,
			call:  func() { got, _ = w.Spell(list) },
			check: func(tb assert.TB) { assert.Equal(tb, got, "list[int]", "Spell writes the list") },
		},
		{
			name: "SpellValue", allocs: nameSpellAllocs,
			call:  func() { got, _ = w.SpellValue(decimalRef) },
			check: func(tb assert.TB) { assert.Equal(tb, got, "Decimal", "SpellValue writes the base") },
		},
	}
}
