// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The marks of TypeScript's numeric literals.
const (
	// zeroDigit opens the prefix of a radix other than ten.
	zeroDigit = '0'
	// radixMarks are the letters that follow a leading 0 to open a
	// hexadecimal, an octal and a binary literal. hexRadix and octalRadix
	// are the first two in lower case.
	radixMarks = "xXoObB"
	hexRadix   = 'x'
	octalRadix = 'o'
	// caseBit is the bit that an ASCII letter sets in lower case.
	caseBit = 0x20
	// exponentMarks open the exponent of a decimal literal.
	exponentMarks = "eE"
	// bigintMark ends a bigint literal.
	bigintMark = "n"
	// separator separates the digits of a numeric literal.
	separator = '_'
	// point separates the integer part of a decimal literal from its
	// fraction.
	point = "."
	// signs are the two signs that can open an exponent.
	signs = "+-"
	// decimalDigits are the digits of a decimal literal.
	decimalDigits = "0123456789"
	// maxRadix is the largest radix that digitValue decodes, and the
	// value that it returns for a byte that is not a digit.
	maxRadix = 36
)

// The marks of TypeScript's string literals. quotes lists the three
// quotes, templateQuote is the quote of a template literal, and
// substitution opens a substitution in a template literal.
const (
	quotes        = "\"'`"
	templateQuote = '`'
	substitution  = "${"
)

// The marks of TypeScript's escapes.
const (
	// escapeMark opens an escape.
	escapeMark = '\\'
	// nulEscape is the digit whose escape is the NUL character.
	nulEscape = '0'
	// hexEscape opens an escape of two hexadecimal digits, and
	// unicodeMark opens an escape of a code point.
	hexEscape   = 'x'
	unicodeMark = 'u'
	// fixedDigits is the number of hexadecimal digits of a \u escape
	// without braces.
	fixedDigits = 4
	// braceOpen and braceClose enclose a code point of one to
	// maxBraceDigits hexadecimal digits.
	braceOpen      = '{'
	braceClose     = '}'
	maxBraceDigits = 6
)

// The line terminators of TypeScript. A backslash before any of them
// continues the line. A template literal turns a carriage return, and
// a carriage return before a line feed, into a line feed.
const (
	lineFeed           = '\n'
	carriageReturn     = '\r'
	crlf               = "\r\n"
	lineSeparator      = '\U00002028'
	paragraphSeparator = '\U00002029'
)

// The surrogates of UTF-16, which a TypeScript string can contain and a
// Go string cannot contain alone. A high surrogate opens a pair, and a
// low surrogate closes it.
const (
	highSurrogate = 0xD800
	lowSurrogate  = 0xDC00
	lastSurrogate = 0xDFFF
	supplementary = 0x10000
)

// escapes maps each named escape of one character to the character that
// it encodes.
var escapes = map[byte]byte{'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t', 'v': '\v'}

// Unquote returns the content of a string literal, or of a template
// literal without a substitution. The text includes the quotes. Unquote
// decodes the escapes as TypeScript does:
//
//   - a named escape, such as \n, as its character;
//   - \0 as the NUL character when no digit follows it;
//   - \x with two hexadecimal digits as their character;
//   - \u with four hexadecimal digits, or with a code point in braces, as
//     the code point;
//   - a surrogate pair of \u escapes as the one code point that it
//     encodes;
//   - a line continuation as nothing;
//   - any other escaped character as the character itself.
//
// In a template literal, a carriage return, and a carriage return before
// a line feed, become a line feed.
//
// Unquote reports false for text that is not a literal: a missing quote,
// a line break in a string literal, and a substitution in a template
// literal. It also reports false for a legacy octal escape, \8 and \9, a
// lone surrogate and a malformed escape, which TypeScript refuses.
//
// # Allocation contract
//
// A literal without an escape and without a carriage return returns a
// part of text and allocates nothing. Any other literal allocates its
// content once.
func Unquote(text string) (string, bool) {
	if len(text) < 2 || text[0] != text[len(text)-1] || strings.IndexByte(quotes, text[0]) < 0 {
		return "", false
	}
	quote := text[0]
	body := text[1 : len(text)-1]
	for i := 0; i < len(body); i++ {
		switch c := body[i]; {
		case c == escapeMark && strings.HasPrefix(body[i+1:], crlf):
			i += len(crlf)
		case c == escapeMark:
			i++
		case c == quote,
			quote == templateQuote && strings.HasPrefix(body[i:], substitution),
			quote != templateQuote && (c == lineFeed || c == carriageReturn):
			return "", false
		}
	}
	if strings.IndexByte(body, escapeMark) < 0 &&
		(quote != templateQuote || strings.IndexByte(body, carriageReturn) < 0) {
		return body, true
	}
	var b strings.Builder
	b.Grow(len(body))
	for i := 0; i < len(body); {
		c := body[i]
		switch {
		case c == carriageReturn:
			b.WriteByte(lineFeed)
			i++
			if i < len(body) && body[i] == lineFeed {
				i++
			}
		case c != escapeMark:
			b.WriteByte(c)
			i++
		default:
			n, ok := unescape(&b, body[i+1:])
			if !ok {
				return "", false
			}
			i += 1 + n
		}
	}
	return b.String(), true
}

// ParseNumber returns the value of a numeric literal, which is the
// double nearest to the literal's number. The literal is decimal,
// hexadecimal, octal or binary, and can have separators between its
// digits. ParseNumber reports false for text that is not a numeric
// literal of TypeScript, such as a legacy octal literal, and for a
// literal beyond the largest double. A sign is not part of a literal.
//
// # Allocation contract
//
// A literal without separators allocates nothing, and a literal with
// separators allocates its digits. A literal in a radix other than ten
// beyond 2^64 also allocates its exact value.
func ParseNumber(text string) (float64, bool) {
	base, digits, valid := numeral(text, false)
	if !valid {
		return 0, false
	}
	if base == 10 {
		f, err := strconv.ParseFloat(digits, 64)
		return f, err == nil
	}
	if u, err := strconv.ParseUint(digits, base, 64); err == nil {
		return float64(u), true
	}
	n, _ := new(big.Int).SetString(digits, base)
	f, _ := new(big.Float).SetInt(n).Float64()
	return f, !math.IsInf(f, 0)
}

// ParseBigInt returns the decimal digits of the value of a bigint
// literal. The text includes the n. A bigint literal is an integer in any
// radix followed by n, and ParseBigInt reports false for text that is
// not one. A decimal bigint starts with a digit other than 0 unless it is
// 0, so its digits are its decimal text already. A sign is not part of a
// literal.
//
// # Allocation contract
//
// A decimal literal without separators returns a part of text and
// allocates nothing. A decimal literal with separators allocates its
// digits. A literal in another radix allocates its decimal text, and a
// literal beyond 2^64 also allocates its exact value.
func ParseBigInt(text string) (string, bool) {
	body, marked := strings.CutSuffix(text, bigintMark)
	if !marked {
		return "", false
	}
	base, digits, valid := numeral(body, true)
	if !valid {
		return "", false
	}
	if base == 10 {
		return digits, true
	}
	if u, err := strconv.ParseUint(digits, base, 64); err == nil {
		return strconv.FormatUint(u, 10), true
	}
	n, _ := new(big.Int).SetString(digits, base)
	return n.String(), true
}

// numeral splits a numeric literal without a bigint's n into its radix
// and its digits without separators. It reports whether the text is a
// literal of TypeScript. A separator is valid only between two digits of
// the radix. A bigint is an integer, so a decimal bigint has no point and
// no exponent. The digits are a part of the text, or a copy without the
// separators when the text has one.
func numeral(text string, bigint bool) (int, string, bool) {
	base, body := 10, text
	if len(text) > 2 && text[0] == zeroDigit && strings.IndexByte(radixMarks, text[1]) >= 0 {
		switch text[1] | caseBit {
		case hexRadix:
			base = 16
		case octalRadix:
			base = 8
		default:
			base = 2
		}
		body = text[2:]
	}
	for i := range len(body) {
		if body[i] == separator &&
			(i == 0 || i == len(body)-1 || digitValue(body[i-1]) >= base || digitValue(body[i+1]) >= base) {
			return 0, "", false
		}
	}
	digits := body
	if strings.IndexByte(body, separator) >= 0 {
		digits = strings.ReplaceAll(body, string(separator), "")
	}
	if base != 10 {
		for i := range len(digits) {
			if digitValue(digits[i]) >= base {
				return 0, "", false
			}
		}
		return base, digits, digits != ""
	}
	return base, digits, decimal(digits, bigint)
}

// decimal reports whether digits without separators are a decimal
// literal. A decimal literal has an integer part, a fraction after a
// point and an exponent after e, and it needs the integer part or the
// fraction. An integer part of more than one digit starts with a digit
// other than 0, because TypeScript refuses the legacy octal form. A
// bigint has only the integer part.
func decimal(digits string, bigint bool) bool {
	mantissa, exponent, scaled := digits, "", false
	if i := strings.IndexAny(digits, exponentMarks); i >= 0 {
		mantissa, exponent, scaled = digits[:i], digits[i+1:], true
	}
	whole, fraction, pointed := strings.Cut(mantissa, point)
	switch {
	case bigint && (pointed || scaled),
		strings.Trim(whole, decimalDigits) != "" || strings.Trim(fraction, decimalDigits) != "",
		whole == "" && fraction == "",
		len(whole) > 1 && whole[0] == zeroDigit:
		return false
	case scaled:
		if exponent != "" && strings.IndexByte(signs, exponent[0]) >= 0 {
			exponent = exponent[1:]
		}
		return exponent != "" && strings.Trim(exponent, decimalDigits) == ""
	default:
		return true
	}
}

// unescape writes the character of the escape at the start of s, which
// is the text after the backslash. It returns the number of bytes of s
// in the escape.
func unescape(b *strings.Builder, s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	c := s[0]
	switch {
	case c == nulEscape && (len(s) == 1 || digitValue(s[1]) >= 10):
		b.WriteByte(0)
		return 1, true
	case digitValue(c) < 10:
		return 0, false
	case c == carriageReturn && len(s) > 1 && s[1] == lineFeed:
		return len(crlf), true
	case c == carriageReturn || c == lineFeed:
		return 1, true
	case c == hexEscape:
		if len(s) < 3 || digitValue(s[1]) >= 16 || digitValue(s[2]) >= 16 {
			return 0, false
		}
		v, _ := strconv.ParseUint(s[1:3], 16, 8)
		b.WriteRune(rune(v))
		return 3, true
	case c == unicodeMark:
		return unicodeEscape(b, s)
	}
	if written, named := escapes[c]; named {
		b.WriteByte(written)
		return 1, true
	}
	r, size := utf8.DecodeRuneInString(s)
	if r != lineSeparator && r != paragraphSeparator {
		b.WriteRune(r)
	}
	return size, true
}

// unicodeEscape writes the code point of the \u escape at the start of
// s, which starts with the u. It returns the number of bytes of s in the
// escape. A high surrogate needs a \u escape of a low surrogate after it,
// and unicodeEscape writes the one code point that the pair encodes.
func unicodeEscape(b *strings.Builder, s string) (int, bool) {
	r, n, ok := codePoint(s)
	switch {
	case !ok, r >= lowSurrogate && r <= lastSurrogate:
		return 0, false
	case r >= highSurrogate && r < lowSurrogate:
		rest, escaped := strings.CutPrefix(s[n:], string(escapeMark))
		low, m, paired := codePoint(rest)
		if !escaped || !paired || low < lowSurrogate || low > lastSurrogate {
			return 0, false
		}
		r = supplementary + (r-highSurrogate)<<10 + (low - lowSurrogate)
		n += 1 + m
	}
	b.WriteRune(r)
	return n, true
}

// codePoint decodes the code point of the \u escape at the start of s,
// which starts with the u. The escape has four hexadecimal digits, or
// one to six digits in braces up to U+10FFFF. codePoint returns the code
// point and the number of bytes of s in the escape.
func codePoint(s string) (rune, int, bool) {
	if len(s) < 2 || s[0] != unicodeMark {
		return 0, 0, false
	}
	if s[1] == braceOpen {
		end := strings.IndexByte(s, braceClose)
		if end < 3 || end > 2+maxBraceDigits {
			return 0, 0, false
		}
		v, err := strconv.ParseUint(s[2:end], 16, 32)
		if err != nil || v > unicode.MaxRune {
			return 0, 0, false
		}
		return rune(v), end + 1, true
	}
	if len(s) < 1+fixedDigits {
		return 0, 0, false
	}
	v, err := strconv.ParseUint(s[1:1+fixedDigits], 16, 32)
	if err != nil {
		return 0, 0, false
	}
	return rune(v), 1 + fixedDigits, true
}

// digitValue returns the value of an ASCII digit in a radix up to 36,
// with letters of either case. It returns maxRadix for any other byte.
func digitValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c|caseBit >= 'a' && c|caseBit <= 'z':
		return int(c|caseBit-'a') + 10
	default:
		return maxRadix
	}
}
