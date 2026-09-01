// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend_test

import (
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-typescript/backend"
)

// execute runs one template over one declaration the way the
// render pass does, with the body builtin stubbed to a marker.
func execute(t *testing.T, src string, data any) string {
	t.Helper()

	tmpl, err := template.New("kind").
		Funcs(backend.Funcs()).
		Funcs(template.FuncMap{
			"body":    func(any) string { return "    body();\n" },
			"use":     func(string) string { return "" },
			"imports": func() string { return "IMPORTS\n" },
			"decls":   func() string { return "DECLS\n" },
			"slots":   func() string { return "" },
			"slot":    func(string) string { return "" },
		}).
		Parse(src)
	assert.NoError(t, err, "the template parses")
	var b strings.Builder
	assert.NoError(t, tmpl.Execute(&b, data), "the template executes")
	return b.String()
}

// Each kind template is pinned byte for byte, and the absent
// method kind is pinned too: members render inside their host.
func TestTemplates(t *testing.T) {
	t.Parallel()

	t.Run("a method has no module-level spelling", func(t *testing.T) {
		t.Parallel()

		_, held := backend.KindTemplates()[symbol.KindMethod]
		assert.False(t, held,
			"TypeScript states members inside their type, so a standalone "+
				"method degrades to a reported kind rather than guessed text")
	})

	t.Run("class carries fields and methods with bodies", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Doc: []string{"Row is one record."}, Name: "Row"}
		s.Fields.Append(&emit.Field{
			Doc: []string{"Key addresses the row."}, Name: "key", Type: ref("string"),
		})
		s.Methods.Append(&emit.Method{
			Name: "load", Returns: []*emit.Return{{Type: ref("Row")}},
		})
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"/**\n * Row is one record.\n */\n"+
				"export class Row {\n"+
				"  /**\n   * Key addresses the row.\n   */\n"+
				"  key: string;\n"+
				"  load(): Row {\n    body();\n  }\n"+
				"}\n",
			"members at member depth, docs indented whole")
	})

	t.Run("interface carries signatures alone", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{Name: "Store"}
		i.Methods.Append(&emit.Method{
			Name:   "load",
			Params: []*emit.Param{{Name: "key", Type: ref("string")}},
			Returns: []*emit.Return{
				{Type: ref("Row")},
			},
		})
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"export interface Store {\n  load(key: string): Row;\n}\n",
			"a signature closes with a semicolon and carries no body")
	})

	t.Run("function, alias, constant and variable", func(t *testing.T) {
		t.Parallel()

		f := &emit.Function{Name: "load", Returns: []*emit.Return{{Type: ref("Row")}}}
		assert.Equal(t, execute(t, backend.FunctionTemplate, f),
			"export function load(): Row {\n    body();\n}\n", "the function shape")
		assert.Equal(t,
			execute(t, backend.AliasTemplate, &emit.Alias{Name: "ID", Target: ref("string")}),
			"export type ID = string;\n", "the alias shape")
		assert.Equal(t,
			execute(t, backend.ConstantTemplate,
				&emit.Constant{Name: "MAX", Type: ref("number"), Value: "10"}),
			"export const MAX: number = 10;\n", "a typed constant")
		assert.Equal(t,
			execute(t, backend.ConstantTemplate, &emit.Constant{
				Name: "MAX", Type: ref("number"), Value: "10",
				Comment: "rows per call",
			}),
			"export const MAX: number = 10; // rows per call\n",
			"a trailing comment behind the semicolon")
		assert.Equal(t,
			execute(t, backend.VariableTemplate, &emit.Variable{Name: "count"}),
			"export let count;\n", "an untyped binding stays untyped")
	})

	t.Run("generics", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Name: "Box", TypeParams: []*emit.TypeParam{{Name: "T"}}}
		s.Fields.Append(&emit.Field{Name: "item", Type: ref("T")})
		s.Methods.Append(&emit.Method{
			Name:       "map",
			TypeParams: []*emit.TypeParam{{Name: "U", Bounds: []*emit.TypeRef{ref("Codec")}}},
			Params:     []*emit.Param{{Name: "item", Type: ref("U")}},
			Returns:    []*emit.Return{{Type: ref("U")}},
		})
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"export class Box<T> {\n"+
				"  item: T;\n"+
				"  map<U extends Codec>(item: U): U {\n    body();\n  }\n"+
				"}\n",
			"the class's parameter list behind its name, the member's behind its own")

		i := &emit.Interface{
			Name: "Keyed",
			TypeParams: []*emit.TypeParam{
				{Name: "K", Bounds: []*emit.TypeRef{ref("Codec")}},
			},
		}
		i.Methods.Append(&emit.Method{
			Name:    "pick",
			Params:  []*emit.Param{{Name: "key", Type: ref("K")}},
			Returns: []*emit.Return{{Type: ref("K")}},
		})
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"export interface Keyed<K extends Codec> {\n"+
				"  pick(key: K): K;\n"+
				"}\n",
			"the bound behind the parameter, members referencing it")

		f := &emit.Function{
			Name:       "sort",
			TypeParams: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{ref("Codec")}}},
			Params:     []*emit.Param{{Name: "items", Type: ref("T")}},
			Returns:    []*emit.Return{{Type: ref("T")}},
		}
		assert.Equal(t, execute(t, backend.FunctionTemplate, f),
			"export function sort<T extends Codec>(items: T): T {\n    body();\n}\n",
			"the function's parameter list behind its name")

		a := &emit.Alias{
			Name:       "Match",
			TypeParams: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{ref("Codec")}}},
			Target:     &emit.TypeRef{Spelling: "Keyed", Args: []*emit.TypeRef{ref("T")}},
		}
		assert.Equal(t, execute(t, backend.AliasTemplate, a),
			"export type Match<T extends Codec> = Keyed<T>;\n",
			"the alias parameterizes and its target restates the argument")
	})

	t.Run("supertypes", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{
			Name:       "Row",
			Extends:    []*emit.TypeRef{ref("Base")},
			Implements: []*emit.TypeRef{ref("Keyed")},
		}
		s.Fields.Append(&emit.Field{Name: "key", Type: ref("string")})
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"export class Row extends Base implements Keyed {\n"+
				"  key: string;\n"+
				"}\n",
			"the heritage clauses behind the name")

		i := &emit.Interface{
			Name:    "Store",
			Extends: []*emit.TypeRef{ref("Keyed"), ref("Closer")},
		}
		i.Methods.Append(&emit.Method{
			Name:    "get",
			Returns: []*emit.Return{{Type: ref("string")}},
		})
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"export interface Store extends Keyed, Closer {\n"+
				"  get(): string;\n"+
				"}\n",
			"the widened contracts joined behind extends")
	})

	t.Run("modifiers", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{
			Name:        "Row",
			Abstract:    true,
			Annotations: emit.Annotations{{Name: "injectable"}},
		}
		s.Fields.Append(&emit.Field{
			Name:       "key",
			Visibility: symbol.VisibilityPrivate,
			Level:      symbol.LevelType,
			Mutability: symbol.MutabilityImmutable,
			Type:       ref("string"),
			Value:      `"r"`,
		})
		s.Methods.Append(
			&emit.Method{
				Name: "load", Async: true, Override: true,
				Returns: []*emit.Return{{Type: ref("Row")}},
			},
			&emit.Method{
				Name: "pick", Abstract: true,
				Returns: []*emit.Return{{Type: ref("Row")}},
			},
		)
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"@injectable\n"+
				"export abstract class Row {\n"+
				"  private static readonly key: string = \"r\";\n"+
				"  override async load(): Row {\n    body();\n  }\n"+
				"  abstract pick(): Row;\n"+
				"}\n",
			"decorators above, keywords in stated order, the abstract "+
				"method a signature alone")

		i := &emit.Interface{Name: "Store"}
		i.Fields.Append(&emit.Field{
			Name: "kind", Mutability: symbol.MutabilityImmutable,
			Type: ref("string"),
		})
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"export interface Store {\n  readonly kind: string;\n}\n",
			"readonly on the property, nothing else")

		f := &emit.Function{
			Name: "load", Async: true,
			Returns: []*emit.Return{{Type: ref("Row")}},
		}
		assert.Equal(t, execute(t, backend.FunctionTemplate, f),
			"export async function load(): Row {\n    body();\n}\n",
			"async behind export")

		v := &emit.Variable{
			Name: "max", Mutability: symbol.MutabilityImmutable,
			Type: ref("number"), Value: "8",
		}
		assert.Equal(t, execute(t, backend.VariableTemplate, v),
			"export const max: number = 8;\n",
			"an immutable binding is const with its initializer")
	})

	t.Run("enum", func(t *testing.T) {
		t.Parallel()

		e := &emit.Enum{Doc: []string{"Phase names a step."}, Name: "Phase"}
		e.Variants.Append(
			&emit.EnumVariant{Doc: []string{"Open admits writes."}, Name: "Open"},
			&emit.EnumVariant{Name: "Closed", Value: "9"},
		)
		assert.Equal(t, execute(t, backend.EnumTemplate, e),
			"/**\n * Phase names a step.\n */\n"+
				"export enum Phase {\n"+
				"  /**\n   * Open admits writes.\n   */\n"+
				"  Open,\n"+
				"  Closed = 9,\n"+
				"}\n",
			"one member per variant, a stated value behind equals")

		withMembers := &emit.Enum{Name: "Phase"}
		withMembers.Methods.Append(&emit.Method{Name: "describe"})
		tmpl, err := template.New("kind").Funcs(backend.Funcs()).Parse(backend.EnumTemplate)
		assert.NoError(t, err, "the template parses")
		var b strings.Builder
		assert.HasError(t, tmpl.Execute(&b, withMembers),
			"an enum carrying members refuses")
	})

	t.Run("the file skeleton is imports then declarations", func(t *testing.T) {
		t.Parallel()

		got := execute(t, backend.FileTemplate, struct {
			Name string
			Pkg  symbol.Identity
		}{Name: "store.stub.ts"})
		assert.Equal(t, got, "IMPORTS\nDECLS\n",
			"no package clause, because the file is the module")
	})
}
