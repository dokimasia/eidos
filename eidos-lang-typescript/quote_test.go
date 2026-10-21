// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/prop"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The text that the allocation checks and the benchmarks quote, and the
// spellings that they requote: a type without a string, a union of
// literals in the double quote, and the same union in the single quote.
const (
	plainText      = "plain"
	unquotedType   = "Map<string, T>"
	doubleLiterals = `"a" | "b"`
	singleLiterals = `'a' | 'b'`
)

// quoteRoom is the capacity of the buffer that the allocation checks
// quote into.
const quoteRoom = 16

// requotedAllocs is the allocation of the spelling that Requote writes
// anew.
const requotedAllocs = 1

// quoteCharacters are the characters of the texts that the quote
// properties draw: a letter, both quotes, the backslash, a named escape,
// a control character without a name and a line separator.
const quoteCharacters = "a'\"\\\n\x01" + lineSeparator

// errNotLiteral is the error of the inverse of the quote property for a
// text that Unquote does not read as a literal.
var errNotLiteral = errors.New("not a literal")

// TestQuote checks the literal of each text that the backend writes, and
// each string literal of a type that it writes in the quote of a style.
func TestQuote(t *testing.T) {
	t.Parallel()

	t.Run("AppendQuote", func(t *testing.T) {
		t.Parallel()

		writes := []struct {
			name      string
			give      string
			giveStyle plugin.Choice
			want      string
		}{
			{
				name: "writes a text in the double quote under the double style", give: plainText,
				giveStyle: typescript.DoubleQuote, want: plainString,
			},
			{
				name: "writes a text in the single quote under the single style", give: plainText,
				giveStyle: typescript.SingleQuote, want: `'plain'`,
			},
			{
				name: "writes the double quote under a choice of another policy", give: plainText,
				giveStyle: typescript.BigInt, want: plainString,
			},
			{
				name: "writes the single quote for a text with more double quotes than single quotes",
				give: `say "hi"`, giveStyle: typescript.DoubleQuote, want: `'say "hi"'`,
			},
			{
				name: "writes the double quote for a text with more single quotes than double quotes",
				give: "it's", giveStyle: typescript.SingleQuote, want: `"it's"`,
			},
			{
				name: "writes the quote of the style for a text with as many quotes of each kind",
				give: `one ' and one "`, giveStyle: typescript.DoubleQuote, want: `"one ' and one \""`,
			},
			{
				name: "escapes the backslash", give: `a\b`, giveStyle: typescript.DoubleQuote, want: `"a\\b"`,
			},
			{
				name: "writes the named escapes", give: "\b\f\n\r\t\v", giveStyle: typescript.DoubleQuote,
				want: `"\b\f\n\r\t\v"`,
			},
			{
				name: "writes another control character as an escape of four hexadecimal digits",
				give: "\x01\x7f", giveStyle: typescript.DoubleQuote, want: `"\u0001\u007f"`,
			},
			{
				name: "writes a line separator as an escape of four hexadecimal digits",
				give: "a" + lineSeparator + "b", giveStyle: typescript.DoubleQuote, want: `"a\u2028b"`,
			},
			{
				name: "writes a character beyond the basic plane as itself", give: smile,
				giveStyle: typescript.DoubleQuote, want: `"` + smile + `"`,
			},
		}
		for _, tt := range writes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, string(typescript.AppendQuote(nil, tt.give, tt.giveStyle)), tt.want,
					"AppendQuote writes the literal")
			})
		}

		t.Run("appends the literal to the bytes of dst", func(t *testing.T) {
			t.Parallel()

			got := typescript.AppendQuote([]byte("x = "), plainText, typescript.DoubleQuote)
			assert.Equal(t, string(got), "x = "+plainString, "the literal follows the bytes of dst")
		})

		t.Run("returns a literal that Unquote reads as the text", func(t *testing.T) {
			t.Parallel()

			for _, style := range []plugin.Choice{typescript.DoubleQuote, typescript.SingleQuote} {
				prop.RoundTrip(t, func(text string) (string, error) {
					return string(typescript.AppendQuote(nil, text, style)), nil
				}, func(literal string) (string, error) {
					text, ok := typescript.Unquote(literal)
					if !ok {
						return "", errNotLiteral
					}
					return text, nil
				}, "Unquote reads the text back from its literal in the "+string(style)+" style",
					prop.Using(prop.String(prop.Alphabet(quoteCharacters))))
			}
		})
	})

	t.Run("Requote", func(t *testing.T) {
		t.Parallel()

		rewrites := []struct {
			name      string
			give      string
			giveStyle plugin.Choice
			want      string
		}{
			{
				name: "returns a spelling without a string literal as it is", give: unquotedType,
				giveStyle: typescript.SingleQuote, want: unquotedType,
			},
			{
				name: "returns a literal in the quote of the style as it is", give: doubleLiterals,
				giveStyle: typescript.DoubleQuote, want: doubleLiterals,
			},
			{
				name: "writes each literal of a union in the double quote", give: singleLiterals,
				giveStyle: typescript.DoubleQuote, want: doubleLiterals,
			},
			{
				name: "writes each literal of a union in the single quote", give: doubleLiterals,
				giveStyle: typescript.SingleQuote, want: singleLiterals,
			},
			{
				name: "keeps a literal whose content has more quotes of the style than of the other quote",
				give: `'say "hi"'`, giveStyle: typescript.DoubleQuote, want: `'say "hi"'`,
			},
			{
				name: "removes the backslash of a quote that the new quote does not escape",
				give: `'it\'s'`, giveStyle: typescript.DoubleQuote, want: `"it's"`,
			},
			{
				name: "escapes each quote that the new quote needs escaped",
				give: `"a 'b' \"c\" \"d\""`, giveStyle: typescript.SingleQuote, want: `'a \'b\' "c" "d"'`,
			},
			{
				name: "keeps every other escape of a literal", give: `'a\n\\b'`, giveStyle: typescript.DoubleQuote,
				want: `"a\n\\b"`,
			},
			{
				name: "keeps the text of a template literal", give: "`it's \"x\"`",
				giveStyle: typescript.SingleQuote, want: "`it's \"x\"`",
			},
			{
				name: "writes a literal inside a substitution of a template literal", give: "`x${'a' | 'b'}y`",
				giveStyle: typescript.DoubleQuote, want: "`x${\"a\" | \"b\"}y`",
			},
			{
				name: "writes a literal after a template literal", give: "`a` | 'b'",
				giveStyle: typescript.DoubleQuote, want: "`a` | \"b\"",
			},
			{
				name: "writes a literal of an object type inside a substitution", give: "`${{ a: 'x' }['a']}`",
				giveStyle: typescript.DoubleQuote, want: "`${{ a: \"x\" }[\"a\"]}`",
			},
			{
				name: "keeps a literal without its closing quote", give: "'a",
				giveStyle: typescript.DoubleQuote, want: "'a",
			},
			{
				name: "keeps an escaped quote of a template literal", give: "`a\\`b` | 'c'",
				giveStyle: typescript.DoubleQuote, want: "`a\\`b` | \"c\"",
			},
			{
				name: "keeps a template literal without its closing quote", give: "`a 'b'",
				giveStyle: typescript.DoubleQuote, want: "`a 'b'",
			},
		}
		for _, tt := range rewrites {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, typescript.Requote(tt.give, tt.giveStyle), tt.want, "Requote writes the spelling")
			})
		}

		t.Run("writes a literal of one style as AppendQuote writes it in the other style", func(t *testing.T) {
			t.Parallel()

			prop.ForAll(t, "a literal that Requote writes is the literal of the text in the new style",
				func(c *prop.Case) {
					text := c.Draw(prop.String(prop.Alphabet(quoteCharacters)), "text")
					single := string(typescript.AppendQuote(nil, text, typescript.SingleQuote))
					double := string(typescript.AppendQuote(nil, text, typescript.DoubleQuote))
					assert.Equal(c, typescript.Requote(single, typescript.DoubleQuote), double,
						"the literal in the double style")
					assert.Equal(c, typescript.Requote(double, typescript.SingleQuote), single,
						"the literal in the single style")
				})
		})
	})
}

// TestQuoteAllocs checks the allocation ceiling of each writer. The
// ordinary test run does not run a benchmark, so this test checks the
// ceilings.
func TestQuoteAllocs(t *testing.T) {
	buf := make([]byte, 0, quoteRoom)
	var s string
	expect.MaxAllocs(t, func() { buf = typescript.AppendQuote(buf[:0], plainText, typescript.DoubleQuote) }, 0,
		"AppendQuote into a buffer with room allocates nothing")
	expect.Equal(t, string(buf), plainString, "AppendQuote writes the literal")
	expect.MaxAllocs(t, func() { s = typescript.Requote(doubleLiterals, typescript.DoubleQuote) }, 0,
		"Requote of a spelling whose literals have their quote allocates nothing")
	expect.Equal(t, s, doubleLiterals, "Requote returns the spelling")
	expect.MaxAllocs(t, func() { s = typescript.Requote(singleLiterals, typescript.DoubleQuote) }, requotedAllocs,
		"Requote of a spelling with a literal in the other quote allocates the new spelling")
	expect.Equal(t, s, doubleLiterals, "Requote writes the literals in the double quote")
}

// BenchmarkQuote measures each writer at its ceiling, on a string value
// and on the union of literals of a type.
func BenchmarkQuote(b *testing.B) {
	b.Run("AppendQuote", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		buf := make([]byte, 0, quoteRoom)
		for c.Loop() {
			buf = typescript.AppendQuote(buf[:0], plainText, typescript.DoubleQuote)
		}
		assert.Equal(b, string(buf), plainString, "AppendQuote writes the literal")
	})

	b.Run("Requote", func(b *testing.B) {
		b.Run("a spelling whose literals have their quote", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var s string
			for c.Loop() {
				s = typescript.Requote(doubleLiterals, typescript.DoubleQuote)
			}
			assert.Equal(b, s, doubleLiterals, "Requote returns the spelling")
		})
		b.Run("a spelling with literals in the other quote", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(requotedAllocs)
			defer c.End()
			var s string
			for c.Loop() {
				s = typescript.Requote(singleLiterals, typescript.DoubleQuote)
			}
			assert.Equal(b, s, doubleLiterals, "Requote writes the literals in the double quote")
		})
	})
}
