// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-java/backend"
)

// sumOf is the fixture sum: one variant carrying a named payload
// entry and one carrying none.
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
	circle.Fields.Append(&emit.Field{Name: "radius", Type: ref("double")})
	s.Variants.Append(circle, &emit.SumVariant{Name: "empty"})
	return s
}

// The lowering turns a sum into the principal interface and one
// final implementing class per variant; everything else passes
// through untouched.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("a non-sum passes through unchanged", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Name: "row"}
		out, err := backend.Lower(s)
		assert.NoError(t, err, "a struct is not lowered")
		assert.Equal(t, len(out), 1, "one declaration stays one")
		assert.Equal(t, out[0], symbol.Symbol(s), "the same declaration")
	})

	t.Run("a sum becomes the interface and its final classes", func(t *testing.T) {
		t.Parallel()

		sum := sumOf()
		out, err := backend.Lower(sum)
		assert.NoError(t, err, "the canonical sum lowers")
		assert.Equal(t, len(out), 3, "the principal and two variants")

		principal, held := out[0].(*emit.Interface)
		assert.True(t, held, "the principal lowers to an interface")
		assert.Equal(t, principal.Name, "shape",
			"keeping the sum's name, so references follow the settle")
		assert.Equal(t, principal.Doc, sum.Doc, "and the sum's documentation")

		circle, held := out[1].(*emit.Struct)
		assert.True(t, held, "a variant lowers to a class")
		assert.Equal(t, circle.Name, "shapeCircle",
			"joined in the neutral form, so the respell decides the case")
		assert.True(t, circle.Final, "final, because the set is closed")
		assert.Equal(t, circle.Doc, []string{"circle bounds by a radius."},
			"the variant's documentation moves with it")
		assert.Equal(t, len(circle.Implements), 1, "implementing the principal")
		assert.Equal(t, circle.Implements[0].Spelling, "shape", "by name")
		assert.Equal(t, circle.Implements[0].Target, sum.Origin,
			"resolved to the sum's origin, so the settle follows it precisely")
		assert.Equal(t, circle.Fields.Len(), 1, "the payload as fields")
		assert.Equal(t, circle.Fields.Items()[0].Name, "radius", "as stated")

		empty := out[2].(*emit.Struct)
		assert.Equal(t, empty.Fields.Len(), 0,
			"a payloadless variant is an empty class")

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

		principal := out[0].(*emit.Interface)
		assert.Equal(t, len(principal.TypeParams), 1, "on the principal")

		circle := out[1].(*emit.Struct)
		assert.Equal(t, len(circle.TypeParams), 1,
			"each class restates the sum's parameter list")
		assert.Equal(t, len(circle.Implements[0].Args), 1,
			"and the implements clause restates it as an argument")
		assert.Equal(t, circle.Implements[0].Args[0].Spelling, "T", "by name")
	})

	t.Run("an empty sum is the interface alone", func(t *testing.T) {
		t.Parallel()

		out, err := backend.Lower(&emit.Sum{Name: "shape"})
		assert.NoError(t, err,
			"a variantless contract is legal Java, unlike a variantless union")
		assert.Equal(t, len(out), 1, "the principal and nothing else")
	})

	t.Run("refusals", func(t *testing.T) {
		t.Parallel()

		withMethods := sumOf()
		withMethods.Methods.Append(&emit.Method{Name: "area"})
		_, err := backend.Lower(withMethods)
		assert.HasError(t, err,
			"a variant class would owe bodies the model does not carry")

		positional := sumOf()
		positional.Variants.Items()[0].Fields.Append(
			&emit.Field{Type: ref("String")})
		_, err = backend.Lower(positional)
		assert.HasError(t, err, "a field carries a name")
	})
}
