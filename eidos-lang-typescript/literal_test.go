// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	typescript "go.dokimi.dev/eidos/lang/typescript"
)

// The literals that the allocation checks and the benchmarks decode.
const (
	plainString   = `"plain"`
	escapedString = `"a\nb"`
	decimalNumber = "1.5"
	spacedNumber  = "1_000"
	decimalBigInt = "42n"
	hexBigInt     = "0xFFn"
)

// The characters of the escape cases. smile is beyond the basic plane,
// and lineSeparator ends a line of TypeScript.
const (
	smile         = "\U0001F600"
	lineSeparator = "\U00002028"
)

// bigHex is the digits of a hexadecimal number beyond the largest
// double, which is below 16^256.
var bigHex = strings.Repeat("F", 257)

// The allocations of the literal readers above zero. escapedAllocs
// counts the content of a string with an escape, and spacedAllocs counts
// the digits of a number without its separators. hexAllocs counts the
// decimal text of a bigint in a radix other than ten. Its value is above
// 99, so strconv allocates the text.
const (
	escapedAllocs = 1
	spacedAllocs  = 1
	hexAllocs     = 1
)

// TestLiteral checks each literal form that TypeScript accepts, and each
// form that it refuses. The frontend and the rules both decode literals
// through these readers.
func TestLiteral(t *testing.T) {
	t.Parallel()

	t.Run("Unquote", func(t *testing.T) {
		t.Parallel()

		reads := []struct {
			name string
			give string
			want string
		}{
			{name: "returns the content of a string in double quotes", give: plainString, want: "plain"},
			{name: "returns the content of a string in single quotes", give: `'single'`, want: "single"},
			{name: "returns the content of a template literal", give: "`template`", want: "template"},
			{name: "returns the empty content of an empty string", give: `""`, want: ""},
			{name: "decodes the named escapes", give: `"\b\f\n\r\t\v"`, want: "\b\f\n\r\t\v"},
			{name: "decodes an escaped quote and backslash", give: `"\"\'\\"`, want: `"'\`},
			{
				name: `decodes \0 as the null character when no digit follows it`, give: `"a\0b"`,
				want: "a\x00b",
			},
			{
				name: `decodes \x with two hexadecimal digits as the code point`, give: `"\x41\xe9"`,
				want: "A\xc3\xa9",
			},
			{name: `decodes \u with four hexadecimal digits`, give: "\"\\u0041\"", want: "A"},
			{name: `decodes \u with a code point in braces`, give: `"\u{1F600}"`, want: smile},
			{
				name: "decodes a surrogate pair as the code point that it encodes", give: "\"\\uD83D\\uDE00\"",
				want: smile,
			},
			{name: "decodes any other escaped character as itself", give: `"\q\$"`, want: "q$"},
			{name: "decodes a line continuation as nothing", give: "\"a\\\nb\"", want: "ab"},
			{
				name: "decodes a line continuation of a carriage return and a line feed as nothing",
				give: "\"a\\\r\nb\"", want: "ab",
			},
			{
				name: "decodes a line continuation of a line separator as nothing",
				give: "\"a\\" + lineSeparator + "b\"", want: "ab",
			},
			{
				name: "turns a carriage return of a template literal into a line feed", give: "`a\rb`",
				want: "a\nb",
			},
			{
				name: "turns a carriage return and a line feed of a template literal into a line feed",
				give: "`a\r\nb`", want: "a\nb",
			},
			{
				name: "keeps a line separator in a string",
				give: "\"a" + lineSeparator + "b\"", want: "a" + lineSeparator + "b",
			},
		}
		for _, tt := range reads {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := typescript.Unquote(tt.give)
				assert.True(t, ok, "the literal is valid")
				assert.Equal(t, got, tt.want, "Unquote returns the content")
			})
		}

		refusals := []struct {
			name string
			give string
		}{
			{name: "reports false for text without quotes", give: "plain"},
			{name: "reports false for a lone quote", give: `"`},
			{name: "reports false for quotes that differ", give: `"plain'`},
			{name: "reports false for a string with an unescaped quote inside", give: `"a"b"`},
			{name: "reports false for a string with a line feed inside", give: "\"a\nb\""},
			{name: "reports false for a string with a carriage return inside", give: "\"a\rb\""},
			{name: "reports false for a template literal with a substitution", give: "`a${b}`"},
			{name: "reports false for a literal that ends in a backslash", give: `"\"`},
			{name: "reports false for a legacy octal escape", give: `"\01"`},
			{name: `reports false for the escape \8`, give: `"\8"`},
			{name: `reports false for \x with one hexadecimal digit`, give: `"\x4"`},
			{name: `reports false for \x with a letter that is not a digit`, give: `"\xzz"`},
			{name: `reports false for \u with three hexadecimal digits`, give: "\"\\u123\""},
			{name: `reports false for \u with a letter that is not a digit`, give: "\"\\u12zz\""},
			{name: `reports false for \u with empty braces`, give: `"\u{}"`},
			{name: `reports false for \u with braces beyond U+10FFFF`, give: `"\u{110000}"`},
			{name: `reports false for \u with braces around text that is not a number`, give: `"\u{zz}"`},
			{name: `reports false for \u without its closing brace`, give: `"\u{41"`},
			{name: "reports false for a lone high surrogate", give: "\"\\uD800\""},
			{name: "reports false for a lone low surrogate", give: "\"\\uDC00\""},
			{
				name: "reports false for a high surrogate without a low surrogate after it",
				give: "\"\\uD800\\u0041\"",
			},
			{name: "reports false for a high surrogate before another escape", give: "\"\\uD800\\n\""},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, ok := typescript.Unquote(tt.give)
				assert.False(t, ok, "the literal is not valid")
			})
		}
	})

	t.Run("ParseNumber", func(t *testing.T) {
		t.Parallel()

		reads := []struct {
			name string
			give string
			want float64
		}{
			{name: "returns 0", give: "0", want: 0},
			{name: "returns a decimal integer", give: "42", want: 42},
			{name: "returns a decimal fraction", give: decimalNumber, want: 1.5},
			{name: "returns a fraction without an integer part", give: ".5", want: 0.5},
			{name: "returns an integer part without a fraction", give: "5.", want: 5},
			{name: "returns a number with an exponent", give: "1e3", want: 1000},
			{name: "returns a number with a negative exponent", give: "1E-3", want: 0.001},
			{name: "returns a number with a positive exponent", give: "1e+3", want: 1000},
			{name: "returns a hexadecimal number", give: "0x1F", want: 31},
			{name: "returns a hexadecimal number in capitals", give: "0XFF", want: 255},
			{name: "returns an octal number", give: "0o17", want: 15},
			{name: "returns a binary number", give: "0b101", want: 5},
			{name: "returns a number without its separators", give: spacedNumber, want: 1000},
			{name: "returns a number with separators in its exponent", give: "1e1_0", want: 1e10},
			{
				name: "returns the nearest double of a hexadecimal number beyond 2^64",
				give: "0xFFFFFFFFFFFFFFFFFF", want: 4.722366482869645e21,
			},
		}
		for _, tt := range reads {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := typescript.ParseNumber(tt.give)
				assert.True(t, ok, "the literal is valid")
				assert.Equal(t, got, tt.want, "ParseNumber returns the value")
			})
		}

		refusals := []struct {
			name string
			give string
		}{
			{name: "reports false for the empty text", give: ""},
			{name: "reports false for a legacy octal literal", give: "017"},
			{name: "reports false for a decimal literal that starts with 0", give: "08.5"},
			{name: "reports false for a radix mark without digits", give: "0x"},
			{name: "reports false for a digit outside the radix", give: "0b2"},
			{name: "reports false for a separator after the radix mark", give: "0x_1"},
			{name: "reports false for two separators", give: "1__0"},
			{name: "reports false for a trailing separator", give: "1_"},
			{name: "reports false for a separator before the point", give: "1_.5"},
			{name: "reports false for a separator after the exponent's mark", give: "1e_5"},
			{name: "reports false for an exponent without digits", give: "1e"},
			{name: "reports false for an exponent of a sign alone", give: "1e+"},
			{name: "reports false for two points", give: "1.2.3"},
			{name: "reports false for a point alone", give: "."},
			{name: "reports false for a letter", give: "1a"},
			{name: "reports false for a bigint", give: "1n"},
			{name: "reports false for a number beyond the largest double", give: "1e400"},
			{name: "reports false for a hexadecimal number beyond the largest double", give: "0x" + bigHex},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, ok := typescript.ParseNumber(tt.give)
				assert.False(t, ok, "the literal is not valid")
			})
		}
	})

	t.Run("ParseBigInt", func(t *testing.T) {
		t.Parallel()

		reads := []struct {
			name string
			give string
			want string
		}{
			{name: "returns 0", give: "0n", want: "0"},
			{name: "returns a decimal bigint", give: decimalBigInt, want: "42"},
			{name: "returns a hexadecimal bigint as decimal text", give: hexBigInt, want: "255"},
			{
				name: "returns a hexadecimal bigint beyond 2^64 exactly", give: "0xFFFFFFFFFFFFFFFFFFn",
				want: "4722366482869645213695",
			},
			{name: "returns an octal bigint as decimal text", give: "0o17n", want: "15"},
			{name: "returns a binary bigint as decimal text", give: "0b101n", want: "5"},
			{name: "returns a bigint without its separators", give: "1_000n", want: "1000"},
			{
				name: "returns a bigint beyond 2^64 exactly", give: "123456789012345678901234567890n",
				want: "123456789012345678901234567890",
			},
		}
		for _, tt := range reads {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, ok := typescript.ParseBigInt(tt.give)
				assert.True(t, ok, "the literal is valid")
				assert.Equal(t, got, tt.want, "ParseBigInt returns the decimal digits")
			})
		}

		refusals := []struct {
			name string
			give string
		}{
			{name: "reports false for a number without its n", give: "42"},
			{name: "reports false for a fraction", give: "1.5n"},
			{name: "reports false for an exponent", give: "1e3n"},
			{name: "reports false for a bigint that starts with 0", give: "01n"},
			{name: "reports false for an n alone", give: "n"},
			{name: "reports false for a radix mark without digits", give: "0xn"},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, ok := typescript.ParseBigInt(tt.give)
				assert.False(t, ok, "the literal is not valid")
			})
		}
	})
}

// TestLiteralAllocs checks the allocation ceiling of each reader. A
// literal without an escape or a separator is decoded in place and
// allocates nothing. The ordinary test run runs no benchmark, so this
// test checks the ceilings.
func TestLiteralAllocs(t *testing.T) {
	var (
		s  string
		f  float64
		ok bool
	)
	expect.MaxAllocs(t, func() { s, ok = typescript.Unquote(plainString) }, 0,
		"Unquote of a string without an escape allocates nothing")
	expect.MaxAllocs(t, func() { s, ok = typescript.Unquote(escapedString) }, escapedAllocs,
		"Unquote of a string with an escape allocates the content")
	expect.MaxAllocs(t, func() { f, ok = typescript.ParseNumber(decimalNumber) }, 0,
		"ParseNumber of a decimal number allocates nothing")
	expect.MaxAllocs(t, func() { f, ok = typescript.ParseNumber(spacedNumber) }, spacedAllocs,
		"ParseNumber of a number with separators allocates its digits")
	expect.MaxAllocs(t, func() { s, ok = typescript.ParseBigInt(decimalBigInt) }, 0,
		"ParseBigInt of a decimal bigint allocates nothing")
	expect.MaxAllocs(t, func() { s, ok = typescript.ParseBigInt(hexBigInt) }, hexAllocs,
		"ParseBigInt of a hexadecimal bigint allocates its decimal text")
	expect.True(t, ok, "the last literal is valid")
	expect.Equal(t, s, "255", "ParseBigInt returns the decimal digits of the last bigint")
	expect.Equal(t, f, 1000.0, "ParseNumber returns the value of the last number")
}

// BenchmarkLiteral measures each reader at its ceiling, on the forms of a
// decorator's arguments and of a directive's literals.
func BenchmarkLiteral(b *testing.B) {
	b.Run("Unquote", func(b *testing.B) {
		b.Run("a plain string", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var s string
			for c.Loop() {
				s, _ = typescript.Unquote(plainString)
			}
			assert.Equal(b, s, "plain", "Unquote returns the content")
		})
		b.Run("a string with an escape", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(escapedAllocs)
			defer c.End()
			var s string
			for c.Loop() {
				s, _ = typescript.Unquote(escapedString)
			}
			assert.Equal(b, s, "a\nb", "Unquote decodes the escape")
		})
	})

	b.Run("ParseNumber", func(b *testing.B) {
		b.Run("a decimal number", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var f float64
			for c.Loop() {
				f, _ = typescript.ParseNumber(decimalNumber)
			}
			assert.Equal(b, f, 1.5, "ParseNumber returns the value")
		})
		b.Run("a number with separators", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(spacedAllocs)
			defer c.End()
			var f float64
			for c.Loop() {
				f, _ = typescript.ParseNumber(spacedNumber)
			}
			assert.Equal(b, f, 1000.0, "ParseNumber drops the separators")
		})
	})

	b.Run("ParseBigInt", func(b *testing.B) {
		b.Run("a decimal bigint", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var s string
			for c.Loop() {
				s, _ = typescript.ParseBigInt(decimalBigInt)
			}
			assert.Equal(b, s, "42", "ParseBigInt returns the digits")
		})
		b.Run("a hexadecimal bigint", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(hexAllocs)
			defer c.End()
			var s string
			for c.Loop() {
				s, _ = typescript.ParseBigInt(hexBigInt)
			}
			assert.Equal(b, s, "255", "ParseBigInt returns the decimal digits")
		})
	})
}
