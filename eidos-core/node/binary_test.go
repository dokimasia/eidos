// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package node_test

import (
	"encoding/binary"
	"math"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/prop"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// overflow is a varint of eleven bytes, longer than any 64-bit integer
// encodes to.
var overflow = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}

// decodesToFixedPoint is the property [FuzzDecodeBinary] and its ForAll
// twin in [TestBinary] state.
const decodesToFixedPoint = "DecodeBinary must return an error or a symbol whose encoding " +
	"decodes and encodes to the same bytes again"

// The binary encoding is a boundary: a generation and the memo hand
// back bytes a previous run, another build or a damaged file wrote. A
// decode of bad bytes returns an error and never a partial symbol.
func TestBinary(t *testing.T) {
	t.Parallel()

	t.Run("DecodeBinary", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the bytes one symbol read and leaves the rest", func(t *testing.T) {
			t.Parallel()

			first, err := node.AppendBinary(nil, coretest.Struct(coretest.StorePath, "Store"), nil)
			assert.NoError(t, err, "the first symbol encodes")
			both, err := node.AppendBinary(slices.Clone(first), coretest.Struct(coretest.StorePath, "Cache"), nil)
			assert.NoError(t, err, "and the second behind it")

			decoded, read, err := node.DecodeBinary(both, nil)
			assert.NoError(t, err, "the first symbol decodes")
			assert.Equal(t, read, len(first), "from its own bytes alone")
			assert.Equal(t, decoded.(*node.Struct).Name, "Store", "and is the first symbol")
		})

		t.Run("decodes a zero byte to a nil symbol", func(t *testing.T) {
			t.Parallel()

			decoded, read, err := node.DecodeBinary([]byte{0}, nil)
			assert.NoError(t, err, "a zero byte decodes")
			assert.Nil(t, decoded, "to nothing")
			assert.Equal(t, read, 1, "from one byte")
		})

		// A package's files are its last field, so the last byte of an
		// empty package is the count of its files. An import's wildcard is
		// its last field. A package's kind, the six fields of its identity
		// and the file of its position come before the position's line,
		// and the line and the column come before the count of the
		// package's documentation lines.
		pkg := encoded(t, &node.Package{})
		wrongKind := append(slices.Clone(pkg[:len(pkg)-1]), 1)
		wrongKind = append(wrongKind, encoded(t, &node.Struct{})...)
		badBool := encoded(t, &node.Import{})
		badBool[len(badBool)-1] = 2
		longList := slices.Clone(pkg)
		longList[len(longList)-1] = 100
		tests := []struct {
			name string
			give []byte
		}{
			{name: "returns ErrMalformed for a kind the model does not declare", give: []byte{0xff}},
			{name: "returns ErrMalformed for a kind in a field that admits another", give: wrongKind},
			{name: "returns ErrMalformed for a bool other than zero and one", give: badBool},
			{name: "returns ErrMalformed for a list longer than the bytes left", give: longList},
			{
				name: "returns ErrMalformed for a signed integer that overflows",
				give: append([]byte{byte(symbol.KindPackage), 0, 0, 0, 0, 0, 0, 0}, overflow...),
			},
			{
				name: "returns ErrMalformed for a list length that overflows",
				give: append([]byte{byte(symbol.KindPackage), 0, 0, 0, 0, 0, 0, 0, 0, 0}, overflow...),
			},
			{
				name: "returns ErrMalformed for a string longer than the bytes left",
				give: []byte{byte(symbol.KindPackage), 9, 'g', 'o'},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				decoded, read, err := node.DecodeBinary(tt.give, nil)
				assert.ErrorIs(t, err, node.ErrMalformed, "bad bytes do not decode")
				assert.Nil(t, decoded, "and return no partial symbol")
				assert.Equal(t, read, 0, "and no byte count")
			})
		}

		t.Run("returns ErrMalformed for a string the table does not contain", func(t *testing.T) {
			t.Parallel()

			var table node.StringTable
			encodedWithTable, err := node.AppendBinary(nil, coretest.Struct(coretest.StorePath, "Store"), &table)
			assert.NoError(t, err, "the symbol encodes against a table")
			_, _, err = node.DecodeBinary(encodedWithTable, &node.StringTable{})
			assert.ErrorIs(t, err, node.ErrMalformed, "an empty table numbers none of its strings")
		})

		t.Run("returns an error or a symbol that is a fixed point after one more trip", func(t *testing.T) {
			t.Parallel()

			prop.ForAll(t, decodesToFixedPoint, decodesFixed)
		})
	})
}

// FuzzDecodeBinary checks [decodesToFixedPoint] on bytes nothing in this
// repository produced. Each seed is the choices of one case: a two-byte
// little-endian length, then the bytes of the input.
func FuzzDecodeBinary(f *testing.F) {
	inputs := [][]byte{{0xff}, overflow}
	for _, seed := range []symbol.Symbol{
		coretest.EveryKind(coretest.StorePath),
		coretest.Package(coretest.CachePath, coretest.Struct(coretest.CachePath, "Cache")),
		nil,
	} {
		b, err := node.AppendBinary(nil, seed, nil)
		assert.NoError(f, err, "every seed symbol encodes")
		inputs = append(inputs, b)
	}
	for _, input := range inputs {
		assert.InRange(f, len(input), 0, math.MaxUint16, "a seed's length fits the bridge's two bytes")
		f.Add(append(binary.LittleEndian.AppendUint16(nil, uint16(len(input))), input...))
	}

	prop.Fuzz(f, decodesToFixedPoint, decodesFixed)
}

// decodesFixed checks [decodesToFixedPoint] on an input that the case
// draws. An input that does not decode satisfies it, and a symbol that
// decodes encodes, decodes from every byte of its encoding, and encodes
// to the same bytes again.
func decodesFixed(c *prop.Case) {
	in := c.Draw(prop.Bytes(), "input")
	decoded, _, err := node.DecodeBinary(in, nil)
	if err != nil {
		return
	}
	once, err := node.AppendBinary(nil, decoded, nil)
	assert.NoError(c, err, "AppendBinary must take every symbol DecodeBinary returns")
	assert.RoundTrip(c, func(b []byte) (symbol.Symbol, error) {
		again, read, err := node.DecodeBinary(b, nil)
		assert.Equal(c, read, len(b), "DecodeBinary must read every byte AppendBinary wrote")
		return again, err
	}, func(s symbol.Symbol) ([]byte, error) { return node.AppendBinary(nil, s, nil) }, once,
		"a second trip must encode the same bytes")
}

// encoded returns a symbol's encoding without a table.
func encoded(t *testing.T, s symbol.Symbol) []byte {
	t.Helper()

	b, err := node.AppendBinary(nil, s, nil)
	assert.NoError(t, err, "a symbol of the model encodes")
	return b
}
