// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"
)

// TestSpelling pins the layout of the spelling of each form of a type, in
// which Biome and Prettier write the type on one line. A source of any
// layout spells the same.
func TestSpelling(t *testing.T) {
	t.Parallel()

	t.Run("spelling", func(t *testing.T) {
		t.Parallel()

		layouts := []struct {
			name string
			give string
			want string
		}{
			{name: "writes a space around the bar of a union", give: "string|number", want: "string | number"},
			{
				name: "writes a space around the ampersand of an intersection", give: "Imported&Local",
				want: "Imported & Local",
			},
			{
				name: "leaves out the bar before the first member of a union", give: `| "a" | "b"`,
				want: `"a" | "b"`,
			},
			{
				name: "writes a space after each colon of a function type and around its arrow",
				give: "(t:Imported,u?:string)=>void", want: "(t: Imported, u?: string) => void",
			},
			{
				name: "writes a function type with type parameters", give: "<U>(v:Imported,f:(t:Imported)=>U)=>U",
				want: "<U>(v: Imported, f: (t: Imported) => U) => U",
			},
			{
				name: "writes a space around the equals sign of a default type", give: "<G=string>()=>G",
				want: "<G = string>() => G",
			},
			{
				name: "writes a space after the new of a constructor type", give: "new(a:string)=>Imported",
				want: "new (a: string) => Imported",
			},
			{
				name: "writes a space after the abstract of a constructor type", give: "abstract new()=>Imported",
				want: "abstract new () => Imported",
			},
			{
				name: "writes a space inside the braces of an object type", give: "{product:Imported;limit:number}",
				want: "{ product: Imported; limit: number }",
			},
			{
				name: "separates the members of an object type with semicolons", give: "{ a: string, b: number, }",
				want: "{ a: string; b: number }",
			},
			{
				name: "leaves out the separator after the last member of an object type",
				give: "{\n  a: string;\n  b: number;\n}", want: "{ a: string; b: number }",
			},
			{
				name: "leaves out a separator before the first member of an object type", give: "{; a: string}",
				want: "{ a: string }",
			},
			{name: "writes an empty object type as two braces", give: "{ }", want: "{}"},
			{
				name: "writes the members of an object type that are no properties",
				give: "{m?():void;new():Imported;(x:string):void;get a():string;set a(v:string)}",
				want: "{ m?(): void; new (): Imported; (x: string): void; get a(): string; set a(v: string) }",
			},
			{
				name: "writes an index signature", give: "{[key:string]:Imported}",
				want: "{ [key: string]: Imported }",
			},
			{
				name: "writes the modifiers of a mapped type", give: "{-readonly[K in keyof Imported]-?:Imported[K]}",
				want: "{ -readonly [K in keyof Imported]-?: Imported[K] }",
			},
			{
				name: "writes a space around the question mark and the colon of a conditional type",
				give: "Imported extends string?1:2", want: "Imported extends string ? 1 : 2",
			},
			{
				name: "writes a space after keyof before a parenthesis", give: "keyof(Imported|Local)",
				want: "keyof (Imported | Local)",
			},
			{
				name: "writes a space after readonly before a bracket", give: "readonly[string,number]",
				want: "readonly [string, number]",
			},
			{
				name: "writes a space after each comma of a tuple", give: "[a:string,b?:number,...rest:Imported[]]",
				want: "[a: string, b?: number, ...rest: Imported[]]",
			},
			{
				name: "leaves out the comma after the last member of a tuple", give: "[string, number,]",
				want: "[string, number]",
			},
			{
				name: "writes the type arguments of a member of a union", give: "Map<string,number>|undefined",
				want: "Map<string, number> | undefined",
			},
			{
				name: "keeps the text of each string literal", give: `"a  b"|'c'`,
				want: `"a  b" | 'c'`,
			},
			{
				name: "keeps the text of a template literal type", give: "`prefix-${ string }`",
				want: "`prefix-${string}`",
			},
			{
				name: "writes the type inside a template literal type", give: "`${Imported|Local}-x`",
				want: "`${Imported | Local}-x`",
			},
			{
				name: "writes a space inside the braces of an object pattern", give: "({a,b}:Imported)=>void",
				want: "({ a, b }: Imported) => void",
			},
			{
				name: "writes a type predicate", give: "(x:unknown)=>x is string",
				want: "(x: unknown) => x is string",
			},
			{name: "writes a negative number without a space", give: "-1|1", want: "-1 | 1"},
			{
				name: "writes the import of a type query", give: "typeof import('./lib')",
				want: "typeof import('./lib')",
			},
			{name: "leaves out a comment", give: "string /* or */ | number", want: "string | number"},
		}
		for _, tt := range layouts {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, targetOf(t, tt.give).Spelling, tt.want, "the spelling has the layout of the formatters")
			})
		}
	})
}
