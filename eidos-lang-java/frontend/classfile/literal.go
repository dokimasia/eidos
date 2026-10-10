// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile

import (
	"math"
	"strconv"
	"strings"
)

// The spellings of a Java literal (JLS §3.10): the suffixes of a long
// and a float, the quotes of a string and a character, the boolean
// values, the fraction a double without one takes, and the backslash an
// escape opens with.
const (
	longSuffix   = "L"
	floatSuffix  = "f"
	stringQuote  = '"'
	charQuote    = '\''
	trueLiteral  = "true"
	falseLiteral = "false"
	zeroFraction = ".0"
	backslash    = '\\'
)

// The fields of the floating-point classes that name the values no Java
// literal spells.
const (
	floatNaN     = "Float.NaN"
	floatPosInf  = "Float.POSITIVE_INFINITY"
	floatNegInf  = "Float.NEGATIVE_INFINITY"
	doubleNaN    = "Double.NaN"
	doublePosInf = "Double.POSITIVE_INFINITY"
	doubleNegInf = "Double.NEGATIVE_INFINITY"
)

// The bit sizes strconv formats a float and a double at.
const (
	floatBits  = 32
	doubleBits = 64
)

// The range of printable ASCII, which a literal spells as itself, and a
// Unicode escape: its opening, its four hexadecimal digits, and the
// digits' shift and mask.
const (
	printableLow = 0x20
	printableTop = 0x7e
	unicodeOpen  = `\u`
	hexDigits    = "0123456789abcdef"
	unitDigits   = 4
	digitBits    = 4
	digitMask    = 0xf
)

// escapes maps each control character Java spells with a character
// escape to its escape (JLS §3.10.7).
var escapes = map[uint16]string{'\b': `\b`, '\t': `\t`, '\n': `\n`, '\f': `\f`, '\r': `\r`}

// intLiteral spells an int.
func intLiteral(v int32) string { return strconv.FormatInt(int64(v), 10) }

// longLiteral spells a long, with its suffix.
func longLiteral(v int64) string { return strconv.FormatInt(v, 10) + longSuffix }

// boolLiteral spells a boolean, which a class file stores as an int:
// false for zero, and true otherwise.
func boolLiteral(v int32) string {
	if v == 0 {
		return falseLiteral
	}
	return trueLiteral
}

// floatLiteral spells a float with its suffix, in the shortest form that
// reads back as the same value, and an infinity or NaN as the Float
// constant of that value.
func floatLiteral(v float32) string {
	switch f := float64(v); {
	case math.IsNaN(f):
		return floatNaN
	case math.IsInf(f, 1):
		return floatPosInf
	case math.IsInf(f, -1):
		return floatNegInf
	default:
		return strconv.FormatFloat(f, 'g', -1, floatBits) + floatSuffix
	}
}

// doubleLiteral spells a double in the shortest form that reads back as
// the same value, with a fraction where that form has neither a point
// nor an exponent, so the literal is no int's, and an infinity or NaN as
// the Double constant of that value.
func doubleLiteral(v float64) string {
	switch {
	case math.IsNaN(v):
		return doubleNaN
	case math.IsInf(v, 1):
		return doublePosInf
	case math.IsInf(v, -1):
		return doubleNegInf
	}
	s := strconv.FormatFloat(v, 'g', -1, doubleBits)
	if !strings.ContainsAny(s, ".e") {
		s += zeroFraction
	}
	return s
}

// charLiteral spells a char, which a class file stores as an int.
func charLiteral(v int32) string {
	var b strings.Builder
	b.WriteByte(charQuote)
	escape(&b, uint16(v), charQuote)
	b.WriteByte(charQuote)
	return b.String()
}

// stringLiteral spells a string's UTF-16 code units.
func stringLiteral(units []uint16) string {
	var b strings.Builder
	b.WriteByte(stringQuote)
	for _, u := range units {
		escape(&b, u, stringQuote)
	}
	b.WriteByte(stringQuote)
	return b.String()
}

// escape writes one code unit as a Java literal of a quote spells it: a
// control character's escape, the quote and the backslash behind a
// backslash, printable ASCII as itself, and any other unit as a Unicode
// escape, which spells each half of a surrogate pair.
func escape(b *strings.Builder, u uint16, quote byte) {
	switch esc, ok := escapes[u]; {
	case ok:
		b.WriteString(esc)
	case u == uint16(quote) || u == backslash:
		b.WriteByte(backslash)
		b.WriteByte(byte(u))
	case u >= printableLow && u <= printableTop:
		b.WriteByte(byte(u))
	default:
		b.WriteString(unicodeOpen)
		for d := unitDigits - 1; d >= 0; d-- {
			b.WriteByte(hexDigits[u>>(d*digitBits)&digitMask])
		}
	}
}
