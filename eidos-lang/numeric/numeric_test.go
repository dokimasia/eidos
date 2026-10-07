// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package numeric_test

import (
	"go/constant"
	"go/token"
	"math"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/numeric"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/rules"
)

// The widths the cases type numbers at.
const (
	noWidth     = 0
	byteWidth   = 8
	halfWidth   = 16
	singleWidth = 32
	doubleWidth = 64
	wideWidth   = 128
)

// The allocations of the three readings.
const (
	// textAllocs is the decimal text of a number, which the value keeps.
	textAllocs = 1
	// wideIntAllocs is 2^64 at a width of 128 bits: the exact arithmetic
	// of its bounds and its comparisons, and the decimal text, which
	// math/big writes.
	wideIntAllocs = 12
	// floatAllocs is a float read from an exact rational: math/big's
	// conversion of the rational to a float64, two allocations, and the
	// decimal text.
	floatAllocs = 2 + textAllocs
)

// Every language's rules type an author's number through these
// three, so the bounds, the precision and the canonical text are
// pinned at each edge.
func TestNumeric(t *testing.T) {
	t.Parallel()

	t.Run("Int", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the smallest value of a signed width", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Int(constant.MakeInt64(math.MinInt32), rules.ScalarInt, singleWidth)
			assert.True(t, ok, "the smallest int32 is an int32")
			assert.Equal(t, v, emit.Number(emit.LiteralInt, "-2147483648", singleWidth),
				"as decimal text at the width")
		})

		t.Run("returns the largest value of a signed width", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Int(constant.MakeInt64(math.MaxInt32), rules.ScalarInt, singleWidth)
			assert.True(t, ok, "the largest int32 is an int32")
			assert.Equal(t, v.Text, "2147483647", "as decimal text")
		})

		tests := []struct {
			name  string
			give  constant.Value
			class rules.ScalarClass
			bits  int
		}{
			{
				name: "reports false for one past the largest value of a signed width",
				give: constant.MakeInt64(math.MaxInt32 + 1), class: rules.ScalarInt, bits: singleWidth,
			},
			{
				name: "reports false for one below the smallest value of a signed width",
				give: constant.MakeInt64(math.MinInt32 - 1), class: rules.ScalarInt, bits: singleWidth,
			},
			{
				name: "reports false for one past the largest value of a byte",
				give: constant.MakeInt64(math.MaxInt8 + 1), class: rules.ScalarInt, bits: byteWidth,
			},
			{
				name: "reports false for one past the largest value of an unsigned width",
				give: constant.MakeUint64(math.MaxUint32 + 1), class: rules.ScalarUint, bits: singleWidth,
			},
			{
				name: "reports false for a negative value of an unsigned width",
				give: constant.MakeInt64(-1), class: rules.ScalarUint, bits: singleWidth,
			},
			{
				name: "reports false for one past the largest value of an unstated width",
				give: constant.MakeUint64(math.MaxInt64 + 1), class: rules.ScalarInt, bits: noWidth,
			},
			{
				name:  "reports false for one past the largest value of a 128-bit width",
				give:  constant.Shift(constant.MakeInt64(1), token.SHL, wideWidth-1),
				class: rules.ScalarInt,
				bits:  wideWidth,
			},
			{
				name:  "reports false for one past the largest value of a 128-bit unsigned width",
				give:  constant.Shift(constant.MakeInt64(1), token.SHL, wideWidth),
				class: rules.ScalarUint,
				bits:  wideWidth,
			},
			{
				name: "reports false for a negative value of a 128-bit unsigned width",
				give: constant.MakeInt64(-1), class: rules.ScalarUint, bits: wideWidth,
			},
			{
				name: "reports false for a fraction",
				give: constant.MakeFromLiteral("1.5", token.FLOAT, 0), class: rules.ScalarInt, bits: singleWidth,
			},
			{
				name: "reports false for a string",
				give: constant.MakeString("3"), class: rules.ScalarInt, bits: singleWidth,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, ok := numeric.Int(tt.give, tt.class, tt.bits)
				assert.False(t, ok, "the value is outside the type")
			})
		}

		t.Run("returns the largest value of an unsigned width", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Int(constant.MakeUint64(math.MaxUint32), rules.ScalarUint, singleWidth)
			assert.True(t, ok, "the largest uint32 is a uint32")
			assert.Equal(t, v.Text, "4294967295", "as decimal text")
		})

		t.Run("returns the largest value of a 64-bit unsigned width", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Int(constant.MakeUint64(math.MaxUint64), rules.ScalarUint, doubleWidth)
			assert.True(t, ok, "the largest uint64 is a uint64")
			assert.Equal(t, v.Text, "18446744073709551615", "as decimal text")
		})

		t.Run("bounds an unstated width as 64 bits", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Int(constant.MakeInt64(math.MaxInt64), rules.ScalarInt, noWidth)
			assert.True(t, ok, "the largest int64 fits an unstated width")
			assert.Equal(t, v, emit.Number(emit.LiteralInt, "9223372036854775807", noWidth),
				"and the number states no width")
		})

		t.Run("returns a value of a type wider than 64 bits", func(t *testing.T) {
			t.Parallel()

			big := constant.Shift(constant.MakeInt64(1), token.SHL, doubleWidth)
			v, ok := numeric.Int(big, rules.ScalarInt, wideWidth)
			assert.True(t, ok, "2^64 is an int128")
			assert.Equal(t, v.Text, "18446744073709551616", "as decimal text")
		})

		t.Run("returns the largest value of a 128-bit unsigned width", func(t *testing.T) {
			t.Parallel()

			largest := constant.BinaryOp(constant.Shift(constant.MakeInt64(1), token.SHL, wideWidth),
				token.SUB, constant.MakeInt64(1))
			v, ok := numeric.Int(largest, rules.ScalarUint, wideWidth)
			assert.True(t, ok, "2^128-1 is a uint128")
			assert.Equal(t, v.Text, "340282366920938463463374607431768211455", "as decimal text")
		})

		t.Run("returns an integral float as an integer", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Int(literal(t, "3.0", token.FLOAT), rules.ScalarInt, singleWidth)
			assert.True(t, ok, "an integral float is an integer")
			assert.Equal(t, v.Text, "3", "written as one")
		})
	})

	t.Run("Float", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the shortest text at double precision", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Float(literal(t, "0.1", token.FLOAT), doubleWidth)
			assert.True(t, ok, "a tenth is a float64")
			assert.Equal(t, v, emit.Number(emit.LiteralFloat, "0.1", doubleWidth), "written as 0.1")
		})

		t.Run("writes the shortest text at single precision", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Float(literal(t, "0.1", token.FLOAT), singleWidth)
			assert.True(t, ok, "a tenth is a float32")
			assert.Equal(t, v, emit.Number(emit.LiteralFloat, "0.1", singleWidth),
				"written as the shortest text that reads back at single precision")
		})

		t.Run("reads an unstated width at double precision", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Float(literal(t, "1.5", token.FLOAT), noWidth)
			assert.True(t, ok, "an unstated width takes the value")
			assert.Equal(t, v, emit.Number(emit.LiteralFloat, "1.5", noWidth), "and the number states no width")
		})

		t.Run("returns an integer as a float", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Float(constant.MakeInt64(5), doubleWidth)
			assert.True(t, ok, "an integer reads as a float")
			assert.Equal(t, v.Text, "5", "written without a point")
		})

		t.Run("returns a value a float64 takes past the largest float32", func(t *testing.T) {
			t.Parallel()

			v, ok := numeric.Float(literal(t, "1e39", token.FLOAT), doubleWidth)
			assert.True(t, ok, "a float64 takes 1e39")
			assert.Equal(t, v.Text, "1e+39", "in exponent notation")
		})

		tests := []struct {
			name string
			give constant.Value
			bits int
		}{
			{
				name: "reports false for a value single precision rounds to infinity",
				give: constant.MakeFloat64(1e39),
				bits: singleWidth,
			},
			{
				name: "reports false for a value double precision rounds to infinity",
				give: literalOf("1e309"),
				bits: doubleWidth,
			},
			{name: "reports false for a string", give: constant.MakeString("1.5"), bits: doubleWidth},
			{name: "reports false for a truth value", give: constant.MakeBool(true), bits: doubleWidth},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, ok := numeric.Float(tt.give, tt.bits)
				assert.False(t, ok, "the value is no float of the width")
			})
		}
	})

	t.Run("Decimal", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give float64
			bits int
			want string
		}{
			{name: "writes zero as 0", give: 0, bits: doubleWidth, want: "0"},
			{name: "writes a plain fraction positionally", give: 123.5, bits: doubleWidth, want: "123.5"},
			{name: "writes 1e-6 positionally", give: 1e-6, bits: doubleWidth, want: "0.000001"},
			{
				name: "writes a value under 1e21 positionally",
				give: 1e20,
				bits: doubleWidth,
				want: "100000000000000000000",
			},
			{name: "writes 1e21 in exponent notation", give: 1e21, bits: doubleWidth, want: "1e+21"},
			{name: "writes a value under 1e-6 without a padded exponent", give: 1e-7, bits: doubleWidth, want: "1e-7"},
			{name: "writes both digits of a two-digit exponent", give: -1.5e-10, bits: doubleWidth, want: "-1.5e-10"},
			{
				name: "writes the shortest text that reads back at single precision",
				give: float64(float32(0.1)), bits: singleWidth, want: "0.1",
			},
			{
				name: "writes every digit of a widened float32 at double precision",
				give: float64(float32(0.1)), bits: doubleWidth, want: "0.10000000149011612",
			},
			{
				name: "compares the upper cutoff at single precision",
				give: float64(float32(1e21)), bits: singleWidth, want: "1e+21",
			},
			{
				name: "writes a value single precision rounds up to the cutoff in exponent notation",
				give: 9.9999999e20, bits: singleWidth, want: "1e+21",
			},
			{name: "reads an unstated width at double precision", give: 0.1, bits: noWidth, want: "0.1"},
			{name: "reads a width no float has at double precision", give: 0.1, bits: halfWidth, want: "0.1"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, numeric.Decimal(tt.give, tt.bits), tt.want, "the canonical text")
			})
		}
	})
}

// Each reading allocates its text, and an integer of a wider type its
// bounds, in the ordinary run, which runs no benchmark.
func TestNumericAllocs(t *testing.T) {
	large, small := constant.MakeInt64(math.MaxInt32), constant.MakeInt64(42)
	wide := constant.Shift(constant.MakeInt64(1), token.SHL, doubleWidth)
	tenth := constant.MakeFloat64(0.1)
	var (
		v  emit.Value
		ok bool
		s  string
	)
	assert.MaxAllocs(t, func() { v, ok = numeric.Int(large, rules.ScalarInt, singleWidth) }, textAllocs,
		"Int allocates the text")
	assert.True(t, ok, "Int reads the largest int32")
	assert.Equal(t, v.Text, "2147483647", "Int returns the largest int32")
	assert.MaxAllocs(t, func() { v, ok = numeric.Int(small, rules.ScalarInt, singleWidth) }, 0,
		"Int allocates nothing for a value whose text is static")
	assert.True(t, ok, "Int reads the small value")
	assert.Equal(t, v.Text, "42", "Int returns the small value")
	assert.MaxAllocs(t, func() { v, ok = numeric.Int(wide, rules.ScalarInt, wideWidth) }, wideIntAllocs,
		"Int allocates the exact bounds of a wider type")
	assert.True(t, ok, "Int returns 2^64 as an int128")
	assert.MaxAllocs(t, func() { v, ok = numeric.Float(tenth, doubleWidth) }, floatAllocs,
		"Float allocates the conversion and the text")
	assert.True(t, ok, "Float reads a tenth")
	assert.Equal(t, v.Text, "0.1", "Float returns a tenth")
	assert.MaxAllocs(t, func() { s = numeric.Decimal(-1.5e-10, doubleWidth) }, textAllocs,
		"Decimal allocates the text")
	assert.Equal(t, s, "-1.5e-10", "Decimal writes the text")
}

// BenchmarkNumeric measures each reading of an author's number: an
// integer at a machine width and at a wider one, a float, and the text
// of a float.
func BenchmarkNumeric(b *testing.B) {
	ints := []struct {
		name   string
		give   constant.Value
		bits   int
		allocs uint64
		want   string
	}{
		{
			name:   "a 32-bit type",
			give:   constant.MakeInt64(math.MaxInt32),
			bits:   singleWidth,
			allocs: textAllocs,
			want:   "2147483647",
		},
		{
			name: "a type wider than 64 bits", give: constant.Shift(constant.MakeInt64(1), token.SHL, doubleWidth),
			bits: wideWidth, allocs: wideIntAllocs, want: "18446744073709551616",
		},
	}
	b.Run("Int", func(b *testing.B) {
		for _, tt := range ints {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var v emit.Value
				for c.Loop() {
					v, _ = numeric.Int(tt.give, rules.ScalarInt, tt.bits)
				}
				assert.Equal(b, v.Text, tt.want, "Int returns the value as text")
			})
		}
	})

	b.Run("Float", func(b *testing.B) {
		tenth := constant.MakeFloat64(0.1)
		c := bench.Start(b).MaxAllocs(floatAllocs)
		defer c.End()
		var v emit.Value
		for c.Loop() {
			v, _ = numeric.Float(tenth, doubleWidth)
		}
		assert.Equal(b, v.Text, "0.1", "Float returns a tenth")
	})

	b.Run("Decimal", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(textAllocs)
		defer c.End()
		var s string
		for c.Loop() {
			s = numeric.Decimal(-1.5e-10, doubleWidth)
		}
		assert.Equal(b, s, "-1.5e-10", "Decimal writes the text")
	})
}

// literal reads one Go number literal into its exact constant.
func literal(tb assert.TB, text string, kind token.Token) constant.Value {
	tb.Helper()

	v := constant.MakeFromLiteral(text, kind, 0)
	assert.NotEqual(tb, v.Kind(), constant.Unknown, "the fixture literal "+text+" reads")
	return v
}

// literalOf reads one Go float literal into its exact constant, for a
// table built before a test runs.
func literalOf(text string) constant.Value {
	return constant.MakeFromLiteral(text, token.FLOAT, 0)
}
