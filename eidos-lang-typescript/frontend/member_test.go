// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The class and the interface the member cases declare.
const (
	className = "C"
	ifaceName = "I"
)

// A member's modifiers each become a fact of the model, so the mapping
// of every class and interface member is pinned.
func TestMember(t *testing.T) {
	t.Parallel()

	t.Run("classBody", func(t *testing.T) {
		t.Parallel()

		t.Run("leaves out a method's implementation after its overload signatures", func(t *testing.T) {
			t.Parallel()

			c := classOf(t, "  fill(a: string): void;\n  fill(a: number): void;\n  fill(a: any): void {}\n")
			assert.Equal(t, methodNames(c.Methods), []string{"fill", "fill"}, "the two signatures callers see")
		})

		t.Run("applies a decorator to the member after it", func(t *testing.T) {
			t.Parallel()

			c := classOf(t, "  @log\n  run(): void {}\n")
			assert.Equal(t, c.Methods[0].Annotations, symbol.Annotations{{Name: "log"}}, "the decorator's name")
		})

		t.Run("lowers an index signature as a method named [] that takes the key and returns the value",
			func(t *testing.T) {
				t.Parallel()

				m := classOf(t, "  [key: string]: number;\n").Methods[0]
				assert.Equal(t, m.Name, "[]", "the indexer's name")
				assert.True(t, m.Indexer, "an index signature")
				assert.Equal(t, m.Params[0].Type.Spelling, "string", "the key")
				assert.Equal(t, m.Returns[0].Type.Spelling, "number", "the value")
			})

		t.Run("reports UnaddressedCarrier for a carrier on an overloaded method's implementation", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export class C {\n  fill(a: string): void;\n  "+carrierLine+
				"  fill(a: any): void {}\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier},
				"the implementation declares nothing")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a static block", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export class C {\n  // +fixture:gen:table name=t\n  static {}\n}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "a static block declares nothing")
		})
	})

	t.Run("method", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a constructor's parameter properties as fields", func(t *testing.T) {
			t.Parallel()

			c := classOf(t, "  constructor(private a: string, readonly b: number, c: boolean) {}\n")
			assert.Equal(t, fieldNames(c.Fields), []string{"a", "b"}, "a bare parameter declares no field")
			assert.Equal(t, c.Fields[0].Visibility, symbol.VisibilityPrivate, "the modifier's accessibility")
			assert.Equal(t, c.Fields[1].Mutability, symbol.MutabilityImmutable, "readonly is immutable")
		})

		t.Run("skips a comment among a constructor's parameters", func(t *testing.T) {
			t.Parallel()

			c := classOf(t, "  constructor(private a: string, /* and */ readonly b: number) {}\n")
			assert.Equal(t, fieldNames(c.Fields), []string{"a", "b"}, "a comment declares no field")
		})

		t.Run("leaves out a private parameter property at signature depth", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, fstest.MapFS{aFile: {Data: []byte(
				"export class C {\n  constructor(private a: string, readonly b: number) {}\n}\n",
			)}}, aFile, plugin.DepthSignatures)
			c := named[*node.Struct](t, fileIn(t, gb, aPackage).Decls, className)
			assert.Equal(t, fieldNames(c.Fields), []string{"b"}, "no other module can read a")
		})

		t.Run("stamps typescript.parameterProperty on a parameter property's field", func(t *testing.T) {
			t.Parallel()

			gb, c := classBuilt(t, "  constructor(private a: string, b: number) {}\n")
			v, stamped := stampOn(gb, c.Fields[0], typescript.ParameterPropertyKey)
			assert.True(t, stamped, "the field a parameter declares")
			assert.Equal(t, v, any(true), "is marked")
		})

		t.Run("stamps typescript.parameterProperty on a parameter property's parameter", func(t *testing.T) {
			t.Parallel()

			gb, c := classBuilt(t, "  constructor(private a: string, b: number) {}\n")
			_, stamped := stampOn(gb, c.Methods[0].Params[0], typescript.ParameterPropertyKey)
			assert.True(t, stamped, "the parameter that declares a field")
		})

		t.Run("stamps no typescript.parameterProperty on a bare constructor parameter", func(t *testing.T) {
			t.Parallel()

			gb, c := classBuilt(t, "  constructor(private a: string, b: number) {}\n")
			_, stamped := stampOn(gb, c.Methods[0].Params[1], typescript.ParameterPropertyKey)
			assert.False(t, stamped, "b declares no field")
		})

		t.Run("stamps a private parameter property's parameter at signature depth", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, fstest.MapFS{aFile: {Data: []byte(
				"export class C {\n  constructor(private a: string) {}\n}\n",
			)}}, aFile, plugin.DepthSignatures)
			c := named[*node.Struct](t, fileIn(t, gb, aPackage).Decls, className)
			_, stamped := stampOn(gb, c.Methods[0].Params[0], typescript.ParameterPropertyKey)
			assert.True(t, stamped, "the constructor takes the parameter whatever field the depth keeps")
		})
	})

	t.Run("methodOf", func(t *testing.T) {
		t.Parallel()

		t.Run("marks a method named constructor as constructing", func(t *testing.T) {
			t.Parallel()

			assert.True(t, classOf(t, "  constructor() {}\n").Methods[0].Constructs, "the class's constructor")
		})

		t.Run("lowers a static method at the type level", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, classOf(t, "  static make(): C { return new C(); }\n").Methods[0].Level,
				symbol.LevelType, "static belongs to the class")
		})

		t.Run("marks an async method", func(t *testing.T) {
			t.Parallel()

			assert.True(t, classOf(t, "  async run(): Promise<void> {}\n").Methods[0].Async, "async is a fact")
		})

		t.Run("marks a getter and a setter by their accessor", func(t *testing.T) {
			t.Parallel()

			c := classOf(t, "  get v(): number { return 1; }\n  set v(x: number) {}\n")
			assert.Equal(t, c.Methods[0].Accessor, symbol.AccessorGet, "get reads the property")
			assert.Equal(t, c.Methods[1].Accessor, symbol.AccessorSet, "set writes it")
		})

		t.Run("marks an abstract method", func(t *testing.T) {
			t.Parallel()

			src := "export abstract class C {\n  abstract run(): void;\n}\n"
			c := named[*node.Struct](t, declsOf(t, src), className)
			assert.True(t, c.Methods[0].Abstract, "a subclass supplies it")
		})

		t.Run("marks an override", func(t *testing.T) {
			t.Parallel()

			assert.True(t, classOf(t, "  override run(): void {}\n").Methods[0].Override, "override is a keyword")
		})

		t.Run("lowers a # method as hard and private", func(t *testing.T) {
			t.Parallel()

			m := classOf(t, "  #run(): void {}\n").Methods[0]
			assert.True(t, m.Hard, "the runtime enforces the privacy")
			assert.Equal(t, m.Visibility, symbol.VisibilityPrivate, "and it is private")
		})

		t.Run("lowers a this parameter as the method's receiver", func(t *testing.T) {
			t.Parallel()

			m := classOf(t, "  run(this: C, a: string): void {}\n").Methods[0]
			assert.Equal(t, m.Receiver.Type.Spelling, className, "this types the receiver")
			assert.Length(t, m.Params, 1, "and is no argument")
		})

		t.Run("leaves out a private method at signature depth", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, fstest.MapFS{aFile: {Data: []byte(
				"export class C {\n  private hidden(): void {}\n  shown(): void {}\n}\n",
			)}}, aFile, plugin.DepthSignatures)
			c := named[*node.Struct](t, fileIn(t, gb, aPackage).Decls, className)
			assert.Equal(t, methodNames(c.Methods), []string{"shown"}, "no other module can call it")
		})

		t.Run("stamps typescript.generator on a generator method", func(t *testing.T) {
			t.Parallel()

			gb, c := classBuilt(t, "  *items(): Iterable<number> {}\n")
			_, stamped := stampOn(gb, c.Methods[0], typescript.GeneratorKey)
			assert.True(t, stamped, "the * declares a generator")
		})

		t.Run("stamps typescript.optional on a method declared with ?", func(t *testing.T) {
			t.Parallel()

			gb, i := ifaceBuilt(t, "  run?(): void;\n")
			_, stamped := stampOn(gb, i.Methods[0], typescript.OptionalKey)
			assert.True(t, stamped, "a value of the type may lack the method")
		})

		t.Run("stamps nothing on a method without * or ?", func(t *testing.T) {
			t.Parallel()

			gb, _ := classBuilt(t, "  run(): void {}\n")
			assert.Empty(t, gb.StampRecords(), "a plain method has no mark to record")
		})
	})

	t.Run("field", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a property's modifiers", func(t *testing.T) {
			t.Parallel()

			f := classOf(t, "  protected static readonly count?: number = 1;\n").Fields[0]
			assert.Equal(t, f.Visibility, symbol.VisibilityProtected, "protected")
			assert.Equal(t, f.Level, symbol.LevelType, "static")
			assert.Equal(t, f.Mutability, symbol.MutabilityImmutable, "readonly")
			assert.True(t, f.Optional, "the ? mark")
			assert.Equal(t, f.Value, "1", "the initializer verbatim")
		})

		t.Run("lowers a property's decorators as annotations", func(t *testing.T) {
			t.Parallel()

			f := classOf(t, "  @column('id') id: string;\n").Fields[0]
			assert.Equal(t, f.Annotations, symbol.Annotations{{Name: "column", Args: []string{"'id'"}}},
				"the decorator's name and its argument verbatim")
		})

		t.Run("leaves out a # property at signature depth", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, fstest.MapFS{aFile: {Data: []byte(
				"export class C {\n  #hidden = 1;\n  shown = 2;\n}\n",
			)}}, aFile, plugin.DepthSignatures)
			c := named[*node.Struct](t, fileIn(t, gb, aPackage).Decls, className)
			assert.Equal(t, fieldNames(c.Fields), []string{"shown"}, "no other module can read it")
		})

		t.Run("stamps typescript.definiteAssignment on a property declared with !", func(t *testing.T) {
			t.Parallel()

			gb, c := classBuilt(t, "  id!: string;\n")
			_, stamped := stampOn(gb, c.Fields[0], typescript.DefiniteAssignmentKey)
			assert.True(t, stamped, "the class assigns it where the compiler cannot see")
		})

		t.Run("stamps nothing on a property without !", func(t *testing.T) {
			t.Parallel()

			gb, _ := classBuilt(t, "  id: string;\n")
			assert.Empty(t, gb.StampRecords(), "a plain property has no mark to record")
		})
	})

	t.Run("indexer", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps typescript.readonly on a readonly index signature", func(t *testing.T) {
			t.Parallel()

			gb, i := ifaceBuilt(t, "  readonly [key: string]: number;\n")
			_, stamped := stampOn(gb, i.Methods[0], typescript.ReadonlyKey)
			assert.True(t, stamped, "a holder cannot assign the entries")
		})

		t.Run("stamps nothing on an index signature without readonly", func(t *testing.T) {
			t.Parallel()

			gb, _ := ifaceBuilt(t, "  [key: string]: number;\n")
			assert.Empty(t, gb.StampRecords(), "a plain index signature has no mark to record")
		})
	})

	t.Run("objectMembers", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an interface's property and method signatures", func(t *testing.T) {
			t.Parallel()

			i := ifaceOf(t, "  readonly name?: string;\n  run(a: number): void;\n")
			assert.Equal(t, fieldNames(i.Fields), []string{"name"}, "the property is a field")
			assert.True(t, i.Fields[0].Optional, "optional")
			assert.Equal(t, i.Fields[0].Mutability, symbol.MutabilityImmutable, "and readonly")
			assert.True(t, i.Methods[0].Abstract, "an interface's method has no body")
		})

		t.Run("lowers an interface's index signature as a method named []", func(t *testing.T) {
			t.Parallel()

			m := ifaceOf(t, "  [index: number]: string;\n").Methods[0]
			assert.Equal(t, m.Name, "[]", "the indexer's name")
			assert.True(t, m.Indexer, "an index signature")
		})

		t.Run("declares nothing for a mapped signature in an interface", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, ifaceOf(t, "  [K in Keys]: string;\n").Methods, "a mapped signature names no key type")
		})

		t.Run("lowers a construct signature as a method named new that constructs", func(t *testing.T) {
			t.Parallel()

			m := ifaceOf(t, "  new (a: number): I;\n").Methods[0]
			assert.Equal(t, m.Name, "new", "the construct signature's name")
			assert.True(t, m.Constructs, "it constructs")
			assert.Equal(t, m.Returns[0].Type.Spelling, ifaceName, "what it constructs")
		})

		t.Run("lowers no result for a construct signature without a type", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, ifaceOf(t, "  new (a: number);\n").Methods[0].Returns,
				"a construct signature without a type has no result")
		})

		t.Run("stamps an interface's call signatures as written", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export interface I {\n  (a: string): number;\n}\n")
			assert.Length(t, gb.StampRecords(), 1, "one stamp")
			s := gb.StampRecords()[0].Stamp
			assert.Equal(t, s.Key, typescript.CallSignatureKey, "under the call signature key")
			assert.Equal(t, s.Value, any([]string{"(a:string):number"}), "the signature as written")
		})
	})

	t.Run("callSignatures", func(t *testing.T) {
		t.Parallel()

		t.Run("stamps an aliased object type's call signatures on the alias", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, "export type F = { (a: string): number };\n")
			assert.Equal(t, gb.StampRecords()[0].Stamp.Value, any([]string{"(a:string):number"}),
				"the model has no member for a callable object")
		})
	})
}

// classOf parses one exported class body and returns the class.
func classOf(tb testing.TB, body string) *node.Struct {
	tb.Helper()

	return named[*node.Struct](tb, declsOf(tb, "export class "+className+" {\n"+body+"}\n"), className)
}

// ifaceOf parses one exported interface body and returns the interface.
func ifaceOf(tb testing.TB, body string) *node.Interface {
	tb.Helper()

	return named[*node.Interface](tb, declsOf(tb, "export interface "+ifaceName+" {\n"+body+"}\n"), ifaceName)
}

// classBuilt parses one exported class body and returns the unit's
// builder beside the class, for the cases that read the stamps.
func classBuilt(tb testing.TB, body string) (*plugin.GraphBuilder, *node.Struct) {
	tb.Helper()

	gb, _ := parsedSource(tb, "export class "+className+" {\n"+body+"}\n")
	return gb, named[*node.Struct](tb, fileIn(tb, gb, aPackage).Decls, className)
}

// ifaceBuilt parses one exported interface body and returns the unit's
// builder beside the interface, for the cases that read the stamps.
func ifaceBuilt(tb testing.TB, body string) (*plugin.GraphBuilder, *node.Interface) {
	tb.Helper()

	gb, _ := parsedSource(tb, "export interface "+ifaceName+" {\n"+body+"}\n")
	return gb, named[*node.Interface](tb, fileIn(tb, gb, aPackage).Decls, ifaceName)
}

// methodNames returns the names of methods, in order.
func methodNames(methods []*node.Method) []string {
	out := make([]string, 0, len(methods))
	for _, m := range methods {
		out = append(out, m.Name)
	}
	return out
}

// fieldNames returns the names of fields, in order.
func fieldNames(fields []*node.Field) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, f.Name)
	}
	return out
}
