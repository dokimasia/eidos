// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The allocations of a list of type parameters: the text of the list of PEP
// 695, and the statement of a type variable that the module has declared
// before.
const (
	typeParamsAllocs = 1
	typeVarsAllocs   = 1
)

// The type parameters that the specs write: a plain one, one with the
// constraints int and str, and one with a default.
var (
	plainParam       = &emit.TypeParam{Name: "T"}
	constrainedParam = &emit.TypeParam{Name: "T", Bounds: []*emit.TypeRef{{
		Spelling: "(int, str)", Form: symbol.FormUnion, Elems: []*emit.TypeRef{intRef, strRef},
	}}}
	defaultParam = &emit.TypeParam{Name: "T", Default: intRef}
)

// TestTypeParams pins the type parameters of each version: the syntax of
// PEP 695 from Python 3.12, and the type variables of the module that
// Python 3.11, a default before Python 3.13 and a variance need.
func TestTypeParams(t *testing.T) {
	t.Parallel()

	t.Run("Writer", func(t *testing.T) {
		t.Parallel()

		t.Run("TypeParams", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name         string
				give         []*emit.TypeParam
				giveVersion  plugin.Choice
				want         string
				wantTypeVars string
				wantImports  []render.Entry
			}{
				{name: "returns nothing for no type parameters", giveVersion: python.Version312},
				{
					name: "returns a parameter in brackets from Python 3.12",
					give: []*emit.TypeParam{plainParam}, giveVersion: python.Version312, want: "[T]",
				},
				{
					name: "returns the bound of a parameter after a colon",
					give: []*emit.TypeParam{boundParam}, giveVersion: python.Version312, want: "[T: Codec]",
				},
				{
					name: "returns the constraints of a parameter in parentheses",
					give: []*emit.TypeParam{constrainedParam}, giveVersion: python.Version312, want: "[T: (int, str)]",
				},
				{
					name: "returns the default of a parameter after an equals sign from Python 3.13",
					give: []*emit.TypeParam{defaultParam}, giveVersion: python.Version313, want: "[T = int]",
				},
				{
					name: "returns a type variable tuple and a parameter specification with their stars",
					give: []*emit.TypeParam{{Name: "*Ts"}, {Name: "**P"}}, giveVersion: python.Version312,
					want: "[*Ts, **P]",
				},
				{
					name: "returns nothing for Python 3.11 and declares a type variable",
					give: []*emit.TypeParam{plainParam}, giveVersion: python.Version311,
					wantTypeVars: "T = TypeVar(\"T\")\n",
					wantImports:  []render.Entry{{Path: "typing", Name: "TypeVar"}},
				},
				{
					name: "returns nothing for Python 3.11 and declares the bound of a type variable",
					give: []*emit.TypeParam{boundParam}, giveVersion: python.Version311,
					wantTypeVars: "T = TypeVar(\"T\", bound=Codec)\n",
					wantImports:  []render.Entry{{Path: "typing", Name: "TypeVar"}},
				},
				{
					name: "returns nothing for Python 3.11 and declares the constraints of a type variable",
					give: []*emit.TypeParam{constrainedParam}, giveVersion: python.Version311,
					wantTypeVars: "T = TypeVar(\"T\", int, str)\n",
					wantImports:  []render.Entry{{Path: "typing", Name: "TypeVar"}},
				},
				{
					name: "returns nothing for a default before Python 3.13 and declares it from typing_extensions",
					give: []*emit.TypeParam{defaultParam}, giveVersion: python.Version312,
					wantTypeVars: "T = TypeVar(\"T\", default=int)\n",
					wantImports:  []render.Entry{{Path: "typing_extensions", Name: "TypeVar"}},
				},
				{
					name:        "returns nothing for a covariant parameter and declares it covariant",
					give:        []*emit.TypeParam{{Name: "T", Variance: symbol.VarianceOut}},
					giveVersion: python.Version313, wantTypeVars: "T = TypeVar(\"T\", covariant=True)\n",
					wantImports: []render.Entry{{Path: "typing", Name: "TypeVar"}},
				},
				{
					name:        "returns nothing for a contravariant parameter and declares it contravariant",
					give:        []*emit.TypeParam{{Name: "T", Variance: symbol.VarianceIn}},
					giveVersion: python.Version313, wantTypeVars: "T = TypeVar(\"T\", contravariant=True)\n",
					wantImports: []render.Entry{{Path: "typing", Name: "TypeVar"}},
				},
				{
					name:         "returns nothing for Python 3.11 and declares a type variable tuple and a parameter specification",
					give:         []*emit.TypeParam{{Name: "*Ts"}, {Name: "**P"}},
					giveVersion:  python.Version311,
					wantTypeVars: "P = ParamSpec(\"P\")\nTs = TypeVarTuple(\"Ts\")\n",
					wantImports: []render.Entry{
						{Path: "typing", Name: "ParamSpec"}, {Path: "typing", Name: "TypeVarTuple"},
					},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, tt.giveVersion, false)
					got, err := w.TypeParams(tt.give)
					assert.NoError(t, err, "Python declares the type parameters")
					assert.Equal(t, got, tt.want, "the type parameters")
					assert.Equal(t, w.TypeVars(), tt.wantTypeVars, "the type variables of the module")
					imports := []render.Entry{}
					for _, e := range set.Entries() {
						if e.Name != "" && e.Path[0] != 0 {
							imports = append(imports, e)
						}
					}
					assert.Equal(t, imports, tt.wantImports, "the imports of the type variables", assert.EquateEmpty())
				})
			}

			t.Run("returns a string for the bound of a type variable that names a declaration of the file",
				func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, python.Version311, false)
					set.Reserve(rowName)
					_, err := w.TypeParams([]*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{rowRef}}})
					assert.NoError(t, err, "Python declares the type variable")
					assert.Equal(t, w.TypeVars(), "T = TypeVar(\"T\", bound=\"Row\")\n",
						"the bound of a type variable that runs before every declaration")
				})

			t.Run("returns nothing for a type variable whose name a declaration of the file takes",
				func(t *testing.T) {
					t.Parallel()

					w, set := newWriter(t, python.Version311, false)
					set.Reserve("T")
					got, err := w.TypeParams([]*emit.TypeParam{plainParam})
					assert.NoError(t, err, "the declaration of the file declares the variable")
					assert.Empty(t, got, "the type parameters")
					assert.Empty(t, w.TypeVars(), "the module declares no type variable of its own")
				})

			t.Run("returns a type variable under the name with a number where the module binds its name otherwise",
				func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, python.Version311, false)
					_, err := w.TypeParams([]*emit.TypeParam{plainParam})
					assert.NoError(t, err, "the first declaration declares T")
					_, err = w.TypeParams([]*emit.TypeParam{boundParam})
					assert.NoError(t, err, "the second declaration declares its own type variable")
					assert.Equal(t, w.TypeVars(), "T = TypeVar(\"T\")\nT2 = TypeVar(\"T2\", bound=Codec)\n",
						"the type variables of the module")
					got, err := w.Spell(&emit.TypeRef{Spelling: "T"})
					assert.NoError(t, err, "the reference spells")
					assert.Equal(t, got, "T2", "the reference to the parameter of the second declaration")
				})

			t.Run("returns the star of an unpacked type variable tuple that takes another name", func(t *testing.T) {
				t.Parallel()

				w, _ := newWriter(t, python.Version312, false)
				_, err := w.TypeParams(
					[]*emit.TypeParam{{Name: "*Ts", Default: &emit.TypeRef{Spelling: "Unpack[tuple[int]]"}}},
				)
				assert.NoError(t, err, "the first declaration declares Ts")
				_, err = w.TypeParams(
					[]*emit.TypeParam{{Name: "*Ts", Default: &emit.TypeRef{Spelling: "Unpack[tuple[str]]"}}},
				)
				assert.NoError(t, err, "the second declaration declares its own type variable tuple")
				got, err := w.Spell(&emit.TypeRef{Spelling: "*Ts"})
				assert.NoError(t, err, "the reference spells")
				assert.Equal(t, got, "*Ts2", "the unpacked type variable tuple of the second declaration")
			})

			t.Run("returns the name of a type variable that a declaration of the file takes", func(t *testing.T) {
				t.Parallel()

				w, set := newWriter(t, python.Version311, false)
				_, err := w.TypeParams([]*emit.TypeParam{plainParam})
				assert.NoError(t, err, "the first declaration declares T")
				set.Reserve("T2")
				_, err = w.TypeParams([]*emit.TypeParam{boundParam})
				assert.NoError(t, err, "the second declaration declares its own type variable")
				assert.Equal(t, w.TypeVars(), "T = TypeVar(\"T\")\nT3 = TypeVar(\"T3\", bound=Codec)\n",
					"the type variable takes the first name that no declaration of the file takes")
			})

			t.Run("returns the type variables of the module in the order of their names", func(t *testing.T) {
				t.Parallel()

				w, _ := newWriter(t, python.Version311, false)
				_, err := w.TypeParams([]*emit.TypeParam{boundParam})
				assert.NoError(t, err, "the first declaration declares T")
				_, err = w.TypeParams([]*emit.TypeParam{{Name: "U"}})
				assert.NoError(t, err, "the second declaration declares U")
				assert.Equal(t, w.TypeVars(), "T = TypeVar(\"T\", bound=Codec)\nU = TypeVar(\"U\")\n",
					"the statements in the order of the names")
			})

			t.Run("returns one declaration for a type variable that two declarations share", func(t *testing.T) {
				t.Parallel()

				w, _ := newWriter(t, python.Version311, false)
				for range 2 {
					_, err := w.TypeParams([]*emit.TypeParam{boundParam})
					assert.NoError(t, err, "a type variable serves every declaration that declares it alike")
				}
				assert.Equal(t, w.TypeVars(), "T = TypeVar(\"T\", bound=Codec)\n", "the one type variable")
			})

			refusals := []struct {
				name        string
				give        []*emit.TypeParam
				giveVersion plugin.Choice
				want        string
			}{
				{
					name: "returns an error for a value parameter",
					give: []*emit.TypeParam{{Name: "N", Const: true}}, giveVersion: python.Version312,
					want: "python: a type parameter takes a type, and N takes a value",
				},
				{
					name: "returns an error for a bound of a type variable tuple",
					give: []*emit.TypeParam{
						{Name: "*Ts", Bounds: []*emit.TypeRef{intRef}},
					},
					giveVersion: python.Version312,
					want:        "python: a type variable tuple and a parameter specification take no bound, and *Ts states one",
				},
				{
					name:        "returns an error for a parameter with two bounds",
					give:        []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{intRef, strRef}}},
					giveVersion: python.Version312,
					want:        "python: a type parameter of Python has one bound, and T states 2",
				},
				{
					name: "returns the error of a bound",
					give: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{{
						Spelling: "A & B", Form: symbol.FormIntersection,
					}}}},
					giveVersion: python.Version312,
					want:        "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of a constraint",
					give: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{{
						Spelling: "(int, A & B)", Form: symbol.FormUnion,
						Elems: []*emit.TypeRef{intRef, {Spelling: "A & B", Form: symbol.FormIntersection}},
					}}}},
					giveVersion: python.Version312,
					want:        "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of a default",
					give: []*emit.TypeParam{{Name: "T", Default: &emit.TypeRef{
						Spelling: "A & B", Form: symbol.FormIntersection,
					}}},
					giveVersion: python.Version313,
					want:        "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
				{
					name: "returns the error of the bound of a type variable",
					give: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{{
						Spelling: "A & B", Form: symbol.FormIntersection,
					}}}},
					giveVersion: python.Version311,
					want:        "python: A & B is an intersection of types, which Python declares as a Protocol",
				},
			}
			for _, tt := range refusals {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					w, _ := newWriter(t, tt.giveVersion, false)
					_, err := w.TypeParams(tt.give)
					assert.HasError(t, err, "Python refuses the type parameters")
					assert.Equal(t, err.Error(), tt.want, "the refusal names the parameter")
				})
			}
		})
	})
}

// TypeParams allocates the text of a list of PEP 695, and the statement of a
// type variable that the module has declared before. The ordinary run, which
// runs no benchmark, checks those ceilings here.
func TestTypeParamsAllocs(t *testing.T) {
	checkAllocs(t, typeParamsCalls(t))
}

// BenchmarkTypeParams measures the type parameters of a generic class of
// each syntax.
func BenchmarkTypeParams(b *testing.B) {
	b.Run("Writer", func(b *testing.B) {
		benchCalls(b, typeParamsCalls(b))
	})
}

// typeParamsCalls returns the calls of TypeParams for a list of PEP 695 and
// for a type variable of the module.
func typeParamsCalls(tb assert.TB) []allocCall {
	modern, _ := newWriter(tb, python.Version312, false)
	legacy, _ := newWriter(tb, python.Version311, false)
	params := []*emit.TypeParam{boundParam}
	var got string
	return []allocCall{
		{
			name: "TypeParams", caseName: "PEP 695", allocs: typeParamsAllocs,
			call:  func() { got, _ = modern.TypeParams(params) },
			check: func(tb assert.TB) { assert.Equal(tb, got, "[T: Codec]", "TypeParams writes the list") },
		},
		{
			name: "TypeParams", caseName: "a type variable", allocs: typeVarsAllocs,
			call: func() { got, _ = legacy.TypeParams(params) },
			check: func(tb assert.TB) {
				assert.Equal(
					tb,
					legacy.TypeVars(),
					"T = TypeVar(\"T\", bound=Codec)\n",
					"TypeParams declares the variable",
				)
			},
		},
	}
}
