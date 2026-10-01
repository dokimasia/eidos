// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile_test

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
)

// The descriptors of the fields a constant initializes.
const (
	byteDesc    = "B"
	charDesc    = "C"
	doubleDesc  = "D"
	floatDesc   = "F"
	longDesc    = "J"
	shortDesc   = "S"
	booleanDesc = "Z"
)

// A constant value is spelled as the Java literal that states it, so the
// spelling of every constant type, and of the values no literal states,
// is pinned.
func TestLiteral(t *testing.T) {
	t.Parallel()

	t.Run("intLiteral", func(t *testing.T) {
		t.Parallel()

		t.Run("spells an int constant in decimal", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldNamed(t, fixture(t, boxFile), "LIMIT").Value, "16", "LIMIT")
		})

		t.Run("spells a negative int constant with its sign", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, constantOf(t, intDesc, tagInteger, u4(-5)), "-5", "minus five")
		})

		t.Run("spells a byte constant in decimal", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, constantOf(t, byteDesc, tagInteger, u4(7)), "7", "a byte is stored as an int")
		})

		t.Run("spells a short constant in decimal", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, constantOf(t, shortDesc, tagInteger, u4(300)), "300", "a short is stored as an int")
		})
	})

	t.Run("longLiteral", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a long constant with its suffix", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldNamed(t, fixture(t, boxFile), "BIG").Value, "1099511627776L", "1L << 40")
		})
	})

	t.Run("boolLiteral", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a nonzero boolean constant true", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, constantOf(t, booleanDesc, tagInteger, u4(2)), "true", "any nonzero int")
		})

		t.Run("spells a zero boolean constant false", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, constantOf(t, booleanDesc, tagInteger, u4(0)), "false", "zero")
		})
	})

	t.Run("floatLiteral", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give float32
			want string
		}{
			{name: "spells a float constant with its suffix", give: 1.5, want: "1.5f"},
			{name: "spells a large float constant with an exponent", give: 1e20, want: "1e+20f"},
			{name: "spells a float NaN as Float.NaN", give: float32(math.NaN()), want: "Float.NaN"},
			{name: "spells a positive float infinity", give: float32(math.Inf(1)), want: "Float.POSITIVE_INFINITY"},
			{name: "spells a negative float infinity", give: float32(math.Inf(-1)), want: "Float.NEGATIVE_INFINITY"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				bits := binary.BigEndian.AppendUint32(nil, math.Float32bits(tt.give))
				assert.Equal(t, constantOf(t, floatDesc, tagFloat, bits), tt.want, "the literal")
			})
		}
	})

	t.Run("doubleLiteral", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give float64
			want string
		}{
			{name: "spells a double constant", give: 0.5, want: "0.5"},
			{name: "spells an integral double constant with a fraction", give: 2, want: "2.0"},
			{name: "spells a large double constant with an exponent", give: 1e300, want: "1e+300"},
			{name: "spells a double NaN as Double.NaN", give: math.NaN(), want: "Double.NaN"},
			{name: "spells a positive double infinity", give: math.Inf(1), want: "Double.POSITIVE_INFINITY"},
			{name: "spells a negative double infinity", give: math.Inf(-1), want: "Double.NEGATIVE_INFINITY"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				bits := binary.BigEndian.AppendUint64(nil, math.Float64bits(tt.give))
				assert.Equal(t, constantOf(t, doubleDesc, tagDouble, bits), tt.want, "the literal")
			})
		}
	})

	t.Run("charLiteral", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a char constant between single quotes", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldNamed(t, fixture(t, boxFile), "LETTER").Value, "'x'", "LETTER")
		})

		t.Run("spells a single quote in a char constant behind a backslash", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, constantOf(t, charDesc, tagInteger, u4('\'')), `'\''`, "the quote")
		})

		t.Run("spells a double quote in a char constant as itself", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, constantOf(t, charDesc, tagInteger, u4('"')), `'"'`, "no escape")
		})
	})

	t.Run("stringLiteral", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a string constant with its escapes", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldNamed(t, fixture(t, boxFile), "NAME").Value, `"a\"b\\c\n`+unicodeEscape(0xe9)+`"`,
				"NAME")
		})

		t.Run("spells a single quote in a string constant as itself", func(t *testing.T) {
			t.Parallel()

			got, err := stringConstant("it's")
			assert.NoError(t, err, "the constant decodes")
			assert.Equal(t, got, `"it's"`, "no escape")
		})
	})

	t.Run("escape", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "spells a backspace as its escape", give: "\b", want: `"\b"`},
			{name: "spells a tab as its escape", give: "\t", want: `"\t"`},
			{name: "spells a form feed as its escape", give: "\f", want: `"\f"`},
			{name: "spells a carriage return as its escape", give: "\r", want: `"\r"`},
			{name: "spells another control character as a Unicode escape", give: "\x01", want: `"\u0001"`},
			{name: "spells a space as itself", give: " ", want: `" "`},
			{name: "spells a tilde as itself", give: "~", want: `"~"`},
			{name: "spells a delete as a Unicode escape", give: "\x7f", want: `"\u007f"`},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := stringConstant(tt.give)
				assert.NoError(t, err, "the constant decodes")
				assert.Equal(t, got, tt.want, "the literal")
			})
		}
	})
}

// constantOf assembles a class with one static field of a descriptor
// whose constant is of a tag and payload, and returns the field's value.
func constantOf(tb assert.TB, desc string, tag byte, payload []byte) string {
	tb.Helper()

	v, err := constantField(desc, tag, payload...)
	assert.NoError(tb, err, "the constant decodes")
	return v
}

// unicodeEscape returns each code unit's Java Unicode escape: a
// backslash, u, and the unit's four lowercase hexadecimal digits.
func unicodeEscape(units ...uint16) string {
	var b strings.Builder
	for _, u := range units {
		fmt.Fprintf(&b, "%cu%04x", '\\', u)
	}
	return b.String()
}

// javaString returns the Java string literal of code units, each
// spelled as its Unicode escape.
func javaString(units ...uint16) string { return `"` + unicodeEscape(units...) + `"` }
