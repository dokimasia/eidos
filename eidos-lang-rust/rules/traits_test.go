// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust"
	rustrules "go.dokimi.dev/eidos/lang/rust/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestTraits pins how the rules read the traits that a type derives and
// implements, through the capabilities that read them.
func TestTraits(t *testing.T) {
	t.Parallel()

	t.Run("derives", func(t *testing.T) {
		t.Parallel()

		marked := []*node.EnumVariant{{Name: "A", Annotations: symbol.Annotations{{Name: "default"}}}}
		tests := []struct {
			name string
			give symbol.Annotations
			want string
		}{
			{
				name: "reads a trait of a derive by its name",
				give: symbol.Annotations{{Name: "derive", Args: []string{"Default"}}},
				want: "A",
			},
			{
				name: "reads a trait of a derive by its path",
				give: symbol.Annotations{{Name: "derive", Args: []string{"core::default::Default"}}}, want: "A",
			},
			{
				name: "reads a trait between blanks",
				give: symbol.Annotations{{Name: "derive", Args: []string{" Default "}}},
				want: "A",
			},
			{
				name: "reads no trait of another attribute",
				give: symbol.Annotations{{Name: "doc", Args: []string{"Default"}}},
			},
			{name: "reads no other trait", give: symbol.Annotations{{Name: "derive", Args: []string{"Debug"}}}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				e := &node.Enum{Annotations: tt.give, Variants: marked}
				assert.Equal(t, rustrules.Rules{}.EnumOf(e, rules.View{}).Zero, tt.want, "the default variant")
			})
		}
	})

	t.Run("implementsTrait", func(t *testing.T) {
		t.Parallel()

		derived := symbol.Annotations{{Name: "derive", Args: []string{"thiserror::Error"}}}
		tests := []struct {
			name string
			give *node.TypeRef
			want bool
		}{
			{name: "reads a trait impl of the prelude", give: &node.TypeRef{Spelling: errorName}, want: true},
			{
				name: "reads a trait impl of a path of the standard library",
				give: &node.TypeRef{Spelling: "core::error::Error", Package: "core/error"}, want: true,
			},
			{name: "reads no trait impl of another crate", give: &node.TypeRef{Spelling: errorName, Package: "anyhow"}},
			{
				name: "reads no trait impl of a trait of the crate",
				give: &node.TypeRef{Spelling: errorName, Target: storeErrID},
			},
			{
				name: "reads no trait impl with type arguments",
				give: &node.TypeRef{Spelling: errorName, Args: []*node.TypeRef{{Spelling: u8Name}}},
			},
			{
				name: "reads no structural reference",
				give: &node.TypeRef{Spelling: "(u8,)", Form: symbol.FormTuple},
			},
			{name: "reads no nil reference"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s := &node.Struct{Name: "Failure", Implements: []*node.TypeRef{tt.give}}
				_, is := rustrules.Rules{}.ErrorContract(s, rules.View{})
				assert.Equal(t, is, tt.want, "whether the struct implements the error trait")
			})
		}

		t.Run("reads the derive of thiserror beside the trait impls", func(t *testing.T) {
			t.Parallel()

			s := &node.Struct{Name: "Failure", Annotations: derived}
			_, is := rustrules.Rules{}.ErrorContract(s, rules.View{})
			assert.True(t, is, "the struct derives the error")
		})
	})

	t.Run("hasTrait", func(t *testing.T) {
		t.Parallel()

		shelf := symbol.Identity{Lang: rust.Lang, Package: storePkg, Name: "Shelf", Kind: symbol.KindStruct}
		tests := []struct {
			name string
			give *node.TypeRef
			want bool
		}{
			{
				name: "reads a trait that a struct derives",
				give: &node.TypeRef{Spelling: "Order", Target: orderID},
				want: true,
			},
			{
				name: "reads a trait that a trait impl implements for a struct",
				give: &node.TypeRef{Spelling: "Shelf", Target: shelf},
				want: true,
			},
			{
				name: "reads a trait that an enum derives",
				give: &node.TypeRef{Spelling: "Status", Target: statusID},
				want: true,
			},
			{
				name: "reads no trait that an enum does not derive",
				give: &node.TypeRef{Spelling: "Color", Target: colorID},
			},
			{
				name: "reads a trait that a data enum derives",
				give: &node.TypeRef{Spelling: "Shape", Target: shapeID},
				want: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				ok, _ := rustrules.Rules{}.Comparable(tt.give, f.view)
				assert.Equal(t, ok, tt.want, "whether the type has PartialEq")
			})
		}

		t.Run("reads a trait impl of Default for a struct", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			got, ok := rustrules.Rules{}.ZeroValue(&node.TypeRef{Spelling: "Id", Target: idID}, f.view)
			assert.True(t, ok, "Id has a zero value")
			assert.Equal(t, got, emit.Call(symbol.Identity{
				Lang: rust.Lang, Package: modelsPkg, Owner: "Id", Name: "default", Kind: symbol.KindMethod,
			}), "the zero calls the default of the trait impl")
		})
	})
}
