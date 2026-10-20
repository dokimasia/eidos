// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/python"
	pythonrules "go.dokimi.dev/eidos/lang/python/rules"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The modules of the fixture whose names hide the names that the rules read,
// and a module that the view does not contain.
const (
	shadowPkg  = "shop/shadow"
	outsidePkg = "outside/lib"
)

// TestScope pins the bindings of the imports of a file and the canonical
// names of the callees, the decorators and the bases that the rules read.
func TestScope(t *testing.T) {
	t.Parallel()

	t.Run("bindingOf", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give []*node.Import
			spec string
			want symbol.Identity
		}{
			{
				name: "binds the alias of an import to its module",
				give: []*node.Import{{Path: modelsPkg, Alias: "m"}},
				spec: "m.Order", want: orderID,
			},
			{
				name: "binds the first name of an import of a nested module to the top module",
				give: []*node.Import{{Path: modelsPkg}},
				spec: "shop.models.Order", want: orderID,
			},
			{
				name: "binds the alias of a name of an import to the declaration",
				give: []*node.Import{{Path: modelsPkg, Names: []*node.Binding{{Name: "Outer", Alias: "O"}}}},
				spec: "O.Inner", want: innerID,
			},
			{
				name: "binds a name of an import to the declaration",
				give: []*node.Import{{Path: modelsPkg, Names: []*node.Binding{{Name: "Outer"}}}},
				spec: "Outer.Inner", want: innerID,
			},
			{
				name: "binds a name over the binding of an earlier import",
				give: []*node.Import{{Path: outsidePkg, Alias: "m"}, {Path: modelsPkg, Alias: "m"}},
				spec: "m.Order", want: orderID,
			},
			{
				name: "skips a nil import",
				give: []*node.Import{nil, {Path: modelsPkg, Alias: "m"}},
				spec: "m.Order", want: orderID,
			},
			{
				name: "skips a nil name of an import",
				give: []*node.Import{{Path: modelsPkg, Names: []*node.Binding{nil, {Name: "Outer"}}}},
				spec: "Outer.Inner", want: innerID,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				scope := rules.Scope{Subject: fetchID, File: &node.File{Imports: tt.give}}
				assert.Equal(t, f.resolvedType(t, scope, tt.spec), tt.want, "the declaration that "+tt.spec+" names")
			})
		}

		refusals := []struct {
			name string
			give *node.File
		}{
			{
				name: "binds no name for a star import",
				give: &node.File{Imports: []*node.Import{{Path: modelsPkg, Wildcard: true}}},
			},
			{name: "binds no name without a file"},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				scope := rules.Scope{Subject: fetchID, File: tt.give}
				_, err := pythonrules.Rules{}.Resolve(scope, "shop.models.Order", directive.ResolveTypeInScope, f.view)
				assert.HasError(t, err, "no import binds the name shop")
			})
		}
	})

	t.Run("canonical", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			pkg  string
			give string
			want emit.Value
		}{
			{
				name: "reads a name that the module declares as a name of the module",
				pkg:  shadowPkg, give: "Shadowed",
			},
			{
				name: "reads a name that an import binds under an alias as the name of the import",
				pkg:  shadowPkg, give: "Made",
				want: emit.Conversion(
					rules.EmitRef(&node.TypeRef{Spelling: "Made", Target: symbol.Identity{
						Lang: python.Lang, Package: shadowPkg, Name: "Made", Kind: symbol.KindEnum,
					}}),
					emit.Number(emit.LiteralInt, "2", 0),
				),
			},
			{
				name: "reads a dotted name whose first name nothing binds as a name of no module",
				pkg:  brokenPkg, give: "Unbound",
			},
			{
				name: "reads a dotted name through a declaration that an import binds as a name inside it",
				pkg:  brokenPkg, give: "Nested",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := loaded(t)
				id := symbol.Identity{Lang: python.Lang, Package: tt.pkg, Name: tt.give, Kind: symbol.KindEnum}
				e, is := f.decl(t, id).(*node.Enum)
				assert.True(t, is, tt.give+" is an enum")
				assert.Equal(t, pythonrules.Rules{}.EnumOf(e, f.view).OutOfRange, tt.want,
					"the value after the members whose values are known")
			})
		}

		t.Run("reads a dotted name through an imported module as the rest of the name", func(t *testing.T) {
			t.Parallel()

			f := loaded(t)
			order, is := f.decl(t, orderID).(*node.Struct)
			assert.True(t, is, "Order is a class")
			cs := pythonrules.Rules{}.Constructors(order, f.view)
			assert.Length(t, cs, 1, "dataclasses.dataclass generates one constructor")
			expect.Length(t, cs[0].Params, 5, "the constructor takes the fields of the dataclass")
		})

		t.Run("reads a name that nothing binds as a name of builtins", func(t *testing.T) {
			t.Parallel()

			e := builtEnum([]*node.EnumVariant{{Name: "A", Value: "auto()"}})
			assert.Equal(t, pythonrules.Rules{}.EnumOf(e, rules.View{}).OutOfRange, emit.Value{},
				"auto of builtins gives no value that the rules know")
		})
	})

	t.Run("declares", func(t *testing.T) {
		t.Parallel()

		t.Run("reports false for a module outside the view", func(t *testing.T) {
			t.Parallel()

			s := &node.Struct{
				ID: symbol.Identity{
					Lang:    python.Lang,
					Package: outsidePkg,
					Name:    "Row",
					Kind:    symbol.KindStruct,
				},
				Annotations: symbol.Annotations{{Name: "dataclass"}},
			}
			cs := pythonrules.Rules{}.Constructors(s, rules.View{})
			assert.Length(t, cs, 1, "a class without a package constructs through object")
			expect.Empty(t, cs[0].Params, "the decorator dataclass of builtins generates nothing")
		})
	})
}
