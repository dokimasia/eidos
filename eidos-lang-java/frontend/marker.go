// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"

	"go.dokimi.dev/eidos/lang/numeric"
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The parts of a number literal of Java that a marker's argument lifts:
// the minus sign before it, the separator between its digits, the bit
// that an ASCII letter sets in lower case, the type suffixes of a long, a
// float and a double, and the widths of a float and a double.
const (
	minusSign    = "-"
	separator    = "_"
	caseBit      = 0x20
	longSuffix   = 'l'
	floatSuffix  = 'f'
	doubleSuffix = 'd'
	floatBits    = 32
	doubleBits   = 64
)

// The parts of an escape of Java's string literals: the mark that opens
// a Unicode escape, and the width of its hexadecimal digits.
const (
	unicodeMark   = "u"
	unicodeDigits = 4
)

// escapes maps the character after the backslash of each escape of one
// character to the character that the escape encodes.
var escapes = map[byte]string{
	'b': "\b", 's': " ", 't': "\t", 'n': "\n", 'f': "\f", 'r': "\r", '"': `"`, '\'': "'", '\\': `\`,
}

// marker returns the marker of an annotation of the brand. Its path is
// the annotation's name split at its dots, and its arguments are the
// annotation's element values, lifted from the syntax by
// [lowering.elementValue]. An element-value pair lifts as a keyed
// argument, and the single element of the single-element form as a
// positional one. The marker's Refusal describes the first element value
// that does not lift.
func (l *lowering) marker(n treesitter.Node, name string) plugin.Sugar {
	s := plugin.Sugar{Path: strings.Split(name, nameSeparator), Pos: n.Pos()}
	for _, arg := range l.children(n.Child(l.v.fieldArguments)) {
		key, value := "", arg
		if arg.Kind() == l.v.elementValuePair {
			key, value = arg.Child(l.v.fieldKey).Text(), arg.Child(l.v.fieldValue)
		}
		v, refusal := l.elementValue(value)
		if refusal != "" {
			s.Refusal = refusal
			return s
		}
		s.Args = append(s.Args, directive.RawArg{Key: key, Value: v})
	}
	return s
}

// elementValue lifts one element value of a marker into a directive
// value:
//
//   - a string literal to its content, as [lowering.stringValue] decodes
//     it;
//   - a number literal, with at most one minus sign, to its decimal text,
//     as [lowering.number] writes it;
//   - true and false as written;
//   - an array of element values to the list of their values.
//
// elementValue returns the reason that a value does not lift, or the
// empty string when it lifts.
func (l *lowering) elementValue(n treesitter.Node) (directive.RawValue, string) {
	k := n.Kind()
	switch {
	case k == l.v.stringLiteral:
		if s, ok := l.stringValue(n); ok {
			return directive.RawValue{Text: s, Quoted: true}, ""
		}
	case slices.Contains(l.v.numbers[:], k):
		if s, ok := l.number(n); ok {
			return directive.RawValue{Text: s}, ""
		}
	case k == l.v.unaryExpression && n.Child(l.v.fieldOperator).Text() == minusSign &&
		slices.Contains(l.v.numbers[:], n.Child(l.v.fieldOperand).Kind()):
		if s, ok := l.number(n.Child(l.v.fieldOperand)); ok {
			if negated, cut := strings.CutPrefix(s, minusSign); cut {
				return directive.RawValue{Text: negated}, ""
			}
			return directive.RawValue{Text: minusSign + s}, ""
		}
	case k == l.v.trueNode || k == l.v.falseNode:
		return directive.RawValue{Text: n.Text()}, ""
	case k == l.v.elementValueArrayInitializer:
		// A list value is a list also when it has no element, so the list
		// is never nil.
		list := []directive.RawValue{}
		for _, e := range l.children(n) {
			v, refusal := l.elementValue(e)
			if refusal != "" {
				return directive.RawValue{}, refusal
			}
			list = append(list, v)
		}
		return directive.RawValue{List: list}, ""
	}
	return directive.RawValue{}, fmt.Sprintf("the element value %s is not a literal or an array of literals",
		n.Compact())
}

// stringValue returns the content of a string literal, its escapes
// decoded by [unescape]. It reports false for a text block, whose
// content Java strips of its incidental white space, and for an escape
// that Java refuses or that encodes a lone surrogate.
func (l *lowering) stringValue(n treesitter.Node) (string, bool) {
	var b strings.Builder
	for part := range n.NamedChildren() {
		switch part.Kind() {
		case l.v.stringFragment:
			b.WriteString(part.Text())
		case l.v.escapeSequence:
			if !unescape(&b, part.Text()) {
				return "", false
			}
		default:
			return "", false
		}
	}
	return b.String(), true
}

// number returns the decimal text of a number literal of Java, with its
// separators and its type suffix dropped. A float literal is the
// shortest decimal text that parses back to its float or its double. A
// hexadecimal, an octal and a binary integer literal is the bit pattern
// of its int or its long in two's complement, as Java reads it, so
// 0xFFFFFFFF is -1. number reports false for an integer beyond its
// type's bits and for a float beyond its type's range.
func (l *lowering) number(n treesitter.Node) (string, bool) {
	digits := strings.ReplaceAll(n.Text(), separator, "")
	last := digits[len(digits)-1] | caseBit
	switch n.Kind() {
	case l.v.decimalFloatingPointLiteral, l.v.hexFloatingPointLiteral:
		bits := doubleBits
		switch last {
		case floatSuffix:
			bits, digits = floatBits, digits[:len(digits)-1]
		case doubleSuffix:
			digits = digits[:len(digits)-1]
		}
		f, err := strconv.ParseFloat(digits, bits)
		if err != nil {
			return "", false
		}
		return numeric.Decimal(f, bits), true
	}
	long := last == longSuffix
	if long {
		digits = digits[:len(digits)-1]
	}
	base := 10
	switch n.Kind() {
	case l.v.hexIntegerLiteral:
		base, digits = 16, digits[2:]
	case l.v.binaryIntegerLiteral:
		base, digits = 2, digits[2:]
	case l.v.octalIntegerLiteral:
		base = 8
	}
	u, err := strconv.ParseUint(digits, base, doubleBits)
	switch {
	case err != nil:
		return "", false
	case base == 10:
		return strconv.FormatUint(u, 10), true
	case long:
		return strconv.FormatInt(int64(u), 10), true
	case u > math.MaxUint32:
		return "", false
	}
	return strconv.FormatInt(int64(int32(uint32(u))), 10), true
}

// unescape writes the character of one escape of Java's string literals
// into b, and reports whether Java accepts the escape. text is the
// escape with its backslash: an escape of one character, an octal escape
// of up to 255, or a Unicode escape of four hexadecimal digits that is
// not a surrogate.
func unescape(b *strings.Builder, text string) bool {
	body := text[1:]
	if s, named := escapes[body[0]]; named && len(body) == 1 {
		b.WriteString(s)
		return true
	}
	if hex, unicode := strings.CutPrefix(body, unicodeMark); unicode {
		v, err := strconv.ParseUint(hex, 16, 16)
		if err != nil || len(hex) != unicodeDigits || utf16.IsSurrogate(rune(v)) {
			return false
		}
		b.WriteRune(rune(v))
		return true
	}
	v, err := strconv.ParseUint(body, 8, 8)
	if err != nil {
		return false
	}
	b.WriteRune(rune(v))
	return true
}
