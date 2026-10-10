// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package conformance

import (
	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// Inventory returns the neutral feature list, in corpus order.
//
// It grows with the wave, the node model its source, and a feature
// is never removed: coverage totality over a growing list is what
// keeps a language's silence impossible. A language that cannot
// spell a feature declares the refusal in its coverage, and never
// omits the entry.
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
			Doc: "references inside composites resolve to the named types they mention",
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
						for _, field := range holder.Fields {
							expect.Contains(tb, targetsBelowRoot(field.Type), target,
								field.Name+" resolves to the sibling's declaration below its reference's root")
						}
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
						assert.Equal(tb, plain.Fields[0].Type.Target, symbol.Identity{},
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
				var stamped []string
				for id := range c.Graph.Stamps() {
					stamped = append(stamped, id.Package)
				}
				assert.Contains(tb, stamped, c.Pkg(""), "the load stamps a subject in the feature's package")
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
		{
			ID:  "directive_sugar",
			Doc: "a marker in the language's own syntax for metadata, lowered to a canonical directive",
			Declares: []Decl{
				{
					Name: "Table", Kind: symbol.KindStruct,
					Check: func(tb assert.TB, c *Ctx) {
						decl, is := c.Decl.(node.Declaration)
						assert.True(tb, is, "the subject is a declaration")
						raws := c.Graph.DirectivesOf(decl.Identity())
						assert.Length(tb, raws, 1, "the marker's directive attaches")
						expect.Equal(tb, string(raws[0].Name), "gen:table", "under its canonical spelling")
						assert.Length(tb, raws[0].Args, 1, "with the marker's one argument")
						expect.Equal(tb, raws[0].Args[0].Key, "name", "the argument is keyed")
						expect.Equal(tb, raws[0].Args[0].Value, directive.RawValue{Text: "t", Quoted: true},
							"the argument is the marker's string literal")
					},
				},
			},
		},
	}
}

// targetsBelowRoot returns the targets of the references below a tree's
// root, in walk order: each element of a structural form, such as a
// pointer's, a map's or a function type's, and each type argument of an
// instantiation, such as Option<Target>'s. The root itself does not
// count, and neither does a reference that resolved to nothing.
func targetsBelowRoot(root *node.TypeRef) []symbol.Identity {
	var out []symbol.Identity
	node.Walk(root, func(s symbol.Symbol) bool {
		if ref, is := s.(*node.TypeRef); is && ref != root && !ref.Target.IsZero() {
			out = append(out, ref.Target)
		}
		return true
	})
	return out
}
