// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The sum the cases lower, its variants, and the class and field the
// first variant lowers to.
const (
	shapeName   = "shape"
	circleName  = "circle"
	emptyName   = "empty"
	circleClass = "shapeCircle"
	radiusName  = "radius"
)

// sumLowerAllocs is a sum of two variants, one with a field: the list
// of permits and per variant its reference and its joined name's two,
// the list of outputs and the principal, and per variant its class,
// its joined name's two, its implements list and reference, and the
// field list of the variant with a field.
const sumLowerAllocs = 1 + 2*3 + 1 + 1 + 2*5 + 1

// The lowering turns a sum into the principal interface and one final
// implementing class per variant, refuses a file-level type that
// states what only a member type spells, and passes everything else
// through untouched.
func TestLower(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a struct through unchanged", func(t *testing.T) {
			t.Parallel()

			out, err := backend.Lower(&emit.Struct{Name: "row", Visibility: symbol.VisibilityPackage})
			assert.NoError(t, err, "a struct is not lowered")
			assert.Empty(t, out, "a nil list keeps the declaration unchanged")
		})

		t.Run("passes a host of a private static member type", func(t *testing.T) {
			t.Parallel()

			host := &emit.Struct{Name: "row"}
			host.Types.Append(&emit.Struct{
				Name: "cell", Visibility: symbol.VisibilityPrivate, Level: symbol.LevelType,
			})
			out, err := backend.Lower(host)
			assert.NoError(t, err, "the kind template spells the member type")
			assert.Empty(t, out, "the host passes through unchanged")
		})

		refused := []struct {
			name string
			give symbol.Symbol
		}{
			{
				name: "returns an error for a static file-level class",
				give: &emit.Struct{Name: "row", Level: symbol.LevelType},
			},
			{
				name: "returns an error for a private file-level class",
				give: &emit.Struct{Name: "row", Visibility: symbol.VisibilityPrivate},
			},
			{
				name: "returns an error for a protected file-level interface",
				give: &emit.Interface{Name: "store", Visibility: symbol.VisibilityProtected},
			},
			{
				name: "returns an error for a private file-level enum",
				give: &emit.Enum{Name: "phase", Visibility: symbol.VisibilityPrivate},
			},
			{
				name: "returns an error for a private file-level sum",
				give: &emit.Sum{Name: shapeName, Visibility: symbol.VisibilityPrivate},
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.Lower(tt.give)
				assert.HasError(t, err, "javac rejects static, private and protected at file scope")
			})
		}

		t.Run("returns the principal interface followed by one class per variant", func(t *testing.T) {
			t.Parallel()

			out, err := backend.Lower(sumOf())
			assert.NoError(t, err, "the sum lowers")
			assert.Length(t, out, 3, "the principal and two variants")
		})

		t.Run("returns a principal that keeps the sum's name", func(t *testing.T) {
			t.Parallel()

			principal, _ := lowered(t, sumOf())
			assert.Equal(t, principal.Name, shapeName, "references follow the settle")
		})

		t.Run("returns a principal that keeps the sum's documentation", func(t *testing.T) {
			t.Parallel()

			sum := sumOf()
			principal, _ := lowered(t, sum)
			assert.Equal(t, principal.Doc, sum.Doc, "the sum's docblock")
		})

		t.Run("returns a sealed principal", func(t *testing.T) {
			t.Parallel()

			principal, _ := lowered(t, sumOf())
			assert.True(t, principal.Sealed, "the set of variants is closed")
		})

		t.Run("returns a principal that permits each variant class", func(t *testing.T) {
			t.Parallel()

			principal, _ := lowered(t, sumOf())
			assert.Equal(t, []string{principal.Permits[0].Spelling, principal.Permits[1].Spelling},
				[]string{circleClass, "shapeEmpty"}, "in the neutral form, so the respell decides the case")
		})

		t.Run("returns permits resolved to the sum's origin", func(t *testing.T) {
			t.Parallel()

			sum := sumOf()
			principal, _ := lowered(t, sum)
			assert.Equal(t, principal.Permits[0].Target, sum.Origin, "the settle follows it precisely")
		})

		t.Run("returns a variant class joined in the neutral form", func(t *testing.T) {
			t.Parallel()

			_, classes := lowered(t, sumOf())
			assert.Equal(t, classes[0].Name, circleClass, "the respell decides the case")
		})

		t.Run("returns a final variant class", func(t *testing.T) {
			t.Parallel()

			_, classes := lowered(t, sumOf())
			assert.True(t, classes[0].Final, "the set of variants is closed")
		})

		t.Run("returns a variant class with its variant's documentation", func(t *testing.T) {
			t.Parallel()

			_, classes := lowered(t, sumOf())
			assert.Equal(t, classes[0].Doc, []string{"circle bounds by a radius."}, "the docblock moves with it")
		})

		t.Run("returns a variant class that implements the principal by name", func(t *testing.T) {
			t.Parallel()

			_, classes := lowered(t, sumOf())
			assert.Equal(t, classes[0].Implements[0].Spelling, shapeName, "the principal's name")
		})

		t.Run("returns a variant class whose implements resolves to the sum's origin", func(t *testing.T) {
			t.Parallel()

			sum := sumOf()
			_, classes := lowered(t, sum)
			assert.Equal(t, classes[0].Implements[0].Target, sum.Origin, "the settle follows it precisely")
		})

		t.Run("returns a variant class with its payload as fields", func(t *testing.T) {
			t.Parallel()

			_, classes := lowered(t, sumOf())
			assert.Equal(t, classes[0].Fields.Items()[0].Name, radiusName, "the payload entry as stated")
		})

		t.Run("returns an empty class for a variant without a payload", func(t *testing.T) {
			t.Parallel()

			_, classes := lowered(t, sumOf())
			assert.Equal(t, classes[1].Fields.Len(), 0, "no field")
		})

		t.Run("returns every output under the sum's origin", func(t *testing.T) {
			t.Parallel()

			sum := sumOf()
			out, err := backend.Lower(sum)
			assert.NoError(t, err, "the sum lowers")
			for _, d := range out {
				id, _ := emit.OriginOf(d)
				assert.Equal(t, id, sum.Origin, "the sum's own")
			}
		})

		t.Run("restates a generic sum's parameters on the principal", func(t *testing.T) {
			t.Parallel()

			principal, _ := lowered(t, sumOf(&emit.TypeParam{Name: "T"}))
			assert.Length(t, principal.TypeParams, 1, "the sum's parameter list")
		})

		t.Run("restates a generic sum's parameters on each variant class", func(t *testing.T) {
			t.Parallel()

			_, classes := lowered(t, sumOf(&emit.TypeParam{Name: "T"}))
			assert.Length(t, classes[0].TypeParams, 1, "the sum's parameter list")
		})

		t.Run("restates a generic sum's parameters as the implements arguments", func(t *testing.T) {
			t.Parallel()

			_, classes := lowered(t, sumOf(&emit.TypeParam{Name: "T"}))
			assert.Equal(t, classes[0].Implements[0].Args[0].Spelling, "T", "by name")
		})

		t.Run("returns an error naming permits for a sum without variants", func(t *testing.T) {
			t.Parallel()

			_, err := backend.Lower(&emit.Sum{Name: shapeName})
			assert.HasError(t, err, "javac rejects a sealed interface with no permits clause")
			assert.Contains(t, err.Error(), "permits", "the error names the clause it owes")
		})

		t.Run("returns an error for a sum with methods", func(t *testing.T) {
			t.Parallel()

			withMethods := sumOf()
			withMethods.Methods.Append(&emit.Method{Name: "area"})
			_, err := backend.Lower(withMethods)
			assert.HasError(t, err, "a variant class would owe bodies the model does not state")
		})

		t.Run("returns an error for a payload entry without a name", func(t *testing.T) {
			t.Parallel()

			positional := sumOf()
			positional.Variants.Items()[0].Fields.Append(&emit.Field{Type: ref("String")})
			_, err := backend.Lower(positional)
			assert.HasError(t, err, "a field has a name")
		})
	})
}

// A declaration Java states as it is allocates nothing, and a sum
// allocates the declarations and references it lowers to. The ordinary
// run, which runs no benchmark, checks those ceilings here.
func TestLowerAllocs(t *testing.T) {
	checkAllocs(t, lowerCalls())
}

// BenchmarkLower measures the lowering the settle runs over every
// file-level declaration.
func BenchmarkLower(b *testing.B) {
	benchCalls(b, lowerCalls())
}

// lowerCalls returns a call of Lower over a struct and over a sum.
func lowerCalls() []allocCall {
	row, sum := &emit.Struct{Name: "row"}, sumOf()
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
			name: "Lower", caseName: "a sum", allocs: sumLowerAllocs,
			call: func() { out, err = backend.Lower(sum) },
			check: func(tb assert.TB) {
				assert.NoError(tb, err, "Lower reshapes the sum")
				assert.Length(tb, out, 3, "Lower returns the principal and two classes")
			},
		},
	}
}

// sumOf returns the fixture sum: one variant with a named payload entry
// and one with none.
func sumOf(params ...*emit.TypeParam) *emit.Sum {
	s := &emit.Sum{
		Origin: symbol.Identity{
			Package: "svc", Name: shapeName, Kind: symbol.KindSum,
		},
		Doc:        []string{"shape is one closed figure."},
		Name:       shapeName,
		TypeParams: params,
	}
	circle := &emit.SumVariant{
		Doc:  []string{"circle bounds by a radius."},
		Name: circleName,
	}
	circle.Fields.Append(&emit.Field{Name: radiusName, Type: ref("double")})
	s.Variants.Append(circle, &emit.SumVariant{Name: emptyName})
	return s
}

// lowered lowers a sum and returns its principal and its variant
// classes.
func lowered(tb assert.TB, sum *emit.Sum) (*emit.Interface, []*emit.Struct) {
	tb.Helper()

	out, err := backend.Lower(sum)
	assert.NoError(tb, err, "the sum lowers")
	principal, is := out[0].(*emit.Interface)
	assert.True(tb, is, "the principal lowers to an interface")
	classes := make([]*emit.Struct, 0, len(out)-1)
	for _, d := range out[1:] {
		class, isClass := d.(*emit.Struct)
		assert.True(tb, isClass, "a variant lowers to a class")
		classes = append(classes, class)
	}
	return principal, classes
}
