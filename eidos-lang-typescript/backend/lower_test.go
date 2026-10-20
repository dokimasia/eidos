// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The lowering cases name the fixture sum's module, the sum, its two
// variants and the payload entry of the first, and the interfaces the
// lowering names after the variants.
const (
	svcModule   = "svc"
	shapeName   = "shape"
	circleName  = "circle"
	emptyName   = "empty"
	radiusName  = "radius"
	numberType  = "number"
	typeParam   = "T"
	shapeCircle = "ShapeCircle"
	shapeEmpty  = "ShapeEmpty"
)

// The documentation the sum fixture states.
const (
	shapeDoc  = "shape is one closed figure."
	circleDoc = "circle bounds by a radius."
)

// The lowering pins the discriminant's property name, and the circle
// variant's literal type, which it quotes in TypeScript's grammar.
const (
	discriminantName = "kind"
	circleLiteral    = "'circle'"
)

// The refusal cases add a method a union cannot host, a decorator, a
// type for a payload entry without a name, and a digit that makes a
// name no identifier.
const (
	areaName      = "area"
	decoratorName = "injectable"
	stringType    = "string"
	digitPrefix   = "9"
)

// measuredName is the contract that the cases give the fixture sum.
const measuredName = "Measured"

// covariantParam is a covariant type parameter, which a source with
// variance annotations states. Each case takes a copy of it through new.
var covariantParam = emit.TypeParam{Name: typeParam, Variance: symbol.VarianceOut}

// lowerAllocs is a sum of two variants: the list of outputs, the union
// alias, its target and the joined union, and per variant its
// interface, the two steps of its name, its list of fields, the
// discriminant field, its type and the quoted literal.
const lowerAllocs = 4 + 2*7

// The lowering turns a sum into variant interfaces and the union
// alias, and passes everything else through untouched.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a struct through unchanged", func(t *testing.T) {
			t.Parallel()

			out, err := backend.Lower(&emit.Struct{Name: rowName})
			assert.NoError(t, err, "a struct is not lowered")
			assert.Empty(t, out, "a nil list keeps the declaration unchanged")
		})

		t.Run("puts the overload signatures of a class method before its implementation", func(t *testing.T) {
			t.Parallel()

			host := classOf(nil, []*emit.Method{{Name: areaName}, {Name: areaName, Overload: true}})
			out, err := backend.Lower(host)
			assert.NoError(t, err, "the class states one implementation of area")
			assert.Empty(t, out, "the class passes through")
			methods := host.Methods.Items()
			assert.Length(t, methods, 2, "the signature and the implementation")
			expect.True(t, methods[0].Overload, "the signature comes first")
			expect.False(t, methods[1].Overload, "the implementation comes after it")
		})

		t.Run("returns an error for a class with the deleter of a property", func(t *testing.T) {
			t.Parallel()

			host := classOf(nil, []*emit.Method{{Name: areaName, Accessor: symbol.AccessorDelete}})
			_, err := backend.Lower(host)
			assert.HasError(t, err, "TypeScript writes a getter and a setter alone")
		})

		t.Run("returns one interface per variant before the union alias", func(t *testing.T) {
			t.Parallel()

			out := lowered(t, sumOf())
			assert.Length(t, out, 3, "two variants and the alias")
			_, first := out[0].(*emit.Interface)
			_, second := out[1].(*emit.Interface)
			_, last := out[2].(*emit.Alias)
			expect.True(t, first, "the first variant's interface comes first")
			expect.True(t, second, "the second variant's interface follows")
			expect.True(t, last, "the union alias comes last")
		})

		t.Run("names a variant's interface in its final spelling", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, variantOf(t, sumOf(), 0).Name, shapeCircle,
				"the union restates the interface as written")
		})

		t.Run("moves a variant's documentation onto its interface", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, variantOf(t, sumOf(), 0).Doc, []string{circleDoc}, "the variant's own")
		})

		t.Run("leads a variant's interface with the discriminant", func(t *testing.T) {
			t.Parallel()

			lead := variantOf(t, sumOf(), 0).Fields.Items()[0]
			assert.Equal(t, lead.Name, discriminantName, "the discriminant property")
			assert.Equal(t, lead.Type.Spelling, circleLiteral,
				"typed to the variant's literal name, quoted in TypeScript's grammar")
		})

		t.Run("places a variant's payload behind the discriminant", func(t *testing.T) {
			t.Parallel()

			fields := variantOf(t, sumOf(), 0).Fields.Items()
			assert.Length(t, fields, 2, "the discriminant, then the payload")
			assert.Equal(t, fields[1].Name, radiusName, "the payload entry")
		})

		t.Run("gives a payloadless variant the discriminant alone", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, variantOf(t, sumOf(), 1).Fields.Len(), 1, "no payload")
		})

		t.Run("keeps the sum's name on the union alias", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, unionOf(t, sumOf()).Name, shapeName, "so references follow the settle")
		})

		t.Run("moves the sum's documentation onto the union alias", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, unionOf(t, sumOf()).Doc, []string{shapeDoc}, "the sum's own")
		})

		t.Run("joins the interfaces with the union bar", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, unionOf(t, sumOf()).Target.Spelling, shapeCircle+" | "+shapeEmpty,
				"the final spellings")
		})

		t.Run("gives every output the sum's origin", func(t *testing.T) {
			t.Parallel()

			sum := sumOf()
			for _, d := range lowered(t, sum) {
				id, held := emit.OriginOf(d)
				assert.True(t, held, "every output states an origin")
				assert.Equal(t, id, sum.Origin, "the sum's own")
			}
		})

		t.Run("restates a generic sum's parameters on each interface", func(t *testing.T) {
			t.Parallel()

			circle := variantOf(t, sumOf(&emit.TypeParam{Name: typeParam}), 0)
			assert.Length(t, circle.TypeParams, 1, "each variant restates the sum's list")
			assert.Equal(t, circle.TypeParams[0].Name, typeParam, "by name")
		})

		t.Run("restates a generic sum's parameters as the union's arguments", func(t *testing.T) {
			t.Parallel()

			alias := unionOf(t, sumOf(&emit.TypeParam{Name: typeParam}))
			assert.Length(t, alias.TypeParams, 1, "the alias takes the sum's list")
			assert.Equal(t, alias.Target.Spelling, shapeCircle+"<T> | "+shapeEmpty+"<T>",
				"each interface instantiated")
		})

		t.Run("extends the sum's contracts on each interface", func(t *testing.T) {
			t.Parallel()

			sum := sumOf()
			sum.Implements = []*emit.TypeRef{ref(measuredName)}
			assert.Equal(t, variantOf(t, sum, 1).Extends, sum.Implements, "the contracts of the second variant")
		})

		t.Run("gives each interface its own copy of the sum's contracts", func(t *testing.T) {
			t.Parallel()

			sum := sumOf()
			sum.Implements = []*emit.TypeRef{ref(measuredName)}
			out := lowered(t, sum)
			circle, isCircle := out[0].(*emit.Interface)
			assert.True(t, isCircle, "the first variant lowers to an interface")
			empty, isEmpty := out[1].(*emit.Interface)
			assert.True(t, isEmpty, "the second variant lowers to an interface")
			assert.NoDuplicates(t, func() ([]*emit.TypeRef, error) {
				return []*emit.TypeRef{sum.Implements[0], circle.Extends[0], empty.Extends[0]}, nil
			}, "the settle rewrites each reference of an output once", assert.ByIdentity())
		})

		withMethods := sumOf()
		withMethods.Methods.Append(&emit.Method{Name: areaName})
		decorated := sumOf()
		decorated.Annotations = symbol.Annotations{{Name: decoratorName}}
		decoratedVariant := sumOf()
		decoratedVariant.Variants.Items()[0].Annotations = symbol.Annotations{{Name: decoratorName}}
		positional := sumOf()
		positional.Variants.Items()[0].Fields.Append(&emit.Field{Type: ref(stringType)})
		unnamable := sumOf()
		unnamable.Name = digitPrefix + shapeName
		discriminated := sumOf()
		discriminated.Variants.Items()[0].Fields.Append(&emit.Field{Name: discriminantName, Type: ref(stringType)})
		refusals := []struct {
			name string
			give *emit.Sum
		}{
			{name: "returns an error for a sum with methods", give: withMethods},
			{name: "returns an error for a sum without variants", give: &emit.Sum{Name: shapeName}},
			{name: "returns an error for a decorated sum", give: decorated},
			{name: "returns an error for a decorated variant", give: decoratedVariant},
			{name: "returns an error for a payload entry without a name", give: positional},
			{name: "returns an error for a variant whose interface name is no identifier", give: unnamable},
			{name: "returns an error for a payload entry named like the discriminant", give: discriminated},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.Lower(tt.give)
				assert.HasError(t, err, "a union cannot state it")
			})
		}

		members := []struct {
			name string
			give symbol.Symbol
			want string
		}{
			{
				name: "returns an error for a class with two properties of one name",
				give: classOf([]*emit.Field{field(radiusName), field(radiusName)}, nil),
				want: "two properties named radius",
			},
			{
				name: "returns an error for an interface with two properties of one name",
				give: interfaceOf([]*emit.Field{field(radiusName), field(radiusName)}, nil),
				want: "two properties named radius",
			},
			{
				name: "returns an error for a class with a property and a method of one name",
				give: classOf([]*emit.Field{field(areaName)}, []*emit.Method{{Name: areaName}}),
				want: "a property and a method named area",
			},
			{
				name: "returns an error for a class with two methods of one name",
				give: classOf(nil, []*emit.Method{{Name: areaName}, {Name: areaName}}),
				want: "two methods named area",
			},
			{
				name: "returns an error for a class with two getters of one name",
				give: classOf(nil, []*emit.Method{
					{Name: areaName, Accessor: symbol.AccessorGet}, {Name: areaName, Accessor: symbol.AccessorGet},
				}),
				want: "two getters named area",
			},
			{
				name: "returns an error for a class with two setters of one name",
				give: classOf(nil, []*emit.Method{
					{Name: areaName, Accessor: symbol.AccessorSet}, {Name: areaName, Accessor: symbol.AccessorSet},
				}),
				want: "two setters named area",
			},
			{
				name: "returns an error for the overload signatures of a class method without an implementation",
				give: classOf(nil, []*emit.Method{{Name: areaName, Overload: true}}),
				want: "the overload signatures of area and no implementation",
			},
			{
				name: "returns an error for two implementations of an overloaded class method",
				give: classOf(nil, []*emit.Method{
					{Name: areaName, Overload: true}, {Name: areaName}, {Name: areaName},
				}),
				want: "two methods named area",
			},
		}
		for _, tt := range members {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.Lower(tt.give)
				assert.HasError(t, err, "TypeScript declares one member of each name")
				assert.Contains(t, err.Error(), tt.want, "the error names the member")
			})
		}

		passed := []struct {
			name string
			give symbol.Symbol
		}{
			{
				name: "passes an interface with two method signatures of one name",
				give: interfaceOf(nil, []*emit.Method{{Name: areaName}, {Name: areaName}}),
			},
			{
				name: "passes a class with a getter and a setter of one name",
				give: classOf(nil, []*emit.Method{
					{Name: areaName, Accessor: symbol.AccessorGet}, {Name: areaName, Accessor: symbol.AccessorSet},
				}),
			},
			{
				name: "passes a class with two abstract methods of one name",
				give: classOf(nil, []*emit.Method{{Name: areaName, Abstract: true}, {Name: areaName, Abstract: true}}),
			},
			{
				name: "passes a class with the overload signatures of a method and its implementation",
				give: classOf(nil, []*emit.Method{{Name: areaName, Overload: true}, {Name: areaName}}),
			},
			{
				name: "passes a class with the abstract overload signatures of a method",
				give: classOf(nil, []*emit.Method{{Name: areaName, Overload: true, Abstract: true}}),
			},
			{
				name: "passes a class with an index signature beside a method",
				give: classOf(nil, []*emit.Method{{Name: "[]", Indexer: true}, {Name: areaName}}),
			},
		}
		for _, tt := range passed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				out, err := backend.Lower(tt.give)
				assert.NoError(t, err, "TypeScript states the members")
				assert.Empty(t, out, "the declaration passes through unchanged")
			})
		}

		// Each case returns a declaration and the type parameters that
		// TypeScript writes for it, which the lowering can replace, so each
		// case builds its own declaration.
		variances := []struct {
			name string
			give func() (symbol.Symbol, *[]*emit.TypeParam)
			want symbol.Variance
		}{
			{
				name: "leaves out the variance of a type parameter of a function",
				give: func() (symbol.Symbol, *[]*emit.TypeParam) {
					f := &emit.Function{Name: areaName, TypeParams: []*emit.TypeParam{new(covariantParam)}}
					return f, &f.TypeParams
				},
				want: symbol.VarianceInvariant,
			},
			{
				name: "leaves out the variance of a type parameter of a class method",
				give: func() (symbol.Symbol, *[]*emit.TypeParam) {
					m := &emit.Method{Name: areaName, TypeParams: []*emit.TypeParam{new(covariantParam)}}
					return classOf(nil, []*emit.Method{m}), &m.TypeParams
				},
				want: symbol.VarianceInvariant,
			},
			{
				name: "leaves out the variance of a type parameter of an interface method",
				give: func() (symbol.Symbol, *[]*emit.TypeParam) {
					m := &emit.Method{Name: areaName, TypeParams: []*emit.TypeParam{new(covariantParam)}}
					return interfaceOf(nil, []*emit.Method{m}), &m.TypeParams
				},
				want: symbol.VarianceInvariant,
			},
			{
				name: "leaves out the variance of a type parameter of an alias of a tuple",
				give: func() (symbol.Symbol, *[]*emit.TypeParam) {
					a := &emit.Alias{
						Name: rowName, TypeParams: []*emit.TypeParam{new(covariantParam)},
						Target: &emit.TypeRef{Spelling: "readonly [T, T]", Form: symbol.FormTuple},
					}
					return a, &a.TypeParams
				},
				want: symbol.VarianceInvariant,
			},
			{
				name: "leaves out the variance of a type parameter of an alias without a target",
				give: func() (symbol.Symbol, *[]*emit.TypeParam) {
					a := &emit.Alias{Name: rowName, TypeParams: []*emit.TypeParam{new(covariantParam)}}
					return a, &a.TypeParams
				},
				want: symbol.VarianceInvariant,
			},
			{
				name: "keeps the variance of a type parameter of an alias of a function type",
				give: func() (symbol.Symbol, *[]*emit.TypeParam) {
					a := &emit.Alias{
						Name: rowName, TypeParams: []*emit.TypeParam{new(covariantParam)},
						Target: &emit.TypeRef{Spelling: "() => T", Form: symbol.FormFunc},
					}
					return a, &a.TypeParams
				},
				want: symbol.VarianceOut,
			},
			{
				name: "keeps the variance of a type parameter of an alias of an object type",
				give: func() (symbol.Symbol, *[]*emit.TypeParam) {
					a := &emit.Alias{
						Name: rowName, TypeParams: []*emit.TypeParam{new(covariantParam)},
						Target: &emit.TypeRef{Spelling: "{ value: T }", Form: symbol.FormInline},
					}
					return a, &a.TypeParams
				},
				want: symbol.VarianceOut,
			},
			{
				name: "keeps the variance of a type parameter of an alias of a mapped type",
				give: func() (symbol.Symbol, *[]*emit.TypeParam) {
					a := &emit.Alias{
						Name: rowName, TypeParams: []*emit.TypeParam{new(covariantParam)},
						Target: &emit.TypeRef{Spelling: "{ [K in Keys]: T }"},
					}
					return a, &a.TypeParams
				},
				want: symbol.VarianceOut,
			},
			{
				name: "keeps the variance of a type parameter of a class",
				give: func() (symbol.Symbol, *[]*emit.TypeParam) {
					s := classOf(nil, nil)
					s.TypeParams = []*emit.TypeParam{new(covariantParam)}
					return s, &s.TypeParams
				},
				want: symbol.VarianceOut,
			},
		}
		for _, tt := range variances {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				d, params := tt.give()
				out, err := backend.Lower(d)
				assert.NoError(t, err, "TypeScript states the declaration")
				assert.Empty(t, out, "the declaration stays in place")
				assert.Length(t, *params, 1, "the declaration keeps its type parameter")
				assert.Equal(t, (*params)[0].Variance, tt.want, "the variance that TypeScript writes")
			})
		}

		t.Run("leaves out the variance of a type parameter of the union alias of a sum", func(t *testing.T) {
			t.Parallel()

			alias := unionOf(t, sumOf(new(covariantParam)))
			assert.Length(t, alias.TypeParams, 1, "the alias takes the parameter of the sum")
			assert.Equal(t, alias.TypeParams[0].Variance, symbol.VarianceInvariant,
				"TypeScript admits no variance annotation on an alias of a union")
		})

		t.Run("copies the type parameters whose variance it leaves out", func(t *testing.T) {
			t.Parallel()

			params := []*emit.TypeParam{new(covariantParam)}
			f := &emit.Function{Name: areaName, TypeParams: params}
			_, err := backend.Lower(f)
			assert.NoError(t, err, "TypeScript states the function")
			expect.Equal(t, params[0].Variance, symbol.VarianceOut, "another declaration can share the parameter")
			expect.NotEqual(t, f.TypeParams[0], params[0], "the function has a copy", assert.ByIdentity())
		})
	})
}

// A declaration TypeScript states as it is allocates nothing, and a sum
// allocates its interfaces and its union alias. The lowering leaves the
// sum unchanged, so every counted call lowers the same one. The ordinary
// run, which runs no benchmark, checks those ceilings here.
func TestLowerAllocs(t *testing.T) {
	checkAllocs(t, lowerCalls())
}

// BenchmarkLower measures the lowering the settle runs over every
// declaration.
func BenchmarkLower(b *testing.B) {
	benchCalls(b, lowerCalls())
}

// lowerCalls returns a call of Lower over a struct, and over a sum of
// two variants.
func lowerCalls() []allocCall {
	row, sum := &emit.Struct{Name: rowName}, sumOf()
	var (
		out []symbol.Symbol
		err error
	)
	return []allocCall{
		{
			name: "Lower", caseName: "a struct",
			call: func() { out, err = backend.Lower(row) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Lower passes the struct")
				assert.Empty(tb, out, "Lower keeps the struct in place")
			},
		},
		{
			name: "Lower", caseName: "a sum of two variants", allocs: lowerAllocs,
			call: func() { out, err = backend.Lower(sum) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Lower reshapes the sum")
				assert.Length(tb, out, 3, "Lower returns the two interfaces and the alias")
			},
		},
	}
}

// sumOf returns the fixture sum: one variant with a named payload entry
// and one with none, stating the given type parameters.
func sumOf(params ...*emit.TypeParam) *emit.Sum {
	s := &emit.Sum{
		Origin:     symbol.Identity{Package: svcModule, Name: shapeName, Kind: symbol.KindSum},
		Doc:        []string{shapeDoc},
		Name:       shapeName,
		TypeParams: params,
	}
	circle := &emit.SumVariant{Doc: []string{circleDoc}, Name: circleName}
	circle.Fields.Append(&emit.Field{Name: radiusName, Type: ref(numberType)})
	s.Variants.Append(circle, &emit.SumVariant{Name: emptyName})
	return s
}

// classOf returns a class of the given properties and methods.
func classOf(fields []*emit.Field, methods []*emit.Method) *emit.Struct {
	s := &emit.Struct{Name: rowName}
	s.Fields.Append(fields...)
	s.Methods.Append(methods...)
	return s
}

// interfaceOf returns an interface of the given properties and methods.
func interfaceOf(fields []*emit.Field, methods []*emit.Method) *emit.Interface {
	i := &emit.Interface{Name: rowName}
	i.Fields.Append(fields...)
	i.Methods.Append(methods...)
	return i
}

// field returns a property of a number named name.
func field(name string) *emit.Field {
	return &emit.Field{Name: name, Type: ref(numberType)}
}

// lowered lowers a sum and asserts it lowers.
func lowered(tb assert.TB, sum *emit.Sum) []symbol.Symbol {
	tb.Helper()

	out, err := backend.Lower(sum)
	assert.NoError(tb, err, "the sum lowers")
	return out
}

// variantOf lowers a sum and returns the interface of its i-th
// variant.
func variantOf(tb assert.TB, sum *emit.Sum, i int) *emit.Interface {
	tb.Helper()

	iface, is := lowered(tb, sum)[i].(*emit.Interface)
	assert.True(tb, is, "a variant lowers to an interface")
	return iface
}

// unionOf lowers a sum and returns its union alias.
func unionOf(tb assert.TB, sum *emit.Sum) *emit.Alias {
	tb.Helper()

	out := lowered(tb, sum)
	alias, is := out[len(out)-1].(*emit.Alias)
	assert.True(tb, is, "the sum lowers to an alias last")
	return alias
}
