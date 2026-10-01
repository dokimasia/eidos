// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Each declaration kind maps to one model kind, so the shape every
// kind lowers to is pinned.
func TestDecl(t *testing.T) {
	t.Parallel()

	t.Run("declaration", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an exported declaration as public", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, named[*node.Struct](t, declsOf(t, "export class A {}\n"), "A").Visibility,
				symbol.VisibilityPublic, "another module can import it")
		})

		t.Run("lowers a module's declaration it does not export as package-visible", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, named[*node.Struct](t, declsOf(t, "export {};\nclass A {}\n"), "A").Visibility,
				symbol.VisibilityPackage, "the module alone can name it")
		})

		t.Run("lowers a declaration an export clause publishes as public", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, named[*node.Struct](t, declsOf(t, "class A {}\nexport { A };\n"), "A").Visibility,
				symbol.VisibilityPublic, "the clause exports it by name")
		})

		t.Run("lowers a declaration the default export publishes as public", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, named[*node.Struct](t, declsOf(t, "class A {}\nexport default A;\n"), "A").Visibility,
				symbol.VisibilityPublic, "another module imports it as the default")
		})

		t.Run("leaves out a declaration the module does not export at signature depth", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, fstest.MapFS{aFile: {Data: []byte("export class A {}\nclass B {}\n")}},
				aFile, plugin.DepthSignatures)
			assert.Length(t, fileIn(t, gb, aPackage).Decls, 1, "the unexported class is no other module's API")
		})

		t.Run("lowers an anonymous default class under the name default", func(t *testing.T) {
			t.Parallel()

			named[*node.Struct](t, declsOf(t, "export default class {}\n"), "default")
		})

		t.Run("leaves out a function's implementation after its overload signatures", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "export function f(a: string): void;\nexport function f(a: number): void;\n"+
				"export function f(a: any): void {}\n")
			assert.Length(t, decls, 2, "one Function per signature, and none for the implementation")
		})

		t.Run("keeps a function without overload signatures", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, declsOf(t, "export function f(a: string): void {}\n"), 1, "the function itself")
		})

		t.Run("reports UnaddressedCarrier for a carrier on an overloaded function's implementation",
			func(t *testing.T) {
				t.Parallel()

				_, found := parsedSource(t, "export function f(a: string): void;\n"+carrierLine+
					"export function f(a: any): void {}\n")
				assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier},
					"the implementation declares nothing")
			})

		t.Run("reports UnaddressedCarrier for a carrier on a top-level statement", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export {};\n"+carrierLine+"if (ready) {}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "a statement declares nothing")
		})
	})

	t.Run("class", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers an abstract class as abstract", func(t *testing.T) {
			t.Parallel()

			assert.True(t, named[*node.Struct](t, declsOf(t, "export abstract class A {}\n"), "A").Abstract,
				"no value of it can be made directly")
		})

		t.Run("lowers the superclass with its type arguments", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "export class A extends Base<string> {}\n"), "A")
			assert.Equal(t, st.Extends[0].Spelling, "Base", "the superclass")
			assert.Equal(t, st.Extends[0].Args[0].Spelling, "string", "and its argument")
		})

		t.Run("lowers the interfaces a class implements", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "export class A implements I, J<number> {}\n"), "A")
			assert.Length(t, st.Implements, 2, "each interface")
		})

		t.Run("skips a comment among the interfaces a class implements", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "export class A implements I /* and */, J {}\n"), "A")
			assert.Length(t, st.Implements, 2, "a comment is no interface")
		})

		t.Run("lowers the decorators on a class and on its export", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "@outer\nexport @inner class A {}\n"), "A")
			assert.Equal(t, st.Annotations, symbol.Annotations{{Name: "outer"}, {Name: "inner"}},
				"both decorators, in source order")
		})
	})

	t.Run("iface", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers the interfaces an interface extends", func(t *testing.T) {
			t.Parallel()

			it := named[*node.Interface](t, declsOf(t, "export interface A extends B, C<D> {}\n"), "A")
			assert.Length(t, it.Extends, 2, "each extended interface")
		})

		t.Run("merges a second declaration of an interface in one file into the first", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "export interface A { a: string }\nexport interface A { b: number }\n")
			assert.Length(t, decls, 1, "one interface")
			assert.Length(t, decls[0].(*node.Interface).Fields, 2, "with both declarations' members")
		})
	})

	t.Run("enum", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers each member as a variant with its value verbatim", func(t *testing.T) {
			t.Parallel()

			e := named[*node.Enum](t, declsOf(t, "export enum E { A, B = 1 << 2, 'c' = 3 }\n"), "E")
			assert.Length(t, e.Variants, 3, "three members")
			assert.Empty(t, e.Variants[0].Value, "a member that assigns nothing")
			assert.Equal(t, e.Variants[1].Value, "1 << 2", "an expression verbatim")
			assert.Equal(t, e.Variants[2].Name, "c", "a quoted name unquoted")
		})

		t.Run("records a variant's trailing comment", func(t *testing.T) {
			t.Parallel()

			e := named[*node.Enum](t, declsOf(t, "export enum E {\n  A, // the a\n  B,\n}\n"), "E")
			assert.Equal(t, e.Variants[0].Comment, "the a", "the comment after the member")
		})

		t.Run("marks a const enum", func(t *testing.T) {
			t.Parallel()

			assert.True(t, named[*node.Enum](t, declsOf(t, "export const enum E { A }\n"), "E").Const,
				"its values are inlined at use")
		})
	})

	t.Run("alias", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a type alias with its type parameters and target", func(t *testing.T) {
			t.Parallel()

			a := named[*node.Alias](t, declsOf(t, "export type A<T> = T[];\n"), "A")
			assert.Length(t, a.TypeParams, 1, "the parameter")
			assert.Equal(t, a.Target.Form, symbol.FormList, "and the type it names")
		})
	})

	t.Run("function", func(t *testing.T) {
		t.Parallel()

		t.Run("marks an async function", func(t *testing.T) {
			t.Parallel()

			fn := named[*node.Function](t, declsOf(t, "export async function g(): Promise<void> {}\n"), "g")
			assert.True(t, fn.Async, "async is a fact")
		})

		t.Run("leaves a plain function unmarked", func(t *testing.T) {
			t.Parallel()

			assert.False(t, functionOf(t, "(): Promise<void>").Async, "returning a promise is no async mark")
		})

		t.Run("lowers a declare function as a function", func(t *testing.T) {
			t.Parallel()

			named[*node.Function](t, declsOf(t, "export declare function g(): void;\n"), "g")
		})
	})

	t.Run("variables", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a const as a constant with its type and value", func(t *testing.T) {
			t.Parallel()

			c := named[*node.Constant](t, declsOf(t, "export const limit: number = 16;\n"), "limit")
			assert.Equal(t, c.Type.Spelling, "number", "the stated type")
			assert.Equal(t, c.Value, "16", "the initializer verbatim")
		})

		t.Run("lowers let and var as mutable variables", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, "export let a = 1;\nexport var b;\n")
			assert.Equal(t, named[*node.Variable](t, decls, "a").Mutability, symbol.MutabilityMutable, "let")
			assert.Equal(t, named[*node.Variable](t, decls, "b").Mutability, symbol.MutabilityMutable, "var")
		})

		t.Run("lowers each name one statement binds", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, declsOf(t, "export const a = 1, b = 2;\n"), 2, "a constant per name")
		})

		t.Run("lowers a variable whose source states no type with a nil type", func(t *testing.T) {
			t.Parallel()

			assert.True(t, named[*node.Variable](t, declsOf(t, "export let a = 1;\n"), "a").Type == nil,
				"the language infers it")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a destructuring statement", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export {};\n// +fixture:gen:table name=t\nconst { a, b } = o;\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier}, "a pattern binds no single name")
		})
	})

	t.Run("decorator", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a qualified decorator's whole name", func(t *testing.T) {
			t.Parallel()

			st := named[*node.Struct](t, declsOf(t, "@ns.tag(1, 'x')\nexport class A {}\n"), "A")
			assert.Equal(t, st.Annotations, symbol.Annotations{{Name: "ns.tag", Args: []string{"1", "'x'"}}},
				"the name it calls and each argument verbatim")
		})
	})
}
