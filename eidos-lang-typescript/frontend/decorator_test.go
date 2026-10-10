// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The decorators of the cases and their directives. stubMarker is a
// marker of the brand, pluginMarker is a marker with a plugin, and
// logMarker is a decorator outside the brand.
const (
	stubMarker   = "@" + brand + ".stub"
	stubName     = directive.Name("stub")
	pluginMarker = "@" + brand + ".gen.table"
	pluginName   = directive.Name("gen:table")
	logMarker    = "@log"
)

// TestDecorator checks how a decorator lowers onto its subject, and how
// the arguments of a decorator of the brand lift into a directive.
func TestDecorator(t *testing.T) {
	t.Parallel()

	t.Run("decorate", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches the directive of a decorator of the brand to its class", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, stubMarker+"\nexport class C {}\n")
			assert.Empty(t, found, "the marker lifts")
			c := named[*node.Struct](t, fileIn(t, gb, aPackage).Decls, className)
			attached := gb.Attachments()
			assert.Length(t, attached, 1, "the marker attaches one directive")
			expect.Equal(t, attached[0].Subject, symbol.Symbol(c), "the directive is on the class", assert.ByIdentity())
			want := directive.Raw{Name: stubName, Pos: position.Pos{File: aFile, Line: 1, Col: 1}}
			expect.Equal(t, attached[0].Raw, want, "the directive has the decorator's position and no arguments")
		})

		t.Run("keeps a decorator of the brand as an annotation", func(t *testing.T) {
			t.Parallel()

			c := named[*node.Struct](t, declsOf(t, stubMarker+"\nexport class C {}\n"), className)
			assert.Equal(t, c.Annotations, symbol.Annotations{{Name: brand + ".stub"}},
				"the class has the decorator as an annotation")
		})

		subjects := []struct {
			name    string
			give    string
			subject func(c *node.Struct) symbol.Symbol
		}{
			{
				name: "attaches the directive of a decorator of a method to the method",
				give: "  " + stubMarker + "\n  run(): void {}\n",
				subject: func(c *node.Struct) symbol.Symbol {
					return c.Methods[0]
				},
			},
			{
				name: "attaches the directive of a decorator of a field to the field",
				give: "  " + stubMarker + "\n  f!: number;\n",
				subject: func(c *node.Struct) symbol.Symbol {
					return c.Fields[0]
				},
			},
		}
		for _, tt := range subjects {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, c := classBuilt(t, tt.give)
				attached := gb.Attachments()
				assert.Length(t, attached, 1, "the marker attaches one directive")
				expect.Equal(t, attached[0].Raw.Name, stubName, "the directive has the marker's name")
				expect.Equal(t, attached[0].Subject, tt.subject(c), "the directive is on the decorated subject",
					assert.ByIdentity())
			})
		}

		t.Run("attaches the prefixed directive of a marker with a plugin", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pluginMarker+"\nexport class C {}\n")
			attached := gb.Attachments()
			assert.Length(t, attached, 1, "the marker attaches one directive")
			assert.Equal(t, attached[0].Raw.Name, pluginName, "the name has the plugin's prefix")
		})

		t.Run("attaches nothing for a decorator outside the brand", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, logMarker+"(f())\nexport class C {}\n")
			expect.Empty(t, found, "a decorator outside the brand is metadata, whatever its arguments are")
			expect.Empty(t, gb.Attachments(), "a decorator outside the brand attaches no directive")
		})

		t.Run("attaches nothing for a decorator whose expression is not a path of names", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, "@("+brand+".stub)\nexport class C {}\n")
			expect.Empty(t, found, "a parenthesized expression is metadata")
			expect.Empty(t, gb.Attachments(), "a parenthesized expression attaches no directive")
		})

		t.Run("reports BadMarker for a decorator of the brand alone", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, "@"+brand+"\nexport class C {}\n")
			expect.Equal(t, codesOf(found), []diag.Code{frontend.BadMarker}, "the brand alone is not a directive name")
			expect.Empty(t, gb.Attachments(), "the marker attaches nothing")
		})

		t.Run("keeps a decorator of a parameter as an annotation", func(t *testing.T) {
			t.Parallel()

			_, c := classBuilt(t, "  run(@log a: string): void {}\n")
			assert.Equal(t, c.Methods[0].Params[0].Annotations, symbol.Annotations{{Name: "log"}},
				"the parameter has the decorator as an annotation")
		})

		unaddressed := []struct {
			name string
			give string
		}{
			{
				name: "reports UnaddressedCarrier for a marker of a parameter",
				give: "  run(" + stubMarker + " a: string): void {}\n",
			},
			{
				name: "reports UnaddressedCarrier for a marker of an overloaded method's implementation",
				give: "  run(a: string): void;\n  " + stubMarker + "\n  run(a: any): void {}\n",
			},
		}
		for _, tt := range unaddressed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, "export class C {\n"+tt.give+"}\n")
				assert.Contains(t, codesOf(found), frontend.UnaddressedCarrier, "the marker attaches nowhere")
				expect.Empty(t, gb.Attachments(), "the marker attaches nothing")
			})
		}
	})

	t.Run("marker", func(t *testing.T) {
		t.Parallel()

		lifts := []struct {
			name string
			give string
			want []directive.RawArg
		}{
			{
				name: "lifts a string as its content", give: `("writer")`,
				want: []directive.RawArg{{Value: directive.RawValue{Text: "writer", Quoted: true}}},
			},
			{
				name: "lifts the escapes of a string as their characters", give: `('it\'s\n')`,
				want: []directive.RawArg{{Value: directive.RawValue{Text: "it's\n", Quoted: true}}},
			},
			{
				name: "lifts a template literal without a substitution as its content", give: "(`plain`)",
				want: []directive.RawArg{{Value: directive.RawValue{Text: "plain", Quoted: true}}},
			},
			{
				name: "lifts a hexadecimal number as its decimal text", give: "(0x10)",
				want: []directive.RawArg{{Value: directive.RawValue{Text: "16"}}},
			},
			{
				name: "lifts a number with separators as its decimal text", give: "(1_000)",
				want: []directive.RawArg{{Value: directive.RawValue{Text: "1000"}}},
			},
			{
				name: "lifts a number with a minus sign as negative decimal text", give: "(-2.5)",
				want: []directive.RawArg{{Value: directive.RawValue{Text: "-2.5"}}},
			},
			{
				name: "lifts true as written", give: "(true)",
				want: []directive.RawArg{{Value: directive.RawValue{Text: "true"}}},
			},
			{
				name: "lifts false as written", give: "(false)",
				want: []directive.RawArg{{Value: directive.RawValue{Text: "false"}}},
			},
			{
				name: "lifts an array as the list of its elements", give: `(["a", 1])`,
				want: []directive.RawArg{{Value: directive.RawValue{List: []directive.RawValue{
					{Text: "a", Quoted: true}, {Text: "1"},
				}}}},
			},
			{
				name: "lifts an empty array as an empty list", give: "([])",
				want: []directive.RawArg{{Value: directive.RawValue{List: []directive.RawValue{}}}},
			},
			{
				name: "lifts an object literal as keyed arguments", give: `({ tag: "test", n: 2 })`,
				want: []directive.RawArg{
					{Key: "tag", Value: directive.RawValue{Text: "test", Quoted: true}},
					{Key: "n", Value: directive.RawValue{Text: "2"}},
				},
			},
			{
				name: "lifts a quoted key of an object literal as its content", give: `({ "content-type": "json" })`,
				want: []directive.RawArg{{Key: "content-type", Value: directive.RawValue{Text: "json", Quoted: true}}},
			},
			{
				name: "lifts positional arguments before the keyed ones", give: `("x", { tag: "y" })`,
				want: []directive.RawArg{
					{Value: directive.RawValue{Text: "x", Quoted: true}},
					{Key: "tag", Value: directive.RawValue{Text: "y", Quoted: true}},
				},
			},
			{
				name: "skips a comment among the arguments", give: `(/* who */ "writer")`,
				want: []directive.RawArg{{Value: directive.RawValue{Text: "writer", Quoted: true}}},
			},
			{
				name: "skips a comment among the elements of an array", give: `([/* first */ "a"])`,
				want: []directive.RawArg{{Value: directive.RawValue{List: []directive.RawValue{
					{Text: "a", Quoted: true},
				}}}},
			},
			{
				name: "skips a comment among the members of an object literal", give: `({ /* the tag */ tag: "a" })`,
				want: []directive.RawArg{{Key: "tag", Value: directive.RawValue{Text: "a", Quoted: true}}},
			},
		}
		for _, tt := range lifts {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, stubMarker+tt.give+"\nexport class C {}\n")
				assert.Empty(t, found, "the arguments lift")
				attached := gb.Attachments()
				assert.Length(t, attached, 1, "the marker attaches one directive")
				assert.Equal(t, attached[0].Raw.Args, tt.want, "the directive has the lifted arguments")
			})
		}

		refusals := []struct {
			name string
			give string
			want string
		}{
			{name: "reports BadMarker for a call", give: "(f())", want: "the argument f() is not a string"},
			{
				name: "reports BadMarker for a shorthand member of an object literal", give: "({ tag })",
				want: "the member tag of the object is not a key with a value",
			},
			{
				name: "reports BadMarker for an object literal before the last argument", give: `({ tag: "a" }, "b")`,
				want: `the argument {tag:"a"} is not a string`,
			},
			{
				name: "reports BadMarker for a computed key of an object literal", give: "({ [k]: 1 })",
				want: "the key [k] is neither a name nor a string",
			},
			{
				name: "reports BadMarker for a number with a plus sign", give: "(+1)",
				want: "the argument +1 is not a string",
			},
			{name: "reports BadMarker for a bigint", give: "(1n)", want: "the argument 1n is not a string"},
			{
				name: "reports BadMarker for a string with an escape that TypeScript refuses", give: `('\07')`,
				want: `the argument '\07' is not a string`,
			},
			{
				name: "reports BadMarker for a minus sign before a name", give: "(-n)",
				want: "the argument -n is not a string",
			},
			{
				name: "reports BadMarker for a template literal with a substitution", give: "(`a${b}`)",
				want: "is not a string",
			},
			{
				name: "reports BadMarker for an element of an array that does not lift", give: `(["a", f()])`,
				want: "the argument f() is not a string",
			},
			{
				name: "reports BadMarker for a value of an object literal that does not lift", give: "({ tag: f() })",
				want: "the argument f() is not a string",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, stubMarker+tt.give+"\nexport class C {}\n")
				assert.Equal(t, codesOf(found), []diag.Code{frontend.BadMarker},
					"the frontend reports the marker under BadMarker")
				expect.Contains(t, found[0].Msg, tt.want, "the finding describes the argument")
				expect.Equal(t, found[0].Pos, position.Pos{File: aFile, Line: 1, Col: 1},
					"the finding is at the decorator")
				expect.Empty(t, gb.Attachments(), "the marker attaches nothing")
			})
		}
	})
}
