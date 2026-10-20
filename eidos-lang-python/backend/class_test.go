// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// classAllocs is the allocations of a class with fields: the head and the
// lines of the fields.
const classAllocs = 2

// aisleField is an immutable field of a class, which the cases of a class
// with the base NamedTuple share. A writer reads a field and never writes
// one.
var aisleField = &emit.Field{Name: "aisle", Type: intRef, Mutability: symbol.MutabilityImmutable}

// aisleLine is the line of [aisleField] in a NamedTuple.
const aisleLine = "    aisle: int\n"

// TestClass pins the classes of structs, Protocols, TypedDicts and enums:
// their heads with their bases, the members of an enum, and the nested
// types and the methods that their templates place.
func TestClass(t *testing.T) {
	t.Parallel()

	t.Run("Writer", func(t *testing.T) {
		t.Parallel()

		t.Run("Struct", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name        string
				give        *emit.Struct
				giveVersion plugin.Choice
				want        backend.Class
				wantImports []render.Entry
			}{
				{
					name: "returns a class with the body ... on its line for a struct without members",
					give: &emit.Struct{Name: rowName}, want: backend.Class{Head: "class Row: ...\n"},
				},
				{
					name: "returns the comment and the docstring of a class",
					give: &emit.Struct{Name: rowName, Comment: "one per fetch", Doc: []string{"Row is one record."}},
					want: backend.Class{Head: "class Row:  # one per fetch\n    \"\"\"Row is one record.\"\"\"\n"},
				},
				{
					name: "returns the types that a struct extends, implements and embeds as its bases",
					give: &emit.Struct{
						Name: rowName, Extends: []*emit.TypeRef{{Spelling: "Base"}},
						Implements: []*emit.TypeRef{{Spelling: "Mixin"}},
						Embeds:     emit.SlotOf(&emit.Embed{Ref: &emit.TypeRef{Spelling: "Embedded"}}),
					},
					want: backend.Class{
						Head: "class Row(" + hugMark + "Base" + sepMark + "Mixin" + sepMark + "Embedded" + closeMark + "): ...\n",
					},
				},
				{
					name:        "returns Generic of the type variables of the module for Python 3.11",
					give:        &emit.Struct{Name: "Box", TypeParams: []*emit.TypeParam{{Name: "T"}}},
					want:        backend.Class{Head: "class Box(" + hugMark + "Generic[T]" + closeMark + "): ...\n"},
					wantImports: []render.Entry{{Path: "typing", Name: "Generic"}, {Path: "typing", Name: "TypeVar"}},
				},
				{
					name:        "returns the type parameters of PEP 695 from Python 3.12",
					give:        &emit.Struct{Name: "Box", TypeParams: []*emit.TypeParam{{Name: "T"}}},
					giveVersion: python.Version312,
					want:        backend.Class{Head: "class Box[T]: ...\n"},
				},
				{
					name:        "returns ABC for an abstract class without bases",
					give:        &emit.Struct{Name: rowName, Abstract: true},
					want:        backend.Class{Head: "class Row(" + hugMark + "ABC" + closeMark + "): ...\n"},
					wantImports: []render.Entry{{Path: "abc", Name: "ABC"}},
				},
				{
					name: "returns no ABC for an abstract class with a metaclass",
					give: &emit.Struct{Name: rowName, Abstract: true, Annotations: symbol.Annotations{{
						Name: string(python.KeywordsMarker), Args: []string{"metaclass=ABCMeta"},
					}}},
					want: backend.Class{Head: "class Row(" + hugMark + "metaclass=ABCMeta" + closeMark + "): ...\n"},
				},
				{
					name: "returns the keywords of a class statement after the bases",
					give: &emit.Struct{
						Name: rowName, Extends: []*emit.TypeRef{{Spelling: "Base"}},
						Annotations: symbol.Annotations{keywordsMarker},
					},
					want: backend.Class{
						Head: "class Row(" + hugMark + "Base" + sepMark + "frozen=True" + closeMark + "): ...\n",
					},
				},
				{
					name:        "returns the decorators of a class before its statement",
					give:        &emit.Struct{Name: rowName, Final: true},
					want:        backend.Class{Head: "@final\nclass Row: ...\n"},
					wantImports: []render.Entry{{Path: "typing", Name: "final"}},
				},
				{
					name: "returns Generic of each type variable of the module in the order of the parameters",
					give: &emit.Struct{Name: "Pair", TypeParams: []*emit.TypeParam{{Name: "K"}, {Name: "V"}}},
					want: backend.Class{
						Head: "class Pair(" + hugMark + "Generic[K, V]" + closeMark + "): ...\n",
					},
					wantImports: []render.Entry{{Path: "typing", Name: "Generic"}, {Path: "typing", Name: "TypeVar"}},
				},
				{
					name: "returns Generic of an unpacked type variable tuple",
					give: &emit.Struct{Name: "Row", TypeParams: []*emit.TypeParam{{Name: "*Ts"}}},
					want: backend.Class{Head: "class Row(" + hugMark + "Generic[*Ts]" + closeMark + "): ...\n"},
					wantImports: []render.Entry{
						{Path: "typing", Name: "Generic"}, {Path: "typing", Name: "TypeVarTuple"},
					},
				},
				{
					name: "returns the immutable fields of a NamedTuple without Final",
					give: &emit.Struct{
						Name: "Point", Extends: []*emit.TypeRef{{Spelling: "NamedTuple", Package: "typing"}},
						Fields: emit.SlotOf(aisleField),
					},
					want: backend.Class{
						Head:   "class Point(" + hugMark + "NamedTuple" + closeMark + "):\n",
						Fields: aisleLine,
					},
					wantImports: []render.Entry{{Path: "typing", Name: "NamedTuple"}},
				},
				{
					name: "returns the immutable fields of a qualified NamedTuple without Final",
					give: &emit.Struct{
						Name: "Point", Extends: []*emit.TypeRef{{Spelling: "typing.NamedTuple", Package: "typing"}},
						Fields: emit.SlotOf(aisleField),
					},
					want: backend.Class{
						Head: "class Point(" + hugMark + "typing.NamedTuple" + closeMark + "):\n", Fields: aisleLine,
					},
					wantImports: []render.Entry{{Path: "typing"}},
				},
				{
					name: "returns Final for an immutable field of a class whose base NamedTuple is no class of typing",
					give: &emit.Struct{
						Name: "Point", Extends: []*emit.TypeRef{{Spelling: "NamedTuple", Package: "shop/tuples"}},
						Fields: emit.SlotOf(aisleField),
					},
					want: backend.Class{
						Head:   "class Point(" + hugMark + "NamedTuple" + closeMark + "):\n",
						Fields: "    aisle: Final[int]\n",
					},
					wantImports: []render.Entry{
						{Path: "shop/tuples", Name: "NamedTuple"}, {Path: "typing", Name: "Final"},
					},
				},
				{
					name: "returns the dynamic type of a field of a NamedTuple without a type in the class syntax",
					give: &emit.Struct{
						Name: "Point", Extends: []*emit.TypeRef{{Spelling: "NamedTuple", Package: "typing"}},
						Fields: emit.SlotOf(aisleField, &emit.Field{Name: "label", Value: `"x"`}),
					},
					want: backend.Class{
						Head:   "class Point(" + hugMark + "NamedTuple" + closeMark + "):\n",
						Fields: aisleLine + "    label: object = \"x\"\n",
					},
					wantImports: []render.Entry{{Path: "typing", Name: "NamedTuple"}},
				},
				{
					name: "returns the class of a NamedTuple whose fields have no type",
					give: &emit.Struct{
						Name: "Point", Extends: []*emit.TypeRef{{Spelling: "NamedTuple", Package: "typing"}},
						Fields: emit.SlotOf(&emit.Field{Name: "x"}, &emit.Field{Name: "y", Value: "0"}),
					},
					want: backend.Class{
						Head:   "class Point(" + hugMark + "NamedTuple" + closeMark + "):\n",
						Fields: "    x: object\n    y: object = 0\n",
					},
					wantImports: []render.Entry{{Path: "typing", Name: "NamedTuple"}},
				},
				{
					name: "returns the functional form of namedtuple for a field whose name is an underscore and its index",
					give: &emit.Struct{
						Name: "Point", Extends: []*emit.TypeRef{{Spelling: "NamedTuple", Package: "typing"}},
						Fields: emit.SlotOf(&emit.Field{Name: "x"}, &emit.Field{Name: "_1", Value: "0"}),
					},
					want: backend.Class{
						Head: "Point = namedtuple(" + hugMark + "\"Point\"" + sepMark + "[\"x\", \"_1\"]" + sepMark +
							"rename=True" + sepMark + "defaults=[0]" + closeMark + ")\n",
					},
					wantImports: []render.Entry{{Path: "collections", Name: "namedtuple"}},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, cmp.Or(tt.giveVersion, python.Version311), false)
					got, err := w.Struct(nil, tt.give)
					assert.NoError(t, err, "Python declares the class")
					assert.Equal(t, got, tt.want, "the class")
					assert.Equal(t, importsOf(set), tt.wantImports, "the imports of the class", assert.EquateEmpty())
				})
			}

			hidden := []struct {
				name string
				give *emit.Struct
				want string
			}{
				{
					name: "returns builtins.bytes for the type of a field where a field of the class has its name",
					give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(
						&emit.Field{Name: "bytes", Type: bytesRef}, &emit.Field{Name: "data", Type: bytesRef},
					)},
					want: "    bytes: builtins.bytes\n    data: builtins.bytes\n",
				},
				{
					name: "returns builtins.bytes for the type of a field where a method of the class has its name",
					give: &emit.Struct{
						Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: "data", Type: bytesRef}),
						Methods: emit.SlotOf(&emit.Method{Name: "bytes"}),
					},
					want: "    data: builtins.bytes\n",
				},
				{
					name: "returns builtins.Exception for the type of a field where a nested type of the class has its name",
					give: &emit.Struct{
						Name: rowName,
						Fields: emit.SlotOf(
							&emit.Field{Name: "error", Type: &emit.TypeRef{Spelling: "Exception", Package: "builtins"}},
						),
						Types: emit.SlotOf[symbol.Symbol](&emit.Struct{Name: "Exception"}),
					},
					want: "    error: builtins.Exception\n",
				},
			}
			for _, tt := range hidden {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, python.Version311, false)
					got, err := w.Struct(nil, tt.give)
					assert.NoError(t, err, "Python declares the class")
					assert.Equal(t, got.Fields, tt.want, "the fields")
					assert.Contains(t, set.Entries(), render.Entry{Path: "builtins"}, "the import of builtins")
				})
			}

			t.Run("returns a builtin by its name in a base where a member of the class has its name",
				func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, python.Version311, false)
					got, err := w.Struct(nil, &emit.Struct{
						Name: "Fault", Extends: []*emit.TypeRef{{Spelling: "Exception", Package: "builtins"}},
						Types: emit.SlotOf[symbol.Symbol](&emit.Struct{Name: "Exception"}),
					})
					assert.NoError(t, err, "Python declares the class")
					assert.Equal(t, got.Head, "class Fault("+hugMark+"Exception"+closeMark+"):\n", "the head")
					assert.Empty(t, set.Entries(), "a base needs no import")
				})

			t.Run("returns the renamed type variable of a class in its bases, its fields and its methods",
				func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version311, false)
					_, err := w.Struct(nil, &emit.Struct{Name: "Box", TypeParams: []*emit.TypeParam{{Name: "T"}}})
					assert.NoError(t, err, "the first class declares T")
					pack := &emit.Struct{
						Name: "Pack", TypeParams: []*emit.TypeParam{boundParam},
						Fields: emit.SlotOf(&emit.Field{Name: "item", Type: &emit.TypeRef{Spelling: "T"}}),
					}
					got, err := w.Struct(nil, pack)
					assert.NoError(t, err, "the second class declares its own type variable")
					assert.Equal(t, got, backend.Class{
						Head: "class Pack(" + hugMark + "Generic[T2]" + closeMark + "):\n", Fields: "    item: T2\n",
					}, "the class of the renamed type variable")
					method, err := w.Method(pack, &emit.Method{
						Name: "put", Params: emit.SlotOf(&emit.Param{Name: "item", Type: &emit.TypeRef{Spelling: "T"}}),
					}, "")
					assert.NoError(t, err, "the method of the class spells")
					want := "    def put(" + hugMark + "self" + sepMark + "item: T2" + closeMark + ") -> None: ...\n"
					assert.Equal(t, method, want, "the method spells the renamed type variable")
				})

			t.Run("returns the nested types and the methods of a struct for its template", func(t *testing.T) {
				t.Parallel()

				inner := &emit.Struct{Name: "Inner"}
				load := &emit.Method{Name: "load"}
				w, _ := newWriter(t, python.Version311, false)
				got, err := w.Struct(nil, &emit.Struct{
					Name: rowName, Types: emit.SlotOf[symbol.Symbol](inner), Methods: emit.SlotOf(load),
				})
				assert.NoError(t, err, "Python declares the class")
				expect.Permutation(t, got.Types, []symbol.Symbol{inner}, "the nested type", assert.ByIdentity())
				expect.Permutation(t, got.Methods, []*emit.Method{load}, "the method", assert.ByIdentity())
			})

			t.Run("binds the imports of the header", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				_, err := w.Struct(&emit.File{Annotations: symbol.Annotations{jsonImport}}, &emit.Struct{Name: rowName})
				assert.NoError(t, err, "Python declares the class")
				assert.Contains(t, set.Entries(), render.Entry{Path: "json"}, "the import of the header")
			})

			t.Run("returns the error of an import of the header", func(t *testing.T) {
				t.Parallel()

				w, _ := newWriter(t, python.Version311, false)
				_, err := w.Struct(&emit.File{Annotations: symbol.Annotations{badImport}}, &emit.Struct{Name: rowName})
				assert.HasError(t, err, "the class refuses the header")
				assert.Equal(t, err.Error(), badImportRefusal, "the refusal names the argument")
			})

			refusals := []struct {
				name string
				give *emit.Struct
				want string
			}{
				{
					name: "returns the error of a base",
					give: &emit.Struct{
						Name: rowName, Extends: []*emit.TypeRef{{Spelling: "A & B", Form: symbol.FormIntersection}},
					},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of a type that a struct implements",
					give: &emit.Struct{
						Name: rowName, Implements: []*emit.TypeRef{{Spelling: "A & B", Form: symbol.FormIntersection}},
					},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of a type that a struct embeds",
					give: &emit.Struct{
						Name: rowName, Embeds: emit.SlotOf(&emit.Embed{
							Ref: &emit.TypeRef{Spelling: "A & B", Form: symbol.FormIntersection},
						}),
					},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of the type parameters of a struct",
					give: &emit.Struct{Name: "Box", TypeParams: []*emit.TypeParam{{Name: "N", Const: true}}},
					want: "python: a type parameter takes a type, and N takes a value",
				},
				{
					name: "returns the error of a field of a struct",
					give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: "row-key"})},
					want: `python: a field of a class has an identifier as its name, and "row-key" is not one`,
				},
				{
					name: "returns an error for a field of a NamedTuple whose name starts with an underscore",
					give: &emit.Struct{
						Name: "Point", Extends: []*emit.TypeRef{{Spelling: "NamedTuple", Package: "typing"}},
						Fields: emit.SlotOf(&emit.Field{Name: "_aisle", Type: intRef}),
					},
					want: "python: NamedTuple refuses a field whose name starts with an underscore, and _aisle has one",
				},
				{
					name: "returns an error for the functional form of a field with an underscore that is not its index",
					give: &emit.Struct{
						Name: "Point", Extends: []*emit.TypeRef{{Spelling: "NamedTuple", Package: "typing"}},
						Fields: emit.SlotOf(&emit.Field{Name: "x"}, &emit.Field{Name: "_x"}),
					},
					want: "python: namedtuple renames a field with a leading underscore to an underscore and its index, " +
						"and _x is field 1",
				},
				{
					name: "returns an error for the functional form of a field without a value after a field with one",
					give: &emit.Struct{
						Name: "Point", Extends: []*emit.TypeRef{{Spelling: "NamedTuple", Package: "typing"}},
						Fields: emit.SlotOf(&emit.Field{Name: "x", Value: "0"}, &emit.Field{Name: "_1"}),
					},
					want: "python: namedtuple gives its defaults to the last fields, and _1 has no value after a field " +
						"with one",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version312, false)
					_, err := w.Struct(nil, tt.give)
					assert.HasError(t, err, "Python refuses the class")
					assert.Equal(t, err.Error(), tt.want, "the refusal names the part")
				})
			}

			t.Run("returns an error for a base that the module declares later", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				set.Reserve(rowName, nodeName)
				set.Enter(rowName)
				_, err := w.Struct(nil, &emit.Struct{Name: rowName, Extends: []*emit.TypeRef{{Spelling: nodeName}}})
				assert.HasError(t, err, "a module evaluates a base when it runs the class statement")
				assert.Equal(t, err.Error(),
					"python: Python evaluates Node when the module runs, and the module declares it later",
					"the refusal names the base")
			})
		})

		t.Run("Interface", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name        string
				give        *emit.Interface
				giveVersion plugin.Choice
				want        backend.Class
				wantImports []render.Entry
			}{
				{
					name: "returns a Protocol after the interfaces that it extends",
					give: &emit.Interface{Name: "Store", Extends: []*emit.TypeRef{{Spelling: "Closer"}}},
					want: backend.Class{
						Head: "class Store(" + hugMark + "Closer" + sepMark + "Protocol" + closeMark + "): ...\n",
					},
					wantImports: []render.Entry{{Path: "typing", Name: "Protocol"}},
				},
				{
					name:        "returns Protocol of the type variables of the module for Python 3.11",
					give:        &emit.Interface{Name: "Keyed", TypeParams: []*emit.TypeParam{{Name: "K"}}},
					want:        backend.Class{Head: "class Keyed(" + hugMark + "Protocol[K]" + closeMark + "): ...\n"},
					wantImports: []render.Entry{{Path: "typing", Name: "Protocol"}, {Path: "typing", Name: "TypeVar"}},
				},
				{
					name:        "returns a Protocol with the type parameters of PEP 695 from Python 3.12",
					give:        &emit.Interface{Name: "Keyed", TypeParams: []*emit.TypeParam{{Name: "K"}}},
					giveVersion: python.Version312,
					want:        backend.Class{Head: "class Keyed[K](" + hugMark + "Protocol" + closeMark + "): ...\n"},
					wantImports: []render.Entry{{Path: "typing", Name: "Protocol"}},
				},
				{
					name: "returns a TypedDict with its keys",
					give: &emit.Interface{
						Name: "Movie", Annotations: symbol.Annotations{typedDictMarker},
						Fields: emit.SlotOf(&emit.Field{Name: nameField, Type: strRef}),
					},
					want: backend.Class{
						Head: "class Movie(" + hugMark + "TypedDict" + closeMark + "):\n", Fields: "    name: str\n",
					},
					wantImports: []render.Entry{{Path: "typing", Name: "TypedDict"}},
				},
				{
					name: "returns builtins.str for the type of a key where a key of the TypedDict has its name",
					give: &emit.Interface{
						Name: "Movie", Annotations: symbol.Annotations{typedDictMarker},
						Fields: emit.SlotOf(
							&emit.Field{Name: "str", Type: &emit.TypeRef{Spelling: "str", Package: "builtins"}},
						),
					},
					want: backend.Class{
						Head:   "class Movie(" + hugMark + "TypedDict" + closeMark + "):\n",
						Fields: "    str: builtins.str\n",
					},
					wantImports: []render.Entry{{Path: "builtins"}, {Path: "typing", Name: "TypedDict"}},
				},
				{
					name: "returns the bases of a TypedDict in place of TypedDict",
					give: &emit.Interface{
						Name: "Movie", Annotations: symbol.Annotations{typedDictMarker},
						Extends: []*emit.TypeRef{{Spelling: "Base"}},
					},
					want: backend.Class{Head: "class Movie(" + hugMark + "Base" + closeMark + "): ...\n"},
				},
				{
					name: "returns Generic after TypedDict for the type variables of the module",
					give: &emit.Interface{
						Name: "Page", Annotations: symbol.Annotations{typedDictMarker},
						TypeParams: []*emit.TypeParam{{Name: "T"}},
					},
					want: backend.Class{
						Head: "class Page(" + hugMark + "TypedDict" + sepMark + "Generic[T]" + closeMark + "): ...\n",
					},
					wantImports: []render.Entry{
						{Path: "typing", Name: "Generic"},
						{Path: "typing", Name: "TypeVar"},
						{Path: "typing", Name: "TypedDict"},
					},
				},
				{
					name: "returns the functional form of a TypedDict with a key that is no identifier",
					give: &emit.Interface{
						Name: "Movie", Annotations: symbol.Annotations{typedDictMarker, {
							Name: string(python.KeywordsMarker), Args: []string{partialKeyword},
						}},
						Fields: emit.SlotOf(&emit.Field{Name: "year-of", Type: intRef, Optional: true}),
					},
					want: backend.Class{
						Head: "Movie = TypedDict(" + hugMark + "\"Movie\"" + sepMark + "{\"year-of\": int}" + sepMark +
							"total=False" + closeMark + ")\n",
					},
					wantImports: []render.Entry{{Path: "typing", Name: "TypedDict"}},
				},
				{
					name: "returns Required for a key that a value has in the functional form of total=False",
					give: &emit.Interface{
						Name: "Movie", Annotations: symbol.Annotations{typedDictMarker, {
							Name: string(python.KeywordsMarker), Args: []string{partialKeyword},
						}},
						Fields: emit.SlotOf(&emit.Field{Name: "year-of", Type: intRef}),
					},
					want: backend.Class{
						Head: "Movie = TypedDict(" + hugMark + "\"Movie\"" + sepMark + "{\"year-of\": Required[int]}" +
							sepMark + "total=False" + closeMark + ")\n",
					},
					wantImports: []render.Entry{
						{Path: "typing", Name: "Required"}, {Path: "typing", Name: "TypedDict"},
					},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, cmp.Or(tt.giveVersion, python.Version311), false)
					got, err := w.Interface(nil, tt.give)
					assert.NoError(t, err, "Python declares the class")
					assert.Equal(t, got, tt.want, "the class")
					assert.Equal(t, importsOf(set), tt.wantImports, "the imports of the class", assert.EquateEmpty())
				})
			}

			t.Run("returns builtins.property for a read-only field where a member of the Protocol has its name",
				func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, python.Version311, false)
					got, err := w.Interface(nil, &emit.Interface{
						Name: "Store", Methods: emit.SlotOf(&emit.Method{Name: "property"}),
						Fields: emit.SlotOf(&emit.Field{
							Name: nameField, Type: strRef, Mutability: symbol.MutabilityImmutable,
						}),
					})
					assert.NoError(t, err, "Python declares the Protocol")
					assert.Equal(t, got.Fields, "    @builtins.property\n    def name(self) -> str: ...\n",
						"the read-only property")
					assert.Contains(t, set.Entries(), render.Entry{Path: "builtins"}, "the import of builtins")
				})

			t.Run("returns the nested types and the methods of a Protocol for its template", func(t *testing.T) {
				t.Parallel()

				inner := &emit.Struct{Name: "Inner"}
				get := &emit.Method{Name: "get"}
				w, _ := newWriter(t, python.Version311, false)
				got, err := w.Interface(nil, &emit.Interface{
					Name: "Store", Types: emit.SlotOf[symbol.Symbol](inner), Methods: emit.SlotOf(get),
				})
				assert.NoError(t, err, "Python declares the Protocol")
				expect.Permutation(t, got.Types, []symbol.Symbol{inner}, "the nested type", assert.ByIdentity())
				expect.Permutation(t, got.Methods, []*emit.Method{get}, "the method", assert.ByIdentity())
			})

			t.Run("binds the imports of the header", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				header := &emit.File{Annotations: symbol.Annotations{jsonImport}}
				_, err := w.Interface(header, &emit.Interface{Name: "Store"})
				assert.NoError(t, err, "Python declares the Protocol")
				assert.Contains(t, set.Entries(), render.Entry{Path: "json"}, "the import of the header")
			})

			t.Run("returns the error of an import of the header", func(t *testing.T) {
				t.Parallel()

				w, _ := newWriter(t, python.Version311, false)
				header := &emit.File{Annotations: symbol.Annotations{badImport}}
				_, err := w.Interface(header, &emit.Interface{Name: "Store"})
				assert.HasError(t, err, "the Protocol refuses the header")
				assert.Equal(t, err.Error(), badImportRefusal, "the refusal names the argument")
			})

			refusals := []struct {
				name string
				give *emit.Interface
				want string
			}{
				{
					name: "returns an error for a TypedDict with methods",
					give: &emit.Interface{
						Name: "Movie", Annotations: symbol.Annotations{typedDictMarker},
						Methods: emit.SlotOf(&emit.Method{Name: "get"}),
					},
					want: "python: a TypedDict declares keys alone, and Movie states methods",
				},
				{
					name: "returns an error for a TypedDict with nested types",
					give: &emit.Interface{
						Name: "Movie", Annotations: symbol.Annotations{typedDictMarker},
						Types: emit.SlotOf[symbol.Symbol](&emit.Struct{Name: "Inner"}),
					},
					want: "python: a TypedDict declares keys alone, and Movie states nested types",
				},
				{
					name: "returns an error for the functional form of a TypedDict with bases",
					give: &emit.Interface{
						Name: "Movie", Annotations: symbol.Annotations{typedDictMarker},
						Extends: []*emit.TypeRef{{Spelling: "Base"}},
						Fields:  emit.SlotOf(&emit.Field{Name: "year-of", Type: intRef}),
					},
					want: "python: the functional form of a TypedDict has no bases and no type parameters, " +
						"and Movie states them",
				},
				{
					name: "returns the error of the type parameters of a Protocol",
					give: &emit.Interface{Name: "Keyed", TypeParams: []*emit.TypeParam{{Name: "N", Const: true}}},
					want: "python: a type parameter takes a type, and N takes a value",
				},
				{
					name: "returns the error of the type parameters of a TypedDict",
					give: &emit.Interface{
						Name: "Page", Annotations: symbol.Annotations{typedDictMarker},
						TypeParams: []*emit.TypeParam{{Name: "N", Const: true}},
					},
					want: "python: a type parameter takes a type, and N takes a value",
				},
				{
					name: "returns the error of a field of a Protocol",
					give: &emit.Interface{Name: "Store", Fields: emit.SlotOf(&emit.Field{Name: "row-key"})},
					want: `python: a field of a class has an identifier as its name, and "row-key" is not one`,
				},
				{
					name: "returns the error of a key of a TypedDict",
					give: &emit.Interface{
						Name: "Movie", Annotations: symbol.Annotations{typedDictMarker},
						Fields: emit.SlotOf(&emit.Field{Name: nameField, Type: strRef, Value: `"a"`}),
					},
					want: "python: a key of a TypedDict takes no value, and name states one",
				},
				{
					name: "returns the error of a key of the functional form of a TypedDict",
					give: &emit.Interface{
						Name: "Movie", Annotations: symbol.Annotations{typedDictMarker},
						Fields: emit.SlotOf(&emit.Field{
							Name: "year-of", Type: &emit.TypeRef{Spelling: "A & B", Form: symbol.FormIntersection},
						}),
					},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version312, false)
					_, err := w.Interface(nil, tt.give)
					assert.HasError(t, err, "Python refuses the class")
					assert.Equal(t, err.Error(), tt.want, "the refusal names the part")
				})
			}
		})

		t.Run("Enum", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name        string
				give        *emit.Enum
				want        backend.Class
				wantImports []render.Entry
			}{
				{
					name: "returns an enum with Enum and auto for its members without values",
					give: &emit.Enum{Name: phaseName, Variants: emit.SlotOf(
						&emit.EnumVariant{Name: "OPEN", Doc: []string{"Open admits writes."}},
						&emit.EnumVariant{Name: "CLOSED", Comment: "final"},
					)},
					want: backend.Class{
						Head: "class Phase(" + hugMark + "Enum" + closeMark + "):\n",
						Fields: "    OPEN = auto()\n    \"\"\"Open admits writes.\"\"\"\n" +
							"    CLOSED = auto()  # final\n",
					},
					wantImports: []render.Entry{{Path: "enum", Name: "Enum"}, {Path: "enum", Name: "auto"}},
				},
				{
					name: "returns the values of the members of an enum",
					give: &emit.Enum{
						Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN", Value: "1"}),
					},
					want: backend.Class{
						Head: "class Phase(" + hugMark + "Enum" + closeMark + "):\n", Fields: "    OPEN = 1\n",
					},
					wantImports: []render.Entry{{Path: "enum", Name: "Enum"}},
				},
				{
					name:        "returns IntEnum for an enum of an underlying int",
					give:        &emit.Enum{Name: phaseName, Underlying: intRef},
					want:        backend.Class{Head: "class Phase(" + hugMark + "IntEnum" + closeMark + "): ...\n"},
					wantImports: []render.Entry{{Path: "enum", Name: "IntEnum"}},
				},
				{
					name:        "returns StrEnum for an enum of an underlying str",
					give:        &emit.Enum{Name: phaseName, Underlying: strRef},
					want:        backend.Class{Head: "class Phase(" + hugMark + "StrEnum" + closeMark + "): ...\n"},
					wantImports: []render.Entry{{Path: "enum", Name: "StrEnum"}},
				},
				{
					name: "returns the underlying type and Enum for any other underlying type",
					give: &emit.Enum{Name: phaseName, Underlying: decimalRef},
					want: backend.Class{
						Head: "class Phase(" + hugMark + "Decimal" + sepMark + "Enum" + closeMark + "): ...\n",
					},
					wantImports: []render.Entry{{Path: "decimal", Name: "Decimal"}, {Path: "enum", Name: "Enum"}},
				},
				{
					name: "returns the bases of the bases marker with the enum classes imported from enum",
					give: &emit.Enum{Name: phaseName, Annotations: symbol.Annotations{{
						Name: string(python.BasesMarker), Args: []string{"Codes", "IntFlag"},
					}}},
					want: backend.Class{
						Head: "class Phase(" + hugMark + "Codes" + sepMark + "IntFlag" + closeMark + "): ...\n",
					},
					wantImports: []render.Entry{{Path: "enum", Name: "IntFlag"}},
				},
				{
					name: "returns the fields of an enum after its members",
					give: &emit.Enum{
						Name:     "Planet",
						Variants: emit.SlotOf(&emit.EnumVariant{Name: "MERCURY", Value: "330, 2440"}),
						Fields: emit.SlotOf(
							&emit.Field{Name: "mass", Type: intRef},
							&emit.Field{Name: "radius", Type: intRef},
						),
					},
					want: backend.Class{
						Head:   "class Planet(" + hugMark + "Enum" + closeMark + "):\n",
						Fields: "    MERCURY = 330, 2440\n    mass: int\n    radius: int\n",
					},
					wantImports: []render.Entry{{Path: "enum", Name: "Enum"}},
				},
				{
					name: "returns a field of an enum with a value as nonmember of the cast of the value",
					give: &emit.Enum{
						Name: phaseName, Fields: emit.SlotOf(&emit.Field{
							Name: "count", Type: intRef, Value: "0", Level: symbol.LevelType,
						}),
					},
					want: backend.Class{
						Head:   "class Phase(" + hugMark + "Enum" + closeMark + "):\n",
						Fields: "    count = nonmember(cast(\"int\", 0))\n",
					},
					wantImports: []render.Entry{
						{Path: "enum", Name: "Enum"}, {Path: "enum", Name: "nonmember"}, {Path: "typing", Name: "cast"},
					},
				},
				{
					name: "returns None among the types of the cast of a field of an enum that a value may lack",
					give: &emit.Enum{
						Name: phaseName, Fields: emit.SlotOf(&emit.Field{
							Name: "count", Type: intRef, Optional: true, Level: symbol.LevelType,
						}),
					},
					want: backend.Class{
						Head:   "class Phase(" + hugMark + "Enum" + closeMark + "):\n",
						Fields: "    count = nonmember(cast(\"int | None\", None))\n",
					},
					wantImports: []render.Entry{
						{Path: "enum", Name: "Enum"}, {Path: "enum", Name: "nonmember"}, {Path: "typing", Name: "cast"},
					},
				},
				{
					name: "returns a field of the members of an enum that a value may lack without a value",
					give: &emit.Enum{
						Name: phaseName, Fields: emit.SlotOf(&emit.Field{Name: "count", Type: intRef, Optional: true}),
					},
					want: backend.Class{
						Head:   "class Phase(" + hugMark + "Enum" + closeMark + "):\n",
						Fields: "    count: int | None\n",
					},
					wantImports: []render.Entry{{Path: "enum", Name: "Enum"}},
				},
				{
					name: "returns a field of an enum without a type as nonmember of its value",
					give: &emit.Enum{Name: phaseName, Fields: emit.SlotOf(&emit.Field{
						Name: "count", Value: "0", Level: symbol.LevelType,
					})},
					want: backend.Class{
						Head:   "class Phase(" + hugMark + "Enum" + closeMark + "):\n",
						Fields: "    count = nonmember(0)\n",
					},
					wantImports: []render.Entry{{Path: "enum", Name: "Enum"}, {Path: "enum", Name: "nonmember"}},
				},
				{
					name: "returns the functional form of an enum with a member that is no identifier",
					give: &emit.Enum{Name: phaseName, Variants: emit.SlotOf(
						&emit.EnumVariant{Name: "dark-red"}, &emit.EnumVariant{Name: "BLUE", Value: "2"},
					)},
					want: backend.Class{
						Head: "Phase = Enum(" + hugMark + "\"Phase\"" + sepMark + "{\"dark-red\": auto(), \"BLUE\": 2}" +
							closeMark + ")\n",
					},
					wantImports: []render.Entry{{Path: "enum", Name: "Enum"}, {Path: "enum", Name: "auto"}},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, python.Version311, false)
					got, err := w.Enum(nil, tt.give)
					assert.NoError(t, err, "Python declares the enum")
					assert.Equal(t, got, tt.want, "the enum")
					assert.Equal(t, importsOf(set), tt.wantImports, "the imports of the enum", assert.EquateEmpty())
				})
			}

			t.Run("returns auto as an import of the header binds it", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				_, err := w.Bind(&emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{"from enum import Enum", "from enum import auto"},
				}}})
				assert.NoError(t, err, "the header binds its imports")
				phase := &emit.Enum{Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN"})}
				got, err := w.Enum(nil, phase)
				assert.NoError(t, err, "Python declares the enum")
				assert.Equal(t, got.Fields, "    OPEN = auto()\n", "the member")
				assert.Length(t, set.Entries(), 2, "the imports of the header alone")
			})

			t.Run("returns a field of an enum without its value in a stub file", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, true)
				got, err := w.Enum(nil, &emit.Enum{
					Name: phaseName, Fields: emit.SlotOf(&emit.Field{
						Name: "count", Type: intRef, Value: "0", Level: symbol.LevelType,
					}),
				})
				assert.NoError(t, err, "Python declares the enum")
				assert.Equal(t, got.Fields, "    count: ClassVar[int]\n", "the attribute without a value")
				assert.Equal(t, importsOf(set), []render.Entry{
					{Path: "enum", Name: "Enum"}, {Path: "typing", Name: "ClassVar"},
				}, "the imports of Enum and of ClassVar")
			})

			t.Run("returns the methods of an enum for its template", func(t *testing.T) {
				t.Parallel()

				label := &emit.Method{Name: "label"}
				w, _ := newWriter(t, python.Version311, false)
				got, err := w.Enum(nil, &emit.Enum{Name: phaseName, Methods: emit.SlotOf(label)})
				assert.NoError(t, err, "Python declares the enum")
				assert.Permutation(t, got.Methods, []*emit.Method{label}, "the method", assert.ByIdentity())
			})

			t.Run("binds the imports of the header", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				_, err := w.Enum(&emit.File{Annotations: symbol.Annotations{jsonImport}}, &emit.Enum{Name: phaseName})
				assert.NoError(t, err, "Python declares the enum")
				assert.Contains(t, set.Entries(), render.Entry{Path: "json"}, "the import of the header")
			})

			t.Run("returns the error of an import of the header", func(t *testing.T) {
				t.Parallel()

				w, _ := newWriter(t, python.Version311, false)
				_, err := w.Enum(&emit.File{Annotations: symbol.Annotations{badImport}}, &emit.Enum{Name: phaseName})
				assert.HasError(t, err, "the enum refuses the header")
				assert.Equal(t, err.Error(), badImportRefusal, "the refusal names the argument")
			})

			refusals := []struct {
				name string
				give *emit.Enum
				want string
			}{
				{
					name: "returns an error for the functional form of an enum with methods",
					give: &emit.Enum{
						Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "dark-red"}),
						Methods: emit.SlotOf(&emit.Method{Name: "label"}),
					},
					want: "python: the functional form of an enum has members alone, and Phase states methods",
				},
				{
					name: "returns an error for the functional form of an enum with fields",
					give: &emit.Enum{
						Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "dark-red"}),
						Fields: emit.SlotOf(&emit.Field{Name: "count", Type: intRef}),
					},
					want: "python: the functional form of an enum has members alone, and Phase states fields",
				},
				{
					name: "returns the error of an underlying type",
					give: &emit.Enum{
						Name: phaseName, Underlying: &emit.TypeRef{Spelling: "A & B", Form: symbol.FormIntersection},
					},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of a field of an enum",
					give: &emit.Enum{Name: phaseName, Fields: emit.SlotOf(&emit.Field{Name: "row-key"})},
					want: `python: a field of a class has an identifier as its name, and "row-key" is not one`,
				},
				{
					name: "returns the error of the type of a field of an enum with a value",
					give: &emit.Enum{Name: phaseName, Fields: emit.SlotOf(&emit.Field{
						Name:  "count",
						Value: "0",
						Level: symbol.LevelType,
						Type:  &emit.TypeRef{Spelling: "A & B", Form: symbol.FormIntersection},
					})},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns an error for a field of the members of an enum with a value",
					give: &emit.Enum{Name: phaseName, Fields: emit.SlotOf(&emit.Field{
						Name: "count", Type: intRef, Value: "0",
					})},
					want: "python: an enum sets the attributes of its members in __init__, and count states a value " +
						"in the class",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version312, false)
					_, err := w.Enum(nil, tt.give)
					assert.HasError(t, err, "Python refuses the enum")
					assert.Equal(t, err.Error(), tt.want, "the refusal names the part")
				})
			}
		})
	})
}

// Struct allocates the head and the lines of the fields of a class, and
// Enum the head and the members of an enum. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestClassAllocs(t *testing.T) {
	checkAllocs(t, classCalls(t))
}

// BenchmarkClass measures the class of a struct with fields and of an enum
// with members.
func BenchmarkClass(b *testing.B) {
	b.Run("Writer", func(b *testing.B) {
		benchCalls(b, classCalls(b))
	})
}

// classCalls returns the calls of Struct for a struct with a base and two
// fields, and of Enum for an enum with two members.
func classCalls(tb assert.TB) []allocCall {
	w, _ := newWriter(tb, python.Version311, false)
	row := &emit.Struct{
		Name: rowName, Doc: []string{"Row is one record."}, Extends: []*emit.TypeRef{{Spelling: "Base"}},
		Fields: emit.SlotOf(&emit.Field{Name: idField, Type: intRef}, &emit.Field{Name: nameField, Type: strRef}),
	}
	phase := &emit.Enum{Name: phaseName, Variants: emit.SlotOf(
		&emit.EnumVariant{Name: "OPEN"}, &emit.EnumVariant{Name: "CLOSED"},
	)}
	var got backend.Class
	return []allocCall{
		{
			name: "Struct", allocs: classAllocs,
			call: func() { got, _ = w.Struct(nil, row) },
			check: func(tb assert.TB) {
				assert.Equal(tb, got.Fields, "    id: int\n    name: str\n", "Struct writes the fields")
			},
		},
		{
			name: "Enum", allocs: classAllocs,
			call: func() { got, _ = w.Enum(nil, phase) },
			check: func(tb assert.TB) {
				assert.Equal(tb, got.Fields, "    OPEN = auto()\n    CLOSED = auto()\n", "Enum writes the members")
			},
		},
	}
}
