// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python

import (
	"errors"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// The marks of Python's string literals: the letters of a prefix, the
// letters of a raw literal, the two quotes, and the number of quotes of a
// triple-quoted literal.
const (
	prefixLetters = "rRuUbBfFtT"
	rawLetters    = "rR"
	singleQuote   = '\''
	doubleQuote   = '"'
	tripleQuotes  = 3
)

// notTextLetters are the letters of a prefix whose literal is no one
// string: bytes, an f-string and a t-string.
const notTextLetters = "bBfFtT"

// The characters that break a line of a literal: the line feed, and the
// carriage return, which Python reads as a line feed alone and before one.
// tab is the third character that Quote escapes with one letter.
const (
	lineFeed       = '\n'
	carriageReturn = '\r'
	tab            = '\t'
)

// The characters of Python's numeric literals: the decimal digits, the
// separator of digits, the characters of a float literal, the marks of
// which a float literal has at least one, the point and the exponent, and
// the signs, which no literal starts with.
const (
	decimalDigits = "0123456789"
	digitSep      = '_'
	floatLetters  = "0123456789_.eE+-"
	floatMarks    = ".eE"
	signs         = "+-"
)

// The alphabets of the digits of Python's integer literals after their
// prefix, each with the separator of digits, the letters of the prefixes of
// the hexadecimal, octal and binary literals, and the length of a prefix.
const (
	decimalAlphabet = "0123456789_"
	hexAlphabet     = "0123456789abcdefABCDEF_"
	octalAlphabet   = "01234567_"
	binaryAlphabet  = "01_"
	hexMarks        = "xX"
	octalMarks      = "oO"
	binaryMarks     = "bB"
	prefixLen       = 2
)

// The marks of Python's escapes.
const (
	// escapeMark opens an escape.
	escapeMark = '\\'
	// octalDigits is the most digits of an octal escape.
	octalDigits = 3
	// The letters that open an escape of a code point by two hexadecimal
	// digits, by four, by eight, and by its name.
	hexEscape   = 'x'
	shortEscape = 'u'
	longEscape  = 'U'
	nameEscape  = 'N'
	// The numbers of the hexadecimal digits of the three escapes.
	hexDigits   = 2
	shortDigits = 4
	longDigits  = 8
	// hexBase and octalBase are the bases of the digits of the escapes, and
	// decimalBase is the base of the text that ParseInt returns.
	hexBase     = 16
	octalBase   = 8
	decimalBase = 10
	// controlLimit is the first character that is not a control character,
	// and deleteChar is the control character after the printable ASCII.
	controlLimit = 0x20
	deleteChar   = 0x7f
	// maxShort is the largest code point that a four-digit escape writes.
	maxShort = 0xffff
)

// simpleEscapes maps the letter of each escape of one character to the
// character.
var simpleEscapes = map[byte]byte{
	'\\': '\\', '\'': '\'', '"': '"', 'a': '\a', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t',
	'v': '\v',
}

// escapeDigits maps the letter of each escape of a code point by its
// hexadecimal digits to the number of the digits.
var escapeDigits = map[byte]int{hexEscape: hexDigits, shortEscape: shortDigits, longEscape: longDigits}

// Unquote returns the value of a Python string literal as written, with its
// prefix and its quotes: a short or a triple-quoted literal in single or
// double quotes, raw or not. Unquote reads the text that the parser took as
// one string literal. It checks the prefix and the quotes, and not the body
// for an unescaped quote.
//
// A carriage return in the body reads as a line feed, alone and before a
// line feed, as Python reads the lines of a source file. An escape that
// Python does not define keeps its backslash, as Python reads it.
//
// Unquote reports false in these cases:
//
//   - a text whose prefix or quotes do not open and close a literal;
//   - bytes, an f-string and a t-string, which are no constant string;
//   - an escape of a character by its name, which Unquote does not decode;
//   - an escape of a code point whose digits do not read, which Python
//     refuses, and an escape of a surrogate, which UTF-8 cannot encode.
//
// # Allocation contract
//
// Unquote returns a raw literal and a literal without an escape and a
// carriage return as a part of text, without an allocation. Any other
// literal allocates its value once.
func Unquote(text string) (string, bool) {
	body := strings.TrimLeft(text, prefixLetters)
	prefix := text[:len(text)-len(body)]
	if len(prefix) > 1 || strings.ContainsAny(prefix, notTextLetters) || body == "" {
		return "", false
	}
	quote := body[0]
	if quote != singleQuote && quote != doubleQuote {
		return "", false
	}
	width := 1
	if len(body) >= tripleQuotes && body[1] == quote && body[2] == quote {
		width = tripleQuotes
	}
	if len(body) < 2*width || strings.TrimRight(body[len(body)-width:], string(quote)) != "" {
		return "", false
	}
	value := body[width : len(body)-width]
	raw := strings.ContainsAny(prefix, rawLetters)
	if strings.IndexByte(value, carriageReturn) < 0 && (raw || strings.IndexByte(value, escapeMark) < 0) {
		return value, true
	}
	return unescape(value, raw)
}

// ParseInt returns the decimal text of the value of a Python integer
// literal: a decimal literal, or a binary, an octal or a hexadecimal
// literal with its prefix, each with underscores between its digits. It
// reports false for a text that is no integer literal, such as a decimal
// literal with a leading zero that is not zero, which Python 3 refuses, and
// a text with a sign, which is no part of a literal.
//
// # Allocation contract
//
// A decimal literal without underscores returns text itself and allocates
// nothing. Any other literal allocates its decimal text once, and a literal
// whose value needs more than 64 bits also allocates the value. A text that
// is no integer literal allocates nothing, except a text of the digits of a
// literal with underscores that Python refuses, such as 1__0, which
// allocates the error of the parse.
func ParseInt(text string) (string, bool) {
	if text == "" || strings.IndexByte(decimalDigits, text[0]) < 0 {
		return "", false
	}
	if text[0] != '0' && strings.Trim(text, decimalDigits) == "" {
		return text, true
	}
	alphabet, digits := decimalAlphabet, text
	if len(text) > prefixLen && text[0] == '0' {
		switch {
		case strings.IndexByte(hexMarks, text[1]) >= 0:
			alphabet, digits = hexAlphabet, text[prefixLen:]
		case strings.IndexByte(octalMarks, text[1]) >= 0:
			alphabet, digits = octalAlphabet, text[prefixLen:]
		case strings.IndexByte(binaryMarks, text[1]) >= 0:
			alphabet, digits = binaryAlphabet, text[prefixLen:]
		}
	}
	if strings.Trim(digits, alphabet) != "" {
		return "", false
	}
	padded := len(text) > 1 && text[0] == '0' && (text[1] == digitSep || strings.IndexByte(decimalDigits, text[1]) >= 0)
	n, err := strconv.ParseUint(text, 0, 64)
	switch {
	case err == nil && padded && n != 0, err != nil && !errors.Is(err, strconv.ErrRange):
		return "", false
	case err == nil:
		return strconv.FormatUint(n, decimalBase), true
	}
	wide, read := new(big.Int).SetString(text, 0)
	if !read || padded {
		return "", false
	}
	return wide.String(), true
}

// ParseFloat returns the value of a Python float literal: decimal digits
// with underscores between them, a point, and an exponent, as 1_000.5,
// .5, 5. and 1e-3. It returns 0 and false for a text that is no float
// literal, such as an imaginary literal and a text with a sign, and for a
// literal whose value overflows a double, which Python reads as infinity.
// ParseFloat does not allocate.
func ParseFloat(text string) (float64, bool) {
	unsigned := text != "" && strings.IndexByte(signs, text[0]) < 0
	if !unsigned || strings.Trim(text, floatLetters) != "" || !strings.ContainsAny(text, floatMarks) {
		return 0, false
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// Quote returns the Python string literal of s as the formatter of ruff
// writes a string: in double quotes, or in single quotes where s contains a
// double quote and no single quote, because ruff prefers the quote that
// needs fewer escapes. It escapes the backslash, the quote, the line feed,
// the carriage return and the tab with one letter, every other control
// character with \x, and every other character that Unicode does not print
// with \u or \U. Each byte that is not UTF-8 takes \x, which Python reads as
// the code point of the byte.
//
// # Allocation contract
//
// Quote allocates the literal once. It measures the literal before it
// writes it.
func Quote(s string) string {
	quote := byte(doubleQuote)
	if strings.ContainsRune(s, doubleQuote) && !strings.ContainsRune(s, singleQuote) {
		quote = singleQuote
	}
	n := len(s) + 2
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if letter, digits := escapeOf(r, size, quote); letter != 0 {
			n += 2 + digits - size
		}
		i += size
	}
	var b strings.Builder
	b.Grow(n)
	b.WriteByte(quote)
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		letter, digits := escapeOf(r, size, quote)
		switch {
		case letter == 0:
			b.WriteString(s[i : i+size])
		case digits == 0:
			b.WriteByte(escapeMark)
			b.WriteByte(letter)
		case r == utf8.RuneError && size == 1:
			writeHex(&b, letter, rune(s[i]), digits)
		default:
			writeHex(&b, letter, r, digits)
		}
		i += size
	}
	b.WriteByte(quote)
	return b.String()
}

// escapeOf returns the escape that Quote writes for the rune r of size bytes
// of a string between the quotes quote: its letter and the number of
// hexadecimal digits after it. An escape of one character has no digits,
// and a character that Quote writes as it is has the zero letter. A byte
// that is not UTF-8 decodes as utf8.RuneError of one byte, and its escape is
// \x.
func escapeOf(r rune, size int, quote byte) (letter byte, digits int) {
	switch {
	case r == utf8.RuneError && size == 1:
		return hexEscape, hexDigits
	case r == escapeMark:
		return escapeMark, 0
	case r == rune(quote):
		return quote, 0
	case r == lineFeed:
		return 'n', 0
	case r == carriageReturn:
		return 'r', 0
	case r == tab:
		return 't', 0
	case r < controlLimit || r == deleteChar:
		return hexEscape, hexDigits
	case !unicode.IsPrint(r) && r <= maxShort:
		return shortEscape, shortDigits
	case !unicode.IsPrint(r):
		return longEscape, longDigits
	default:
		return 0, 0
	}
}

// unescape returns the value of the body of a literal with its line breaks
// read as line feeds, and, for a literal that is not raw, its escapes
// decoded. A backslash before a line break continues the line. unescape
// reports false for an escape of a character by its name, and for an escape
// of a code point whose digits do not read or that is a surrogate.
func unescape(body string, raw bool) (string, bool) {
	var b strings.Builder
	b.Grow(len(body))
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case c == carriageReturn:
			b.WriteByte(lineFeed)
			if i+1 < len(body) && body[i+1] == lineFeed {
				i++
			}
			continue
		case c != escapeMark || raw || i+1 == len(body):
			b.WriteByte(c)
			continue
		}
		i++
		next := body[i]
		if simple, known := simpleEscapes[next]; known {
			b.WriteByte(simple)
			continue
		}
		if digits, coded := escapeDigits[next]; coded {
			if i+digits >= len(body) {
				return "", false
			}
			n, err := strconv.ParseUint(body[i+1:i+1+digits], hexBase, 32)
			if err != nil || n > unicode.MaxRune || utf16.IsSurrogate(rune(n)) {
				return "", false
			}
			b.WriteRune(rune(n))
			i += digits
			continue
		}
		switch {
		case next == lineFeed:
		case next == carriageReturn:
			if i+1 < len(body) && body[i+1] == lineFeed {
				i++
			}
		case next == nameEscape:
			return "", false
		case next >= '0' && next <= '7':
			n := rune(0)
			end := i
			for ; end < len(body) && end < i+octalDigits && body[end] >= '0' && body[end] <= '7'; end++ {
				n = n*octalBase + rune(body[end]-'0')
			}
			b.WriteRune(n)
			i = end - 1
		default:
			b.WriteByte(escapeMark)
			b.WriteByte(next)
		}
	}
	return b.String(), true
}

// writeHex writes an escape that opens with a backslash and the letter,
// followed by the code point r in the digits hexadecimal digits.
func writeHex(b *strings.Builder, letter byte, r rune, digits int) {
	var buf [longDigits]byte
	text := strconv.AppendInt(buf[:0], int64(r), hexBase)
	b.WriteByte(escapeMark)
	b.WriteByte(letter)
	for range digits - len(text) {
		b.WriteByte('0')
	}
	b.Write(text)
}
