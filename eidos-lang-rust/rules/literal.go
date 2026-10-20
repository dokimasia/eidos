// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"errors"
	"go/constant"
	"go/token"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/lang/naming"
)

// The keywords of the truth values of Rust, which are also the texts of the
// truth values of the value tree.
const (
	trueText  = "true"
	falseText = "false"
)

// The marks of the literals of Rust that the scanner reads.
const (
	// minusSign negates a number.
	minusSign = "-"
	// digitSep separates the digits of a number, as in 1_000.
	digitSep = "_"
	// The quotes of a character literal and of a string literal.
	charQuote   = '\''
	stringQuote = '"'
	// The prefixes of a raw string, a byte string and a raw byte string,
	// and the mark that a raw string repeats around its quotes.
	rawPrefix      = "r"
	bytesPrefix    = "b"
	rawBytesPrefix = "br"
	rawHash        = "#"
	// escapeMark opens an escape.
	escapeMark = '\\'
	// The parentheses of the variant Some that a literal of an option
	// states, as Some(1).
	someOpen  = someVariant + "("
	someClose = ")"
)

// The prefixes of the integer literals of another radix, with their bases.
const (
	hexPrefix    = "0x"
	octalPrefix  = "0o"
	binaryPrefix = "0b"
	prefixLen    = 2
	hexBase      = 16
	octalBase    = 8
	binaryBase   = 2
	decimalBase  = 10
)

// The widths of the floats of Rust, which a float literal reads at.
const (
	singleBits = 32
	doubleBits = 64
)

// The escapes of a string literal of Rust beyond the escapes of one
// character: the escape of an ASCII character by two hexadecimal digits,
// the most that the escape of a string takes, and the escape of a code
// point by up to six hexadecimal digits in braces.
const (
	hexEscape     = 'x'
	maxASCII      = 0x7f
	unicodeEscape = 'u'
	unicodeOpen   = '{'
	unicodeClose  = '}'
	maxUnicode    = 6
)

// floatSuffixes are the type suffixes of a float literal. A literal of
// another radix takes no float suffix, because f is a hexadecimal digit.
var floatSuffixes = []string{"f32", "f64"}

// intSuffixes are the type suffixes of an integer literal, the longest of a
// common prefix first, so u128 does not read as u1 followed by 28.
var intSuffixes = []string{
	"isize", "usize", "i128", "u128", "i16", "i32", "i64", "u16", "u32", "u64", "i8", "u8",
}

// simpleEscapes maps the letter of each escape of one character of Rust to
// the character.
var simpleEscapes = map[byte]byte{
	'n': '\n', 'r': '\r', 't': '\t', '\\': '\\', '0': 0, '\'': '\'', '"': '"',
}

// literalKind is the class of one literal. The zero kind is no literal.
type literalKind uint8

const (
	// literalInt is an integer literal of any radix, with at most one minus
	// sign.
	literalInt literalKind = iota + 1
	// literalFloat is a float literal, with at most one minus sign.
	literalFloat
	// literalString is a string literal or a raw string literal.
	literalString
	// literalBytes is a byte string literal or a raw byte string literal.
	literalBytes
	// literalChar is a character literal.
	literalChar
	// literalBool is true or false.
	literalBool
	// literalUnit is the unit value ().
	literalUnit
	// literalPath is a path of identifiers, such as Color::Red or MAX. A
	// path refers to a variant, a constant or a static.
	literalPath
	// literalSome is the variant Some of an option with the text of its
	// value.
	literalSome
)

// literal is one scanned literal of Rust.
type literal struct {
	// text is the content of a string, a character literal and a byte string
	// as written, the text of a truth value, a path as written, the text of
	// the value of Some, and the digits of a number without its sign, its
	// prefix, its underscores and its suffix.
	text string
	// suffix is the type suffix of a number, such as u8, and empty for a
	// number without one.
	suffix string
	// base is the base of the digits of an integer.
	base int
	kind literalKind
	// negative marks a number with a minus sign.
	negative bool
}

// scanLiteral reads one literal of Rust from text, and reports whether the
// text is one. White space may surround the literal. A literal is one of
// these:
//
//   - an integer literal of any radix and a float literal, each with
//     underscores between its digits, an optional type suffix and at most
//     one minus sign;
//   - a character literal, a string literal, a raw string literal, a byte
//     string literal and a raw byte string literal;
//   - true, false and the unit value ();
//   - a path of identifiers, such as Color::Red;
//   - Some of the text of a value, as Some(1).
//
// Any other expression is no literal, such as a call or a tuple.
//
// # Allocation contract
//
// scanLiteral allocates the digits of a number with underscores, and the
// content of a string with an escape. A number without underscores, a truth
// value, a path, a raw string and a string without an escape allocate
// nothing.
func scanLiteral(text string) (literal, bool) {
	text = strings.TrimSpace(text)
	switch text {
	case trueText, falseText:
		return literal{kind: literalBool, text: text}, true
	case unitType:
		return literal{kind: literalUnit}, true
	}
	if inner, some := strings.CutPrefix(text, someOpen); some {
		body, closed := strings.CutSuffix(inner, someClose)
		return literal{kind: literalSome, text: body}, closed
	}
	negative := false
	if rest, signed := strings.CutPrefix(text, minusSign); signed {
		negative, text = true, strings.TrimSpace(rest)
	}
	if text != "" && text[0] >= '0' && text[0] <= '9' {
		return scanNumber(text, negative)
	}
	if negative || text == "" {
		return literal{}, false
	}
	switch {
	case text[0] == charQuote:
		return literal{kind: literalChar, text: text}, validChar(text)
	case text[0] == stringQuote:
		s, ok := unquote(text)
		return literal{kind: literalString, text: s}, ok
	case strings.HasPrefix(text, rawBytesPrefix+string(stringQuote)) || strings.HasPrefix(text, rawBytesPrefix+rawHash):
		_, ok := rawContent(text[len(rawBytesPrefix):])
		return literal{kind: literalBytes, text: text}, ok && ascii(text)
	case strings.HasPrefix(text, bytesPrefix+string(stringQuote)):
		return literal{kind: literalBytes, text: text}, validBytes(text[len(bytesPrefix):]) && ascii(text)
	case strings.HasPrefix(text, rawPrefix+string(stringQuote)) || strings.HasPrefix(text, rawPrefix+rawHash):
		s, ok := rawContent(text[len(rawPrefix):])
		return literal{kind: literalString, text: s}, ok
	}
	for segment := range strings.SplitSeq(text, pathSep) {
		if !naming.IsIdentifier(segment) {
			return literal{}, false
		}
	}
	return literal{kind: literalPath, text: text}, true
}

// scanNumber reads an integer or a float literal of Rust that starts with a
// digit, negated where negative is set. An integer of another radix has its
// prefix and takes an integer suffix alone. A decimal literal is a float
// where it has a point, an exponent or a float suffix. A point needs a digit
// after it where an exponent follows, as Rust reads 1.e5 as a field access.
// The digits of an integer read when the literal takes its value, through
// [literal.integer].
func scanNumber(text string, negative bool) (literal, bool) {
	digits := strings.ReplaceAll(text, digitSep, "")
	lit := literal{kind: literalInt, base: decimalBase, negative: negative}
	if len(digits) > prefixLen {
		switch digits[:prefixLen] {
		case hexPrefix:
			lit.base = hexBase
		case octalPrefix:
			lit.base = octalBase
		case binaryPrefix:
			lit.base = binaryBase
		}
	}
	for _, s := range intSuffixes {
		if body, cut := strings.CutSuffix(digits, s); cut && body != "" {
			digits, lit.suffix = body, s
			break
		}
	}
	if lit.suffix == "" && lit.base == decimalBase {
		for _, s := range floatSuffixes {
			if body, cut := strings.CutSuffix(digits, s); cut && body != "" {
				digits, lit.suffix = body, s
				break
			}
		}
	}
	lit.text = digits
	if lit.base != decimalBase {
		lit.text = digits[prefixLen:]
	}
	if lit.base == decimalBase && (strings.ContainsAny(digits, ".eE") || lit.suffix == floatSuffixes[0] ||
		lit.suffix == floatSuffixes[1]) {

		lit.kind = literalFloat
		return lit, validFloat(digits)
	}
	return lit, true
}

// integer returns the value of the integer literal lit, with its sign: a
// machine integer where the digits fit 64 bits, and an integer of exact
// arithmetic otherwise. It reports false for digits that are no digits of
// the base of lit.
//
// # Allocation contract
//
// A value of at most 64 bits allocates the box of the value, and nothing for
// a value below 256. A wider value allocates the integer of exact arithmetic.
func (lit literal) integer() (constant.Value, bool) {
	n, err := strconv.ParseUint(lit.text, lit.base, 64)
	var v constant.Value
	switch {
	case err == nil:
		v = constant.MakeUint64(n)
	case errors.Is(err, strconv.ErrRange):
		wide, read := new(big.Int).SetString(lit.text, lit.base)
		if !read {
			return nil, false
		}
		v = constant.Make(wide)
	default:
		return nil, false
	}
	if lit.negative {
		v = constant.UnaryOp(token.SUB, v, 0)
	}
	return v, true
}

// float returns the value of the float literal or the decimal integer
// literal lit, with its sign, at the precision of a float of bits, a single
// precision for 32 bits and a double precision otherwise. It reports false
// for an integer of another radix and for a value beyond the range of the
// precision. It allocates nothing.
func (lit literal) float(bits int) (float64, bool) {
	precision := doubleBits
	if bits == singleBits {
		precision = singleBits
	}
	if lit.base != decimalBase {
		return 0, false
	}
	f, err := strconv.ParseFloat(lit.text, precision)
	if err != nil {
		return 0, false
	}
	if lit.negative {
		f = -f
	}
	return f, true
}

// validFloat reports whether digits, a float literal without underscores and
// without a suffix that starts with a digit, has the shape of a decimal float
// of Rust: digits, then a point with optional digits, then an optional
// exponent with an optional sign and digits. A point needs a digit after it
// where an exponent follows. It allocates nothing.
func validFloat(digits string) bool {
	i := 0
	for i < len(digits) && digits[i] >= '0' && digits[i] <= '9' {
		i++
	}
	if i < len(digits) && digits[i] == '.' {
		i++
		start := i
		for i < len(digits) && digits[i] >= '0' && digits[i] <= '9' {
			i++
		}
		if i < len(digits) && i == start {
			return false
		}
	}
	if i < len(digits) && (digits[i] == 'e' || digits[i] == 'E') {
		i++
		if i < len(digits) && (digits[i] == '+' || digits[i] == '-') {
			i++
		}
		start := i
		for i < len(digits) && digits[i] >= '0' && digits[i] <= '9' {
			i++
		}
		if i == start {
			return false
		}
	}
	return i == len(digits)
}

// validChar reports whether text is a character literal of Rust: one
// character or one escape between single quotes. It allocates the content of
// an escape.
func validChar(text string) bool {
	if len(text) < 3 || text[len(text)-1] != charQuote {
		return false
	}
	body := text[1 : len(text)-1]
	if body[0] != escapeMark {
		r, size := utf8.DecodeRuneInString(body)
		return r != utf8.RuneError && size == len(body) && r != charQuote
	}
	s, ok := unescape(body)
	return ok && utf8.RuneCountInString(s) == 1
}

// unquote returns the content of a string literal of Rust between double
// quotes, with its escapes decoded, and reports false for a text that is no
// one literal. A literal without an escape returns a part of text.
func unquote(text string) (string, bool) {
	if len(text) < 2 || text[0] != stringQuote || text[len(text)-1] != stringQuote {
		return "", false
	}
	body := text[1 : len(text)-1]
	if strings.IndexByte(body, escapeMark) < 0 {
		return body, strings.IndexByte(body, stringQuote) < 0
	}
	return unescape(body)
}

// unescape decodes the escapes of the body of a string literal of Rust: the
// escapes of one character, \x of an ASCII character, \u{...} of a code point
// of up to six hexadecimal digits, and a backslash before a line break, which
// skips the break and the white space after it. It reports false for another
// escape, for an escape whose digits do not read, and for a double quote
// without an escape.
func unescape(body string) (string, bool) {
	var b strings.Builder
	b.Grow(len(body))
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case c == stringQuote:
			return "", false
		case c != escapeMark:
			b.WriteByte(c)
			continue
		case i+1 == len(body):
			return "", false
		}
		i++
		next := body[i]
		if simple, known := simpleEscapes[next]; known {
			b.WriteByte(simple)
			continue
		}
		switch next {
		case hexEscape:
			if i+2 >= len(body) {
				return "", false
			}
			n, err := strconv.ParseUint(body[i+1:i+3], hexBase, 8)
			if err != nil || n > maxASCII {
				return "", false
			}
			b.WriteByte(byte(n))
			i += 2
		case unicodeEscape:
			end := strings.IndexByte(body[i:], unicodeClose)
			if end < 2 || body[i+1] != unicodeOpen {
				return "", false
			}
			hex := strings.ReplaceAll(body[i+2:i+end], digitSep, "")
			n, err := strconv.ParseUint(hex, hexBase, 32)
			if err != nil || len(hex) > maxUnicode || n > unicode.MaxRune {
				return "", false
			}
			if r := rune(n); utf8.ValidRune(r) {
				b.WriteRune(r)
				i += end
				continue
			}
			return "", false
		case '\n':
			for i+1 < len(body) && strings.IndexByte(" \t\n\r", body[i+1]) >= 0 {
				i++
			}
		default:
			return "", false
		}
	}
	return b.String(), true
}

// rawContent returns the content of a raw string literal of Rust after its
// prefix r: the hashes, the opening quote, the content, the closing quote and
// as many hashes. A quote inside the content that as many hashes follow ends
// the literal early, so rawContent reports false for it, as for any other
// text that is no one raw literal. It allocates nothing.
func rawContent(text string) (string, bool) {
	hashes := len(text) - len(strings.TrimLeft(text, rawHash))
	body := text[hashes:]
	if len(body) < 2+hashes || body[0] != stringQuote {
		return "", false
	}
	end := len(body) - hashes - 1
	if body[end] != stringQuote || strings.TrimLeft(body[end+1:], rawHash) != "" {
		return "", false
	}
	content := body[1:end]
	for i := range len(content) {
		if content[i] == stringQuote && len(content)-i-1 >= hashes &&
			strings.TrimLeft(content[i+1:i+1+hashes], rawHash) == "" {

			return "", false
		}
	}
	return content, true
}

// validBytes reports whether text is the quoted part of a byte string
// literal: a double quote, characters and escapes without a double quote
// that no backslash escapes, and a closing double quote. It allocates
// nothing.
func validBytes(text string) bool {
	if len(text) < 2 || text[0] != stringQuote || text[len(text)-1] != stringQuote {
		return false
	}
	body := text[1 : len(text)-1]
	for i := 0; i < len(body); i++ {
		switch {
		case body[i] == escapeMark && i+1 == len(body):
			return false
		case body[i] == escapeMark:
			i++
		case body[i] == stringQuote:
			return false
		}
	}
	return true
}

// quoteChar returns the character literal of Rust of r: the backslash, the
// single quote and the named escapes \n \r \t \0 escaped, every other
// control character as a braced Unicode escape, and every other character as
// itself. It allocates the literal.
func quoteChar(r rune) string {
	var b strings.Builder
	b.WriteByte(charQuote)
	switch {
	case r == escapeMark || r == charQuote:
		b.WriteByte(escapeMark)
		b.WriteRune(r)
	case r == '\n':
		b.WriteString(`\n`)
	case r == '\r':
		b.WriteString(`\r`)
	case r == '\t':
		b.WriteString(`\t`)
	case r == 0:
		b.WriteString(`\0`)
	case r < ' ' || r == maxASCII:
		var digits [maxUnicode]byte
		b.WriteByte(escapeMark)
		b.WriteByte(unicodeEscape)
		b.WriteByte(unicodeOpen)
		b.Write(strconv.AppendInt(digits[:0], int64(r), hexBase))
		b.WriteByte(unicodeClose)
	default:
		b.WriteRune(r)
	}
	b.WriteByte(charQuote)
	return b.String()
}

// ascii reports whether text contains only ASCII characters, as the content
// of a byte string does. It allocates nothing.
func ascii(text string) bool {
	for i := range len(text) {
		if text[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
