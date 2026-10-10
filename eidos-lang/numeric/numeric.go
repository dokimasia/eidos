// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package numeric

import (
	"go/constant"
	"go/token"
	"math"
	"strconv"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/rules"
)

// The widths in bits the package reads a number at.
const (
	// widest is the width an integer of unstated width bounds as, and
	// the precision every float but a single-precision one reads at.
	widest = 64
	// single is a single-precision float's width.
	single = 32
)

// The magnitudes between which [Decimal] writes positional notation.
const (
	smallestPositional = 1e-6
	largestPositional  = 1e21
)

// decimalBuffer is the size of the stack buffer [Decimal] formats into.
// It is longer than any text Decimal writes, so formatting allocates
// nothing and the conversion to a string allocates the result alone.
const decimalBuffer = 32

// Int returns an integral value inside the range of an integer type
// as decimal text. A class of [rules.ScalarUint] bounds the value
// from 0 to 2^bits-1, and every other class from -2^(bits-1) to
// 2^(bits-1)-1. A width of 0 bounds as 64 bits. The returned number
// states the width passed in. A value that is no integer, or one
// outside the range, reports false.
//
// # Allocation contract
//
// A width of at most 64 bits checks the range in machine integers and
// allocates the decimal text alone, and nothing for a value from 0 to
// 99, whose text is static. A wider type checks the range in exact
// arithmetic, which allocates its bounds.
func Int(value constant.Value, class rules.ScalarClass, bits int) (emit.Value, bool) {
	n := constant.ToInt(value)
	if n.Kind() != constant.Int {
		return emit.Value{}, false
	}
	width := bits
	if width == 0 {
		width = widest
	}
	if width <= widest {
		if !fits(n, class, width) {
			return emit.Value{}, false
		}
		return emit.Number(emit.LiteralInt, n.ExactString(), bits), true
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

// fits reports whether an integer is inside the range of an integer
// type of a width from 1 to 64 bits, in machine integers.
func fits(n constant.Value, class rules.ScalarClass, width int) bool {
	if class == rules.ScalarUint {
		u, exact := constant.Uint64Val(n)
		return exact && (width == widest || u < 1<<width)
	}
	v, exact := constant.Int64Val(n)
	if !exact {
		return false
	}
	if width == widest {
		return true
	}
	half := int64(1) << (width - 1)
	return v >= -half && v < half
}

// Float returns a finite value inside the range of a float type as
// the shortest decimal text that reads back to it at the type's
// precision. A width of 32 reads at single precision, and every
// other width reads at double precision. The returned number states
// the width passed in. A value that is no number, or one the
// precision rounds to infinity, reports false.
func Float(value constant.Value, bits int) (emit.Value, bool) {
	x := constant.ToFloat(value)
	if x.Kind() != constant.Float {
		return emit.Value{}, false
	}
	width := widest
	f, _ := constant.Float64Val(x)
	if bits == single {
		width = single
		narrow, _ := constant.Float32Val(x)
		f = float64(narrow)
	}
	if math.IsInf(f, 0) {
		return emit.Value{}, false
	}
	return emit.Number(emit.LiteralFloat, Decimal(f, width), bits), true
}

// Decimal returns the shortest decimal text that reads back to a
// float at a precision: positional notation from 1e-6 up to 1e21,
// and exponent notation outside it with the exponent unpadded. This
// is the rule encoding/json writes floats by, with the cutoffs
// compared at the same precision. A width of 32 is single
// precision, and every other width is double precision. It allocates
// the text, one allocation.
func Decimal(f float64, bits int) string {
	if bits != single {
		bits = widest
	}
	format := byte('f')
	if abs := math.Abs(f); abs != 0 {
		narrow := float32(abs)
		if bits == widest && (abs < smallestPositional || abs >= largestPositional) ||
			bits == single && (narrow < smallestPositional || narrow >= largestPositional) {

			format = 'e'
		}
	}
	var buf [decimalBuffer]byte
	b := strconv.AppendFloat(buf[:0], f, format, -1, bits)
	if n := len(b); format == 'e' && n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
		b[n-2] = b[n-1]
		b = b[:n-1]
	}
	return string(b)
}
