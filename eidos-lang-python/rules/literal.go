// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strings"
	"unicode/utf8"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/spell"
)

// The keywords of the truth values of Python, and the texts of the truth
// values of the value tree.
const (
	trueKeyword  = "True"
	falseKeyword = "False"
	trueText     = "true"
	falseText    = "false"
)

// The marks of the literals of Python that the scanner reads: the signs
// that can open a number, the letters of the prefix of a string literal, the
// letters that make it a bytes literal or a raw literal, the longest prefix
// of a bytes literal, the quotes, the prefix of a raw literal, and the
// digits of zero, which has no sign.
const (
	minusSign     = "-"
	signs         = "+" + minusSign
	prefixLetters = "rRuUbBfFtT"
	bytesLetters  = "bB"
	rawLetters    = "rR"
	maxPrefix     = 2
	quotes        = `"'`
	rawPrefix     = "r"
	zeroDigits    = "0"
)

// literalKind is the class of one literal. The zero kind is no literal.
type literalKind uint8

const (
	// literalInt is an integer literal of any radix, with at most one sign.
	literalInt literalKind = iota + 1
	// literalFloat is a float literal, with at most one sign.
	literalFloat
	// literalString is a string literal, raw or not, in any quotes.
	literalString
	// literalBytes is a bytes literal.
	literalBytes
	// literalBool is True or False.
	literalBool
	// literalNone is None.
	literalNone
	// literalName is a dotted name of identifiers, such as Color.RED. A
	// name refers to a member of an enum or to a constant.
	literalName
)

// literal is one scanned literal of Python.
type literal struct {
	kind literalKind
	// text is the decimal digits of an integer, with the minus sign of a
	// negative one, the content of a string, a bytes literal as written,
	// the text of a truth value of the value tree, and a name as written.
	text string
	// number is the value of a float literal.
	number float64
}

// scanLiteral reads one literal of Python from text, and reports whether the
// text is one. White space may surround the literal. A literal is one of
// these:
//
//   - an integer literal of any radix, with underscores between its digits,
//     and a float literal, each with at most one sign;
//   - a string literal, raw or not, of any quotes, and a bytes literal of
//     ASCII characters;
//   - True, False and None;
//   - a dotted name of identifiers, such as Color.RED.
//
// A string with the prefix f or t is no constant, so it is no literal, and
// neither is any other expression, such as a call or a tuple.
//
// # Allocation contract
//
// scanLiteral allocates the digits of an integer literal that are not its
// text, such as the decimal digits of a hexadecimal literal and the minus
// sign of a negative integer, the content of a string with an escape, and
// the raw form that checks the quotes of a bytes literal. A truth value,
// None, a name and a string without an escape allocate nothing.
func scanLiteral(text string) (literal, bool) {
	text = strings.TrimSpace(text)
	sign := ""
	if text != "" && strings.IndexByte(signs, text[0]) >= 0 {
		sign, text = text[:1], strings.TrimSpace(text[1:])
	}
	if digits, isInt := python.ParseInt(text); isInt {
		if sign == minusSign && digits != zeroDigits {
			digits = minusSign + digits
		}
		return literal{kind: literalInt, text: digits}, true
	}
	if f, isFloat := python.ParseFloat(text); isFloat {
		if sign == minusSign {
			f = -f
		}
		return literal{kind: literalFloat, number: f}, true
	}
	switch {
	case sign != "":
		return literal{}, false
	case text == trueKeyword:
		return literal{kind: literalBool, text: trueText}, true
	case text == falseKeyword:
		return literal{kind: literalBool, text: falseText}, true
	case text == noneName:
		return literal{kind: literalNone}, true
	}
	if s, isString := python.Unquote(text); isString {
		return literal{kind: literalString, text: s}, true
	}
	body := strings.TrimLeft(text, prefixLetters)
	prefix := text[:len(text)-len(body)]
	if body != "" && strings.IndexByte(quotes, body[0]) >= 0 {
		valid := strings.ContainsAny(prefix, bytesLetters) && len(prefix) <= maxPrefix &&
			strings.Trim(prefix, rawLetters+bytesLetters) == "" &&
			strings.IndexAny(prefix, bytesLetters) == strings.LastIndexAny(prefix, bytesLetters)
		_, quoted := python.Unquote(rawPrefix + body)
		if valid && quoted && !strings.ContainsFunc(body, func(r rune) bool { return r >= utf8.RuneSelf }) {
			return literal{kind: literalBytes, text: text}, true
		}
		return literal{}, false
	}
	for segment := range strings.SplitSeq(text, python.ModuleSep) {
		if !spell.IsIdentifier(segment) {
			return literal{}, false
		}
	}
	return literal{kind: literalName, text: text}, true
}
