// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// sumOf is the fixture sum: one variant with a named payload entry
// and one with none.
func sumOf(params ...*emit.TypeParam) *emit.Sum {
	s := &emit.Sum{
		Origin: symbol.Identity{
			Package: "svc", Name: "shape", Kind: symbol.KindSum,
		},
		Doc:        []string{"shape is one closed figure."},
		Name:       "shape",
		TypeParams: params,
	}
	circle := &emit.SumVariant{
		Doc:  []string{"circle bounds by a radius."},
		Name: "circle",
	}
	circle.Fields.Append(&emit.Field{Name: "radius", Type: ref("number")})
	s.Variants.Append(circle, &emit.SumVariant{Name: "empty"})
	return s
}

// The lowering turns a sum into variant interfaces and the union
// alias; everything else passes through untouched.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("a non-sum passes through unchanged", func(t *testing.T) {
		t.Parallel()

		out, err := backend.Lower(&emit.Struct{Name: "row"})
		assert.NoError(t, err, "a struct is not lowered")
		assert.Length(t, out, 0, "a nil list keeps the declaration unchanged")
	})

	t.Run("a sum becomes variant interfaces and the union alias", func(t *testing.T) {
		t.Parallel()

		sum := sumOf()
		out, err := backend.Lower(sum)
		assert.NoError(t, err, "the canonical sum lowers")
		assert.Equal(t, len(out), 3, "two variants and the alias")

		circle, held := out[0].(*emit.Interface)
		assert.True(t, held, "the first variant lowers to an interface")
		assert.Equal(t, circle.Name, "ShapeCircle",
			"named final, because the union restates it as written")
		assert.Equal(t, circle.Doc, []string{"circle bounds by a radius."},
			"the variant's documentation moves with it")
		assert.Equal(t, circle.Fields.Len(), 2,
			"the discriminant, then the payload")
		lead := circle.Fields.Items()[0]
		assert.Equal(t, lead.Name, "kind", "the discriminant property")
		assert.Equal(t, lead.Type.Spelling, `'circle'`,
			"typed to the variant's literal name, which is data and "+
				"never respells, quoted in TypeScript's grammar")
		assert.Equal(t, circle.Fields.Items()[1].Name, "radius",
			"the payload behind it")

		empty, held := out[1].(*emit.Interface)
		assert.True(t, held, "the second variant lowers to an interface")
		assert.Equal(t, empty.Fields.Len(), 1,
			"a payloadless variant has the discriminant alone")

		alias, held := out[2].(*emit.Alias)
		assert.True(t, held, "the principal lowers to an alias")
		assert.Equal(t, alias.Name, "shape",
			"keeping the sum's name, so references follow the settle")
		assert.Equal(t, alias.Doc, sum.Doc, "and the sum's documentation")
		assert.Equal(t, alias.Target.Spelling, "ShapeCircle | ShapeEmpty",
			"the union joins the interfaces by their final spellings")

		for _, d := range out {
			id, held := emit.OriginOf(d)
			assert.True(t, held, "every output states an origin")
			assert.Equal(t, id, sum.Origin, "the sum's own")
		}
	})

	t.Run("a generic sum restates its parameters", func(t *testing.T) {
		t.Parallel()

		out, err := backend.Lower(sumOf(&emit.TypeParam{Name: "T"}))
		assert.NoError(t, err, "the generic sum lowers")

		circle := out[0].(*emit.Interface)
		assert.Equal(t, len(circle.TypeParams), 1,
			"each variant restates the sum's parameter list")
		assert.Equal(t, circle.TypeParams[0].Name, "T", "by name")

		alias := out[2].(*emit.Alias)
		assert.Equal(t, len(alias.TypeParams), 1, "the alias parameterizes")
		assert.Equal(t, alias.Target.Spelling,
			"ShapeCircle<T> | ShapeEmpty<T>",
			"and the union restates the parameters as arguments")
	})

	t.Run("refusals", func(t *testing.T) {
		t.Parallel()

		withMethods := sumOf()
		withMethods.Methods.Append(&emit.Method{Name: "area"})
		_, err := backend.Lower(withMethods)
		assert.HasError(t, err, "a union has no members")

		_, err = backend.Lower(&emit.Sum{Name: "shape"})
		assert.HasError(t, err, "a union joins at least one variant")

		decorated := sumOf()
		decorated.Annotations = symbol.Annotations{{Name: "injectable"}}
		_, err = backend.Lower(decorated)
		assert.HasError(t, err, "decorators apply to classes")

		decoratedVariant := sumOf()
		first := decoratedVariant.Variants.Items()[0]
		first.Annotations = symbol.Annotations{{Name: "injectable"}}
		_, err = backend.Lower(decoratedVariant)
		assert.HasError(t, err, "on a variant too")

		positional := sumOf()
		positional.Variants.Items()[0].Fields.Append(
			&emit.Field{Type: ref("string")},
		)
		_, err = backend.Lower(positional)
		assert.HasError(t, err, "a property has a name")
	})
}
