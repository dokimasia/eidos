// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// execute runs one template over one declaration the way the
// render pass does, with the body builtin stubbed to a marker.
func execute(t *testing.T, src string, data any) string {
	t.Helper()

	got, err := run(t, src, data)
	assert.NoError(t, err, "the template executes")
	return got
}

// run runs one template over one declaration and returns the text
// or the refusal.
func run(t *testing.T, src string, data any) (string, error) {
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
	err = tmpl.Execute(&b, data)
	return b.String(), err
}

// Each kind template is pinned byte for byte, and the absent
// method kind is pinned too: members render inside their host.
func TestTemplates(t *testing.T) {
	t.Parallel()

	t.Run("a method has no module-level spelling", func(t *testing.T) {
		t.Parallel()

		_, is := backend.KindTemplates()[symbol.KindMethod]
		assert.False(t, is,
			"TypeScript states members inside their type, so a standalone "+
				"method is reported as a kind the target cannot spell")
	})

	t.Run("class spells fields and methods with bodies", func(t *testing.T) {
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

	t.Run("interface spells signatures alone", func(t *testing.T) {
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
			"a signature closes with a semicolon and has no body")
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
			"export let count;\n", "an untyped binding is left untyped")
		_, err := run(t, backend.VariableTemplate, &emit.Variable{
			Name: "count", Mutability: symbol.MutabilityImmutable, Type: ref("number"),
		})
		assert.HasError(t, err,
			"a const without an initializer refuses, because const count: number; "+
				"does not compile")
	})

	t.Run("trailing comments close every kind that ends a line", func(t *testing.T) {
		t.Parallel()

		i := &emit.Interface{Name: "Store", Comment: "read side"}
		i.Methods.Append(&emit.Method{
			Name: "get", Comment: "by key",
			Params:  []*emit.Param{{Name: "key", Type: ref("string"), Comment: "the row key"}},
			Returns: []*emit.Return{{Type: ref("string"), Comment: "the row"}},
		})
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"export interface Store {\n"+
				"  get(key: string /* the row key */): string /* the row */; // by key\n"+
				"} // read side\n",
			"a signature's comments spell as block comments, the method's and the interface's close their lines")
		sort := &emit.Function{Name: "sort", Comment: "stable"}
		assert.Equal(t, execute(t, backend.FunctionTemplate, sort),
			"export function sort(): void {\n    body();\n} // stable\n",
			"a function's comment follows its closing brace")
		assert.Equal(t,
			execute(t, backend.AliasTemplate, &emit.Alias{Name: "ID", Target: ref("string"), Comment: "opaque"}),
			"export type ID = string; // opaque\n", "an alias's comment closes its line")
		e := &emit.Enum{Name: "Phase", Comment: "closed set"}
		e.Variants.Append(&emit.EnumVariant{Name: "Open"})
		assert.Equal(t, execute(t, backend.EnumTemplate, e),
			"export enum Phase {\n  Open,\n} // closed set\n", "and an enum's closes its brace")
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
			Annotations: symbol.Annotations{{Name: "injectable"}},
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
				"  override async load(): Promise<Row> {\n    body();\n  }\n"+
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
			"export async function load(): Promise<Row> {\n    body();\n}\n",
			"async behind export, the result a promise")
		assert.Equal(t, execute(t, backend.FunctionTemplate, &emit.Function{Name: "flush", Async: true}),
			"export async function flush(): Promise<void> {\n    body();\n}\n",
			"an async callable returning nothing promises void")

		v := &emit.Variable{
			Name: "max", Mutability: symbol.MutabilityImmutable,
			Type: ref("number"), Value: "8",
		}
		assert.Equal(t, execute(t, backend.VariableTemplate, v),
			"export const max: number = 8;\n",
			"an immutable binding is const with its initializer")
	})

	t.Run("accessors, hard names and signatures", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Name: "Cache"}
		s.Fields.Append(&emit.Field{Name: "store", Hard: true, Type: ref("Map")})
		s.Methods.Append(
			&emit.Method{
				Name: "size", Accessor: symbol.AccessorGet,
				Returns: []*emit.Return{{Type: ref("number")}},
			},
			&emit.Method{
				Name: "size", Accessor: symbol.AccessorSet,
				Params: []*emit.Param{{Name: "n", Type: ref("number")}},
			},
			&emit.Method{
				Name:    "index",
				Indexer: true,
				Params:  []*emit.Param{{Name: "key", Type: ref("string")}},
				Returns: []*emit.Return{{Type: ref("Row")}},
			},
		)
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"export class Cache {\n"+
				"  #store: Map;\n"+
				"  get size(): number {\n"+
				"    body();\n"+
				"  }\n"+
				"  set size(n: number) {\n"+
				"    body();\n"+
				"  }\n"+
				"  [key: string]: Row;\n"+
				"}\n",
			"the hard prefix on the name, the accessor keyword before it, "+
				"a setter without an annotation, the index signature whole")

		i := &emit.Interface{Name: "Rows"}
		i.Methods.Append(
			&emit.Method{
				Name:    "index",
				Indexer: true,
				Params:  []*emit.Param{{Name: "key", Type: ref("string")}},
				Returns: []*emit.Return{{Type: ref("Row")}},
			},
			&emit.Method{
				Name:       "make",
				Constructs: true,
				Returns:    []*emit.Return{{Type: ref("Rows")}},
			},
		)
		assert.Equal(t, execute(t, backend.InterfaceTemplate, i),
			"export interface Rows {\n"+
				"  [key: string]: Row;\n"+
				"  new (): Rows;\n"+
				"}\n",
			"index and construct signatures spell nameless")

		for _, m := range []*emit.Method{
			{
				Name: "index", Indexer: true, Async: true,
				Params:  []*emit.Param{{Name: "key", Type: ref("string")}},
				Returns: []*emit.Return{{Type: ref("Row")}},
			},
			{Name: "make", Constructs: true, Level: symbol.LevelType},
		} {
			guarded := &emit.Interface{Name: "Rows"}
			guarded.Methods.Append(m)
			_, err := run(t, backend.InterfaceTemplate, guarded)
			assert.HasError(t, err,
				"a modifier on an interface's index or construct signature refuses")
		}
	})

	t.Run("a class index signature takes static alone", func(t *testing.T) {
		t.Parallel()

		index := func() *emit.Method {
			return &emit.Method{
				Name: "index", Indexer: true,
				Params:  []*emit.Param{{Name: "key", Type: ref("string")}},
				Returns: []*emit.Return{{Type: ref("Row")}},
			}
		}
		s := &emit.Struct{Name: "Cache"}
		static := index()
		static.Level = symbol.LevelType
		s.Methods.Append(static)
		assert.Equal(t, execute(t, backend.StructTemplate, s),
			"export class Cache {\n  static [key: string]: Row;\n}\n",
			"static before the brackets")

		narrowed := &emit.Struct{Name: "Cache"}
		private := index()
		private.Visibility = symbol.VisibilityPrivate
		narrowed.Methods.Append(private)
		_, err := run(t, backend.StructTemplate, narrowed)
		assert.HasError(t, err, "an accessibility on an index signature refuses")
	})

	t.Run("a constructing method spells the constructor form", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Name: "Store"}
		s.Methods.Append(&emit.Method{
			Name: "make", Constructs: true, Visibility: symbol.VisibilityProtected,
			Params: []*emit.Param{{Name: "db", Type: ref("Db")}},
		})
		got := execute(t, backend.StructTemplate, s)
		assert.Contains(t, got, "protected constructor(db: Db) {",
			"its accessibility before the keyword, the name and results dropped, because "+
				"the language grants neither")

		for _, m := range []*emit.Method{
			{Name: "make", Constructs: true, Level: symbol.LevelType},
			{Name: "make", Constructs: true, Async: true},
			{Name: "make", Constructs: true, TypeParams: []*emit.TypeParam{{Name: "T"}}},
		} {
			refused := &emit.Struct{Name: "Store"}
			refused.Methods.Append(m)
			_, err := run(t, backend.StructTemplate, refused)
			assert.HasError(t, err,
				"static, async and type parameters on a constructor refuse")
		}
	})

	t.Run("a quoted method key keeps its wire spelling", func(t *testing.T) {
		t.Parallel()

		s := &emit.Struct{Name: "Client"}
		s.Methods.Append(&emit.Method{Name: "do-fetch"})
		got := execute(t, backend.StructTemplate, s)
		assert.Contains(t, got, `'do-fetch'()`,
			"TypeScript admits the quoted member, so the wire name is kept")
	})

	t.Run("const enum", func(t *testing.T) {
		t.Parallel()

		e := &emit.Enum{Name: "Phase", Const: true}
		e.Variants.Append(&emit.EnumVariant{Name: "Active", Comment: "the default"})
		assert.Equal(t, execute(t, backend.EnumTemplate, e),
			"export const enum Phase {\n"+
				"  Active, // the default\n"+
				"}\n",
			"the const keyword before enum, the trailing comment behind the comma")
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
		_, err := run(t, backend.EnumTemplate, withMembers)
		assert.HasError(t, err, "an enum with members refuses")
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
