// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile

import (
	"math"
	"strconv"
	"strings"
)

// The spellings of a Java literal (JLS §3.10): the suffixes of a long
// and a float, the boolean values, and the fraction a double without one
// takes. The root's Quote functions spell a string and a char.
const (
	longSuffix   = "L"
	floatSuffix  = "f"
	trueLiteral  = "true"
	falseLiteral = "false"
	zeroFraction = ".0"
)

// The fields of the floating-point classes that name the values no Java
// literal spells.
const (
	floatNaN     = "Float.NaN"
	floatPosInf  = "Float.POSITIVE_INFINITY"
	floatNegInf  = "Float.NEGATIVE_INFINITY"
	doubleNaN    = "Double.NaN"
	doublePosInf = "Double.POSITIVE_INFINITY"
	doubleNegInf = "Double.NEGATIVE_INFINITY"
)

// The bit sizes strconv formats a float and a double at.
const (
	floatBits  = 32
	doubleBits = 64
)

// intLiteral spells an int.
func intLiteral(v int32) string { return strconv.FormatInt(int64(v), 10) }

// longLiteral spells a long, with its suffix.
func longLiteral(v int64) string { return strconv.FormatInt(v, 10) + longSuffix }

// boolLiteral spells a boolean, which a class file stores as an int:
// false for zero, and true otherwise.
func boolLiteral(v int32) string {
	if v == 0 {
		return falseLiteral
	}
	return trueLiteral
}

// floatLiteral spells a float with its suffix, in the shortest form that
// reads back as the same value, and an infinity or NaN as the Float
// constant of that value.
func floatLiteral(v float32) string {
	switch f := float64(v); {
	case math.IsNaN(f):
		return floatNaN
	case math.IsInf(f, 1):
		return floatPosInf
	case math.IsInf(f, -1):
		return floatNegInf
	default:
		return strconv.FormatFloat(f, 'g', -1, floatBits) + floatSuffix
	}
}

// doubleLiteral spells a double in the shortest form that reads back as
// the same value, with a fraction where that form has neither a point
// nor an exponent, so the literal is no int's, and an infinity or NaN as
// the Double constant of that value.
func doubleLiteral(v float64) string {
	switch {
	case math.IsNaN(v):
		return doubleNaN
	case math.IsInf(v, 1):
		return doublePosInf
	case math.IsInf(v, -1):
		return doubleNegInf
	}
	s := strconv.FormatFloat(v, 'g', -1, doubleBits)
	if !strings.ContainsAny(s, ".e") {
		s += zeroFraction
	}
	return s
}
