// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"math"
	"slices"

	typescript "go.dokimi.dev/eidos/lang/typescript"
)

// shiftMask keeps the low five bits of a shift count, which are the bits
// that a shift operator of TypeScript uses.
const shiftMask = 31

// levels lists the binary operators of a constant enum expression by
// TypeScript's precedence, from the loosest level to the tightest.
var levels = [][]string{
	{pipe}, {caret}, {ampersand}, {shiftLeft, shiftRight, shiftRightUnsigned}, {plus, minus}, {star, slash, percent},
}

// enumValue is the value of one enum member. It is a number, or a text
// in a string enum.
type enumValue struct {
	number float64
	text   string
	isText bool
}

// member is one enum member with its name and its value.
type member struct {
	name  string
	value enumValue
	known bool // true when a constant enum expression decides the value
}

// expression is the state of the evaluation of one constant enum
// expression.
type expression struct {
	l   lexer // the lexer over the expression's text
	tok token // the current token
	// enum is the name of the enum, which can qualify a reference to a
	// member.
	enum string
	// earlier lists the members that the enum declares before the member
	// under evaluation.
	earlier []member
}

// evaluate returns the value of a constant enum expression of TypeScript,
// as the compiler computes it. The expression can contain these parts:
//
//   - a numeric literal, a string literal, and a template literal
//     without a substitution;
//   - a reference to an earlier member, by its name or through the
//     enum's name;
//   - a parenthesized expression;
//   - the unary operators +, - and ~;
//   - the binary operators +, -, *, /, %, <<, >>, >>>, &, | and ^.
//
// Arithmetic uses doubles, and a bitwise operator converts its operands
// to 32-bit integers. The operator + also joins two texts. evaluate
// reports false for an expression outside this grammar, for a reference
// to a member without a known value, and for an operator other than + on
// a text. It also reports false for a number that is not finite, which
// TypeScript refuses in a constant enum expression.
func evaluate(text, enum string, earlier []member) (enumValue, bool) {
	e := expression{l: lexer{text: text}, enum: enum, earlier: earlier}
	e.tok = e.l.next()
	v, ok := e.binary(0)
	if !ok || e.tok.kind != tokenEnd || !v.isText && (math.IsNaN(v.number) || math.IsInf(v.number, 0)) {
		return enumValue{}, false
	}
	return v, true
}

// binary evaluates the operators of one precedence level and of every
// tighter level, from left to right.
func (e *expression) binary(level int) (enumValue, bool) {
	if level == len(levels) {
		return e.unary()
	}
	left, ok := e.binary(level + 1)
	for ok && e.tok.kind == tokenOperator && slices.Contains(levels[level], e.tok.text) {
		op := e.tok.text
		e.tok = e.l.next()
		right, evaluated := e.binary(level + 1)
		if !evaluated {
			return enumValue{}, false
		}
		left, ok = apply(op, left, right)
	}
	return left, ok
}

// unary evaluates a unary operator and its operand, or an operand.
func (e *expression) unary() (enumValue, bool) {
	if e.tok.kind != tokenOperator || e.tok.text != plus && e.tok.text != minus && e.tok.text != tilde {
		return e.primary()
	}
	op := e.tok.text
	e.tok = e.l.next()
	v, ok := e.unary()
	if !ok || v.isText {
		return enumValue{}, false
	}
	switch op {
	case minus:
		v.number = -v.number
	case tilde:
		v.number = float64(^int32Of(v.number))
	}
	return v, true
}

// primary evaluates a literal, a reference to an earlier member, or a
// parenthesized expression.
func (e *expression) primary() (enumValue, bool) {
	t := e.tok
	e.tok = e.l.next()
	switch {
	case t.kind == tokenNumber:
		f, _ := typescript.ParseNumber(t.text)
		return enumValue{number: f}, true
	case t.kind == tokenString:
		s, _ := typescript.Unquote(t.text)
		return enumValue{text: s, isText: true}, true
	case t.kind == tokenName:
		name := t.text
		if name == e.enum && e.tok.kind == tokenOperator && e.tok.text == pathSep {
			if e.tok = e.l.next(); e.tok.kind != tokenName {
				return enumValue{}, false
			}
			name, e.tok = e.tok.text, e.l.next()
		}
		for _, m := range e.earlier {
			if m.name == name {
				return m.value, m.known
			}
		}
		return enumValue{}, false
	case t.kind == tokenOperator && t.text == openParen:
		v, ok := e.binary(0)
		if !ok || e.tok.kind != tokenOperator || e.tok.text != closeParen {
			return enumValue{}, false
		}
		e.tok = e.l.next()
		return v, true
	default:
		return enumValue{}, false
	}
}

// apply applies one binary operator to two values, as TypeScript does in
// a constant enum expression.
func apply(op string, a, b enumValue) (enumValue, bool) {
	if a.isText || b.isText {
		if op != plus || !a.isText || !b.isText {
			return enumValue{}, false
		}
		return enumValue{text: a.text + b.text, isText: true}, true
	}
	x, y := a.number, b.number
	shift := uint32(int32Of(y)) & shiftMask
	var f float64
	switch op {
	case plus:
		f = x + y
	case minus:
		f = x - y
	case star:
		f = x * y
	case slash:
		f = x / y
	case percent:
		f = math.Mod(x, y)
	case shiftLeft:
		f = float64(int32Of(x) << shift)
	case shiftRight:
		f = float64(int32Of(x) >> shift)
	case shiftRightUnsigned:
		f = float64(uint32(int32Of(x)) >> shift)
	case ampersand:
		f = float64(int32Of(x) & int32Of(y))
	case pipe:
		f = float64(int32Of(x) | int32Of(y))
	default:
		f = float64(int32Of(x) ^ int32Of(y))
	}
	return enumValue{number: f}, true
}

// int32Of converts a double to the 32-bit integer that a bitwise operator
// of TypeScript uses. The integer is the integer part modulo 2^32 in
// two's complement, and 0 for NaN and an infinity.
func int32Of(x float64) int32 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	m := math.Mod(math.Trunc(x), 1<<32)
	if m < 0 {
		m += 1 << 32
	}
	return int32(uint32(m))
}
