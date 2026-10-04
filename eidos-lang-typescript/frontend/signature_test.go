// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// fnName is the function every signature case declares.
const fnName = "f"

// A signature's parts are each a subject or a reference the
// resolution step reads, so how a parameter list, a type parameter list
// and a return type lower is pinned.
func TestSignature(t *testing.T) {
	t.Parallel()

	t.Run("params", func(t *testing.T) {
		t.Parallel()

		t.Run("names each parameter and types it", func(t *testing.T) {
			t.Parallel()

			p := functionOf(t, "(a: string)").Params[0]
			assert.Equal(t, p.Name, "a", "the parameter's name")
			assert.Equal(t, p.Type.Spelling, "string", "and its type")
		})

		t.Run("marks a ? parameter optional", func(t *testing.T) {
			t.Parallel()

			assert.True(t, functionOf(t, "(a?: string)").Params[0].Optional, "the caller may omit it")
		})

		t.Run("marks a rest parameter positionally variadic under its pattern's name", func(t *testing.T) {
			t.Parallel()

			p := functionOf(t, "(...rest: string[])").Params[0]
			assert.Equal(t, p.Variadic, symbol.VariadicPositional, "the remaining arguments")
			assert.Equal(t, p.Name, "rest", "under the rest pattern's name")
		})

		rests := []struct {
			name string
			give string
			want string
		}{
			{
				name: "types a rest parameter of an array type as the element",
				give: "(...rest: string[])", want: "string",
			},
			{
				name: "types a rest parameter of a readonly array as the element",
				give: "(...rest: readonly string[])", want: "string",
			},
			{name: "types a rest parameter of Array<T> as T", give: "(...rest: Array<string>)", want: "string"},
			{
				name: "types a rest parameter of ReadonlyArray<T> as T",
				give: "(...rest: ReadonlyArray<string>)", want: "string",
			},
			{
				name: "keeps a rest parameter's tuple type, which states no element",
				give: "(...rest: [string, number])", want: "[string,number]",
			},
			{
				name: "keeps a rest parameter's generic type other than an array",
				give: "(...rest: Set<string>)", want: "Set",
			},
			{
				name: "keeps a rest parameter's Array of two arguments, which states no element",
				give: "(...rest: Array<string, number>)", want: "Array",
			},
		}
		for _, tt := range rests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, functionOf(t, tt.give).Params[0].Type.Spelling, tt.want,
					"the type of one argument the parameter takes")
			})
		}

		t.Run("leaves a rest parameter without a type untyped", func(t *testing.T) {
			t.Parallel()

			assert.True(t, functionOf(t, "(...rest)").Params[0].Type == nil, "the source states none")
		})

		t.Run("keeps a parameter's default verbatim", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, functionOf(t, "(a: number = 1 + 2)").Params[0].Default, "1 + 2",
				"the default is unevaluated")
		})

		t.Run("leaves a this parameter out of a function's parameters", func(t *testing.T) {
			t.Parallel()

			fn := functionOf(t, "(this: Window, a: string)")
			assert.Length(t, fn.Params, 1, "this types the receiver and is no argument")
			assert.Equal(t, fn.Params[0].Name, "a", "the argument remains")
		})

		t.Run("leaves a destructuring parameter unnamed", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, functionOf(t, "({ a, b }: Options)").Params[0].Name,
				"its identity is its position")
		})

		t.Run("lowers a parameter's decorators as annotations", func(t *testing.T) {
			t.Parallel()

			methods := named[*node.Struct](t, declsOf(t, "export class C {\n  m(@inject('x') a: string) {}\n}\n"),
				"C").Methods
			want := symbol.Annotations{{Name: "inject", Args: []string{"'x'"}}}
			assert.Equal(t, methods[0].Params[0].Annotations, want, "the decorator's name and its argument verbatim")
		})

		t.Run("reports UnaddressedCarrier for a carrier on a parameter", func(t *testing.T) {
			t.Parallel()

			_, found := parsedSource(t, "export function f(\n  // +fixture:gen:table name=t\n  a: string,\n) {}\n")
			assert.Equal(t, codesOf(found), []diag.Code{frontend.UnaddressedCarrier},
				"no rule takes a parameter as its subject")
		})
	})

	t.Run("typeParams", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers a type parameter's constraint as its bound", func(t *testing.T) {
			t.Parallel()

			tp := functionOf(t, "<T extends Base>(a: T)").TypeParams[0]
			assert.Equal(t, tp.Name, "T", "the parameter's name")
			assert.Equal(t, tp.Bounds[0].Spelling, "Base", "its constraint")
		})

		t.Run("skips a comment among the type parameters", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, functionOf(t, "<T, /* and */ U>(a: T, b: U)").TypeParams, 2, "a comment declares none")
		})

		t.Run("lowers a type parameter's default", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, functionOf(t, "<T = string>(a: T)").TypeParams[0].Default.Spelling, "string",
				"the argument a caller that supplies none gets")
		})
	})

	t.Run("returns", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers the return type as one result", func(t *testing.T) {
			t.Parallel()

			fn := functionOf(t, "(): Promise<number>")
			assert.Length(t, fn.Returns, 1, "one result")
			assert.Equal(t, fn.Returns[0].Type.Spelling, "Promise", "the type as written")
		})

		t.Run("lowers no result for a function that states no return type", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, functionOf(t, "()").Returns, "the source states none")
		})

		t.Run("lowers a type predicate as a Named result", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, functionOf(t, "(a: unknown): a is string").Returns[0].Type.Spelling, "a is string",
				"the predicate is the spelling")
		})
	})
}

// functionOf parses one exported function and returns it.
func functionOf(tb assert.TB, signature string) *node.Function {
	tb.Helper()

	return named[*node.Function](tb, declsOf(tb, "export function "+fnName+signature+" {}\n"), fnName)
}
