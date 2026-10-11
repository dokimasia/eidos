// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"strings"
	"unicode"
)

// nameMarks are the two characters outside the Unicode property ID_Start
// that can start an identifier of TypeScript.
const nameMarks = "$_"

// IsIdentifierStart reports whether r can start an identifier of
// TypeScript. ECMAScript admits $, _ and every code point with the
// Unicode property ID_Start. ID_Start contains the letters, the letter
// numbers and the characters of Other_ID_Start, except the characters of
// Pattern_Syntax. IsIdentifierStart reads the properties from the unicode
// package. [unicode.Version] is the edition of its tables.
// IsIdentifierStart does not allocate.
func IsIdentifierStart(r rune) bool {
	return strings.ContainsRune(nameMarks, r) ||
		unicode.In(r, unicode.L, unicode.Nl, unicode.Other_ID_Start) && !unicode.Is(unicode.Pattern_Syntax, r)
}

// IsIdentifierPart reports whether r can continue an identifier of
// TypeScript. ECMAScript admits $ and every code point with the Unicode
// property ID_Continue. ID_Continue contains the characters of ID_Start,
// the combining marks, the decimal digits, the connectors and the
// characters of Other_ID_Continue. Other_ID_Continue contains the
// zero-width non-joiner and the zero-width joiner. ECMAScript's grammar
// lists both characters as well. IsIdentifierPart does not allocate.
func IsIdentifierPart(r rune) bool {
	return IsIdentifierStart(r) ||
		unicode.In(r, unicode.Mn, unicode.Mc, unicode.Nd, unicode.Pc, unicode.Other_ID_Continue)
}
