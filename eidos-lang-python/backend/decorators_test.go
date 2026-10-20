// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The allocations of the decorator lines: none for a declaration without
// decorators, and the lines of a property.
const (
	noDecoratorsAllocs = 0
	propertyAllocs     = 1
)

// dataclass is the decorator of a dataclass, frozen.
var dataclass = symbol.Annotation{Name: "dataclass", Args: []string{"frozen=True"}}

// TestDecorators pins the decorator lines of a class and of a callable: the
// decorators of their facts in the order of Python, their annotations, and
// the imports of each.
func TestDecorators(t *testing.T) {
	t.Parallel()

	t.Run("Writer", func(t *testing.T) {
		t.Parallel()

		t.Run("Decorators", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name        string
				give        symbol.Symbol
				giveVersion plugin.Choice
				want        string
				wantImports []render.Entry
			}{
				{name: "returns nothing for a declaration without decorators", give: &emit.Struct{Name: rowName}},
				{
					name: "returns nothing for a declaration that Python does not decorate",
					give: &emit.Alias{Name: "Id"},
				},
				{
					name: "returns a decorator of the standard library with its import",
					give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{{Name: "dataclass"}}},
					want: "@dataclass\n", wantImports: []render.Entry{{Path: "dataclasses", Name: "dataclass"}},
				},
				{
					name:        "returns the arguments of a decorator in a group",
					give:        &emit.Struct{Name: rowName, Annotations: symbol.Annotations{dataclass}},
					want:        "@dataclass(" + hugMark + "frozen=True" + closeMark + ")\n",
					wantImports: []render.Entry{{Path: "dataclasses", Name: "dataclass"}},
				},
				{
					name: "returns a dotted decorator with the import of its module",
					give: &emit.Function{Name: "load", Annotations: symbol.Annotations{{Name: "functools.cache"}}},
					want: "@functools.cache\n", wantImports: []render.Entry{{Path: "functools"}},
				},
				{
					name: "returns a decorator that the table does not list as it is",
					give: &emit.Function{Name: "load", Annotations: symbol.Annotations{{Name: "traced"}}},
					want: "@traced\n",
				},
				{
					name: "returns deprecated from typing_extensions before Python 3.13",
					give: &emit.Function{
						Name: "load", Annotations: symbol.Annotations{{Name: "deprecated", Args: []string{`"old"`}}},
					},
					giveVersion: python.Version312,
					want:        "@deprecated(" + hugMark + `"old"` + closeMark + ")\n",
					wantImports: []render.Entry{{Path: "typing_extensions", Name: "deprecated"}},
				},
				{
					name: "returns deprecated from warnings from Python 3.13",
					give: &emit.Function{
						Name: "load", Annotations: symbol.Annotations{{Name: "deprecated", Args: []string{`"old"`}}},
					},
					giveVersion: python.Version313,
					want:        "@deprecated(" + hugMark + `"old"` + closeMark + ")\n",
					wantImports: []render.Entry{{Path: "warnings", Name: "deprecated"}},
				},
				{
					name: "returns the decorators of the facts of a method in the order that Python applies them",
					give: &emit.Method{
						Name: "load", Override: true, Final: true, Level: symbol.LevelType, Abstract: true,
						Accessor: symbol.AccessorGet,
					},
					giveVersion: python.Version312,
					want:        "@override\n@final\n@staticmethod\n@property\n@abstractmethod\n",
					wantImports: []render.Entry{
						{Path: "abc", Name: "abstractmethod"},
						{Path: "typing", Name: "final"},
						{Path: "typing", Name: "override"},
					},
				},
				{
					name: "returns override from typing_extensions before Python 3.12",
					give: &emit.Method{Name: "load", Override: true}, giveVersion: python.Version311,
					want:        "@override\n",
					wantImports: []render.Entry{{Path: "typing_extensions", Name: "override"}},
				},
				{
					name: "returns classmethod for a method of the type with a receiver",
					give: &emit.Method{Name: "create", Level: symbol.LevelType, Receiver: &emit.Param{Name: "cls"}},
					want: "@classmethod\n",
				},
				{
					name: "returns no level decorator for __new__",
					give: &emit.Method{Name: "__new__", Level: symbol.LevelType, Receiver: &emit.Param{Name: "cls"}},
					want: "",
				},
				{
					name: "returns the setter of the property of the name of a setter",
					give: &emit.Method{Name: "size", Accessor: symbol.AccessorSet},
					want: "@size.setter\n",
				},
				{
					name: "returns the deleter of the property of the name of a deleter",
					give: &emit.Method{Name: "size", Accessor: symbol.AccessorDelete},
					want: "@size.deleter\n",
				},
				{
					name: "returns no decorator of a fact twice where an annotation states it",
					give: &emit.Method{
						Name: "load", Final: true, Annotations: symbol.Annotations{{Name: "typing.final"}},
					},
					want: "@typing.final\n", wantImports: []render.Entry{{Path: "typing"}},
				},
				{
					name: "returns final for a final class",
					give: &emit.Struct{Name: rowName, Final: true},
					want: "@final\n", wantImports: []render.Entry{{Path: "typing", Name: "final"}},
				},
				{
					name: "returns no decorator for a marker",
					give: &emit.Interface{
						Name: rowName, Annotations: symbol.Annotations{{Name: string(python.TypedDictMarker)}},
					},
				},
				{
					name: "returns the decorators of a method before its annotations",
					give: &emit.Method{
						Name: "load", Level: symbol.LevelType, Annotations: symbol.Annotations{{Name: "traced"}},
					},
					want: "@staticmethod\n@traced\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, cmp.Or(tt.giveVersion, python.Version311), false)
					assert.Equal(t, w.Decorators(tt.give), tt.want, "the decorator lines")
					assert.Equal(
						t,
						set.Entries(),
						tt.wantImports,
						"the imports of the decorators",
						assert.EquateEmpty(),
					)
				})
			}

			declared := []struct {
				name        string
				give        string
				giveDeclare string
			}{
				{
					name: "returns a decorator of the table that the file declares without an import",
					give: "cache", giveDeclare: "cache",
				},
				{
					name: "returns a dotted decorator whose first name the file declares without an import",
					give: "registry.register", giveDeclare: "registry",
				},
			}
			for _, tt := range declared {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, python.Version311, false)
					set.Reserve(tt.giveDeclare)
					got := w.Decorators(&emit.Function{Name: "load", Annotations: symbol.Annotations{{Name: tt.give}}})
					assert.Equal(t, got, "@"+tt.give+"\n", "the decorator line")
					assert.Empty(t, set.Entries(), "the decorator records no import")
				})
			}

			hidden := []struct {
				name        string
				give        *emit.Method
				giveDeclare string
				want        string
			}{
				{
					name: "returns builtins.staticmethod where a declaration of the file hides staticmethod",
					give: &emit.Method{Name: "create", Level: symbol.LevelType}, giveDeclare: "staticmethod",
					want: "@builtins.staticmethod\n",
				},
				{
					name: "returns builtins.classmethod where a declaration of the file hides classmethod",
					give: &emit.Method{
						Name: "create", Level: symbol.LevelType, Receiver: &emit.Param{Name: "cls"},
					},
					giveDeclare: "classmethod", want: "@builtins.classmethod\n",
				},
				{
					name: "returns builtins.property where a declaration of the file hides property",
					give: &emit.Method{Name: "size", Accessor: symbol.AccessorGet}, giveDeclare: "property",
					want: "@builtins.property\n",
				},
			}
			for _, tt := range hidden {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, python.Version311, false)
					set.Reserve(tt.giveDeclare)
					assert.Equal(t, w.Decorators(tt.give), tt.want, "the decorator line")
					assert.Equal(t, set.Entries(), []render.Entry{{Path: "builtins"}}, "the import of builtins")
				})
			}

			t.Run("returns a decorator that an import of the header binds without another import", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				_, err := w.Bind(&emit.File{Annotations: symbol.Annotations{{
					Name: string(python.ImportMarker), Args: []string{"from functools import cache as memo"},
				}}})
				assert.NoError(t, err, "the header binds its import")
				got := w.Decorators(&emit.Function{Name: "load", Annotations: symbol.Annotations{{Name: "memo"}}})
				assert.Equal(t, got, "@memo\n", "the decorator line")
				assert.Equal(t, set.Entries(), []render.Entry{{Path: "functools", Name: "memo", Item: "cache"}},
					"the import of the header alone")
			})

			t.Run("returns a dotted decorator of a package that the header imports whole without another import",
				func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, python.Version311, false)
					_, err := w.Bind(&emit.File{Annotations: symbol.Annotations{{
						Name: string(python.ImportMarker), Args: []string{"import os.path"},
					}}})
					assert.NoError(t, err, "the header binds its import")
					got := w.Decorators(&emit.Function{
						Name: "load", Annotations: symbol.Annotations{{Name: "os.path.cached"}},
					})
					assert.Equal(t, got, "@os.path.cached\n", "the decorator line")
					assert.Equal(t, set.Entries(), []render.Entry{{Path: "os/path"}}, "the import of the header alone")
				})
		})
	})
}

// Decorators allocates nothing for a declaration without decorators, and
// the lines of a property. The ordinary run, which runs no benchmark, checks
// those ceilings here.
func TestDecoratorsAllocs(t *testing.T) {
	checkAllocs(t, decoratorsCalls(t))
}

// BenchmarkDecorators measures the decorator lines of a class and of a
// method.
func BenchmarkDecorators(b *testing.B) {
	b.Run("Writer", func(b *testing.B) {
		benchCalls(b, decoratorsCalls(b))
	})
}

// decoratorsCalls returns the calls of Decorators for a class without
// decorators and for a property.
func decoratorsCalls(tb assert.TB) []allocCall {
	w, _ := newWriter(tb, python.Version311, false)
	plain := &emit.Struct{Name: rowName}
	property := &emit.Method{Name: "size", Accessor: symbol.AccessorGet}
	var got string
	return []allocCall{
		{
			name: "Decorators", caseName: "no decorators", allocs: noDecoratorsAllocs,
			call:  func() { got = w.Decorators(plain) },
			check: func(tb assert.TB) { assert.Empty(tb, got, "Decorators writes no line") },
		},
		{
			name: "Decorators", caseName: "a property", allocs: propertyAllocs,
			call:  func() { got = w.Decorators(property) },
			check: func(tb assert.TB) { assert.Equal(tb, got, "@property\n", "Decorators writes the property") },
		},
	}
}
