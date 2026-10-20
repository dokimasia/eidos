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

// declarationAllocs is the allocations of a declaration of a module: its
// text.
const declarationAllocs = 1

// The metadata of the annotated marker that the specs of declarations
// state, and a type that Python does not spell.
var (
	annotatedMeta = symbol.Annotation{Name: string(python.AnnotatedMarker), Args: []string{`"meta"`}}
	unspelledRef  = &emit.TypeRef{Spelling: "A & B", Form: symbol.FormIntersection}
)

// TestDeclarations pins the aliases, the constants and the variables of a
// module in each version.
func TestDeclarations(t *testing.T) {
	t.Parallel()

	t.Run("Writer", func(t *testing.T) {
		t.Parallel()

		t.Run("Alias", func(t *testing.T) {
			t.Parallel()

			pair := []*emit.TypeParam{{Name: "K"}, {Name: "V"}}
			tuple := &emit.TypeRef{
				Form: symbol.FormTuple, Elems: []*emit.TypeRef{{Spelling: "K"}, {Spelling: "V"}},
			}
			tests := []struct {
				name        string
				give        *emit.Alias
				giveVersion plugin.Choice
				want        string
			}{
				{
					name: "returns a NewType for a defined alias",
					give: &emit.Alias{Name: "UserId", Target: intRef, Defined: true},
					want: "UserId = NewType(" + hugMark + "\"UserId\"" + sepMark + "int" + closeMark + ")\n",
				},
				{
					name: "returns a type statement from Python 3.12",
					give: &emit.Alias{Name: "Id", Target: strRef}, giveVersion: python.Version312,
					want: "type Id = str\n",
				},
				{
					name: "returns the type parameters of a type statement",
					give: &emit.Alias{Name: "Pair", TypeParams: pair, Target: tuple}, giveVersion: python.Version312,
					want: "type Pair[K, V] = tuple[K, V]\n",
				},
				{
					name: "returns an assignment annotated with TypeAlias for Python 3.11",
					give: &emit.Alias{Name: "Id", Target: strRef}, want: "Id: TypeAlias = str\n",
				},
				{
					name: "returns an assignment annotated with TypeAlias for type parameters of the module",
					give: &emit.Alias{
						Name: "Pair", Target: tuple,
						TypeParams: []*emit.TypeParam{{Name: "K", Variance: symbol.VarianceOut}, {Name: "V"}},
					},
					giveVersion: python.Version312, want: "Pair: TypeAlias = tuple[K, V]\n",
				},
				{
					name: "returns the comment and the docstring of an alias",
					give: &emit.Alias{
						Name:    "Id",
						Target:  strRef,
						Comment: "opaque",
						Doc:     []string{"ID names a row."},
					},
					giveVersion: python.Version312,
					want:        "type Id = str  # opaque\n\"\"\"ID names a row.\"\"\"\n",
				},
				{
					name: "returns the metadata of the annotated marker around the target",
					give: &emit.Alias{
						Name:        "Id",
						Target:      strRef,
						Annotations: symbol.Annotations{annotatedMeta},
					},
					giveVersion: python.Version312,
					want:        "type Id = Annotated[str, \"meta\"]\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, cmp.Or(tt.giveVersion, python.Version311), false)
					got, err := w.Alias(nil, tt.give)
					assert.NoError(t, err, "Python declares the alias")
					assert.Equal(t, got, tt.want, "the alias")
				})
			}

			t.Run("binds the imports of the header", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version312, false)
				_, err := w.Alias(
					&emit.File{Annotations: symbol.Annotations{jsonImport}},
					&emit.Alias{Name: "Id", Target: strRef},
				)
				assert.NoError(t, err, "Python declares the alias")
				assert.Contains(t, set.Entries(), render.Entry{Path: "json"}, "the import of the header")
			})

			t.Run("returns the error of an import of the header", func(t *testing.T) {
				t.Parallel()

				w, _ := newWriter(t, python.Version312, false)
				_, err := w.Alias(
					&emit.File{Annotations: symbol.Annotations{badImport}},
					&emit.Alias{Name: "Id", Target: strRef},
				)
				assert.HasError(t, err, "the alias refuses the header")
				assert.Equal(t, err.Error(), badImportRefusal, "the refusal names the argument")
			})

			pending := []struct {
				name        string
				giveVersion plugin.Choice
				want        string
			}{
				{
					name:        "returns a string as the target of TypeAlias for a declaration that has not finished rendering",
					giveVersion: python.Version311,
					want:        "Id: TypeAlias = \"Row\"\n",
				},
				{
					name:        "returns the name of a pending declaration as the target of a type statement",
					giveVersion: python.Version312, want: "type Id = Row\n",
				},
			}
			for _, tt := range pending {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, tt.giveVersion, false)
					set.Reserve("Id", rowName)
					set.Enter("Id")
					got, err := w.Alias(nil, &emit.Alias{Name: "Id", Target: rowRef})
					assert.NoError(t, err, "Python declares the alias")
					assert.Equal(t, got, tt.want, "the alias")
				})
			}

			refusals := []struct {
				name string
				give *emit.Alias
				want string
			}{
				{
					name: "returns an error for an alias without a target", give: &emit.Alias{Name: "Id"},
					want: "python: an alias of a module states its target, and Id states none",
				},
				{
					name: "returns an error for a defined alias with type parameters",
					give: &emit.Alias{Name: "Pair", Target: tuple, Defined: true, TypeParams: pair},
					want: "python: NewType declares no type parameters, and Pair states them",
				},
				{
					name: "returns the error of the target of a defined alias",
					give: &emit.Alias{Name: "Id", Target: unspelledRef, Defined: true},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of the target of an alias",
					give: &emit.Alias{Name: "Id", Target: unspelledRef},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of a type parameter",
					give: &emit.Alias{
						Name:       "Id",
						Target:     strRef,
						TypeParams: []*emit.TypeParam{{Name: "N", Const: true}},
					},
					want: "python: a type parameter takes a type, and N takes a value",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version312, false)
					_, err := w.Alias(nil, tt.give)
					assert.HasError(t, err, "Python refuses the alias")
					assert.Equal(t, err.Error(), tt.want, "the refusal names the part")
				})
			}
		})

		t.Run("Constant", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name     string
				give     *emit.Constant
				giveStub bool
				want     string
			}{
				{
					name: "returns a constant without a type annotated with Final",
					give: &emit.Constant{Name: "LIMIT", Value: "8"}, want: "LIMIT: Final = 8\n",
				},
				{
					name: "returns the type of a constant inside Final",
					give: &emit.Constant{Name: "LIMIT", Type: intRef, Value: "8"}, want: "LIMIT: Final[int] = 8\n",
				},
				{
					name: "returns the value of a constant in a stub file",
					give: &emit.Constant{Name: "LIMIT", Type: intRef, Value: "8"}, giveStub: true,
					want: "LIMIT: Final[int] = 8\n",
				},
				{
					name: "returns the comment and the docstring of a constant",
					give: &emit.Constant{
						Name: "LIMIT", Value: "8", Comment: "rows per call", Doc: []string{"Limit bounds one fetch."},
					},
					want: "LIMIT: Final = 8  # rows per call\n\"\"\"Limit bounds one fetch.\"\"\"\n",
				},
				{
					name: "returns the metadata of the annotated marker around the annotation",
					give: &emit.Constant{
						Name: "LIMIT", Type: intRef, Value: "8", Annotations: symbol.Annotations{annotatedMeta},
					},
					want: "LIMIT: Annotated[Final[int], \"meta\"] = 8\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version311, tt.giveStub)
					got, err := w.Constant(nil, tt.give)
					assert.NoError(t, err, "Python declares the constant")
					assert.Equal(t, got, tt.want, "the constant")
				})
			}

			t.Run("binds the imports of the header", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				_, err := w.Constant(&emit.File{Annotations: symbol.Annotations{jsonImport}},
					&emit.Constant{Name: "LIMIT", Value: "8"})
				assert.NoError(t, err, "Python declares the constant")
				assert.Contains(t, set.Entries(), render.Entry{Path: "json"}, "the import of the header")
			})

			t.Run("returns the error of an import of the header", func(t *testing.T) {
				t.Parallel()

				w, _ := newWriter(t, python.Version311, false)
				_, err := w.Constant(&emit.File{Annotations: symbol.Annotations{badImport}},
					&emit.Constant{Name: "LIMIT", Value: "8"})
				assert.HasError(t, err, "the constant refuses the header")
				assert.Equal(t, err.Error(), badImportRefusal, "the refusal names the argument")
			})

			refusals := []struct {
				name string
				give *emit.Constant
				want string
			}{
				{
					name: "returns an error for a constant without a value", give: &emit.Constant{Name: "LIMIT"},
					want: "python: a constant takes a value, and LIMIT states none",
				},
				{
					name: "returns the error of the type of a constant",
					give: &emit.Constant{Name: "LIMIT", Type: unspelledRef, Value: "8"},
					want: "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version311, false)
					_, err := w.Constant(nil, tt.give)
					assert.HasError(t, err, "Python refuses the constant")
					assert.Equal(t, err.Error(), tt.want, "the refusal names the part")
				})
			}
		})

		t.Run("Variable", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name     string
				give     *emit.Variable
				giveStub bool
				want     string
			}{
				{
					name: "returns a variable with its type and its value",
					give: &emit.Variable{Name: "count", Type: intRef, Value: "0"}, want: "count: int = 0\n",
				},
				{
					name: "returns Final for an immutable variable",
					give: &emit.Variable{
						Name:       "count",
						Type:       intRef,
						Value:      "0",
						Mutability: symbol.MutabilityImmutable,
					},
					want: "count: Final[int] = 0\n",
				},
				{
					name: "returns a variable without a type and with a value as an assignment",
					give: &emit.Variable{Name: "count", Value: "0"}, want: "count = 0\n",
				},
				{
					name: "returns a variable without a type and without a value with the type of a dynamic value",
					give: &emit.Variable{Name: "count"}, want: "count: object\n",
				},
				{
					name: "returns the value ... in a stub file",
					give: &emit.Variable{Name: "count", Type: intRef, Value: "0"}, giveStub: true,
					want: "count: int = ...\n",
				},
				{
					name: "returns the type of a dynamic value for a variable without a type in a stub file",
					give: &emit.Variable{Name: "count", Value: "0"}, giveStub: true, want: "count: object = ...\n",
				},
				{
					name: "returns the comment and the docstring of a variable",
					give: &emit.Variable{
						Name:    "count",
						Type:    intRef,
						Value:   "0",
						Comment: "fetches",
						Doc:     []string{"Count tracks fetches."},
					},
					want: "count: int = 0  # fetches\n\"\"\"Count tracks fetches.\"\"\"\n",
				},
				{
					name: "returns the metadata of the annotated marker around the annotation",
					give: &emit.Variable{Name: "count", Type: intRef, Annotations: symbol.Annotations{annotatedMeta}},
					want: "count: Annotated[int, \"meta\"]\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version311, tt.giveStub)
					got, err := w.Variable(nil, tt.give)
					assert.NoError(t, err, "Python declares the variable")
					assert.Equal(t, got, tt.want, "the variable")
				})
			}

			t.Run("binds the imports of the header", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				_, err := w.Variable(&emit.File{Annotations: symbol.Annotations{jsonImport}},
					&emit.Variable{Name: "count", Value: "0"})
				assert.NoError(t, err, "Python declares the variable")
				assert.Contains(t, set.Entries(), render.Entry{Path: "json"}, "the import of the header")
			})

			t.Run("returns the error of an import of the header", func(t *testing.T) {
				t.Parallel()

				w, _ := newWriter(t, python.Version311, false)
				_, err := w.Variable(&emit.File{Annotations: symbol.Annotations{badImport}},
					&emit.Variable{Name: "count", Value: "0"})
				assert.HasError(t, err, "the variable refuses the header")
				assert.Equal(t, err.Error(), badImportRefusal, "the refusal names the argument")
			})

			t.Run("returns the error of the type of a variable", func(t *testing.T) {
				t.Parallel()

				w, _ := newWriter(t, python.Version311, false)
				_, err := w.Variable(nil, &emit.Variable{Name: "count", Type: unspelledRef})
				assert.HasError(t, err, "Python refuses the variable")
				assert.Equal(t, err.Error(),
					"python: A & B is an intersection of types, which Python declares as a Protocol",
					"the refusal names the type")
			})
		})
	})
}

// Alias, Constant and Variable allocate their text. The ordinary run, which
// runs no benchmark, checks those ceilings here.
func TestDeclarationsAllocs(t *testing.T) {
	checkAllocs(t, declarationsCalls(t))
}

// BenchmarkDeclarations measures the declarations of a module.
func BenchmarkDeclarations(b *testing.B) {
	b.Run("Writer", func(b *testing.B) {
		benchCalls(b, declarationsCalls(b))
	})
}

// declarationsCalls returns the calls of Alias for a type statement, and of
// Constant and of Variable with a type.
func declarationsCalls(tb assert.TB) []allocCall {
	w, _ := newWriter(tb, python.Version312, false)
	alias := &emit.Alias{Name: "Id", Target: strRef, Doc: []string{"ID names a row."}}
	constant := &emit.Constant{Name: "LIMIT", Type: intRef, Value: "8"}
	variable := &emit.Variable{Name: "count", Type: intRef, Value: "0"}
	var got string
	return []allocCall{
		{
			name: "Alias", allocs: declarationAllocs,
			call: func() { got, _ = w.Alias(nil, alias) },
			check: func(tb assert.TB) {
				assert.Equal(tb, got, "type Id = str\n\"\"\"ID names a row.\"\"\"\n", "Alias writes the alias")
			},
		},
		{
			name:   "Constant",
			allocs: declarationAllocs,
			call:   func() { got, _ = w.Constant(nil, constant) },
			check:  func(tb assert.TB) { assert.Equal(tb, got, "LIMIT: Final[int] = 8\n", "Constant writes the constant") },
		},
		{
			name: "Variable", allocs: declarationAllocs,
			call:  func() { got, _ = w.Variable(nil, variable) },
			check: func(tb assert.TB) { assert.Equal(tb, got, "count: int = 0\n", "Variable writes the variable") },
		},
	}
}
