// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package java

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// The quotes of a string literal and a char literal, and the backslash
// that opens an escape (JLS §3.10.5, §3.10.4, §3.10.7).
const (
	stringQuote = '"'
	charQuote   = '\''
	backslash   = '\\'
)

// A Unicode escape opens with unicodeOpen and writes the 16 bits of a
// code unit as four hexadecimal digits of four bits each. unitMask keeps
// the 16 bits of a code unit.
const (
	unicodeOpen = `\u`
	hexDigits   = "0123456789abcdef"
	unitBits    = 16
	unitMask    = 0xffff
	digitBits   = 4
	digitMask   = 0xf
)

// firstPrintable is the first printable character of ASCII, and
// deleteChar is the one control character above it.
const (
	firstPrintable = ' '
	deleteChar     = 0x7f
)

// Quote returns s as a Java string literal. It writes each character as
// the satellite's literals write it:
//
//   - a backspace, a tab, a line feed, a form feed and a carriage return
//     as their character escapes;
//   - the double quote and the backslash behind a backslash;
//   - every other control character and the delete character as a
//     Unicode escape;
//   - every other character as itself.
//
// Java decodes a Unicode escape before it reads the literal, and a
// decoded line feed or carriage return ends the literal. Quote therefore
// writes both as character escapes. An invalid byte of s becomes the
// replacement character.
//
// # Allocation contract
//
// Quote sizes its buffer for s and the two quotes, one allocation, and an
// escape grows the buffer once more.
func Quote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte(stringQuote)
	for _, r := range s {
		escape(&b, r, stringQuote)
	}
	b.WriteByte(stringQuote)
	return b.String()
}

// QuoteUnits returns the UTF-16 code units of a string as a Java string
// literal, which a class file stores. A pair of surrogates is its
// character, and Quote's rules apply to it. A lone surrogate has no
// character, so QuoteUnits writes its Unicode escape.
//
// # Allocation contract
//
// QuoteUnits sizes its buffer for one byte per unit and the two quotes,
// one allocation, and an escape or a character outside ASCII grows the
// buffer once more.
func QuoteUnits(units []uint16) string {
	var b strings.Builder
	b.Grow(len(units) + 2)
	b.WriteByte(stringQuote)
	for i := 0; i < len(units); i++ {
		r := rune(units[i])
		if i+1 < len(units) {
			if pair := utf16.DecodeRune(r, rune(units[i+1])); pair != utf8.RuneError {
				r = pair
				i++
			}
		}
		escape(&b, r, stringQuote)
	}
	b.WriteByte(stringQuote)
	return b.String()
}

// QuoteChar returns a Java char literal of the UTF-16 code unit in the
// low 16 bits of r, which is the unit that a char holds. A class file
// stores a char constant as an int. The single quote is behind a
// backslash, the double quote is itself, and Quote's rules apply to every
// other unit. A surrogate is written as its Unicode escape. QuoteChar
// allocates the literal, one allocation.
func QuoteChar(r rune) string {
	var b strings.Builder
	b.Grow(len(unicodeOpen) + unitBits/digitBits + 2)
	b.WriteByte(charQuote)
	escape(&b, r&unitMask, charQuote)
	b.WriteByte(charQuote)
	return b.String()
}

// escape writes one character of a literal that quote encloses, by the
// rules of [Quote]. A surrogate has no character, so escape writes its
// Unicode escape.
func escape(b *strings.Builder, r rune, quote byte) {
	switch {
	case r == '\b':
		b.WriteString(`\b`)
	case r == '\t':
		b.WriteString(`\t`)
	case r == '\n':
		b.WriteString(`\n`)
	case r == '\f':
		b.WriteString(`\f`)
	case r == '\r':
		b.WriteString(`\r`)
	case r == rune(quote) || r == backslash:
		b.WriteByte(backslash)
		b.WriteRune(r)
	case r < firstPrintable || r == deleteChar || utf16.IsSurrogate(r):
		b.WriteString(unicodeOpen)
		for shift := unitBits - digitBits; shift >= 0; shift -= digitBits {
			b.WriteByte(hexDigits[r>>shift&digitMask])
		}
	default:
		b.WriteRune(r)
	}
}
