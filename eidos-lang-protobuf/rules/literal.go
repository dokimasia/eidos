// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go/constant"
	"go/token"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/rules"
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

// The widths in bits a float's text is written at.
const (
	float32Width = 32
	float64Width = 64
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
	// text is a truth value's spelling, a string's content, or an
	// identifier.
	text string
	// number is a number's exact value, and integer reports whether
	// it is written as an integer.
	number  constant.Value
	integer bool
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
func unquote(text string) (string, bool) {
	var b strings.Builder
	rest := strings.TrimSpace(text)
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

// intValue returns an integral value inside an integer type's range
// as decimal text at the type's width. A width of 0 bounds as 64
// bits.
func intValue(value constant.Value, class rules.ScalarClass, bits int) (emit.Value, bool) {
	n := constant.ToInt(value)
	if n.Kind() != constant.Int {
		return emit.Value{}, false
	}
	width := bits
	if width == 0 {
		width = float64Width
	}
	one := constant.MakeInt64(1)
	var lo, hi constant.Value
	if class == rules.ScalarUint {
		lo = constant.MakeInt64(0)
		hi = constant.BinaryOp(constant.Shift(one, token.SHL, uint(width)), token.SUB, one)
	} else {
		half := constant.Shift(one, token.SHL, uint(width-1))
		lo = constant.UnaryOp(token.SUB, half, 0)
		hi = constant.BinaryOp(half, token.SUB, one)
	}
	if constant.Compare(n, token.LSS, lo) || constant.Compare(n, token.GTR, hi) {
		return emit.Value{}, false
	}
	return emit.Number(emit.LiteralInt, n.ExactString(), bits), true
}

// floatValue returns a finite value inside a float type's range as
// decimal text at the type's precision. A width of 0 reads at 64
// bits and states no width.
func floatValue(value constant.Value, bits int) (emit.Value, bool) {
	x := constant.ToFloat(value)
	if x.Kind() != constant.Float {
		return emit.Value{}, false
	}
	width := float64Width
	f, _ := constant.Float64Val(x)
	if bits == float32Width {
		width = float32Width
		narrow, _ := constant.Float32Val(x)
		f = float64(narrow)
	}
	if math.IsInf(f, 0) {
		return emit.Value{}, false
	}
	return emit.Number(emit.LiteralFloat, decimal(f, width), bits), true
}

// decimal writes a float as the shortest decimal text that reads back
// to it at a precision: positional notation from 1e-6 up to 1e21,
// exponent notation outside it with the exponent unpadded, the rule
// encoding/json writes floats by, cutoffs compared at the same
// precision.
func decimal(f float64, bits int) string {
	format := byte('f')
	if abs := math.Abs(f); abs != 0 {
		narrow := float32(abs)
		if bits == float64Width && (abs < 1e-6 || abs >= 1e21) ||
			bits == float32Width && (narrow < 1e-6 || narrow >= 1e21) {
			format = 'e'
		}
	}
	b := strconv.AppendFloat(nil, f, format, -1, bits)
	if n := len(b); format == 'e' && n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
		b[n-2] = b[n-1]
		b = b[:n-1]
	}
	return string(b)
}
