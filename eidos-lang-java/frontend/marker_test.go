// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The markers that the cases write. stubMarker is an annotation of the
// brand, and stubName is the directive that it attaches. markerLine is
// the line of the first declaration below the package clause.
const (
	stubMarker = "@" + brand + ".stub"
	stubName   = directive.Name("stub")
	markerLine = 3
)

// TestMarker checks how an annotation of the brand lowers onto its
// declaration, and how its element values lift into a directive.
func TestMarker(t *testing.T) {
	t.Parallel()

	t.Run("marker", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches the directive of an annotation of the brand to its class", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, pkgClause+stubMarker+"\npublic class A {}\n")
			assert.Empty(t, found, "the marker lifts")
			a := named[*node.Struct](t, fileIn(t, gb, pkgPath).Decls, "A")
			attached := gb.Attachments()
			assert.Length(t, attached, 1, "the marker attaches one directive")
			expect.Equal(t, attached[0].Subject, symbol.Symbol(a), "the directive is on the class", assert.ByIdentity())
			expect.Equal(t, attached[0].Raw,
				directive.Raw{Name: stubName, Pos: position.Pos{File: srcFile, Line: markerLine, Col: 1}},
				"the directive has the annotation's position and no arguments")
		})

		t.Run("keeps an annotation of the brand as an annotation", func(t *testing.T) {
			t.Parallel()

			a := named[*node.Struct](t, declsOf(t, stubMarker+"\npublic class A {}\n"), "A")
			assert.Equal(t, a.Annotations, symbol.Annotations{{Name: brand + ".stub"}},
				"the class has the annotation as an annotation")
		})

		t.Run("attaches the prefixed directive of an annotation with a plugin", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"@"+brand+".gen.table\npublic class A {}\n")
			attached := gb.Attachments()
			assert.Length(t, attached, 1, "the marker attaches one directive")
			assert.Equal(t, attached[0].Raw.Name, directive.Name("gen:table"), "the name has the plugin's prefix")
		})

		t.Run("attaches nothing for an annotation outside the brand", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, pkgClause+"@SuppressWarnings({\"a\", E.X})\npublic class A {}\n")
			expect.Empty(t, found, "an annotation outside the brand is metadata, whatever its values are")
			expect.Empty(t, gb.Attachments(), "an annotation outside the brand attaches no directive")
		})

		t.Run("attaches the directive to each field that a declaration declares", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, pkgClause+"public class A {\n  "+stubMarker+" int a, b;\n}\n")
			a := named[*node.Struct](t, fileIn(t, gb, pkgPath).Decls, "A")
			attached := gb.Attachments()
			assert.Length(t, attached, 2, "the marker attaches one directive to each field")
			expect.Equal(t, attached[0].Subject, symbol.Symbol(a.Fields[0]), "the first is on a", assert.ByIdentity())
			expect.Equal(t, attached[1].Subject, symbol.Symbol(a.Fields[1]), "the second is on b", assert.ByIdentity())
		})

		subjects := []struct {
			name    string
			give    string
			subject func(tb testing.TB, gb *plugin.GraphBuilder) symbol.Symbol
		}{
			{
				name: "attaches the directive of a marker of a method to the method",
				give: "public class A {\n  " + stubMarker + "\n  public void run() {}\n}\n",
				subject: func(tb testing.TB, gb *plugin.GraphBuilder) symbol.Symbol {
					tb.Helper()

					return named[*node.Struct](tb, fileIn(tb, gb, pkgPath).Decls, "A").Methods[0]
				},
			},
			{
				name: "attaches the directive of a marker of a constructor to the constructor",
				give: "public class A {\n  " + stubMarker + "\n  public A() {}\n}\n",
				subject: func(tb testing.TB, gb *plugin.GraphBuilder) symbol.Symbol {
					tb.Helper()

					return named[*node.Struct](tb, fileIn(tb, gb, pkgPath).Decls, "A").Methods[0]
				},
			},
			{
				name: "attaches the directive of a marker of an enum constant to the constant",
				give: "public enum E {\n  " + stubMarker + "\n  ONE\n}\n",
				subject: func(tb testing.TB, gb *plugin.GraphBuilder) symbol.Symbol {
					tb.Helper()

					return named[*node.Enum](tb, fileIn(tb, gb, pkgPath).Decls, "E").Variants[0]
				},
			},
			{
				name: "attaches the directive of a marker of an annotation type element to the element",
				give: "public @interface T {\n  " + stubMarker + "\n  String value();\n}\n",
				subject: func(tb testing.TB, gb *plugin.GraphBuilder) symbol.Symbol {
					tb.Helper()

					return named[*node.Interface](tb, fileIn(tb, gb, pkgPath).Decls, "T").Methods[0]
				},
			},
		}
		for _, tt := range subjects {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, pkgClause+tt.give)
				assert.Empty(t, found, "the marker lifts")
				attached := gb.Attachments()
				assert.Length(t, attached, 1, "the marker attaches one directive")
				expect.Equal(t, attached[0].Raw.Name, stubName, "the directive has the marker's name")
				expect.Equal(t, attached[0].Subject, tt.subject(t, gb), "the directive is on the marked subject",
					assert.ByIdentity())
			})
		}

		t.Run("attaches the directive of a marker of package-info.java to the package", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{packageInfo: {Data: []byte(stubMarker + "\npackage com.acme;\n")}}
			gb, found := parsedTree(t, tree, packageInfo, plugin.DepthFull)
			assert.Empty(t, found, "the marker lifts")
			attached := gb.Attachments()
			assert.Length(t, attached, 1, "the marker attaches one directive")
			expect.Equal(t, attached[0].Subject, symbol.Symbol(packageIn(t, gb, pkgPath)),
				"the directive is on the package", assert.ByIdentity())
		})

		unaddressed := []struct {
			name string
			give string
		}{
			{
				name: "reports UnaddressedCarrier for a marker of a parameter",
				give: "public class A {\n  public void run(" + stubMarker + " int a) {}\n}\n",
			},
			{
				name: "reports UnaddressedCarrier for a marker of a type that an enum declares",
				give: "public enum E {\n  ONE;\n  " + stubMarker + "\n  class Inner {}\n}\n",
			},
		}
		for _, tt := range unaddressed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, pkgClause+tt.give)
				assert.Contains(t, codesOf(found), frontend.UnaddressedCarrier, "the marker attaches nowhere")
				expect.Empty(t, gb.Attachments(), "the marker attaches nothing")
			})
		}
	})

	t.Run("elementValue", func(t *testing.T) {
		t.Parallel()

		lifts := []struct {
			name string
			give string
			want []directive.RawArg
		}{
			{
				name: "lifts an element-value pair as a keyed argument", give: `(tag = "test")`,
				want: []directive.RawArg{{Key: "tag", Value: directive.RawValue{Text: "test", Quoted: true}}},
			},
			{
				name: "lifts the single element as a positional argument", give: `("writer")`,
				want: []directive.RawArg{{Value: directive.RawValue{Text: "writer", Quoted: true}}},
			},
			{
				name: "lifts an array of literals as a list", give: `(tags = {"a", 1})`,
				want: []directive.RawArg{{Key: "tags", Value: directive.RawValue{List: []directive.RawValue{
					{Text: "a", Quoted: true}, {Text: "1"},
				}}}},
			},
			{
				name: "lifts an empty array as an empty list", give: "(tags = {})",
				want: []directive.RawArg{{Key: "tags", Value: directive.RawValue{List: []directive.RawValue{}}}},
			},
			{
				name: "lifts true as written", give: "(ok = true)",
				want: []directive.RawArg{{Key: "ok", Value: directive.RawValue{Text: "true"}}},
			},
			{
				name: "lifts a number with a minus sign as negative decimal text", give: "(n = -2)",
				want: []directive.RawArg{{Key: "n", Value: directive.RawValue{Text: "-2"}}},
			},
			{
				name: "lifts a negated int whose bit pattern is negative as positive decimal text",
				give: "(n = -0xFFFFFFFF)",
				want: []directive.RawArg{{Key: "n", Value: directive.RawValue{Text: "1"}}},
			},
			{
				name: "lifts two pairs in order", give: `(tag = "a", n = 1)`,
				want: []directive.RawArg{
					{Key: "tag", Value: directive.RawValue{Text: "a", Quoted: true}},
					{Key: "n", Value: directive.RawValue{Text: "1"}},
				},
			},
		}
		for _, tt := range lifts {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, pkgClause+stubMarker+tt.give+"\npublic class A {}\n")
				assert.Empty(t, found, "the element values lift")
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
			{
				name: "reports BadMarker for an enum constant", give: "(mode = E.X)",
				want: "the element value E.X is not a literal or an array of literals",
			},
			{name: "reports BadMarker for null", give: "(n = null)", want: "the element value null is not a literal"},
			{
				name: "reports BadMarker for a character", give: "(c = 'c')",
				want: "the element value 'c' is not a literal",
			},
			{
				name: "reports BadMarker for a plus sign", give: "(n = +1)",
				want: "the element value +1 is not a literal",
			},
			{
				name: "reports BadMarker for a minus sign before a string", give: `(n = -"x")`,
				want: `the element value -"x" is not a literal`,
			},
			{
				name: "reports BadMarker for an array with a value that does not lift", give: `(tags = {"a", E.X})`,
				want: "the element value E.X is not a literal",
			},
			{
				name: "reports BadMarker for an int beyond 32 bits", give: "(n = 0x1FFFFFFFF)",
				want: "the element value 0x1FFFFFFFF is not a literal",
			},
			{
				name: "reports BadMarker for a long beyond 64 bits", give: "(n = 0x1FFFFFFFFFFFFFFFFL)",
				want: "the element value 0x1FFFFFFFFFFFFFFFFL is not a literal",
			},
			{
				name: "reports BadMarker for a float beyond the largest double", give: "(n = 1e400)",
				want: "the element value 1e400 is not a literal",
			},
			{
				name: "reports BadMarker for an escape that Java refuses", give: `(s = "\8")`,
				want: `the element value "\8" is not a literal`,
			},
			{
				name: "reports BadMarker for a lone surrogate", give: "(s = \"\\uD800\")",
				want: "is not a literal",
			},
			{
				name: "reports BadMarker for a text block", give: "(s = \"\"\"\n    x\n    \"\"\")",
				want: "is not a literal",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, pkgClause+stubMarker+tt.give+"\npublic class A {}\n")
				assert.Equal(t, codesOf(found), []diag.Code{frontend.BadMarker},
					"the frontend reports the marker under BadMarker")
				expect.Contains(t, found[0].Msg, tt.want, "the finding describes the element value")
				expect.Equal(t, found[0].Pos, position.Pos{File: srcFile, Line: markerLine, Col: 1},
					"the finding is at the annotation")
				expect.Empty(t, gb.Attachments(), "the marker attaches nothing")
			})
		}

		t.Run("reports BadMarker for an annotation of the brand alone", func(t *testing.T) {
			t.Parallel()

			gb, found := parsedSource(t, pkgClause+"@"+brand+"\npublic class A {}\n")
			expect.Equal(t, codesOf(found), []diag.Code{frontend.BadMarker}, "the brand alone is not a directive name")
			expect.Empty(t, gb.Attachments(), "the marker attaches nothing")
		})
	})

	t.Run("stringValue", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "decodes the escapes of one character", give: `"a\tb\"\\"`, want: "a\tb\"\\"},
			{name: "decodes the space escape", give: `"a\sb"`, want: "a b"},
			{name: "decodes an octal escape", give: `"\101"`, want: "A"},
			{name: "decodes a Unicode escape", give: "\"\\u0042\"", want: "B"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, pkgClause+stubMarker+"("+tt.give+")\npublic class A {}\n")
				assert.Empty(t, found, "the string lifts")
				attached := gb.Attachments()
				assert.Length(t, attached, 1, "the marker attaches one directive")
				assert.Equal(t, attached[0].Raw.Args[0].Value, directive.RawValue{Text: tt.want, Quoted: true},
					"the value is the decoded content")
			})
		}
	})

	t.Run("number", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "writes a decimal int as written", give: "42", want: "42"},
			{name: "writes a hexadecimal int as decimal text", give: "0x10", want: "16"},
			{name: "writes an octal int as decimal text", give: "017", want: "15"},
			{name: "writes a binary int as decimal text", give: "0b101", want: "5"},
			{name: "writes a long without its separators and its suffix", give: "1_000L", want: "1000"},
			{
				name: "writes a hexadecimal int of a negative bit pattern as negative text",
				give: "0xFFFFFFFF", want: "-1",
			},
			{
				name: "writes a hexadecimal long of a negative bit pattern as negative text",
				give: "0xFFFFFFFFFFFFFFFFL", want: "-1",
			},
			{name: "writes a float without its suffix as its shortest text", give: "1.5f", want: "1.5"},
			{name: "writes a double with an exponent as decimal text", give: "2.5e3", want: "2500"},
			{name: "writes a double with its suffix as decimal text", give: "1d", want: "1"},
			{name: "writes a hexadecimal float as decimal text", give: "0x1.8p1", want: "3"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gb, found := parsedSource(t, pkgClause+stubMarker+"(n = "+tt.give+")\npublic class A {}\n")
				assert.Empty(t, found, "the number lifts")
				attached := gb.Attachments()
				assert.Length(t, attached, 1, "the marker attaches one directive")
				assert.Equal(t, attached[0].Raw.Args[0].Value, directive.RawValue{Text: tt.want},
					"the value is the decimal text")
			})
		}
	})
}
