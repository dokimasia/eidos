// Copyright ThesmOS B.V. 2026
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

// lowerAllocs is a sum of two variants: the list of outputs, the union
// alias, its target and the joined union, and per variant its
// interface, the three steps of its name, its list of fields, the
// discriminant field, its type and the quoted literal.
const lowerAllocs = 4 + 2*8

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
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.Lower(tt.give)
				assert.HasError(t, err, "a union cannot state it")
			})
		}
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
