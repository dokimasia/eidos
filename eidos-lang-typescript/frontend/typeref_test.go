// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// aliasName is the alias every type case declares, so the case reads
// the type it names.
const aliasName = "T"

// Each structural form the syntax states is a contract the projection
// folds, so the form every type lowers to is pinned.
func TestTyperef(t *testing.T) {
	t.Parallel()

	t.Run("typeRef", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a type without the whitespace between its tokens", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, targetOf(t, "keyof   Record< string ,  number >").Spelling, "keyof Record<string,number>",
				"one space remains between two identifier tokens")
		})

		t.Run("keeps a generic's bare name in the spelling and its arguments in Args", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "Map<string, number>")
			assert.Equal(t, ref.Spelling, "Map", "the bare name")
			assert.Length(t, ref.Args, 2, "the two arguments")
		})

		t.Run("records the module specifier of an imported name as its package", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, targetOf(t, "Imported").Package, "./lib", "the import binds the name")
		})

		t.Run("records no package for a name no import binds", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, targetOf(t, "Local").Package, "the file's own scope declares it")
		})

		t.Run("lowers T[] as a List", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "Imported[]")
			assert.Equal(t, ref.Form, symbol.FormList, "an array type is a list")
			assert.Equal(t, ref.Elems[0].Spelling, "Imported", "of its element")
		})

		t.Run("lowers a readonly array as the List it reads", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "readonly Imported[]")
			assert.Equal(t, ref.Form, symbol.FormList, "readonly reads an array")
			assert.Equal(t, ref.Elems[0].Spelling, "Imported", "whose element resolves")
		})

		t.Run("spells a readonly array with its readonly", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, targetOf(t, "readonly Imported[]").Spelling, "readonly Imported[]",
				"the spelling keeps what the form leaves out")
		})

		t.Run("lowers a readonly tuple as the Tuple it reads", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "readonly [string, Imported]")
			assert.Equal(t, ref.Form, symbol.FormTuple, "readonly reads a tuple")
			assert.Length(t, ref.Elems, 2, "of its members")
		})

		t.Run("keeps a named type after readonly as its own spelling", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "readonly Imported")
			assert.Equal(t, ref.Spelling, "Imported", "the resolution step reads the name")
			assert.Equal(t, ref.Package, "./lib", "and its import")
		})

		t.Run("lowers a tuple as a Tuple of its members", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "[a: string, b?: number, ...rest: Imported[]]")
			assert.Equal(t, ref.Form, symbol.FormTuple, "a tuple type is a tuple")
			assert.Equal(t, formsOf(ref), []symbol.TypeForm{symbol.FormNamed, symbol.FormOptional, symbol.FormNamed},
				"a labeled member's type, an optional member, and a rest member")
		})

		t.Run("lowers an unlabeled optional member of a tuple as Optional", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, formsOf(targetOf(t, "[string, Imported?]")), []symbol.TypeForm{
				symbol.FormNamed, symbol.FormOptional,
			}, "the ? marks the member optional")
		})

		comments := []struct {
			name string
			give string
			want int
		}{
			{name: "skips a comment among a tuple's members", give: "[string, /* and */ number]", want: 2},
			{name: "skips a comment among a union's members", give: "string | /* or */ number", want: 2},
			{
				name: "skips a comment beside an object type's index signature",
				give: "{ /* map */ [k: string]: A }", want: 2,
			},
		}
		for _, tt := range comments {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Length(t, targetOf(t, tt.give).Elems, tt.want, "a comment is no member")
			})
		}

		rests := []struct {
			name string
			give string
		}{
			{
				name: "spells a labeled rest member of a tuple with its ... and the type it spreads",
				give: "[a: string, ...rest: Imported[]]",
			},
			{
				name: "spells an unlabeled rest member of a tuple with its ... and the type it spreads",
				give: "[string, ...Imported[]]",
			},
		}
		for _, tt := range rests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, targetOf(t, tt.give).Elems[1].Spelling, "...Imported[]",
					"the spelling tells a rest member from an array member")
			})
		}

		t.Run("lowers T | undefined as an Optional of T", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "Imported | undefined")
			assert.Equal(t, ref.Form, symbol.FormOptional, "undefined leaves one member")
			assert.Equal(t, ref.Elems[0].Spelling, "Imported", "the member left")
		})

		t.Run("lowers T | null | undefined as an Optional of T", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, targetOf(t, "Imported | null | undefined").Form, symbol.FormOptional,
				"a nested union flattens before the nullish members leave")
		})

		t.Run("lowers a union of two members besides null as a Union of every member", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "string | number | null")
			assert.Equal(t, ref.Form, symbol.FormUnion, "two members remain")
			assert.Length(t, ref.Elems, 3, "and null is one of them")
		})

		t.Run("lowers an intersection as an Intersection of its members", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "Imported & Local")
			assert.Equal(t, ref.Form, symbol.FormIntersection, "a value has both members' types")
			assert.Length(t, ref.Elems, 2, "one child per member")
		})

		t.Run("flattens a chained intersection into one Intersection", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, formsOf(targetOf(t, "Imported & Local & Other")), []symbol.TypeForm{
				symbol.FormNamed, symbol.FormNamed, symbol.FormNamed,
			}, "the grammar nests a chain, and the model lists its members")
		})

		t.Run("keeps an intersection inside a union as one member", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, formsOf(targetOf(t, "Imported & Local | string")), []symbol.TypeForm{
				symbol.FormIntersection, symbol.FormNamed,
			}, "& binds tighter than |")
		})

		t.Run("lowers a function type as a Func of its parameters then its return", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "(a: string, b?: Imported) => number")
			assert.Equal(t, ref.Form, symbol.FormFunc, "a function type is a func")
			assert.Equal(t, ref.Split, 2, "two parameters, then the return")
			assert.Equal(t, ref.Elems[2].Spelling, "number", "the return type")
		})

		t.Run("lowers a constructor type as a Func of its parameters then the type it constructs", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "new (a: string) => Imported")
			assert.Equal(t, ref.Form, symbol.FormFunc, "a constructor type is a func")
			assert.Equal(t, ref.Split, 1, "one parameter, then the result")
			assert.Equal(t, ref.Elems[1].Spelling, "Imported", "the type it constructs")
		})

		t.Run("spells a constructor type with its new", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, targetOf(t, "new (a: string) => Imported").Spelling, "new(a:string)=>Imported",
				"the spelling tells it from a function type")
		})

		t.Run("lowers an untyped parameter of a function type as a reference that spells nothing", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "(a) => void")
			assert.Equal(t, ref.Split, 1, "the parameter keeps its place")
			assert.Empty(t, ref.Elems[0].Spelling, "the source states no type")
		})

		t.Run("lowers an object type with one index signature as a Map", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "{ [key: string]: Imported }")
			assert.Equal(t, ref.Form, symbol.FormMap, "a lone index signature maps keys to values")
			assert.Equal(t, ref.Elems[1].Spelling, "Imported", "the value type")
		})

		t.Run("lowers any other object type as Inline", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, targetOf(t, "{ a: string; b: number }").Form, symbol.FormInline,
				"an object type with members is an inline body")
		})

		t.Run("records an inline object type's properties as the reference's fields", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldNames(targetOf(t, "{ a: string; b?: Imported }").Fields), []string{"a", "b"},
				"each property signature is a field")
		})

		t.Run("records an inline object type's methods as the reference's methods", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "{ run(): void; new (): Imported; [k: number]: string }")
			assert.Equal(t, methodNames(ref.Methods), []string{"run", "new", "[]"},
				"a method, a construct and an index signature")
		})

		t.Run("records the import of an inline member's type", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, targetOf(t, "{ a: Imported }").Fields[0].Type.Package, "./lib",
				"the member's type resolves as any reference does")
		})

		t.Run("lowers a mapped type as Named", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, targetOf(t, "{ [K in keyof Imported]: string }").Form, symbol.FormNamed,
				"the mapping is metadata the spelling keeps")
		})

		t.Run("lowers a conditional type as Named", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, targetOf(t, "Imported extends string ? 1 : 2").Form, symbol.FormNamed,
				"the condition is metadata the spelling keeps")
		})

		t.Run("unwraps the parentheses around a type", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, targetOf(t, "(Imported)[]").Elems[0].Spelling, "Imported",
				"the parentheses are punctuation")
		})

		t.Run("lowers a qualified name as Named with its whole spelling", func(t *testing.T) {
			t.Parallel()

			ref := targetOf(t, "Imported.Inner")
			assert.Equal(t, ref.Spelling, "Imported.Inner", "the spelling keeps the qualifier")
			assert.Equal(t, ref.Package, "./lib", "and its first name's import is the package")
		})
	})
}

// targetOf parses one type alias and returns the type it names.
func targetOf(tb testing.TB, typ string) *node.TypeRef {
	tb.Helper()

	return named[*node.Alias](tb, declsOf(tb, "import { Imported } from './lib';\n"+
		"export type "+aliasName+" = "+typ+";\n"), aliasName).Target
}

// formsOf returns the forms of a reference's children, in order.
func formsOf(ref *node.TypeRef) []symbol.TypeForm {
	out := make([]symbol.TypeForm, 0, len(ref.Elems))
	for _, e := range ref.Elems {
		out = append(out, e.Form)
	}
	return out
}
