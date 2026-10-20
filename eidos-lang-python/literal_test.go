// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/prop"

	"go.dokimi.dev/eidos/lang/python"
)

// The allocations of the literals: the value of a literal with an escape,
// the decimal text of an integer with a prefix, and the literal that Quote
// writes.
const (
	unescapeAllocs = 1
	parseIntAllocs = 1
	quoteAllocs    = 1
)

// The literals that the allocation test and the benchmark measure: a string
// without an escape, a string with one, a decimal integer, a hexadecimal
// integer above the constant texts of strconv, a float, and a text that
// Quote escapes.
const (
	plainLiteral   = `'abc'`
	escapedLiteral = `'a\tb'`
	decimalLiteral = "1234"
	hexLiteral     = "0xFF_FF"
	floatLiteral   = "1_000.5"
	escapedText    = "a\tb\x00\U0001f600"
)

// errNotLiteral is the error of the inverse of the round trip of Quote for a
// text that Unquote does not read as a string literal.
var errNotLiteral = errors.New("python: the text is no string literal")

// TestLiteral pins the values of the literals of Python as Python 3.15
// reads them, and the literals that Quote writes.
func TestLiteral(t *testing.T) {
	t.Parallel()

	t.Run("Unquote", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			give      string
			want      string
			wantValid bool
		}{
			{name: "returns the value of a literal in single quotes", give: `'abc'`, want: "abc", wantValid: true},
			{name: "returns the value of a literal in double quotes", give: `"abc"`, want: "abc", wantValid: true},
			{name: "returns the value of a triple-quoted literal", give: `'''a'b'''`, want: "a'b", wantValid: true},
			{name: "returns the empty value of an empty literal", give: `''`, want: "", wantValid: true},
			{name: "returns the empty value of an empty triple-quoted literal", give: `""""""`, wantValid: true},
			{name: "keeps the escapes of a raw literal", give: `r'\d'`, want: `\d`, wantValid: true},
			{
				name:      "keeps the escapes of a raw literal with a capital prefix",
				give:      `R"\n"`,
				want:      `\n`,
				wantValid: true,
			},
			{name: "reads the prefix u", give: `u'x'`, want: "x", wantValid: true},
			{name: "decodes an escape of one letter", give: `'a\nb'`, want: "a\nb", wantValid: true},
			{
				name:      "decodes the escapes of code points",
				give:      `'\x41\u00e9\U0001F600'`,
				want:      "A\U000000e9\U0001f600",
				wantValid: true,
			},
			{name: "decodes an octal escape", give: `'\101'`, want: "A", wantValid: true},
			{name: "decodes an octal escape above 255", give: `'\400'`, want: "\U00000100", wantValid: true},
			{
				name:      "keeps the backslash of an escape that Python does not define",
				give:      `'\q'`,
				want:      `\q`,
				wantValid: true,
			},
			{
				name:      "continues the line after a backslash before a line feed",
				give:      "'a\\\nb'",
				want:      "ab",
				wantValid: true,
			},
			{
				name:      "continues the line after a backslash before a CRLF",
				give:      "'a\\\r\nb'",
				want:      "ab",
				wantValid: true,
			},
			{name: "reads a CRLF as a line feed", give: "'''a\r\nb'''", want: "a\nb", wantValid: true},
			{name: "reads a carriage return alone as a line feed", give: "'''a\rb'''", want: "a\nb", wantValid: true},
			{
				name:      "reads a CRLF of a raw literal as a line feed",
				give:      "r'''a\r\nb'''",
				want:      "a\nb",
				wantValid: true,
			},
			{
				name:      "keeps the backslash before a line feed of a raw literal",
				give:      "r'a\\\nb'",
				want:      "a\\\nb",
				wantValid: true,
			},
			{name: "reports false for an escape by name", give: `'\N{EM DASH}'`},
			{name: "reports false for bytes", give: `b'x'`},
			{name: "reports false for an f-string", give: `f'x'`},
			{name: "reports false for a t-string", give: `t'x'`},
			{name: "reports false for raw bytes", give: `rb'x'`},
			{name: "reports false for the prefix ur of Python 2", give: `ur'x'`},
			{name: "reports false for a literal without its closing quote", give: `'abc`},
			{name: "reports false for a text without a quote", give: `abc`},
			{name: "reports false for a triple-quoted literal without its closing quotes", give: `'''''`},
			{name: "reports false for the empty text", give: ``},
			{name: "reports false for an escape whose digits end early", give: `'\x4'`},
			{name: "reports false for an escape whose digits are no hexadecimal number", give: `'\x4g'`},
			{name: "reports false for an escape above the last code point", give: `'\U00110000'`},
			{name: "reports false for an escape of a surrogate", give: `'\ud800'`},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, valid := python.Unquote(tt.give)
				assert.Equal(t, valid, tt.wantValid, "Unquote reports whether the text is a constant string literal")
				assert.Equal(t, got, tt.want, "Unquote returns the value that Python reads")
			})
		}

		t.Run("returns the text that Quote quotes", func(t *testing.T) {
			t.Parallel()

			prop.RoundTrip(t,
				func(s string) (string, error) { return python.Quote(s), nil },
				func(literal string) (string, error) {
					s, valid := python.Unquote(literal)
					if !valid {
						return "", errNotLiteral
					}
					return s, nil
				},
				"Unquote returns the text of the literal that Quote writes", prop.Using(prop.String()))
		})
	})

	t.Run("ParseInt", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			give      string
			want      string
			wantValid bool
		}{
			{name: "returns a decimal literal as it is", give: "123", want: "123", wantValid: true},
			{
				name:      "returns a decimal literal beyond 64 bits as it is",
				give:      "99999999999999999999999",
				want:      "99999999999999999999999",
				wantValid: true,
			},
			{name: "returns the value of a literal with underscores", give: "1_000", want: "1000", wantValid: true},
			{name: "returns the value of a hexadecimal literal", give: "0x1F", want: "31", wantValid: true},
			{
				name:      "returns the value of a hexadecimal literal with an underscore after its prefix",
				give:      "0X_1f",
				want:      "31",
				wantValid: true,
			},
			{
				name:      "returns the value of a hexadecimal literal beyond 64 bits",
				give:      "0x1_0000_0000_0000_0000",
				want:      "18446744073709551616",
				wantValid: true,
			},
			{name: "returns the value of an octal literal", give: "0o17", want: "15", wantValid: true},
			{name: "returns the value of a binary literal", give: "0b101", want: "5", wantValid: true},
			{name: "returns zero for zero", give: "0", want: "0", wantValid: true},
			{name: "returns zero for two zeros", give: "00", want: "0", wantValid: true},
			{name: "returns zero for zeros with an underscore", give: "0_0", want: "0", wantValid: true},
			{name: "reports false for a decimal literal with a leading zero", give: "007"},
			{name: "reports false for a literal beyond 64 bits with a leading zero", give: "0777777777777777777777777"},
			{name: "reports false for a zero before an underscore of a decimal literal", give: "0_7"},
			{name: "reports false for two underscores", give: "1__0"},
			{name: "reports false for a trailing underscore", give: "1_"},
			{name: "reports false for a minus sign", give: "-1"},
			{name: "reports false for a plus sign", give: "+1"},
			{name: "reports false for an imaginary literal", give: "1j"},
			{name: "reports false for a long integer of Python 2", give: "10L"},
			{name: "reports false for a prefix without digits", give: "0x"},
			{name: "reports false for a float literal", give: "1e3"},
			{name: "reports false for the empty text", give: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, valid := python.ParseInt(tt.give)
				assert.Equal(t, valid, tt.wantValid, "ParseInt reports whether the text is an integer literal")
				assert.Equal(t, got, tt.want, "ParseInt returns the decimal text of the value that Python reads")
			})
		}
	})

	t.Run("ParseFloat", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			give      string
			want      float64
			wantValid bool
		}{
			{name: "returns the value of a literal with a point", give: "1.5", want: 1.5, wantValid: true},
			{name: "returns the value of a literal with underscores", give: "1_000.5", want: 1000.5, wantValid: true},
			{name: "returns the value of a literal without an integer part", give: ".5", want: 0.5, wantValid: true},
			{name: "returns the value of a literal without a fraction", give: "5.", want: 5, wantValid: true},
			{
				name:      "returns the value of a literal with a negative exponent",
				give:      "1e-3",
				want:      0.001,
				wantValid: true,
			},
			{name: "returns the value of a literal with a signed exponent", give: "1E+3", want: 1000, wantValid: true},
			{name: "returns the value of a literal with leading zeros", give: "00.5", want: 0.5, wantValid: true},
			{
				name:      "returns the value of a literal with a leading zero before its exponent",
				give:      "01e3",
				want:      1000,
				wantValid: true,
			},
			{name: "returns zero for a literal below the smallest double", give: "1e-400", wantValid: true},
			{name: "reports false for an underscore before the exponent", give: "1_e3"},
			{name: "reports false for an imaginary literal", give: "1.5j"},
			{name: "reports false for a literal above the largest double", give: "1e400"},
			{name: "reports false for two underscores", give: "1__0.0"},
			{name: "reports false for a trailing underscore", give: "1.5_"},
			{name: "reports false for a minus sign", give: "-1.5"},
			{name: "reports false for an integer literal", give: "12"},
			{name: "reports false for the empty text", give: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, valid := python.ParseFloat(tt.give)
				assert.Equal(t, valid, tt.wantValid, "ParseFloat reports whether the text is a float literal")
				assert.Equal(t, got, tt.want, "ParseFloat returns the value that Python reads")
			})
		}
	})

	t.Run("Quote", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "writes a printable text as it is", give: "abc", want: `"abc"`},
			{name: "writes a text with a double quote in single quotes", give: `a"b`, want: `'a"b'`},
			{name: "writes a single quote in double quotes as it is", give: `a'b`, want: `"a'b"`},
			{name: "escapes the double quote of a text with both quotes", give: `a"b'c`, want: `"a\"b'c"`},
			{name: "escapes a backslash", give: `a\b`, want: `"a\\b"`},
			{name: "escapes a line feed with its letter", give: "a\nb", want: `"a\nb"`},
			{name: "escapes a carriage return with its letter", give: "a\rb", want: `"a\rb"`},
			{name: "escapes a tab with its letter", give: "a\tb", want: `"a\tb"`},
			{name: "escapes a control character with two digits", give: "\x00\x1f\x7f", want: `"\x00\x1f\x7f"`},
			{
				name: "writes a printable character beyond ASCII as it is",
				give: "\U000000e9\U0001f600",
				want: "\"\U000000e9\U0001f600\"",
			},
			{
				name: "escapes a character below U+10000 that Unicode does not print with four digits",
				give: "\U00000085\U000000a0\U0000200b",
				want: `"\u0085\u00a0\u200b"`,
			},
			{
				name: "escapes a character above U+FFFF that Unicode does not print with eight digits",
				give: "\U000e0001",
				want: `"\U000e0001"`,
			},
			{name: "escapes a byte that is not UTF-8 with two digits", give: "\xff", want: `"\xff"`},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, python.Quote(tt.give), tt.want, "the literal that ruff writes")
			})
		}
	})
}

// TestLiteralAllocs checks the ceilings of the literals in the ordinary run,
// which does not run benchmarks.
func TestLiteralAllocs(t *testing.T) {
	t.Run("Unquote", func(t *testing.T) {
		t.Run("does not allocate for a literal without an escape", func(t *testing.T) {
			got := ""
			assert.MaxAllocs(t, func() { got, _ = python.Unquote(plainLiteral) }, 0,
				"Unquote returns a part of the text")
			assert.Equal(t, got, "abc", "the value of the literal")
		})

		t.Run("allocates the value of a literal with an escape", func(t *testing.T) {
			got := ""
			assert.MaxAllocs(t, func() { got, _ = python.Unquote(escapedLiteral) }, unescapeAllocs,
				"Unquote allocates the decoded value")
			assert.Equal(t, got, "a\tb", "the value of the literal")
		})
	})

	t.Run("ParseInt", func(t *testing.T) {
		t.Run("does not allocate for a decimal literal", func(t *testing.T) {
			got := ""
			assert.MaxAllocs(t, func() { got, _ = python.ParseInt(decimalLiteral) }, 0,
				"ParseInt returns the text")
			assert.Equal(t, got, decimalLiteral, "the decimal text")
		})

		t.Run("allocates the value of a hexadecimal literal", func(t *testing.T) {
			got := ""
			assert.MaxAllocs(t, func() { got, _ = python.ParseInt(hexLiteral) }, parseIntAllocs,
				"ParseInt allocates the value and its text")
			assert.Equal(t, got, "65535", "the decimal text")
		})

		t.Run("does not allocate for a text that is no integer literal", func(t *testing.T) {
			valid := true
			assert.MaxAllocs(t, func() { _, valid = python.ParseInt(floatLiteral) }, 0,
				"ParseInt reads no text outside the alphabet of a literal")
			assert.False(t, valid, "a float literal is no integer literal")
		})
	})

	t.Run("ParseFloat", func(t *testing.T) {
		got := 0.0
		assert.MaxAllocs(t, func() { got, _ = python.ParseFloat(floatLiteral) }, 0, "ParseFloat does not allocate")
		assert.Equal(t, got, 1000.5, "the value of the literal")
	})

	t.Run("Quote", func(t *testing.T) {
		got := ""
		assert.MaxAllocs(t, func() { got = python.Quote(escapedText) }, quoteAllocs,
			"Quote allocates the literal once")
		assert.Equal(t, got, `"a\tb\x00`+"\U0001f600\"", "the literal")
	})
}

// BenchmarkLiteral measures the literals that the frontend reads for each
// marker and docstring, and the literal that the backend writes for each
// string.
func BenchmarkLiteral(b *testing.B) {
	b.Run("Unquote", func(b *testing.B) {
		for _, tt := range []struct {
			name   string
			give   string
			allocs uint64
		}{
			{name: "plain", give: plainLiteral},
			{name: "escaped", give: escapedLiteral, allocs: unescapeAllocs},
		} {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				valid := false
				for c.Loop() {
					_, valid = python.Unquote(tt.give)
				}
				assert.True(b, valid, "the literal reads")
			})
		}
	})

	b.Run("ParseInt", func(b *testing.B) {
		for _, tt := range []struct {
			name   string
			give   string
			allocs uint64
			want   bool
		}{
			{name: "decimal", give: decimalLiteral, want: true},
			{name: "hexadecimal", give: hexLiteral, allocs: parseIntAllocs, want: true},
			{name: "float", give: floatLiteral},
		} {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				valid := false
				for c.Loop() {
					_, valid = python.ParseInt(tt.give)
				}
				assert.Equal(b, valid, tt.want, "ParseInt reports whether the text is an integer literal")
			})
		}
	})

	b.Run("ParseFloat", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		valid := false
		for c.Loop() {
			_, valid = python.ParseFloat(floatLiteral)
		}
		assert.True(b, valid, "the literal reads")
	})

	b.Run("Quote", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(quoteAllocs)
		defer c.End()
		got := ""
		for c.Loop() {
			got = python.Quote(escapedText)
		}
		assert.NotEmpty(b, got, "the literal")
	})
}
