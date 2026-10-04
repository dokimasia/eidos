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
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Each stubbed builtin writes its own marker, so a template's own
// bytes are distinguishable from the pass's.
const (
	bodyStub    = "    body();\n"
	importsStub = "IMPORTS\n"
	declsStub   = "DECLS\n"
)

// templateMapAllocs is a map of kinds onto templates or reasons: the
// map and its one group.
const templateMapAllocs = 2

// Each kind template is pinned byte for byte, and so is the method
// kind's refusal: members render inside their host.
func TestTemplates(t *testing.T) {
	t.Parallel()

	kind := func(k symbol.Kind) string { return backend.KindTemplates()[k] }

	t.Run("KindTemplates", func(t *testing.T) {
		t.Parallel()

		t.Run("returns no template for a method", func(t *testing.T) {
			t.Parallel()

			_, is := backend.KindTemplates()[symbol.KindMethod]
			assert.False(t, is, "TypeScript states members inside their type")
		})

		t.Run("writes a class's members at member depth", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Doc: []string{"Row is one record."}, Name: rowName}
			s.Fields.Append(&emit.Field{Doc: []string{"Key addresses the row."}, Name: "key", Type: ref("string")})
			s.Methods.Append(&emit.Method{Name: "load", Returns: []*emit.Return{{Type: ref(rowName)}}})
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"/**\n * Row is one record.\n */\n"+
					"export class Row {\n"+
					"  /**\n   * Key addresses the row.\n   */\n"+
					"  key: string;\n"+
					"  load(): Row {\n"+bodyStub+"  }\n"+
					"}\n",
				"docs indented whole")
		})

		t.Run("binds the import of a field's type", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Holder"}
			s.Fields.Append(&emit.Field{Name: "row", Type: imported(storeModule, rowName)})
			out, set, err := run(t, kind(symbol.KindStruct), s)
			assert.NoError(t, err, "the template executes")
			assert.Contains(t, out, "  row: Row;\n", "the imported name")
			assert.Equal(t, set.Paths(), []string{storeModule}, "the import of its module")
		})

		t.Run("writes an interface's methods as signatures", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store"}
			i.Methods.Append(&emit.Method{
				Name: "load", Params: []*emit.Param{{Name: "key", Type: ref("string")}},
				Returns: []*emit.Return{{Type: ref(rowName)}},
			})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"export interface Store {\n  load(key: string): Row;\n}\n",
				"a semicolon in place of a body")
		})

		t.Run("writes a function around its body", func(t *testing.T) {
			t.Parallel()

			f := &emit.Function{Name: "load", Returns: []*emit.Return{{Type: ref(rowName)}}}
			assert.Equal(t, execute(t, kind(symbol.KindFunction), f),
				"export function load(): Row {\n"+bodyStub+"}\n", "the function shape")
		})

		t.Run("writes an alias", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, execute(t, kind(symbol.KindAlias), &emit.Alias{Name: "ID", Target: ref("string")}),
				"export type ID = string;\n", "the alias shape")
		})

		t.Run("writes a typed constant", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t,
				execute(t, kind(symbol.KindConstant), &emit.Constant{Name: "MAX", Type: ref("number"), Value: "10"}),
				"export const MAX: number = 10;\n", "the constant shape")
		})

		t.Run("writes a constant's trailing comment behind the semicolon", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t,
				execute(t, kind(symbol.KindConstant), &emit.Constant{
					Name: "MAX", Type: ref("number"), Value: "10", Comment: "rows per call",
				}),
				"export const MAX: number = 10; // rows per call\n", "on the value's line")
		})

		t.Run("writes an untyped binding untyped", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, execute(t, kind(symbol.KindVariable), &emit.Variable{Name: "count"}),
				"export let count;\n", "no annotation")
		})

		t.Run("returns an error for a const without an initializer", func(t *testing.T) {
			t.Parallel()

			err := refused(t, kind(symbol.KindVariable), &emit.Variable{
				Name: "count", Mutability: symbol.MutabilityImmutable, Type: ref("number"),
			})
			assert.HasError(t, err, "const count: number; does not compile")
		})

		t.Run("writes an interface method's comments as block comments", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store", Comment: "read side"}
			i.Methods.Append(&emit.Method{
				Name: "get", Comment: "by key",
				Params:  []*emit.Param{{Name: "key", Type: ref("string"), Comment: "the row key"}},
				Returns: []*emit.Return{{Type: ref("string"), Comment: "the row"}},
			})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"export interface Store {\n"+
					"  get(key: string /* the row key */): string /* the row */; // by key\n"+
					"} // read side\n",
				"the method's comments inside its line")
		})

		t.Run("writes a function's trailing comment after its closing brace", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, execute(t, kind(symbol.KindFunction), &emit.Function{Name: "sort", Comment: "stable"}),
				"export function sort(): void {\n"+bodyStub+"} // stable\n", "on the brace's line")
		})

		t.Run("writes an alias's trailing comment on its line", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t,
				execute(t, kind(symbol.KindAlias), &emit.Alias{Name: "ID", Target: ref("string"), Comment: "opaque"}),
				"export type ID = string; // opaque\n", "behind the semicolon")
		})

		t.Run("writes an enum's trailing comment after its brace", func(t *testing.T) {
			t.Parallel()

			e := &emit.Enum{Name: "Phase", Comment: "closed set"}
			e.Variants.Append(&emit.EnumVariant{Name: "Open"})
			assert.Equal(t, execute(t, kind(symbol.KindEnum), e),
				"export enum Phase {\n  Open,\n} // closed set\n", "on the brace's line")
		})

		t.Run("writes a generic class's parameters behind its name", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Box", TypeParams: []*emit.TypeParam{{Name: "T"}}}
			s.Fields.Append(&emit.Field{Name: "item", Type: ref("T")})
			s.Methods.Append(&emit.Method{
				Name:       "map",
				TypeParams: []*emit.TypeParam{{Name: "U", Bounds: []*emit.TypeRef{ref("Codec")}}},
				Params:     []*emit.Param{{Name: "item", Type: ref("U")}},
				Returns:    []*emit.Return{{Type: ref("U")}},
			})
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"export class Box<T> {\n"+
					"  item: T;\n"+
					"  map<U extends Codec>(item: U): U {\n"+bodyStub+"  }\n"+
					"}\n",
				"the member's list behind its own name")
		})

		t.Run("writes a generic interface's parameters behind its name", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{
				Name:       "Keyed",
				TypeParams: []*emit.TypeParam{{Name: "K", Bounds: []*emit.TypeRef{ref("Codec")}}},
			}
			i.Methods.Append(&emit.Method{
				Name: "pick", Params: []*emit.Param{{Name: "key", Type: ref("K")}},
				Returns: []*emit.Return{{Type: ref("K")}},
			})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"export interface Keyed<K extends Codec> {\n  pick(key: K): K;\n}\n", "the bound behind extends")
		})

		t.Run("writes a generic function's parameters behind its name", func(t *testing.T) {
			t.Parallel()

			f := &emit.Function{
				Name:       "sort",
				TypeParams: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{ref("Codec")}}},
				Params:     []*emit.Param{{Name: "items", Type: ref("T")}},
				Returns:    []*emit.Return{{Type: ref("T")}},
			}
			assert.Equal(t, execute(t, kind(symbol.KindFunction), f),
				"export function sort<T extends Codec>(items: T): T {\n"+bodyStub+"}\n", "the bound behind extends")
		})

		t.Run("writes a generic alias's parameters behind its name", func(t *testing.T) {
			t.Parallel()

			a := &emit.Alias{
				Name:       "Match",
				TypeParams: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{ref("Codec")}}},
				Target:     &emit.TypeRef{Spelling: "Keyed", Args: []*emit.TypeRef{ref("T")}},
			}
			assert.Equal(t, execute(t, kind(symbol.KindAlias), a),
				"export type Match<T extends Codec> = Keyed<T>;\n", "the bound behind extends")
		})

		t.Run("writes a class's heritage behind its name", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{
				Name:       rowName,
				Extends:    []*emit.TypeRef{ref(baseName)},
				Implements: []*emit.TypeRef{ref("Keyed")},
			}
			s.Fields.Append(&emit.Field{Name: "key", Type: ref("string")})
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"export class Row extends Base implements Keyed {\n  key: string;\n}\n", "the clauses")
		})

		t.Run("writes an interface's widened contracts behind extends", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store", Extends: []*emit.TypeRef{ref("Keyed"), ref("Closer")}}
			i.Methods.Append(&emit.Method{Name: "get", Returns: []*emit.Return{{Type: ref("string")}}})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"export interface Store extends Keyed, Closer {\n  get(): string;\n}\n", "comma-joined")
		})

		t.Run("writes a class's modifiers in TypeScript's stated order", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: rowName, Abstract: true, Annotations: symbol.Annotations{{Name: "injectable"}}}
			s.Fields.Append(&emit.Field{
				Name: "key", Visibility: symbol.VisibilityPrivate, Level: symbol.LevelType,
				Mutability: symbol.MutabilityImmutable, Type: ref("string"), Value: `"r"`,
			})
			s.Methods.Append(
				&emit.Method{Name: "load", Async: true, Override: true, Returns: []*emit.Return{{Type: ref(rowName)}}},
				&emit.Method{Name: "pick", Abstract: true, Returns: []*emit.Return{{Type: ref(rowName)}}},
			)
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"@injectable\n"+
					"export abstract class Row {\n"+
					"  private static readonly key: string = \"r\";\n"+
					"  override async load(): Promise<Row> {\n"+bodyStub+"  }\n"+
					"  abstract pick(): Row;\n"+
					"}\n",
				"the decorator above the class, the abstract method a signature alone")
		})

		t.Run("writes readonly on an interface property", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Store"}
			i.Fields.Append(&emit.Field{Name: "kind", Mutability: symbol.MutabilityImmutable, Type: ref("string")})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"export interface Store {\n  readonly kind: string;\n}\n", "nothing else")
		})

		t.Run("writes an async function's result as a promise", func(t *testing.T) {
			t.Parallel()

			f := &emit.Function{Name: "load", Async: true, Returns: []*emit.Return{{Type: ref(rowName)}}}
			assert.Equal(t, execute(t, kind(symbol.KindFunction), f),
				"export async function load(): Promise<Row> {\n"+bodyStub+"}\n", "async behind export")
		})

		t.Run("writes an async function returning nothing as a promise of void", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, execute(t, kind(symbol.KindFunction), &emit.Function{Name: "flush", Async: true}),
				"export async function flush(): Promise<void> {\n"+bodyStub+"}\n", "void inside the promise")
		})

		t.Run("writes an immutable binding as const with its initializer", func(t *testing.T) {
			t.Parallel()

			v := &emit.Variable{Name: "max", Mutability: symbol.MutabilityImmutable, Type: ref("number"), Value: "8"}
			assert.Equal(t, execute(t, kind(symbol.KindVariable), v),
				"export const max: number = 8;\n", "the const shape")
		})

		t.Run("writes a hard-private field behind its prefix", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Cache"}
			s.Fields.Append(&emit.Field{Name: "store", Hard: true, Type: ref("Map")})
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"export class Cache {\n  #store: Map;\n}\n", "the privacy in the name")
		})

		t.Run("writes a getter behind its keyword", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Cache"}
			s.Methods.Append(&emit.Method{
				Name: "size", Accessor: symbol.AccessorGet, Returns: []*emit.Return{{Type: ref("number")}},
			})
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"export class Cache {\n  get size(): number {\n"+bodyStub+"  }\n}\n", "the getter shape")
		})

		t.Run("writes a setter without an annotation", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Cache"}
			s.Methods.Append(&emit.Method{
				Name: "size", Accessor: symbol.AccessorSet, Params: []*emit.Param{{Name: "n", Type: ref("number")}},
			})
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"export class Cache {\n  set size(n: number) {\n"+bodyStub+"  }\n}\n", "the setter shape")
		})

		t.Run("writes a class's index signature whole", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Cache"}
			s.Methods.Append(indexer())
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"export class Cache {\n  [key: string]: Row;\n}\n", "nameless")
		})

		t.Run("writes static before a class index signature's brackets", func(t *testing.T) {
			t.Parallel()

			static := indexer()
			static.Level = symbol.LevelType
			s := &emit.Struct{Name: "Cache"}
			s.Methods.Append(static)
			assert.Equal(t, execute(t, kind(symbol.KindStruct), s),
				"export class Cache {\n  static [key: string]: Row;\n}\n", "the one keyword it takes")
		})

		t.Run("returns an error for an accessibility on a class index signature", func(t *testing.T) {
			t.Parallel()

			private := indexer()
			private.Visibility = symbol.VisibilityPrivate
			s := &emit.Struct{Name: "Cache"}
			s.Methods.Append(private)
			assert.HasError(t, refused(t, kind(symbol.KindStruct), s), "an index signature takes static alone")
		})

		t.Run("writes an interface's index signature nameless", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Rows"}
			i.Methods.Append(indexer())
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"export interface Rows {\n  [key: string]: Row;\n}\n", "the key and element typed")
		})

		t.Run("writes an interface's construct signature nameless", func(t *testing.T) {
			t.Parallel()

			i := &emit.Interface{Name: "Rows"}
			i.Methods.Append(&emit.Method{Name: "make", Constructs: true, Returns: []*emit.Return{{Type: ref("Rows")}}})
			assert.Equal(t, execute(t, kind(symbol.KindInterface), i),
				"export interface Rows {\n  new (): Rows;\n}\n", "behind new")
		})

		asyncIndex := indexer()
		asyncIndex.Async = true
		signatures := []struct {
			name string
			give *emit.Method
		}{
			{name: "returns an error for an async index signature on an interface", give: asyncIndex},
			{
				name: "returns an error for a type-level construct signature on an interface",
				give: &emit.Method{Name: "make", Constructs: true, Level: symbol.LevelType},
			},
		}
		for _, tt := range signatures {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				i := &emit.Interface{Name: "Rows"}
				i.Methods.Append(tt.give)
				assert.HasError(t, refused(t, kind(symbol.KindInterface), i), "the signature is bare")
			})
		}

		t.Run("writes a constructing method as the constructor form", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Store"}
			s.Methods.Append(&emit.Method{
				Name: "make", Constructs: true, Visibility: symbol.VisibilityProtected,
				Params: []*emit.Param{{Name: "db", Type: ref("Db")}},
			})
			assert.Contains(t, execute(t, kind(symbol.KindStruct), s), "protected constructor(db: Db) {",
				"the name and results dropped, because the language grants neither")
		})

		constructors := []struct {
			name string
			give *emit.Method
		}{
			{
				name: "returns an error for a type-level constructor",
				give: &emit.Method{Name: "make", Constructs: true, Level: symbol.LevelType},
			},
			{
				name: "returns an error for an async constructor",
				give: &emit.Method{Name: "make", Constructs: true, Async: true},
			},
			{
				name: "returns an error for a constructor with type parameters",
				give: &emit.Method{Name: "make", Constructs: true, TypeParams: []*emit.TypeParam{{Name: "T"}}},
			},
		}
		for _, tt := range constructors {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s := &emit.Struct{Name: "Store"}
				s.Methods.Append(tt.give)
				assert.HasError(t, refused(t, kind(symbol.KindStruct), s), "a constructor cannot state it")
			})
		}

		t.Run("writes a method key that is no identifier quoted", func(t *testing.T) {
			t.Parallel()

			s := &emit.Struct{Name: "Client"}
			s.Methods.Append(&emit.Method{Name: "do-fetch"})
			assert.Contains(t, execute(t, kind(symbol.KindStruct), s), `'do-fetch'()`, "the wire name is kept")
		})

		t.Run("writes const before a const enum", func(t *testing.T) {
			t.Parallel()

			e := &emit.Enum{Name: "Phase", Const: true}
			e.Variants.Append(&emit.EnumVariant{Name: "Active", Comment: "the default"})
			assert.Equal(t, execute(t, kind(symbol.KindEnum), e),
				"export const enum Phase {\n  Active, // the default\n}\n", "the trailing comment behind the comma")
		})

		t.Run("writes one enum member per variant", func(t *testing.T) {
			t.Parallel()

			e := &emit.Enum{Doc: []string{"Phase names a step."}, Name: "Phase"}
			e.Variants.Append(
				&emit.EnumVariant{Doc: []string{"Open admits writes."}, Name: "Open"},
				&emit.EnumVariant{Name: "Closed", Value: "9"},
			)
			assert.Equal(t, execute(t, kind(symbol.KindEnum), e),
				"/**\n * Phase names a step.\n */\n"+
					"export enum Phase {\n"+
					"  /**\n   * Open admits writes.\n   */\n"+
					"  Open,\n"+
					"  Closed = 9,\n"+
					"}\n",
				"a stated value behind equals")
		})

		t.Run("returns an error for an enum with members", func(t *testing.T) {
			t.Parallel()

			e := &emit.Enum{Name: "Phase"}
			e.Methods.Append(&emit.Method{Name: "describe"})
			assert.HasError(t, refused(t, kind(symbol.KindEnum), e), "an enum has values alone")
		})
	})

	t.Run("FileTemplate", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the imports then the declarations", func(t *testing.T) {
			t.Parallel()

			got := execute(t, backend.FileTemplate, struct {
				Name string
				Pkg  symbol.Identity
			}{Name: "store.stub.ts"})
			assert.Equal(t, got, importsStub+declsStub, "no package clause, because the file is the module")
		})
	})

	t.Run("RefusedKinds", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the method kind alone", func(t *testing.T) {
			t.Parallel()

			refused := backend.RefusedKinds()
			assert.Length(t, refused, 1, "one kind TypeScript cannot spell at module level")
			assert.NotEqual(t, refused[symbol.KindMethod], "", "the method, with its reason")
		})

		t.Run("returns no kind the templates spell", func(t *testing.T) {
			t.Parallel()

			for k := range backend.RefusedKinds() {
				_, spelt := backend.KindTemplates()[k]
				assert.False(t, spelt, "a kind is spelt or refused: "+k.String())
			}
		})
	})
}

// A map of kinds allocates itself. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestTemplatesAllocs(t *testing.T) {
	checkAllocs(t, templatesCalls())
}

// BenchmarkTemplates measures the maps the backend reads once per
// build.
func BenchmarkTemplates(b *testing.B) {
	benchCalls(b, templatesCalls())
}

// templatesCalls returns a call of KindTemplates and of RefusedKinds.
func templatesCalls() []allocCall {
	var kinds map[symbol.Kind]string
	return []allocCall{
		{
			name: "KindTemplates", allocs: templateMapAllocs,
			call:  func() { kinds = backend.KindTemplates() },
			check: func(tb assert.TB) { assert.Length(tb, kinds, 7, "KindTemplates returns seven templates") },
		},
		{
			name: "RefusedKinds", allocs: templateMapAllocs,
			call:  func() { kinds = backend.RefusedKinds() },
			check: func(tb assert.TB) { assert.Length(tb, kinds, 1, "RefusedKinds returns the method") },
		},
	}
}

// run runs one template over one declaration the way the render pass
// does, the builtins stubbed, and returns the text or the refusal
// beside the file's import set.
func run(t *testing.T, src string, data any) (string, *render.ImportSet, error) {
	t.Helper()

	set := &render.ImportSet{}
	tmpl, err := template.New("kind").
		Funcs(backend.Funcs(set)).
		Funcs(template.FuncMap{
			render.BuiltinBody:    func(any) string { return bodyStub },
			render.BuiltinUse:     func(string) string { return "" },
			render.BuiltinImports: func() string { return importsStub },
			render.BuiltinDecls:   func() string { return declsStub },
			render.BuiltinSlots:   func() string { return "" },
			render.BuiltinSlot:    func(string) string { return "" },
		}).
		Parse(src)
	assert.NoError(t, err, "the template parses")
	var b strings.Builder
	err = tmpl.Execute(&b, data)
	return b.String(), set, err
}

// execute runs one template over one declaration and asserts it
// executes.
func execute(t *testing.T, src string, data any) string {
	t.Helper()

	got, _, err := run(t, src, data)
	assert.NoError(t, err, "the template executes")
	return got
}

// refused runs one template over one declaration and returns the
// refusal.
func refused(t *testing.T, src string, data any) error {
	t.Helper()

	_, _, err := run(t, src, data)
	return err
}

// indexer returns an index signature of a string key onto a row.
func indexer() *emit.Method {
	return &emit.Method{
		Name: "index", Indexer: true,
		Params:  []*emit.Param{{Name: "key", Type: ref("string")}},
		Returns: []*emit.Return{{Type: ref(rowName)}},
	}
}
