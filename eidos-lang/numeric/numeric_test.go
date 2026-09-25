// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package numeric_test

import (
	"go/constant"
	"go/token"
	"math"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/numeric"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/rules"
)

// The widths the cases type numbers at.
const (
	noWidth     = 0
	halfWidth   = 16
	singleWidth = 32
	doubleWidth = 64
)

// literal reads one Go number literal into its exact constant.
func literal(tb assert.TB, text string, kind token.Token) constant.Value {
	tb.Helper()

	v := constant.MakeFromLiteral(text, kind, 0)
	assert.NotEqual(tb, v.Kind(), constant.Unknown, "the fixture literal "+text+" reads")
	return v
}

// Every language's rules type an author's number through these
// three, so the bounds, the precision and the canonical text are
// pinned at each edge.
func TestNumeric(t *testing.T) {
	t.Parallel()

	t.Run("Int", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a value at either end of a signed width", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Int(constant.MakeInt64(math.MinInt32), rules.ScalarInt, singleWidth)
			assert.True(t, ok, "the smallest int32 is an int32")
			assert.Equal(t, v, emit.Number(emit.LiteralInt, "-2147483648", singleWidth),
				"as decimal text at the width")
			v, ok = numeric.Int(constant.MakeInt64(math.MaxInt32), rules.ScalarInt, singleWidth)
			assert.True(t, ok && v.Text == "2147483647", "and so is the largest")
		})

		t.Run("refuses a value one past either end of a signed width", func(t *testing.T) {
			t.Parallel()

			_, ok := numeric.Int(constant.MakeInt64(math.MaxInt32+1), rules.ScalarInt, singleWidth)
			assert.False(t, ok, "one past the largest int32 refuses")
			_, ok = numeric.Int(constant.MakeInt64(math.MinInt32-1), rules.ScalarInt, singleWidth)
			assert.False(t, ok, "and so does one below the smallest")
		})

		t.Run("bounds an unsigned width from zero", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Int(constant.MakeUint64(math.MaxUint32), rules.ScalarUint, singleWidth)
			assert.True(t, ok && v.Text == "4294967295", "the largest uint32 is a uint32")
			_, ok = numeric.Int(constant.MakeUint64(math.MaxUint32+1), rules.ScalarUint, singleWidth)
			assert.False(t, ok, "one past it refuses")
			_, ok = numeric.Int(constant.MakeInt64(-1), rules.ScalarUint, singleWidth)
			assert.False(t, ok, "and a negative value refuses for an unsigned type")
		})

		t.Run("bounds a width of zero as 64 bits and states none", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Int(constant.MakeInt64(math.MaxInt64), rules.ScalarInt, noWidth)
			assert.True(t, ok, "the largest int64 fits an unstated width")
			assert.Equal(t, v, emit.Number(emit.LiteralInt, "9223372036854775807", noWidth),
				"and the number states no width")
			_, ok = numeric.Int(constant.MakeUint64(math.MaxInt64+1), rules.ScalarInt, noWidth)
			assert.False(t, ok, "one past it refuses")
		})

		t.Run("reads an integral float and refuses any other value", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Int(literal(t, "3.0", token.FLOAT), rules.ScalarInt, singleWidth)
			assert.True(t, ok && v.Text == "3", "an integral float is an integer")
			_, ok = numeric.Int(literal(t, "1.5", token.FLOAT), rules.ScalarInt, singleWidth)
			assert.False(t, ok, "a fraction is no integer")
			_, ok = numeric.Int(constant.MakeString("3"), rules.ScalarInt, singleWidth)
			assert.False(t, ok, "and a string is no number")
		})
	})

	t.Run("Float", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the shortest text at the type's precision", func(t *testing.T) {
			t.Parallel()

			tenth := literal(t, "0.1", token.FLOAT)
			v, ok := numeric.Float(tenth, doubleWidth)
			assert.True(t, ok, "a tenth is a float64")
			assert.Equal(t, v, emit.Number(emit.LiteralFloat, "0.1", doubleWidth), "written as 0.1")
			v, ok = numeric.Float(tenth, singleWidth)
			assert.True(t, ok, "and a float32")
			assert.Equal(t, v, emit.Number(emit.LiteralFloat, "0.1", singleWidth),
				"written as the shortest text that reads back at single precision")
		})

		t.Run("reads a width of zero at double precision and states none", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Float(literal(t, "1.5", token.FLOAT), noWidth)
			assert.True(t, ok, "an unstated width takes the value")
			assert.Equal(t, v, emit.Number(emit.LiteralFloat, "1.5", noWidth), "and states no width")
			v, ok = numeric.Float(constant.MakeInt64(5), doubleWidth)
			assert.True(t, ok && v.Text == "5", "and an integer reads as a float")
		})

		t.Run("refuses a value the precision rounds to infinity", func(t *testing.T) {
			t.Parallel()

			_, ok := numeric.Float(literal(t, "1e39", token.FLOAT), singleWidth)
			assert.False(t, ok, "past the largest float32")
			v, ok := numeric.Float(literal(t, "1e39", token.FLOAT), doubleWidth)
			assert.True(t, ok && v.Text == "1e+39", "which a float64 still takes")
			_, ok = numeric.Float(literal(t, "1e309", token.FLOAT), doubleWidth)
			assert.False(t, ok, "and past the largest float64")
		})

		t.Run("refuses a value that is no number", func(t *testing.T) {
			t.Parallel()

			_, ok := numeric.Float(constant.MakeString("1.5"), doubleWidth)
			assert.False(t, ok, "a string is no number")
			_, ok = numeric.Float(constant.MakeBool(true), doubleWidth)
			assert.False(t, ok, "and neither is a truth value")
		})
	})

	t.Run("Decimal", func(t *testing.T) {
		t.Parallel()

		t.Run("writes positional notation from 1e-6 up to 1e21", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, numeric.Decimal(0, doubleWidth), "0", "zero")
			assert.Equal(t, numeric.Decimal(123.5, doubleWidth), "123.5", "a plain fraction")
			assert.Equal(t, numeric.Decimal(1e-6, doubleWidth), "0.000001", "the smallest positional value")
			assert.Equal(t, numeric.Decimal(1e20, doubleWidth), "100000000000000000000",
				"and a value just under the upper cutoff")
		})

		t.Run("writes exponent notation outside them with the exponent unpadded", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, numeric.Decimal(1e21, doubleWidth), "1e+21", "the upper cutoff itself")
			assert.Equal(t, numeric.Decimal(1e-7, doubleWidth), "1e-7",
				"and a small value, without the padding zero strconv writes")
			assert.Equal(t, numeric.Decimal(-1.5e-10, doubleWidth), "-1.5e-10",
				"a two-digit exponent keeps both digits")
		})

		t.Run("compares and writes at single precision for a width of 32", func(t *testing.T) {
			t.Parallel()

			tenth := float64(float32(0.1))
			assert.Equal(t, numeric.Decimal(tenth, singleWidth), "0.1",
				"the shortest text that reads back to the float32")
			assert.Equal(t, numeric.Decimal(tenth, doubleWidth), "0.10000000149011612",
				"where double precision needs every digit of the widened value")
			assert.Equal(t, numeric.Decimal(float64(float32(1e21)), singleWidth), "1e+21",
				"and the cutoff compares at single precision")
			assert.Equal(t, numeric.Decimal(9.9999999e20, singleWidth), "1e+21",
				"so a value single precision rounds up to the cutoff takes exponent notation")
		})

		t.Run("reads every other width at double precision", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, numeric.Decimal(0.1, noWidth), "0.1", "an unstated width")
			assert.Equal(t, numeric.Decimal(0.1, halfWidth), "0.1", "and a width no float has")
		})
	})
}
