// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/lang/typescript"
)

// The keywords of TypeScript's literals.
const (
	spellTrue      = "true"
	spellFalse     = "false"
	spellNull      = "null"
	spellUndefined = "undefined"
)

// The marks of TypeScript's literal syntax that the lexer reads.
const (
	// zeroDigit opens the prefix of a hexadecimal literal.
	zeroDigit = '0'
	// hexMarks follow the leading 0 of a hexadecimal literal. In a
	// hexadecimal literal, e is a digit and does not open an exponent.
	hexMarks = "xX"
	// exponentMarks open the exponent of a decimal literal.
	exponentMarks = "eE"
	// signs are the two signs that can open a literal or an exponent.
	signs = plus + minus
	// separator and point are the characters other than letters and
	// digits that a numeric literal contains, and numberMarks lists them.
	separator   = "_"
	point       = "."
	numberMarks = separator + point
	// bigintMark ends a bigint literal.
	bigintMark = "n"
	// quotes open a string literal and a template literal.
	quotes = "\"'`"
	// escapeMark opens an escape inside a quoted literal.
	escapeMark = '\\'
	// pathSep joins the names of a path, such as Color.Red.
	pathSep = "."
	// nameMarks are the characters other than letters that can open a
	// name.
	nameMarks = "$_"
	// joiners are the two zero-width characters that continue a name.
	joiners = "\U0000200c\U0000200d"
)

// The operators of a constant enum expression, and the parentheses that
// group one.
const (
	pipe               = "|"
	caret              = "^"
	ampersand          = "&"
	shiftLeft          = "<<"
	shiftRight         = ">>"
	shiftRightUnsigned = ">>>"
	plus               = "+"
	minus              = "-"
	star               = "*"
	slash              = "/"
	percent            = "%"
	tilde              = "~"
	openParen          = "("
	closeParen         = ")"
)

// operators lists the punctuation of a constant expression. Of two
// operators that share a prefix, the longer one comes first.
var operators = []string{
	shiftRightUnsigned, shiftLeft, shiftRight, openParen, closeParen, pathSep, plus, minus, star, slash,
	percent, tilde, ampersand, pipe, caret,
}

// tokenKind is the class of one token.
type tokenKind uint8

const (
	// tokenEnd is the end of the text.
	tokenEnd tokenKind = 0
	// tokenNumber is a numeric literal.
	tokenNumber tokenKind = 1
	// tokenBigInt is a bigint literal, with its n.
	tokenBigInt tokenKind = 2
	// tokenString is a string literal, or a template literal without a
	// substitution, with its quotes.
	tokenString tokenKind = 3
	// tokenName is an identifier.
	tokenName tokenKind = 4
	// tokenOperator is an operator or a parenthesis.
	tokenOperator tokenKind = 5
	// tokenInvalid is text that is not a token of TypeScript.
	tokenInvalid tokenKind = 6
)

// token is one token of a text.
type token struct {
	kind tokenKind
	text string // a part of the lexer's text
	end  int    // the offset after the token's last byte
}

// lexer splits the text of one TypeScript literal, or of one constant
// expression, into tokens in order. Each token is a part of the text, so
// the lexer allocates nothing.
type lexer struct {
	text string
	at   int // the offset of the next token's first byte
}

// next skips white space and returns the next token. At the end of the
// text, it returns a token of the kind tokenEnd.
func (l *lexer) next() token {
	for l.at < len(l.text) {
		r, size := utf8.DecodeRuneInString(l.text[l.at:])
		if !unicode.IsSpace(r) {
			break
		}
		l.at += size
	}
	start := l.at
	if start == len(l.text) {
		return token{kind: tokenEnd, end: start}
	}
	c := l.text[start]
	switch {
	case unicode.IsDigit(rune(c)) ||
		c == point[0] && start+1 < len(l.text) && unicode.IsDigit(rune(l.text[start+1])):
		return l.number(start)
	case strings.IndexByte(quotes, c) >= 0:
		return l.quoted(start, c)
	}
	if r, size := utf8.DecodeRuneInString(l.text[start:]); nameRune(r, true) {
		l.at += size
		for l.at < len(l.text) {
			r, size := utf8.DecodeRuneInString(l.text[l.at:])
			if !nameRune(r, false) {
				break
			}
			l.at += size
		}
		return token{kind: tokenName, text: l.text[start:l.at], end: l.at}
	}
	for _, op := range operators {
		if strings.HasPrefix(l.text[start:], op) {
			l.at += len(op)
			return token{kind: tokenOperator, text: op, end: l.at}
		}
	}
	_, size := utf8.DecodeRuneInString(l.text[start:])
	l.at += size
	return token{kind: tokenInvalid, text: l.text[start:l.at], end: l.at}
}

// number returns the numeric literal that starts at start. The literal
// is the run of letters, digits, separators and points. In a literal that
// is not hexadecimal, a sign after the mark of the exponent continues
// the run. A run that is neither a numeric literal nor a bigint literal
// of TypeScript is invalid.
func (l *lexer) number(start int) token {
	t := l.text
	hex := start+1 < len(t) && t[start] == zeroDigit && strings.IndexByte(hexMarks, t[start+1]) >= 0
	i := start
	for i < len(t) {
		c := t[i]
		if unicode.IsLetter(rune(c)) || unicode.IsDigit(rune(c)) || strings.IndexByte(numberMarks, c) >= 0 ||
			!hex && strings.IndexByte(signs, c) >= 0 && strings.IndexByte(exponentMarks, t[i-1]) >= 0 {

			i++
			continue
		}
		break
	}
	l.at = i
	text := t[start:i]
	kind := tokenNumber
	if strings.HasSuffix(text, bigintMark) {
		kind = tokenBigInt
		if _, valid := typescript.ParseBigInt(text); !valid {
			kind = tokenInvalid
		}
	} else if _, valid := typescript.ParseNumber(text); !valid {
		kind = tokenInvalid
	}
	return token{kind: kind, text: text, end: i}
}

// quoted returns the string literal or the template literal that starts
// with quote at start, and ends at its closing quote. A literal that
// [typescript.Unquote] refuses is invalid, and so is a literal without
// its closing quote.
func (l *lexer) quoted(start int, quote byte) token {
	t := l.text
	end := len(t)
	for i := start + 1; i < len(t); i++ {
		if t[i] == escapeMark {
			i++
			continue
		}
		if t[i] == quote {
			end = i + 1
			break
		}
	}
	l.at = end
	kind := tokenString
	if _, valid := typescript.Unquote(t[start:end]); !valid {
		kind = tokenInvalid
	}
	return token{kind: kind, text: t[start:end], end: end}
}

// literalKind is the class of one literal. The zero kind is not a
// literal kind.
type literalKind uint8

const (
	// literalNumber is a numeric literal with at most one sign.
	literalNumber literalKind = 1
	// literalBigInt is a bigint literal with at most one minus sign.
	literalBigInt literalKind = 2
	// literalString is a string literal, or a template literal without a
	// substitution.
	literalString literalKind = 3
	// literalBool is true or false.
	literalBool literalKind = 4
	// literalNull is null.
	literalNull literalKind = 5
	// literalUndefined is undefined.
	literalUndefined literalKind = 6
	// literalPath is an identifier, or identifiers joined by dots without
	// white space, such as Color.Red. A path refers to a member of an
	// enum.
	literalPath literalKind = 7
)

// literal is one scanned TypeScript literal.
type literal struct {
	kind literalKind
	// number is the value of a numeric literal.
	number float64
	// text is the content of a string, the keyword of a truth value, the
	// decimal digits of a bigint with its sign, or a path as written.
	text string
}

// scanLiteral reads one TypeScript literal, and reports whether the text
// is one. A literal is one of these:
//
//   - true, false, null or undefined;
//   - a numeric literal or a bigint literal with at most one sign;
//   - a string literal, or a template literal without a substitution;
//   - a path of identifiers that dots join without white space.
//
// A bigint cannot have a plus sign, because TypeScript refuses the unary
// plus of a bigint. Text with anything else, a comment included, is not
// a literal.
func scanLiteral(text string) (literal, bool) {
	l := lexer{text: text}
	t := l.next()
	sign := ""
	if t.kind == tokenOperator && strings.Contains(signs, t.text) {
		sign, t = t.text, l.next()
	}
	var lit literal
	switch {
	case t.kind == tokenNumber:
		f, _ := typescript.ParseNumber(t.text)
		if sign == minus {
			f = -f
		}
		lit = literal{kind: literalNumber, number: f}
	case t.kind == tokenBigInt && sign != plus:
		digits, _ := typescript.ParseBigInt(t.text)
		if sign == minus && digits != string(zeroDigit) {
			digits = sign + digits
		}
		lit = literal{kind: literalBigInt, text: digits}
	case sign != "":
		return literal{}, false
	case t.kind == tokenString:
		s, _ := typescript.Unquote(t.text)
		lit = literal{kind: literalString, text: s}
	case t.kind == tokenName:
		return scanName(&l, t)
	default:
		return literal{}, false
	}
	if l.next().kind != tokenEnd {
		return literal{}, false
	}
	return lit, true
}

// scanName reads the literal that starts with the name first. true,
// false, null and undefined are literals only on their own. Any other
// name starts a path, which continues with each name that a dot joins
// without white space, to the end of the text.
func scanName(l *lexer, first token) (literal, bool) {
	var lit literal
	switch first.text {
	case spellTrue, spellFalse:
		lit = literal{kind: literalBool, text: first.text}
	case spellNull:
		lit = literal{kind: literalNull}
	case spellUndefined:
		lit = literal{kind: literalUndefined}
	default:
		start, end := first.end-len(first.text), first.end
		for {
			dot := l.next()
			if dot.kind == tokenEnd {
				return literal{kind: literalPath, text: l.text[start:end]}, true
			}
			name := l.next()
			if dot.kind != tokenOperator || dot.text != pathSep || dot.end != end+len(pathSep) ||
				name.kind != tokenName || name.end != dot.end+len(name.text) {

				return literal{}, false
			}
			end = name.end
		}
	}
	if l.next().kind != tokenEnd {
		return literal{}, false
	}
	return lit, true
}

// nameRune reports whether a rune can start a TypeScript identifier when
// first is set, or continue one otherwise. A letter, $ and _ can start
// one. A digit, a combining mark, a connector and the two joiners can
// also continue one.
func nameRune(r rune, first bool) bool {
	if strings.ContainsRune(nameMarks, r) || unicode.IsLetter(r) {
		return true
	}
	return !first && (unicode.IsDigit(r) || unicode.In(r, unicode.Mn, unicode.Mc, unicode.Pc) ||
		strings.ContainsRune(joiners, r))
}
