// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go/constant"
	"go/token"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// literalKind names the literal form a text is written in.
type literalKind uint8

const (
	// literalBool is true or false.
	literalBool literalKind = iota + 1
	// literalString is one quoted string or several adjacent ones.
	literalString
	// literalNumber is an integer or a float behind at most one
	// minus sign.
	literalNumber
	// literalIdent is an identifier, which names an enum value.
	literalIdent
)

// The spellings protobuf's number grammar reads.
const (
	minusSign = "-"
	hexLower  = "0x"
	hexUpper  = "0X"
	// floatMarks are the characters that make a number a float.
	floatMarks = ".eE"
)

// literal is one scanned protobuf literal.
type literal struct {
	kind literalKind
	// integer reports whether a number is written as an integer.
	integer bool
	// text is a truth value's spelling, a string's content, or an
	// identifier.
	text string
	// number is a number's exact value.
	number constant.Value
}

// scanLiteral reads one literal in protoc's grammar: true or false, a
// string, an identifier, or a number. Text that is none of these is
// no literal.
func scanLiteral(text string) (literal, bool) {
	switch {
	case text == "":
		return literal{}, false
	case text == sampleTrue || text == sampleFalse:
		return literal{kind: literalBool, text: text}, true
	case text[0] == '"' || text[0] == '\'':
		s, ok := unquote(text)
		return literal{kind: literalString, text: s}, ok
	case isIdentifier(text):
		return literal{kind: literalIdent, text: text}, true
	}
	number, integer, ok := scanNumber(text)
	return literal{kind: literalNumber, number: number, integer: integer}, ok
}

// scanNumber reads a number behind at most one minus sign: a
// hexadecimal integer after 0x, an octal integer after a leading
// zero, a decimal integer, or a decimal float with a point or an
// exponent. It returns the exact value and whether it is an integer.
// protobuf writes no binary, no 0o prefix and no digit separator.
func scanNumber(text string) (constant.Value, bool, bool) {
	body, negative := strings.CutPrefix(text, minusSign)
	body = strings.TrimSpace(body)
	if body == "" || strings.ContainsRune(body, '_') || !isDigit(body[0]) && body[0] != '.' {
		return nil, false, false
	}
	var value constant.Value
	integer := true
	switch {
	case strings.HasPrefix(body, hexLower) || strings.HasPrefix(body, hexUpper):
		u, err := strconv.ParseUint(body[len(hexLower):], 16, 64)
		if err != nil {
			return nil, false, false
		}
		value = constant.MakeUint64(u)
	case strings.ContainsAny(body, floatMarks):
		value, integer = constant.MakeFromLiteral(body, token.FLOAT, 0), false
	case len(body) > 1 && body[0] == '0':
		u, err := strconv.ParseUint(body, 8, 64)
		if err != nil {
			return nil, false, false
		}
		value = constant.MakeUint64(u)
	default:
		value = constant.MakeFromLiteral(body, token.INT, 0)
	}
	if value.Kind() == constant.Unknown {
		return nil, false, false
	}
	if negative {
		value = constant.UnaryOp(token.SUB, value, 0)
	}
	return value, integer, true
}

// unquote reads one string literal, or several adjacent ones, which
// protobuf concatenates, in protoc's escape grammar: the named
// escapes \a \b \f \n \r \t \v \\ \' \" \?, one to three octal
// digits, one or two hexadecimal digits after \x, and a code point
// in four hexadecimal digits after \u or eight after \U. A literal
// ends at its own quote and never spans a line.
//
// Every escape is at least as long as the bytes it writes, so unquote
// sizes its builder to the text inside the outer quotes and allocates
// the content once.
func unquote(text string) (string, bool) {
	rest := strings.TrimSpace(text)
	var b strings.Builder
	b.Grow(max(len(rest)-len(`""`), 0))
	for rest != "" {
		quote := rest[0]
		if quote != '"' && quote != '\'' {
			return "", false
		}
		i, closed := 1, false
		for i < len(rest) && !closed {
			c := rest[i]
			switch c {
			case quote:
				closed = true
				i++
			case '\n', 0:
				return "", false
			case '\\':
				n, ok := unescape(&b, rest[i+1:])
				if !ok {
					return "", false
				}
				i += 1 + n
			default:
				b.WriteByte(c)
				i++
			}
		}
		if !closed {
			return "", false
		}
		rest = strings.TrimSpace(rest[i:])
	}
	return b.String(), true
}

// simpleEscapes maps each one-character escape onto the byte it
// writes.
var simpleEscapes = map[byte]byte{
	'a': '\a', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t', 'v': '\v',
	'\\': '\\', '\'': '\'', '"': '"', '?': '?',
}

// unescape writes the value of the escape s opens, s being the text
// after the backslash, and returns how many bytes of s the escape
// takes.
func unescape(b *strings.Builder, s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	c := s[0]
	switch {
	case c == 'x' || c == 'X':
		n := 0
		for n < 2 && 1+n < len(s) && isHex(s[1+n]) {
			n++
		}
		if n == 0 {
			return 0, false
		}
		v, _ := strconv.ParseUint(s[1:1+n], 16, 8)
		b.WriteByte(byte(v))
		return 1 + n, true
	case c >= '0' && c <= '7':
		n := 1
		for n < 3 && n < len(s) && s[n] >= '0' && s[n] <= '7' {
			n++
		}
		v, _ := strconv.ParseUint(s[:n], 8, 16)
		if v > math.MaxUint8 {
			return 0, false
		}
		b.WriteByte(byte(v))
		return n, true
	case c == 'u' || c == 'U':
		width := 4
		if c == 'U' {
			width = 8
		}
		if len(s) < 1+width {
			return 0, false
		}
		v, err := strconv.ParseUint(s[1:1+width], 16, 32)
		if err != nil || v > utf8.MaxRune {
			return 0, false
		}
		b.WriteRune(rune(v))
		return 1 + width, true
	}
	if written, simple := simpleEscapes[c]; simple {
		b.WriteByte(written)
		return 1, true
	}
	return 0, false
}

// isIdentifier reports whether a text is a bare proto identifier,
// which an enum value's name is.
func isIdentifier(text string) bool {
	for i, r := range text {
		letter := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '_'
		digit := r >= '0' && r <= '9'
		if !letter && (!digit || i == 0) {
			return false
		}
	}
	return text != ""
}

// isDigit reports whether a byte is a decimal digit.
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// isHex reports whether a byte is a hexadecimal digit.
func isHex(c byte) bool {
	return isDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
