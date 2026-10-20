// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python"
	pythonrules "go.dokimi.dev/eidos/lang/python/rules"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// transformPkg is the module of the fixture tree whose classes a
// dataclass_transform of PEP 681 makes dataclasses.
const transformPkg = "shop/transform"

// TestDataclass pins the classes that a dataclass_transform makes
// dataclasses: a class under a decorator that the transform marks, a class
// with a base that it marks, and a class whose metaclass it marks.
func TestDataclass(t *testing.T) {
	t.Parallel()

	t.Run("Constructors", func(t *testing.T) {
		t.Parallel()

		const (
			keyword    = symbol.PassingKeyword
			positional = symbol.PassingDefault
		)
		tests := []struct {
			name         string
			give         string
			wantParams   []string
			wantOptional []bool
			wantPassing  []symbol.Passing
		}{
			{
				name: "generates the __init__ of a class under a decorator that the transform marks", give: "Order",
				wantParams: []string{"id", "memo"}, wantOptional: []bool{false, true},
				wantPassing: []symbol.Passing{keyword, keyword},
			},
			{
				name: "lets an option of the decorator override a default of the transform", give: "Line",
				wantParams: []string{"sku", "count"}, wantOptional: []bool{false, true},
				wantPassing: []symbol.Passing{positional, positional},
			},
			{
				name: "generates the __init__ of a class with a base that the transform marks", give: "Item",
				wantParams: []string{"sku", "tags", "count"}, wantOptional: []bool{false, false, true},
				wantPassing: []symbol.Passing{positional, positional, positional},
			},
			{
				name: "generates the __init__ of a class whose metaclass the transform marks", give: "Record",
				wantParams: []string{"key"}, wantOptional: []bool{false}, wantPassing: []symbol.Passing{positional},
			},
			{
				name: "generates the __init__ of a class whose base has the metaclass", give: "Entry",
				wantParams: []string{"key", "value"}, wantOptional: []bool{false, false},
				wantPassing: []symbol.Passing{positional, keyword},
			},
			{name: "constructs the class that the transform marks through object", give: "ModelBase"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				id := symbol.Identity{Lang: python.Lang, Package: transformPkg, Name: tt.give, Kind: symbol.KindStruct}
				s, _ := f.decl(t, id).(*node.Struct)
				cs := pythonrules.Rules{}.Constructors(s, f.view)
				assert.Length(t, cs, 1, "the class constructs through one signature")
				expect.Equal(t, cs[0].Name, "__init__", "the method that constructs")
				expect.Equal(t, paramNames(cs[0]), tt.wantParams, "the parameters in the order of the signature",
					assert.EquateEmpty())
				expect.Equal(t, paramOptional(cs[0]), tt.wantOptional, "the parameters with a default",
					assert.EquateEmpty())
				expect.Equal(t, paramPassing(cs[0]), tt.wantPassing, "the way a call passes each argument",
					assert.EquateEmpty())
			})
		}
	})

	t.Run("Comparable", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the field without a hash of a class that the transform freezes", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			item := symbol.Identity{Lang: python.Lang, Package: transformPkg, Name: "Item", Kind: symbol.KindStruct}
			hashes, problems := pythonrules.Rules{}.Comparable(&node.TypeRef{Spelling: "Item", Target: item}, f.view)
			assert.False(t, hashes, "a frozen dataclass hashes its fields")
			assert.Length(t, problems, 1, "one field has no hash")
			expect.Equal(t, problems[0].Spelling, "list", "a list has no hash")
		})
	})
}
