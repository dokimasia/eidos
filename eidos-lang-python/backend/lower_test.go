// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// lowerAllocs is the allocations of the lowering of a struct without a
// construct to lower: the source and the tree of the parse of the value of
// its field.
const lowerAllocs = 2

// frozen is the decorator of each dataclass that the lowering of a sum
// declares.
var frozen = symbol.Annotations{{Name: "dataclass", Args: []string{"frozen=True"}}}

// sizedCloser are the members of the intersection of the interfaces Sized
// and Closer. The lowering rewrites an intersection in place and keeps its
// members, so each case builds its own intersection of these members.
var sizedCloser = []*emit.TypeRef{{Spelling: "Sized"}, {Spelling: "Closer"}}

// overloaded are the annotations of an overload signature that a source of
// Python states.
var overloaded = symbol.Annotations{{Name: overloadDecorator}}

// TestLower pins the lowering of the constructs that Python declares in
// other declarations, and the refusals of the constructs that it cannot
// declare.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a frozen dataclass for each variant of a sum and a union of them", func(t *testing.T) {
			t.Parallel()

			radius := &emit.Field{Name: "radius", Type: intRef}
			got, err := backend.Lower(&emit.Sum{
				Name: "Shape", Doc: []string{"Shape is one figure."}, Comment: "tagged",
				Variants: emit.SlotOf(
					&emit.SumVariant{Name: "circle", Doc: []string{"Circle is round."}, Fields: emit.SlotOf(radius)},
					&emit.SumVariant{Name: "empty"},
				),
			})
			assert.NoError(t, err, "Python declares the sum")
			assert.Equal(t, got, []symbol.Symbol{
				&emit.Struct{
					Name: "ShapeCircle", Doc: []string{"Circle is round."}, Annotations: frozen,
					Fields: emit.SlotOf(radius),
				},
				&emit.Struct{Name: "ShapeEmpty", Annotations: frozen},
				&emit.Alias{
					Name: "Shape", Doc: []string{"Shape is one figure."}, Comment: "tagged",
					Target: &emit.TypeRef{Form: symbol.FormUnion, Elems: []*emit.TypeRef{
						{Spelling: "ShapeCircle"}, {Spelling: "ShapeEmpty"},
					}},
				},
			}, "the dataclasses and the union", assert.EquateEmpty())
		})

		t.Run("returns the type parameters of a sum on each dataclass and in the union", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Lower(&emit.Sum{
				Name: "Pack", TypeParams: []*emit.TypeParam{{Name: "T"}},
				Variants: emit.SlotOf(&emit.SumVariant{Name: "some"}),
			})
			assert.NoError(t, err, "Python declares the sum")
			assert.Length(t, got, 2, "one dataclass and the union")
			class, is := got[0].(*emit.Struct)
			assert.True(t, is, "the first declaration is a dataclass")
			assert.Equal(t, class.TypeParams, []*emit.TypeParam{{Name: "T"}}, "the type parameters of the dataclass")
			alias, is := got[1].(*emit.Alias)
			assert.True(t, is, "the second declaration is the union")
			assert.Equal(t, alias.Target.Elems[0].Args[0].Spelling, "T", "the argument of the variant in the union")
		})

		t.Run("returns the contracts of a sum as the bases of each dataclass", func(t *testing.T) {
			t.Parallel()

			sum := &emit.Sum{
				Name: "Shape", Implements: []*emit.TypeRef{{Spelling: "Measured"}},
				Variants: emit.SlotOf(&emit.SumVariant{Name: "circle"}, &emit.SumVariant{Name: "empty"}),
			}
			got, err := backend.Lower(sum)
			assert.NoError(t, err, "Python declares the sum")
			assert.Length(t, got, 3, "two dataclasses and the union")
			circle, is := got[0].(*emit.Struct)
			assert.True(t, is, "the first declaration is a dataclass")
			empty, is := got[1].(*emit.Struct)
			assert.True(t, is, "the second declaration is a dataclass")
			expect.Equal(t, circle.Implements, sum.Implements, "the contracts of the first dataclass")
			expect.NoDuplicates(t, func() ([]*emit.TypeRef, error) {
				return []*emit.TypeRef{sum.Implements[0], circle.Implements[0], empty.Implements[0]}, nil
			}, "the settle rewrites each reference of a dataclass once", assert.ByIdentity())
		})

		t.Run("returns a Protocol for an intersection that a field states", func(t *testing.T) {
			t.Parallel()

			field := &emit.Field{
				Name: "items",
				Type: &emit.TypeRef{Spelling: "Sized & Closer", Form: symbol.FormIntersection, Elems: sizedCloser},
			}
			row := &emit.Struct{Name: rowName, Fields: emit.SlotOf(field)}
			got, err := backend.Lower(row)
			assert.NoError(t, err, "Python declares the intersection")
			assert.Equal(t, got, []symbol.Symbol{
				&emit.Interface{Name: "RowItems", Extends: []*emit.TypeRef{{Spelling: "Sized"}, {Spelling: "Closer"}}},
				row,
			}, "the Protocol and the struct", assert.EquateEmpty())
			assert.Equal(t, field.Type, &emit.TypeRef{Spelling: "RowItems"}, "the field refers to the Protocol")
		})

		t.Run("returns a number after the name of a second Protocol of one member", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Lower(&emit.Function{Name: "load", Params: emit.SlotOf(&emit.Param{
				Name: "key", Type: &emit.TypeRef{Form: symbol.FormTuple, Elems: []*emit.TypeRef{
					{
						Spelling: "Sized & Closer",
						Form:     symbol.FormIntersection,
						Elems:    sizedCloser,
					},
					{Spelling: "Sized & Closer", Form: symbol.FormIntersection, Elems: sizedCloser},
				}},
			})})
			assert.NoError(t, err, "Python declares the intersections")
			assert.Length(t, got, 3, "two Protocols and the function")
			assert.Equal(t, emit.DeclaredName(got[0]), "LoadKey", "the first Protocol")
			assert.Equal(t, emit.DeclaredName(got[1]), "LoadKey2", "the second Protocol")
		})

		names := []struct {
			name string
			give symbol.Symbol
			want string
		}{
			{
				name: "returns a Protocol named after a method and its parameter",
				give: &emit.Interface{Name: "Store", Methods: emit.SlotOf(&emit.Method{
					Name: "get",
					Params: emit.SlotOf(
						&emit.Param{
							Name: "key",
							Type: &emit.TypeRef{
								Spelling: "Sized & Closer",
								Form:     symbol.FormIntersection,
								Elems:    sizedCloser,
							},
						},
					),
				})},
				want: "StoreGetKey",
			},
			{
				name: "returns a Protocol named after a callable and its result",
				give: &emit.Function{
					Name: "load",
					Returns: emit.SlotOf(
						&emit.Return{
							Type: &emit.TypeRef{
								Spelling: "Sized & Closer",
								Form:     symbol.FormIntersection,
								Elems:    sizedCloser,
							},
						},
					),
				},
				want: "LoadResult",
			},
			{
				name: "returns a Protocol named after a constant and its type",
				give: &emit.Constant{
					Name:  "DEFAULT",
					Type:  &emit.TypeRef{Spelling: "Sized & Closer", Form: symbol.FormIntersection, Elems: sizedCloser},
					Value: "None",
				},
				want: "DEFAULTType",
			},
			{
				name: "returns a Protocol named after a variable and its type",
				give: &emit.Variable{
					Name: "current",
					Type: &emit.TypeRef{Spelling: "Sized & Closer", Form: symbol.FormIntersection, Elems: sizedCloser},
				},
				want: "CurrentType",
			},
			{
				name: "returns a Protocol named after an alias and the type that contains an intersection",
				give: &emit.Alias{Name: "Pair", Target: &emit.TypeRef{
					Form: symbol.FormTuple,
					Elems: []*emit.TypeRef{
						{Spelling: "Sized & Closer", Form: symbol.FormIntersection, Elems: sizedCloser},
						intRef,
					},
				}},
				want: "PairType",
			},
		}
		for _, tt := range names {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Lower(tt.give)
				assert.NoError(t, err, "Python declares the intersection")
				assert.Length(t, got, 2, "the Protocol and the declaration")
				assert.Equal(t, emit.DeclaredName(got[0]), tt.want, "the name of the Protocol")
			})
		}

		t.Run("returns a Protocol for an alias of an intersection", func(t *testing.T) {
			t.Parallel()

			got, err := backend.Lower(
				&emit.Alias{
					Name: "Handle",
					Doc:  []string{"A handle."},
					Target: &emit.TypeRef{
						Spelling: "Sized & Closer",
						Form:     symbol.FormIntersection,
						Elems:    sizedCloser,
					},
				},
			)
			assert.NoError(t, err, "Python declares the alias")
			assert.Equal(t, got, []symbol.Symbol{&emit.Interface{
				Name: "Handle", Doc: []string{"A handle."},
				Extends: []*emit.TypeRef{{Spelling: "Sized"}, {Spelling: "Closer"}},
			}}, "the Protocol", assert.EquateEmpty())
		})

		t.Run("returns a private member for a hard-private member without its mark", func(t *testing.T) {
			t.Parallel()

			secret := &emit.Field{Name: "#secret", Hard: true}
			reset := &emit.Method{Name: "#reset", Hard: true}
			inner := &emit.Field{Name: "#inner", Hard: true}
			got, err := backend.Lower(&emit.Struct{
				Name: rowName, Fields: emit.SlotOf(secret), Methods: emit.SlotOf(reset),
				Types: emit.SlotOf[symbol.Symbol](&emit.Interface{Name: "Inner", Fields: emit.SlotOf(inner)}),
			})
			assert.NoError(t, err, "Python declares the members")
			assert.Nil(t, got, "the struct passes through")
			expect := []struct {
				name       string
				visibility symbol.Visibility
			}{{secret.Name, secret.Visibility}, {reset.Name, reset.Visibility}, {inner.Name, inner.Visibility}}
			assert.Equal(t, expect, []struct {
				name       string
				visibility symbol.Visibility
			}{
				{"secret", symbol.VisibilityPrivate},
				{"reset", symbol.VisibilityPrivate},
				{"inner", symbol.VisibilityPrivate},
			}, "each member without its mark and private")
		})

		// Each case returns a declaration and the mark of the callable whose
		// annotations state the decorator overload. The lowering sets the
		// mark, so each case builds its own declaration.
		marks := []struct {
			name string
			give func() (symbol.Symbol, *bool)
		}{
			{
				name: "marks a function whose annotations state the decorator overload",
				give: func() (symbol.Symbol, *bool) {
					f := &emit.Function{Name: "load", Annotations: symbol.Annotations{{Name: "typing.overload"}}}
					return f, &f.Overload
				},
			},
			{
				name: "marks a method whose annotations state the decorator overload",
				give: func() (symbol.Symbol, *bool) {
					m := &emit.Method{Name: "get", Annotations: overloaded}
					return m, &m.Overload
				},
			},
			{
				name: "marks an overload of a method of a class",
				give: func() (symbol.Symbol, *bool) {
					m := &emit.Method{Name: "get", Annotations: overloaded}
					return &emit.Struct{Name: rowName, Methods: emit.SlotOf(m, &emit.Method{Name: "get"})}, &m.Overload
				},
			},
			{
				name: "marks an overload of a method of a Protocol",
				give: func() (symbol.Symbol, *bool) {
					m := &emit.Method{Name: "get", Annotations: overloaded}
					return &emit.Interface{Name: "Store", Methods: emit.SlotOf(m)}, &m.Overload
				},
			},
			{
				name: "marks an overload of a method of an enum",
				give: func() (symbol.Symbol, *bool) {
					m := &emit.Method{Name: "label", Annotations: overloaded}
					enum := &emit.Enum{Name: phaseName, Methods: emit.SlotOf(m, &emit.Method{Name: "label"})}
					return enum, &m.Overload
				},
			},
			{
				name: "marks an overload of a method of a nested class",
				give: func() (symbol.Symbol, *bool) {
					m := &emit.Method{Name: "get", Annotations: overloaded}
					inner := &emit.Struct{Name: "Inner", Methods: emit.SlotOf(m, &emit.Method{Name: "get"})}
					return &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](inner)}, &m.Overload
				},
			},
		}
		for _, tt := range marks {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				d, overload := tt.give()
				_, err := backend.Lower(d)
				assert.NoError(t, err, "Python declares the overload")
				assert.True(t, *overload, "the callable is an overload signature")
			})
		}

		t.Run("groups the overload signatures of a method name before its implementation", func(t *testing.T) {
			t.Parallel()

			implementation, put := &emit.Method{Name: "get"}, &emit.Method{Name: "put"}
			signature := &emit.Method{Name: "get", Overload: true}
			row := &emit.Struct{Name: rowName, Methods: emit.SlotOf(implementation, put, signature)}
			_, err := backend.Lower(row)
			assert.NoError(t, err, "Python declares the overloads")
			assert.Equal(t, row.Methods.Items(), []*emit.Method{signature, implementation, put},
				"the signature right before the implementation")
		})

		passes := []struct {
			name string
			give symbol.Symbol
		}{
			{
				name: "returns nothing for a struct with valid texts and decorators",
				give: &emit.Struct{
					Name: rowName, Annotations: symbol.Annotations{{Name: "dataclass", Args: []string{"frozen=True"}}},
					Fields: emit.SlotOf(&emit.Field{Name: idField, Type: intRef, Value: "field(default=0)"}),
				},
			},
			{
				name: "returns nothing for the overloads of one method",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(
					&emit.Method{Name: "get", Annotations: symbol.Annotations{{Name: overloadDecorator}}},
					&emit.Method{Name: "get"},
				)},
			},
			{
				name: "returns nothing for the overload signatures of a method of a Protocol",
				give: &emit.Interface{Name: "Store", Methods: emit.SlotOf(
					&emit.Method{Name: "get", Overload: true}, &emit.Method{Name: "get", Overload: true},
				)},
			},
			{
				name: "returns nothing for the abstract overload signatures of a method",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(
					&emit.Method{Name: "get", Overload: true, Abstract: true},
					&emit.Method{Name: "get", Overload: true, Abstract: true},
				)},
			},
			{
				name: "returns nothing for the accessors of one property",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(
					&emit.Method{Name: "size", Accessor: symbol.AccessorGet},
					&emit.Method{Name: "size", Accessor: symbol.AccessorSet},
					&emit.Method{Name: "size", Accessor: symbol.AccessorDelete},
				)},
			},
			{
				name: "returns nothing for an index signature beside a field of its name",
				give: &emit.Struct{
					Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: "index"}),
					Methods: emit.SlotOf(&emit.Method{Name: "index", Indexer: true}),
				},
			},
			{
				name: "returns nothing for the parameters of a callable in the order of Python",
				give: &emit.Function{Name: "load", Params: emit.SlotOf(
					&emit.Param{Name: "a", Passing: symbol.PassingPositional},
					&emit.Param{Name: "b"},
					&emit.Param{Name: "c", Default: "1"},
					&emit.Param{Name: "args", Variadic: symbol.VariadicPositional},
					&emit.Param{Name: "d", Passing: symbol.PassingKeyword},
					&emit.Param{Name: "kwargs", Variadic: symbol.VariadicKeyword},
				)},
			},
			{
				name: "returns nothing for a marker that marks the declaration",
				give: &emit.Interface{Name: "Movie", Annotations: symbol.Annotations{typedDictMarker}},
			},
			{name: "returns nothing for a declaration of a kind that the lowering does not check", give: &emit.File{}},
		}
		for _, tt := range passes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Lower(tt.give)
				assert.NoError(t, err, "Python declares the declaration")
				assert.Nil(t, got, "the declaration passes through")
			})
		}

		refusals := []struct {
			name string
			give symbol.Symbol
			want string
		}{
			{
				name: "returns an error for an annotation whose name is no dotted name",
				give: &emit.Function{Name: "load", Annotations: symbol.Annotations{{Name: "my-decorator"}}},
				want: `python: a decorator is a dotted name, and "my-decorator" is not one`,
			},
			{
				name: "returns an error for a marker that the satellite does not declare",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{{Name: "python:protocol"}}},
				want: "python: the satellite declares no marker python:protocol",
			},
			{
				name: "returns an error for a marker on a declaration of another kind",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{typedDictMarker}},
				want: "python: the marker python:typeddict marks no Struct",
			},
			{
				name: "returns an error for a marker that a declaration states twice",
				give: &emit.Struct{Name: rowName, Annotations: symbol.Annotations{keywordsMarker, keywordsMarker}},
				want: "python: a declaration states the marker python:keywords once, and Row states it twice",
			},
			{
				name: "returns an error for an annotation on a declaration that Python does not decorate",
				give: &emit.Alias{Name: "Id", Target: strRef, Annotations: symbol.Annotations{{Name: "dataclass"}}},
				want: "python: Python decorates a class or a callable, and the Alias Id states the annotation dataclass",
			},
			{
				name: "returns an error for an assigned annotation",
				give: &emit.Function{Name: "load", Annotations: symbol.Annotations{{
					Name: "cached", Args: []string{"1"}, Assigned: true,
				}}},
				want: "python: a decorator passes its arguments in parentheses, and cached assigns its argument",
			},
			{
				name: "returns an error for an argument of a decorator that does not parse",
				give: &emit.Function{Name: "load", Annotations: symbol.Annotations{{
					Name: "dataclass", Args: []string{"frozen="},
				}}},
				want: `python: an argument of a decorator is Python source, and "frozen=" of dataclass does not parse`,
			},
			{
				name: "returns an error for the value of a constant that does not parse",
				give: &emit.Constant{Name: "LIMIT", Value: "1 +"},
				want: `python: the text of LIMIT is Python source, and "1 +" does not parse`,
			},
			{
				name: "returns an error for the value of a variable that does not parse",
				give: &emit.Variable{Name: "count", Value: "x = 1"},
				want: `python: the text of count is Python source, and "x = 1" does not parse`,
			},
			{
				name: "returns an error for the value of a field that does not parse",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: idField, Value: "1\n2"})},
				want: "python: the text of id is Python source, and \"1\\n2\" does not parse",
			},
			{
				name: "returns an error for the value of a member of an enum that does not parse",
				give: &emit.Enum{Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{Name: "OPEN", Value: "("})},
				want: `python: the text of OPEN is Python source, and "(" does not parse`,
			},
			{
				name: "returns an error for the default of a parameter that does not parse",
				give: &emit.Function{Name: "load", Params: emit.SlotOf(&emit.Param{Name: "limit", Default: "1, 2"})},
				want: `python: the text of limit is Python source, and "1, 2" does not parse`,
			},
			{
				name: "returns an error for a parameter after the keyword variadic parameter",
				give: &emit.Function{Name: "load", Params: emit.SlotOf(
					&emit.Param{Name: "kwargs", Variadic: symbol.VariadicKeyword}, &emit.Param{Name: "a"},
				)},
				want: "python: the keyword variadic parameter of load is its last parameter",
			},
			{
				name: "returns an error for a positional variadic parameter after a keyword parameter",
				give: &emit.Function{Name: "load", Params: emit.SlotOf(
					&emit.Param{Name: "a", Passing: symbol.PassingKeyword},
					&emit.Param{Name: "args", Variadic: symbol.VariadicPositional},
				)},
				want: "python: the positional variadic parameter of load precedes its keyword parameters",
			},
			{
				name: "returns an error for a parameter that a caller passes by position after a keyword parameter",
				give: &emit.Function{Name: "load", Params: emit.SlotOf(
					&emit.Param{Name: "a", Passing: symbol.PassingKeyword}, &emit.Param{Name: "b"},
				)},
				want: "python: Python orders the parameters of a signature by how a caller passes them, " +
					"and b of load follows a parameter that a caller passes by keyword",
			},
			{
				name: "returns an error for a positional parameter after a parameter that a caller passes otherwise",
				give: &emit.Function{Name: "load", Params: emit.SlotOf(
					&emit.Param{Name: "a"}, &emit.Param{Name: "b", Passing: symbol.PassingPositional},
				)},
				want: "python: Python orders the parameters of a signature by how a caller passes them, " +
					"and the positional parameter b of load follows a parameter that a caller passes otherwise",
			},
			{
				name: "returns an error for a parameter without a default after a parameter with one",
				give: &emit.Function{Name: "load", Params: emit.SlotOf(
					&emit.Param{Name: "a", Default: "1"}, &emit.Param{Name: "b"},
				)},
				want: "python: a parameter without a default follows a parameter with a default in load, " +
					"and Python refuses b there",
			},
			{
				name: "returns an error for a setter without a getter before it",
				give: &emit.Struct{
					Name:    rowName,
					Methods: emit.SlotOf(&emit.Method{Name: "size", Accessor: symbol.AccessorSet}),
				},
				want: "python: a setter belongs to a property, and Row declares the setter size before its getter",
			},
			{
				name: "returns an error for a deleter without a getter before it",
				give: &emit.Struct{
					Name: rowName, Methods: emit.SlotOf(&emit.Method{Name: "size", Accessor: symbol.AccessorDelete}),
				},
				want: "python: a deleter belongs to a property, and Row declares the deleter size before its getter",
			},
			{
				name: "returns an error for two fields of one name",
				give: &emit.Struct{
					Name:   rowName,
					Fields: emit.SlotOf(&emit.Field{Name: idField}, &emit.Field{Name: idField}),
				},
				want: "python: Row declares two fields named id, and a class declares a name once",
			},
			{
				name: "returns an error for a field and a method of one name",
				give: &emit.Struct{
					Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: idField}),
					Methods: emit.SlotOf(&emit.Method{Name: idField}),
				},
				want: "python: Row declares a field and a method named id, and a class declares a name once",
			},
			{
				name: "returns an error for two methods of one name outside the overloads",
				give: &emit.Interface{
					Name:    "Store",
					Methods: emit.SlotOf(&emit.Method{Name: "get"}, &emit.Method{Name: "get"}),
				},
				want: "python: Store declares two methods named get, and a class declares a name once " +
					"outside the overloads and the accessors of one name",
			},
			{
				name: "returns an error for the second implementation of an overloaded method",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(
					&emit.Method{Name: "get", Overload: true}, &emit.Method{Name: "get"}, &emit.Method{Name: "get"},
				)},
				want: "python: Row declares two methods named get, and a class declares a name once " +
					"outside the overloads and the accessors of one name",
			},
			{
				name: "returns an error for the overload signatures of a method without an implementation",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(
					&emit.Method{Name: "get", Overload: true}, &emit.Method{Name: "get", Overload: true},
				)},
				want: "python: Row declares the overload signatures of get and no implementation, " +
					"and a class of a module implements each overloaded name",
			},
			{
				name: "returns an error for the overload signatures of a method whose implementation is at another level",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(
					&emit.Method{Name: "get", Overload: true, Level: symbol.LevelType}, &emit.Method{Name: "get"},
				)},
				want: "python: Row declares the overload signatures of get and no implementation, " +
					"and a class of a module implements each overloaded name",
			},
			{
				name: "returns an error for a sum without a variant", give: &emit.Sum{Name: "Shape"},
				want: "python: a union joins at least one type, and the sum Shape states no variant",
			},
			{
				name: "returns an error for a sum with methods",
				give: &emit.Sum{
					Name: "Shape", Variants: emit.SlotOf(&emit.SumVariant{Name: "circle"}),
					Methods: emit.SlotOf(&emit.Method{Name: "area"}),
				},
				want: "python: a union has no members, and the sum Shape states methods",
			},
			{
				name: "returns an error for a variant with a field without a name",
				give: &emit.Sum{Name: "Shape", Variants: emit.SlotOf(&emit.SumVariant{
					Name: "circle", Fields: emit.SlotOf(&emit.Field{Type: intRef}),
				})},
				want: "python: each field of a variant's declaration has a name, " +
					"and a payload entry of circle does not have one",
			},
			{
				name: "returns an error for the value of a field of a variant that does not parse",
				give: &emit.Sum{Name: "Shape", Variants: emit.SlotOf(&emit.SumVariant{
					Name: "circle", Fields: emit.SlotOf(&emit.Field{Name: "radius", Value: "("}),
				})},
				want: `python: the text of radius is Python source, and "(" does not parse`,
			},
			{
				name: "returns an error for an annotation of a variant",
				give: &emit.Sum{Name: "Shape", Variants: emit.SlotOf(&emit.SumVariant{
					Name: "circle", Annotations: symbol.Annotations{{Name: "python:protocol"}},
				})},
				want: "python: the satellite declares no marker python:protocol",
			},
			{
				name: "returns an error for an annotation of a field of a variant",
				give: &emit.Sum{Name: "Shape", Variants: emit.SlotOf(&emit.SumVariant{
					Name: "circle", Fields: emit.SlotOf(&emit.Field{
						Name: "radius", Annotations: symbol.Annotations{{Name: "python:protocol"}},
					}),
				})},
				want: "python: the satellite declares no marker python:protocol",
			},
			{
				name: "returns an error for an annotation of a sum",
				give: &emit.Sum{
					Name: "Shape", Variants: emit.SlotOf(&emit.SumVariant{Name: "circle"}),
					Annotations: symbol.Annotations{{Name: "python:protocol"}},
				},
				want: "python: the satellite declares no marker python:protocol",
			},
			{
				name: "returns an error for an annotation of a member of an enum",
				give: &emit.Enum{Name: phaseName, Variants: emit.SlotOf(&emit.EnumVariant{
					Name: "OPEN", Annotations: symbol.Annotations{{Name: "python:protocol"}},
				})},
				want: "python: the satellite declares no marker python:protocol",
			},
			{
				name: "returns an error for an annotation of an enum",
				give: &emit.Enum{Name: phaseName, Annotations: symbol.Annotations{{Name: "python:protocol"}}},
				want: "python: the satellite declares no marker python:protocol",
			},
			{
				name: "returns an error for an annotation of a parameter",
				give: &emit.Method{Name: "get", Params: emit.SlotOf(&emit.Param{
					Name: "key", Annotations: symbol.Annotations{{Name: "python:protocol"}},
				})},
				want: "python: the satellite declares no marker python:protocol",
			},
			{
				name: "returns an error for an annotation of a method",
				give: &emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{
					Name: "get", Annotations: symbol.Annotations{{Name: "python:protocol"}},
				})},
				want: "python: the satellite declares no marker python:protocol",
			},
			{
				name: "returns an error for an annotation of a field",
				give: &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{
					Name: idField, Annotations: symbol.Annotations{{Name: "python:protocol"}},
				})},
				want: "python: the satellite declares no marker python:protocol",
			},
			{
				name: "returns an error for an annotation of a constant",
				give: &emit.Constant{
					Name:        "LIMIT",
					Value:       "1",
					Annotations: symbol.Annotations{{Name: "python:protocol"}},
				},
				want: "python: the satellite declares no marker python:protocol",
			},
			{
				name: "returns an error for an annotation of a variable",
				give: &emit.Variable{Name: "count", Annotations: symbol.Annotations{{Name: "python:protocol"}}},
				want: "python: the satellite declares no marker python:protocol",
			},
			{
				name: "returns an error for an annotation of a nested type",
				give: &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](&emit.Struct{
					Name: "Inner", Annotations: symbol.Annotations{{Name: "python:protocol"}},
				})},
				want: "python: the satellite declares no marker python:protocol",
			},
			{
				name: "returns an error for an intersection whose name the convention of Python cannot spell",
				give: &emit.Variable{
					Name: "1st",
					Type: &emit.TypeRef{Spelling: "Sized & Closer", Form: symbol.FormIntersection, Elems: sizedCloser},
				},
				want: "python: \"1st_type\" is not an identifier, and the name of a declaration of the kind Interface " +
					"has no other form",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.Lower(tt.give)
				assert.HasError(t, err, "Python refuses the declaration")
				assert.Equal(t, err.Error(), tt.want, "the refusal names the construct")
			})
		}

		t.Run("returns the error of a name of a variant that the convention of Python cannot spell",
			func(t *testing.T) {
				t.Parallel()

				_, err := backend.Lower(&emit.Sum{Name: "1st", Variants: emit.SlotOf(&emit.SumVariant{Name: "a"})})
				assert.HasError(t, err, "Python refuses the dataclass of the variant")
			})
	})

	t.Run("LowerStub", func(t *testing.T) {
		t.Parallel()

		// Each case returns a declaration with a body and the body, which the
		// lowering of a stub file drops. The lowering changes the declaration,
		// so each case builds its own.
		tests := []struct {
			name string
			give func() (symbol.Symbol, *emit.Body)
		}{
			{
				name: "drops the body of a function",
				give: func() (symbol.Symbol, *emit.Body) {
					f := &emit.Function{Name: "load", Body: emit.Body{Verbatim: "    return\n"}}
					return f, &f.Body
				},
			},
			{
				name: "drops the body of a method",
				give: func() (symbol.Symbol, *emit.Body) {
					m := &emit.Method{Name: "load", Body: emit.Body{Verbatim: "    return\n"}}
					return m, &m.Body
				},
			},
			{
				name: "drops the bodies of the methods of a class",
				give: func() (symbol.Symbol, *emit.Body) {
					m := &emit.Method{Name: "load", Body: emit.Body{Verbatim: "    return\n"}}
					return &emit.Struct{Name: rowName, Methods: emit.SlotOf(m)}, &m.Body
				},
			},
			{
				name: "drops the bodies of the methods of a Protocol",
				give: func() (symbol.Symbol, *emit.Body) {
					m := &emit.Method{Name: "load", Body: emit.Body{Verbatim: "    return\n"}}
					return &emit.Interface{Name: "Store", Methods: emit.SlotOf(m)}, &m.Body
				},
			},
			{
				name: "drops the bodies of the methods of an enum",
				give: func() (symbol.Symbol, *emit.Body) {
					m := &emit.Method{Name: "label", Body: emit.Body{Verbatim: "    return\n"}}
					return &emit.Enum{Name: phaseName, Methods: emit.SlotOf(m)}, &m.Body
				},
			},
			{
				name: "drops the bodies of the methods of a nested class",
				give: func() (symbol.Symbol, *emit.Body) {
					m := &emit.Method{Name: "load", Body: emit.Body{Verbatim: "    return\n"}}
					inner := &emit.Struct{Name: "Inner", Methods: emit.SlotOf(m)}
					return &emit.Struct{Name: rowName, Types: emit.SlotOf[symbol.Symbol](inner)}, &m.Body
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				d, body := tt.give()
				_, err := backend.LowerStub(d)
				assert.NoError(t, err, "Python declares the stub")
				assert.Equal(t, *body, emit.Body{}, "the body of the stub")
			})
		}

		t.Run("returns nothing for the overload signatures of a method without an implementation", func(t *testing.T) {
			t.Parallel()

			got, err := backend.LowerStub(&emit.Struct{Name: rowName, Methods: emit.SlotOf(
				&emit.Method{Name: "get", Overload: true}, &emit.Method{Name: "get", Overload: true},
			)})
			assert.NoError(t, err, "a stub file states the signatures alone")
			assert.Nil(t, got, "the class passes through")
		})
	})
}

// Lower allocates the parse of each text that it checks. The ordinary run,
// which runs no benchmark, checks that ceiling here.
func TestLowerAllocs(t *testing.T) {
	checkAllocs(t, lowerCalls())
}

// BenchmarkLower measures the lowering of a struct with a value to check.
func BenchmarkLower(b *testing.B) {
	benchCalls(b, lowerCalls())
}

// lowerCalls returns the call of Lower for a struct with one field with a
// value.
func lowerCalls() []allocCall {
	row := &emit.Struct{Name: rowName, Fields: emit.SlotOf(&emit.Field{Name: idField, Type: intRef, Value: "0"})}
	var err error
	return []allocCall{
		{
			name: "Lower", allocs: lowerAllocs,
			call:  func() { _, err = backend.Lower(row) },
			check: func(tb assert.TB) { assert.NoError(tb, err, "Lower passes the struct") },
		},
	}
}
