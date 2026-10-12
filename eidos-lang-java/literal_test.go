// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package java_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/java"
)

// The characters and code units that the literal cases spell.
const (
	acuteE        = "\U000000e9"
	grinning      = "\U0001f600"
	replacement   = "\U0000fffd"
	highSurrogate = 0xd83d
	lowSurrogate  = 0xde00
	plainText     = "row"
)

// The Unicode escapes that the cases expect.
const (
	nulEscape    = `\u0000`
	bellEscape   = `\u0007`
	deleteEscape = `\u007f`
	highEscape   = `\ud83d`
	lowEscape    = `\ude00`
)

// literalAllocs is the literal of a string or a char without an escape:
// its buffer, sized once.
const literalAllocs = 1

// TestLiteral checks the escapes of the three literal forms. javac reads
// each expected literal back as the string or the char that the case
// gives.
func TestLiteral(t *testing.T) {
	t.Parallel()

	t.Run("Quote", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "returns two quotes for the empty string", give: "", want: `""`},
			{name: "writes printable ASCII as itself", give: plainText, want: `"row"`},
			{name: "writes a backspace as its escape", give: "\b", want: `"\b"`},
			{name: "writes a tab as its escape", give: "\t", want: `"\t"`},
			{name: "writes a line feed as its escape", give: "\n", want: `"\n"`},
			{name: "writes a form feed as its escape", give: "\f", want: `"\f"`},
			{name: "writes a carriage return as its escape", give: "\r", want: `"\r"`},
			{name: "writes a double quote behind a backslash", give: `"`, want: `"\""`},
			{name: "writes a backslash behind a backslash", give: `\`, want: `"\\"`},
			{name: "writes a single quote as itself", give: "'", want: `"'"`},
			{name: "writes another control character as a Unicode escape", give: "\x07", want: `"` + bellEscape + `"`},
			{name: "writes the delete character as a Unicode escape", give: "\x7f", want: `"` + deleteEscape + `"`},
			{name: "writes a character outside ASCII as itself", give: acuteE, want: `"` + acuteE + `"`},
			{name: "writes a supplementary character as itself", give: grinning, want: `"` + grinning + `"`},
			{name: "writes an invalid byte as the replacement character", give: "\xff", want: `"` + replacement + `"`},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, java.Quote(tt.give), tt.want, "Quote returns the Java literal")
			})
		}
	})

	t.Run("QuoteUnits", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give []uint16
			want string
		}{
			{name: "writes printable ASCII as itself", give: []uint16{'o', 'k'}, want: `"ok"`},
			{name: "writes NUL as a Unicode escape", give: []uint16{0}, want: `"` + nulEscape + `"`},
			{name: "writes a unit outside ASCII as its character", give: []uint16{0xe9}, want: `"` + acuteE + `"`},
			{
				name: "writes a pair of surrogates as their character",
				give: []uint16{highSurrogate, lowSurrogate}, want: `"` + grinning + `"`,
			},
			{
				name: "writes a lone high surrogate as a Unicode escape",
				give: []uint16{highSurrogate}, want: `"` + highEscape + `"`,
			},
			{
				name: "writes a lone low surrogate as a Unicode escape",
				give: []uint16{lowSurrogate}, want: `"` + lowEscape + `"`,
			},
			{
				name: "writes a high surrogate before another unit as a Unicode escape",
				give: []uint16{highSurrogate, 'A'}, want: `"` + highEscape + `A"`,
			},
			{name: "writes a double quote behind a backslash", give: []uint16{'"'}, want: `"\""`},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, java.QuoteUnits(tt.give), tt.want, "QuoteUnits returns the Java literal")
			})
		}
	})

	t.Run("QuoteChar", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rune
			want string
		}{
			{name: "writes a printable character between single quotes", give: 'x', want: `'x'`},
			{name: "writes the code unit in the low 16 bits of a larger value", give: 0x10000 + 'x', want: `'x'`},
			{name: "writes a single quote behind a backslash", give: '\'', want: `'\''`},
			{name: "writes a double quote as itself", give: '"', want: `'"'`},
			{name: "writes a tab as its escape", give: '\t', want: `'\t'`},
			{name: "writes a character outside ASCII as itself", give: 0xe9, want: `'` + acuteE + `'`},
			{name: "writes a surrogate as a Unicode escape", give: highSurrogate, want: `'` + highEscape + `'`},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, java.QuoteChar(tt.give), tt.want, "QuoteChar returns the Java literal")
			})
		}
	})
}

// TestLiteralAllocs checks the ceilings of the three forms without an
// escape. The ordinary test run does not run benchmarks, so this test
// checks them.
func TestLiteralAllocs(t *testing.T) {
	var quotedText, unitsText, charText string
	units := []uint16{'o', 'k'}
	expect.MaxAllocs(t, func() { quotedText = java.Quote(plainText) }, literalAllocs,
		"Quote allocates its buffer")
	expect.MaxAllocs(t, func() { unitsText = java.QuoteUnits(units) }, literalAllocs,
		"QuoteUnits allocates its buffer")
	expect.MaxAllocs(t, func() { charText = java.QuoteChar('x') }, literalAllocs,
		"QuoteChar allocates its buffer")
	expect.Equal(t, quotedText, `"row"`, "Quote returns the literal")
	expect.Equal(t, unitsText, `"ok"`, "QuoteUnits returns the literal")
	expect.Equal(t, charText, `'x'`, "QuoteChar returns the literal")
}

// BenchmarkLiteral measures each form on text without an escape. The
// backend quotes every string value that it writes, and the class-file
// reader quotes every string and char constant.
func BenchmarkLiteral(b *testing.B) {
	b.Run("Quote", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(literalAllocs)
		defer c.End()
		var got string
		for c.Loop() {
			got = java.Quote(plainText)
		}
		assert.Equal(b, got, `"row"`, "Quote returns the literal")
	})

	b.Run("QuoteUnits", func(b *testing.B) {
		units := []uint16{'o', 'k'}
		c := bench.Start(b).MaxAllocs(literalAllocs)
		defer c.End()
		var got string
		for c.Loop() {
			got = java.QuoteUnits(units)
		}
		assert.Equal(b, got, `"ok"`, "QuoteUnits returns the literal")
	})

	b.Run("QuoteChar", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(literalAllocs)
		defer c.End()
		var got string
		for c.Loop() {
			got = java.QuoteChar('x')
		}
		assert.Equal(b, got, `'x'`, "QuoteChar returns the literal")
	})
}
