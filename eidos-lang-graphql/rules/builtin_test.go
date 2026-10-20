// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/graphql"
	graphqlrules "go.dokimi.dev/eidos/lang/graphql/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// coordinateSpelling is the spelling of a field of a document that the
// validator does not type: the schema coordinate of the field.
const coordinateSpelling = "Book.ghost"

// TestBuiltin pins the shapes of the built-in scalars and of the custom
// scalars of the table.
func TestBuiltin(t *testing.T) {
	t.Parallel()

	t.Run("Builtin", func(t *testing.T) {
		t.Parallel()

		coordinate := &node.TypeRef{Spelling: coordinateSpelling}
		tests := []struct {
			name string
			give *node.TypeRef
			want rules.TypeShape
		}{
			{
				name: "returns a signed integer of 32 bits for Int",
				give: &node.TypeRef{Spelling: graphql.IntScalar},
				want: rules.Scalar(graphql.IntScalar, rules.ScalarInt, 32),
			},
			{
				name: "returns a float of 64 bits for Float",
				give: &node.TypeRef{Spelling: graphql.FloatScalar},
				want: rules.Scalar(graphql.FloatScalar, rules.ScalarFloat, 64),
			},
			{
				name: "returns text for String",
				give: &node.TypeRef{Spelling: graphql.StringScalar},
				want: rules.Leaf(symbol.FormText, graphql.StringScalar),
			},
			{
				name: "returns text for ID",
				give: &node.TypeRef{Spelling: graphql.IDScalar},
				want: rules.Leaf(symbol.FormText, graphql.IDScalar),
			},
			{
				name: "returns a bool for Boolean",
				give: &node.TypeRef{Spelling: graphql.BooleanScalar},
				want: rules.Leaf(symbol.FormBool, graphql.BooleanScalar),
			},
			{
				name: "returns opaque for the coordinate of a field that the validator does not type",
				give: coordinate,
				want: rules.Opaque(coordinate),
			},
			{name: "returns opaque for a nil reference", want: rules.Opaque(nil)},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, graphqlrules.New().Builtin(tt.give, rules.View{}), tt.want, "the shape of the spelling")
			})
		}
	})

	t.Run("ShapeOf", func(t *testing.T) {
		t.Parallel()

		shapes := []struct {
			name string
			give symbol.Identity
			want rules.TypeShape
		}{
			{
				name: "returns a signed integer of 64 bits for the scalar of a long",
				give: longID, want: rules.Scalar("Long", rules.ScalarInt, 64),
			},
			{
				name: "returns bytes for the scalar of a base64 string",
				give: base64ID, want: rules.Leaf(symbol.FormBytes, "Base64String"),
			},
			{
				name: "returns the timestamp for the scalar of a date and a time",
				give: dateID, want: rules.Reference("DateTime", rules.WellKnownTimestamp),
			},
		}
		for _, tt := range shapes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				ref := &node.TypeRef{Spelling: tt.give.Name, Target: tt.give}
				shape, is := graphqlrules.Rules{}.ShapeOf(ref, f.decl(t, tt.give), f.view)
				assert.True(t, is, "the table contains the URL of the scalar")
				assert.Equal(t, shape, tt.want, "the shape of the entry of the table")
			})
		}

		refusals := []struct {
			name string
			give symbol.Identity
		}{
			{name: "reports false for a custom scalar outside the table", give: uploadID},
			{
				name: "reports false for a union",
				give: symbol.Identity{
					Lang:    graphql.Lang,
					Package: shopPkg,
					Name:    "SearchResult",
					Kind:    symbol.KindAlias,
				},
			},
			{name: "reports false for an object type", give: bookID},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				ref := &node.TypeRef{Spelling: tt.give.Name, Target: tt.give}
				_, is := graphqlrules.Rules{}.ShapeOf(ref, f.decl(t, tt.give), f.view)
				assert.False(t, is, "a reference to the declaration folds to a reference")
			})
		}

		t.Run("returns the shape that the fold gives a reference to a scalar of the table", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			b := rules.NewBound(graphqlrules.New(), f.view, nil)
			shape := b.TypeOf(&node.TypeRef{Spelling: "Long", Target: longID})
			assert.Equal(t, shape.Form, symbol.FormScalar, "a long folds to a scalar")
			assert.Equal(t, shape.Bits, 64, "a long folds to 64 bits")
		})
	})
}

// TestBuiltinAllocs checks the ceilings of Builtin and ShapeOf in the
// ordinary run, which does not run benchmarks.
func TestBuiltinAllocs(t *testing.T) {
	checkAllocs(t, builtinCalls(t))
}

// BenchmarkBuiltin measures the shape of a built-in scalar, which the fold
// asks for at each reference to one, and the shape of a custom scalar.
func BenchmarkBuiltin(b *testing.B) {
	benchCalls(b, builtinCalls(b))
}

// builtinCalls returns a call of Builtin and of ShapeOf.
func builtinCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	r := graphqlrules.Rules{}
	ref := &node.TypeRef{Spelling: graphql.IntScalar}
	longRef := &node.TypeRef{Spelling: longID.Name, Target: longID}
	long := f.decl(tb, longID)
	var (
		shape rules.TypeShape
		is    bool
	)
	return []allocCall{
		{
			name: "Builtin",
			call: func() { shape = r.Builtin(ref, f.view) },
			check: func(tb assert.TB) {
				assert.Equal(tb, shape.Form, symbol.FormScalar, "Builtin returns the scalar of Int")
			},
		},
		{
			name:  "ShapeOf",
			call:  func() { shape, is = r.ShapeOf(longRef, long, f.view) },
			check: func(tb assert.TB) { assert.True(tb, is, "ShapeOf returns the shape of Long") },
		},
	}
}
