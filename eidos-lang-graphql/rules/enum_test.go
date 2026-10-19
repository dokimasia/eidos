// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	graphqlrules "go.dokimi.dev/eidos/lang/graphql/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The packages of the values of a hand-built enum outside the package of
// the enum.
const (
	otherPkg = "other"
	laterPkg = "later"
)

// enumAllocs is the projection of Status: the list of variants, and the
// reference and the conversion of the value out of range.
const enumAllocs = 1 + 1 + 1

// TestEnum pins the projection of an enum of GraphQL, whose values are
// their names.
func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("EnumOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name of each value as its text", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			status, is := f.decl(t, statusID).(*node.Enum)
			assert.True(t, is, "Status is an enum")
			info := graphqlrules.Rules{}.EnumOf(status, f.view)
			assert.Equal(t, info.Form, rules.EnumIdentifier, "a value of GraphQL is its name")
			assert.Equal(t, info.Variants, []rules.VariantText{
				{Name: "DRAFT", Text: emit.Literal(emit.LiteralString, "DRAFT")},
				{Name: "PUBLISHED", Text: emit.Literal(emit.LiteralString, "PUBLISHED")},
				{Name: "ARCHIVED", Text: emit.Literal(emit.LiteralString, "ARCHIVED")},
			}, "the values in their order")
		})

		t.Run("returns no zero value", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			status, is := f.decl(t, statusID).(*node.Enum)
			assert.True(t, is, "Status is an enum")
			assert.Equal(t, graphqlrules.Rules{}.EnumOf(status, f.view).Zero, "", "GraphQL gives an enum no zero")
		})

		t.Run("returns the empty name converted to the enum as the value out of range", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			status, is := f.decl(t, statusID).(*node.Enum)
			assert.True(t, is, "Status is an enum")
			assert.Equal(t, graphqlrules.Rules{}.EnumOf(status, f.view).OutOfRange,
				emit.Conversion(rules.EmitRef(&node.TypeRef{Spelling: "Status", Target: statusID}),
					emit.Literal(emit.LiteralString, "")),
				"no GraphQL name is empty")
		})

		t.Run("returns the first name that a later value repeats", func(t *testing.T) {
			t.Parallel()

			e := &node.Enum{Name: "Mode", Variants: []*node.EnumVariant{
				{Name: "ON"}, {Name: "OFF"}, {Name: "OFF"}, {Name: "ON"},
			}}
			assert.Equal(t, graphqlrules.Rules{}.EnumOf(e, rules.View{}).Duplicate, "ON",
				"ON comes first among the repeated names")
		})

		t.Run("leaves out a nil value and a value without a name", func(t *testing.T) {
			t.Parallel()

			e := &node.Enum{Name: "Mode", Variants: []*node.EnumVariant{nil, {Name: ""}, {Name: "ON"}}}
			assert.Equal(t, graphqlrules.Rules{}.EnumOf(e, rules.View{}).Variants, []rules.VariantText{
				{Name: "ON", Text: emit.Literal(emit.LiteralString, "ON")},
			}, "the one value with a name")
		})

		t.Run("returns the packages of the values outside the package of the enum", func(t *testing.T) {
			t.Parallel()

			e := &node.Enum{
				ID: statusID, Name: "Status",
				Variants: []*node.EnumVariant{
					{ID: symbol.Identity{Package: otherPkg}, Name: "A"},
					{ID: symbol.Identity{Package: shopPkg}, Name: "B"},
					{ID: symbol.Identity{Package: laterPkg}, Name: "C"},
					{ID: symbol.Identity{Package: otherPkg}, Name: "D"},
				},
			}
			assert.Equal(t, graphqlrules.Rules{}.EnumOf(e, rules.View{}).Foreign, []string{laterPkg, otherPkg},
				"each foreign package once, sorted")
		})

		t.Run("returns no value out of range for an enum without values", func(t *testing.T) {
			t.Parallel()

			info := graphqlrules.Rules{}.EnumOf(&node.Enum{Name: "Empty"}, rules.View{})
			assert.True(t, info.OutOfRange.IsZero(), "no value derives")
		})

		t.Run("returns the zero info for a nil enum", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphqlrules.Rules{}.EnumOf(nil, rules.View{}), rules.EnumInfo{Form: rules.EnumIdentifier},
				"nothing to project")
		})
	})
}

// TestEnumAllocs checks the ceiling of EnumOf in the ordinary run, which
// does not run benchmarks.
func TestEnumAllocs(t *testing.T) {
	checkAllocs(t, enumCalls(t))
}

// BenchmarkEnum measures the projection of the enum of the fixture tree.
func BenchmarkEnum(b *testing.B) {
	benchCalls(b, enumCalls(b))
}

// enumCalls returns a call of EnumOf over Status.
func enumCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	status, is := f.decl(tb, statusID).(*node.Enum)
	assert.True(tb, is, "Status is an enum")
	var info rules.EnumInfo
	return []allocCall{
		{
			name: "EnumOf", allocs: enumAllocs,
			call:  func() { info = graphqlrules.Rules{}.EnumOf(status, f.view) },
			check: func(tb assert.TB) { assert.Length(tb, info.Variants, 3, "EnumOf returns the three values") },
		},
	}
}
