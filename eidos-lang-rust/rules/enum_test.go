// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	rustrules "go.dokimi.dev/eidos/lang/rust/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// enumAllocs is the ceiling of EnumOf: the list of the variants.
const enumAllocs = 1

// TestEnum pins what the rules state about an enum of Rust: its variants by
// name, and the variant that #[default] marks.
func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("EnumOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the names of the variants as their texts", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			e, is := f.decl(t, statusID).(*node.Enum)
			assert.True(t, is, "Status is an enum")
			info := rustrules.Rules{}.EnumOf(e, f.view)
			expect.Equal(t, info.Form, rules.EnumIdentifier, "a variant has its name as its text")
			expect.Equal(t, info.Variants, []rules.VariantText{
				{Name: "New", Text: emit.Literal(emit.LiteralString, "New")},
				{Name: "Paid", Text: emit.Literal(emit.LiteralString, "Paid")},
				{Name: "Shipped", Text: emit.Literal(emit.LiteralString, "Shipped")},
			}, "the variants in declaration order")
			expect.Equal(t, info.Duplicate, "", "Rust refuses a repeated discriminant")
			expect.Equal(t, info.OutOfRange, emit.Value{}, "no value is outside the variants")
			expect.Empty(t, info.Foreign, "the enum declares its variants")
		})

		tests := []struct {
			name string
			give *node.Enum
			want string
		}{
			{
				name: "returns the variant that #[default] marks where the enum derives Default",
				give: &node.Enum{
					Annotations: symbol.Annotations{{Name: "derive", Args: []string{"Debug", "std::default::Default"}}},
					Variants: []*node.EnumVariant{
						{Name: "A"}, nil, {Name: "B", Annotations: symbol.Annotations{{Name: "default"}}},
					},
				},
				want: "B",
			},
			{
				name: "returns no zero where the enum does not derive Default",
				give: &node.Enum{Variants: []*node.EnumVariant{
					{Name: "A", Annotations: symbol.Annotations{{Name: "default"}}},
				}},
			},
			{
				name: "returns no zero where no variant has the attribute #[default]",
				give: &node.Enum{
					Annotations: symbol.Annotations{{Name: "derive", Args: []string{"Default"}}},
					Variants:    []*node.EnumVariant{{Name: "A", Annotations: symbol.Annotations{{Name: "doc"}}}},
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, rustrules.Rules{}.EnumOf(tt.give, rules.View{}).Zero, tt.want, "the zero of the enum")
			})
		}

		t.Run("returns an identifier-formed info without variants for a nil enum", func(t *testing.T) {
			t.Parallel()

			info := rustrules.Rules{}.EnumOf(nil, rules.View{})
			expect.Equal(t, info.Form, rules.EnumIdentifier, "the form of an enum of Rust")
			expect.Empty(t, info.Variants, "a nil enum has no variants")
		})
	})
}

// TestEnumAllocs checks the ceiling of EnumOf in the ordinary run, which
// does not run benchmarks.
func TestEnumAllocs(t *testing.T) {
	checkAllocs(t, enumCalls(t))
}

// BenchmarkEnum measures EnumOf over the enum Status of the fixture under its
// ceiling.
func BenchmarkEnum(b *testing.B) {
	benchCalls(b, enumCalls(b))
}

// enumCalls returns the call of EnumOf that the allocation test and the
// benchmark measure.
func enumCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := loaded(tb)
	e, is := f.decl(tb, statusID).(*node.Enum)
	assert.True(tb, is, "Status is an enum")
	var info rules.EnumInfo
	return []allocCall{{
		name: "EnumOf", allocs: enumAllocs,
		call:  func() { info = rustrules.Rules{}.EnumOf(e, f.view) },
		check: func(tb assert.TB) { assert.Equal(tb, info.Zero, "New", "EnumOf returns the default variant") },
	}}
}
