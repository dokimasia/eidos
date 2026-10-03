// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/wire"
)

// overflow is a varint of eleven bytes, longer than any 64-bit integer
// encodes to.
var overflow = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}

// errFound is a failure a caller finds in a value it decoded.
var errFound = errors.New("wire_test: the value is not one the caller knows")

// record is one encoding of every value a decoder reads, in order: an
// unsigned integer, a signed one, a byte, a bool, a count, a byte
// string, a text and a digest.
func record() []byte {
	digest := sha256.Sum256([]byte("x"))
	b := binary.AppendUvarint(nil, 300)
	b = binary.AppendVarint(b, -7)
	b = append(b, 0xab)
	b = wire.AppendBool(b, true)
	b = binary.AppendUvarint(b, 2)
	b = wire.AppendBytes(b, []byte("ab"))
	b = wire.AppendText(b, "cd")
	return append(b, digest[:]...)
}

// A decoder is the boundary every stored record crosses back into the
// kernel, so it returns each value once and refuses every malformed
// encoding with one error.
func TestDecoder(t *testing.T) {
	t.Parallel()

	t.Run("Read", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every value a record holds in order", func(t *testing.T) {
			t.Parallel()

			b := record()
			d := wire.NewDecoder(b)
			assert.Equal(t, d.Uvarint(), uint64(300), "the unsigned integer")
			assert.Equal(t, d.Varint(), int64(-7), "the signed integer")
			assert.Equal(t, d.Byte(), byte(0xab), "the byte")
			assert.True(t, d.Bool(), "the bool")
			assert.Equal(t, d.Count(), 2, "the count")
			assert.Equal(t, d.Bytes(), []byte("ab"), "the byte string")
			assert.Equal(t, d.Text(), "cd", "the text")
			assert.Equal(t, d.Digest(), sha256.Sum256([]byte("x")), "the digest")
			assert.NoError(t, d.Err(), "a whole record decodes")
			assert.Equal(t, d.Read(), len(b), "and every byte is read")
			assert.Equal(t, d.Len(), 0, "and none is left")
		})
	})

	t.Run("Err", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give []byte
			read func(d *wire.Decoder)
		}{
			{
				name: "returns ErrMalformed for an unsigned integer that overflows",
				give: overflow,
				read: func(d *wire.Decoder) { d.Uvarint() },
			},
			{
				name: "returns ErrMalformed for a signed integer that overflows",
				give: overflow,
				read: func(d *wire.Decoder) { d.Varint() },
			},
			{
				name: "returns ErrMalformed for a byte past the end",
				give: nil,
				read: func(d *wire.Decoder) { d.Byte() },
			},
			{
				name: "returns ErrMalformed for a bool other than zero and one",
				give: []byte{2},
				read: func(d *wire.Decoder) { d.Bool() },
			},
			{
				name: "returns ErrMalformed for a count larger than the bytes left",
				give: []byte{5, 0},
				read: func(d *wire.Decoder) { d.Count() },
			},
			{
				name: "returns ErrMalformed for a text longer than the bytes left",
				give: []byte{3, 'a'},
				read: func(d *wire.Decoder) { d.Text() },
			},
			{
				name: "returns ErrMalformed for a digest past the end",
				give: []byte{1, 2, 3},
				read: func(d *wire.Decoder) { d.Digest() },
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				d := wire.NewDecoder(tt.give)
				tt.read(&d)
				assert.ErrorIs(t, d.Err(), wire.ErrMalformed, "the encoding is malformed")
			})
		}

		t.Run("returns zero values after the first failure", func(t *testing.T) {
			t.Parallel()

			d := wire.NewDecoder(append([]byte{2}, record()...))
			d.Bool()
			assert.Equal(t, d.Uvarint(), uint64(0), "a read after the failure returns zero")
			assert.ErrorIs(t, d.Err(), wire.ErrMalformed, "and the first failure stays")
		})
	})

	t.Run("Skip", func(t *testing.T) {
		t.Parallel()

		t.Run("consumes the bytes another decoder read from the rest", func(t *testing.T) {
			t.Parallel()

			d := wire.NewDecoder([]byte{1, 2, 3})
			d.Byte()
			assert.Equal(t, d.Rest(), []byte{2, 3}, "the rest is what is left to read")
			d.Skip(1)
			assert.Equal(t, d.Byte(), byte(3), "the byte after the skipped one reads next")
			assert.Equal(t, d.Read(), 3, "and every byte counts as read")
		})

		t.Run("returns ErrMalformed for more bytes than are left", func(t *testing.T) {
			t.Parallel()

			d := wire.NewDecoder([]byte{1})
			d.Skip(2)
			assert.ErrorIs(t, d.Err(), wire.ErrMalformed, "the skip runs past the end")
		})
	})

	t.Run("Fail", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps the first failure", func(t *testing.T) {
			t.Parallel()

			d := wire.NewDecoder(record())
			d.Fail(errFound)
			d.Fail(wire.ErrMalformed)
			assert.ErrorIs(t, d.Err(), errFound, "the caller's failure is the decode's")
			assert.Equal(t, d.Len(), 0, "and nothing is left to read")
		})
	})
}

// TestDecoderZeroAlloc checks that every read but a text allocates
// nothing. The check runs alone, because AllocsPerRun refuses to run
// beside parallel tests.
func TestDecoderZeroAlloc(t *testing.T) {
	b := record()
	assert.MaxAllocs(t, func() {
		d := wire.NewDecoder(b)
		d.Uvarint()
		d.Varint()
		d.Byte()
		d.Bool()
		d.Count()
		d.Bytes()
		if d.Err() != nil {
			t.Fatal(d.Err())
		}
	}, 0, "the reads of the record before its text allocate nothing")
}

// BenchmarkDecoder measures a whole record's decode: every read is free
// of allocation but the text, which allocates its string once.
func BenchmarkDecoder(b *testing.B) {
	rec := record()

	b.Run("Read", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var (
			read int
			text string
		)
		for c.Loop() {
			d := wire.NewDecoder(rec)
			d.Uvarint()
			d.Varint()
			d.Byte()
			d.Bool()
			d.Count()
			d.Bytes()
			text = d.Text()
			d.Digest()
			read = d.Read()
		}
		if read != len(rec) || text != "cd" {
			b.Fatalf("the decode read %d of %d bytes and the text %q", read, len(rec), text)
		}
	})
}
