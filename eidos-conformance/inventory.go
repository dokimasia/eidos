// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance

import (
	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// Inventory returns the neutral feature list, in corpus order.
//
// It grows with the wave, the node model its source, and a feature
// is never removed: coverage totality over a growing list is what
// keeps a language's silence impossible. A language that cannot
// spell a feature declares the refusal in its coverage rather than
// omitting the entry.
func Inventory() []Feature {
	return []Feature{
		{
			ID:  "struct_fields",
			Doc: "a record type with typed fields",
			Declares: []Decl{
				{Name: "Point", Kind: symbol.KindStruct},
				{Owner: "Point", Name: "f0", Kind: symbol.KindField},
				{Owner: "Point", Name: "f1", Kind: symbol.KindField},
			},
		},
		{
			ID:  "struct_methods",
			Doc: "a member callable on a record type",
			Declares: []Decl{
				{Name: "Store", Kind: symbol.KindStruct},
				{Owner: "Store", Name: "Get", Kind: symbol.KindMethod, Disc: "int"},
			},
		},
		{
			ID:  "method_overloads",
			Doc: "two callables sharing a name, told apart by their parameters",
			Declares: []Decl{
				{Name: "Box", Kind: symbol.KindStruct},
				{Owner: "Box", Name: "Fill", Kind: symbol.KindMethod, Disc: "int"},
				{Owner: "Box", Name: "Fill", Kind: symbol.KindMethod},
			},
		},
		{
			ID:  "constants",
			Doc: "a binding fixed at compile time",
			Declares: []Decl{
				{Name: "limit", Kind: symbol.KindConstant},
			},
		},
		{
			ID:  "cross_package_ref",
			Doc: "a reference resolving into a sibling package",
			Declares: []Decl{
				{Sub: "dep", Name: "Target", Kind: symbol.KindStruct},
				{
					Name: "Holder", Kind: symbol.KindStruct,
					Check: func(tb assert.TB, c *Ctx) {
						holder, is := c.Decl.(*node.Struct)
						assert.True(tb, is, "the holder loads as a struct")
						assert.Length(tb, holder.Fields, 1, "with its one field")
						assert.Equal(tb, holder.Fields[0].Type.Target, symbol.Identity{
							Lang: c.Lang, Package: c.Pkg("dep"),
							Name: "Target", Kind: symbol.KindStruct,
						}, "the reference targets the sibling's declaration")
					},
				},
			},
		},
		{
			ID:  "composite_refs",
			Doc: "references inside composites reach the named types they mention",
			Declares: []Decl{
				{Sub: "dep", Name: "Target", Kind: symbol.KindStruct},
				{
					Name: "Holder", Kind: symbol.KindStruct,
					Check: func(tb assert.TB, c *Ctx) {
						holder, is := c.Decl.(*node.Struct)
						assert.True(tb, is, "the holder loads as a struct")
						assert.Length(tb, holder.Fields, 3, "with its three fields")
						target := symbol.Identity{
							Lang: c.Lang, Package: c.Pkg("dep"),
							Name: "Target", Kind: symbol.KindStruct,
						}
						optional := holder.Fields[0].Type
						assert.Equal(tb, optional.Form, symbol.FormOptional,
							"a pointer states the optional form")
						assert.True(tb, optional.Target.IsZero(),
							"and a structural reference carries no target of its own")
						assert.Length(tb, optional.Elems, 1, "its child does")
						assert.Equal(tb, optional.Elems[0].Target, target,
							"resolved to the sibling's declaration")
						mapped := holder.Fields[1].Type
						assert.Equal(tb, mapped.Form, symbol.FormMap, "a map states the map form")
						assert.Length(tb, mapped.Elems, 2, "key then value")
						assert.Equal(tb, mapped.Elems[1].Target, target,
							"and the value type, which no decoration strip could reach, resolves")
						fn := holder.Fields[2].Type
						assert.Equal(tb, fn.Form, symbol.FormFunc, "a function type states the func form")
						assert.Equal(tb, fn.Split, 1, "one parameter, then its results")
						assert.Equal(tb, fn.Elems[0].Target, target, "the parameter resolves")
					},
				},
			},
		},
		{
			ID:  "builtin_ref",
			Doc: "a reference to a builtin, which keeps its spelling alone",
			Declares: []Decl{
				{
					Name: "Plain", Kind: symbol.KindStruct,
					Check: func(tb assert.TB, c *Ctx) {
						plain, is := c.Decl.(*node.Struct)
						assert.True(tb, is, "the type loads as a struct")
						assert.Length(tb, plain.Fields, 1, "with its one field")
						assert.True(tb, plain.Fields[0].Type.Target.IsZero(),
							"a builtin resolves to nothing and degrades visibly")
					},
				},
			},
		},
		{
			ID:  "directive_carrier",
			Doc: "a canonical directive attached to its subject",
			Declares: []Decl{
				{
					Name: "Table", Kind: symbol.KindStruct,
					Check: func(tb assert.TB, c *Ctx) {
						decl, names := c.Decl.(node.Declaration)
						assert.True(tb, names, "the subject names itself")
						raws := c.Graph.DirectivesOf(decl.Identity())
						assert.Length(tb, raws, 1, "the carrier's instance attaches")
						assert.Equal(tb, string(raws[0].Name), "gen:table",
							"under its canonical spelling")
					},
				},
			},
		},
		{
			ID:  "test_classification",
			Doc: "a test file stamped by the language's own convention",
			Check: func(tb assert.TB, c *Ctx) {
				stamped := false
				for id := range c.Graph.Stamps() {
					if id.Package == c.Pkg("") {
						stamped = true
					}
				}
				assert.True(tb, stamped,
					"the classification reaches the store under the feature's package")
			},
		},
		{
			ID:  "interfaces",
			Doc: "a shape values are checked against",
			Declares: []Decl{
				{Name: "Reader", Kind: symbol.KindInterface},
			},
		},
		{
			ID:  "enum_values",
			Doc: "a closed set of named values",
			Declares: []Decl{
				{Name: "Color", Kind: symbol.KindEnum},
				{Owner: "Color", Name: "Red", Kind: symbol.KindEnumVariant},
			},
		},
	}
}
