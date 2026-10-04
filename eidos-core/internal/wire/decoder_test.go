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

// The encodings each read's cases and benchmark decode: one value each.
var (
	uvarint300  = binary.AppendUvarint(nil, 300)
	varintMinus = binary.AppendVarint(nil, -7)
	byteAB      = []byte{0xab}
	boolTrue    = wire.AppendBool(nil, true)
	countTwo    = []byte{2, 0, 0}
	bytesAB     = wire.AppendBytes(nil, []byte("ab"))
	textCD      = wire.AppendText(nil, "cd")
	digestX     = sha256.Sum256([]byte("x"))
)

// A decoder is the boundary every stored record crosses back into the
// kernel, so it returns each value once and refuses every malformed
// encoding with one error.
func TestDecoder(t *testing.T) {
	t.Parallel()

	t.Run("NewDecoder", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a decoder over the bytes that has read none", func(t *testing.T) {
			t.Parallel()

			d := wire.NewDecoder(record())
			assert.Equal(t, d.Read(), 0, "nothing is read")
			assert.Equal(t, d.Len(), len(record()), "and every byte is left")
		})
	})

	t.Run("Err", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a record read whole", func(t *testing.T) {
			t.Parallel()

			d := wire.NewDecoder(record())
			d.Uvarint()
			d.Varint()
			d.Byte()
			d.Bool()
			d.Count()
			d.Bytes()
			d.Text()
			d.Digest()
			assert.NoError(t, d.Err(), "a whole record decodes")
		})

		t.Run("returns the first failure after a later read", func(t *testing.T) {
			t.Parallel()

			d := wire.NewDecoder(append([]byte{2}, record()...))
			d.Bool()
			assert.Equal(t, d.Uvarint(), uint64(0), "a read after the failure returns zero")
			assert.ErrorIs(t, d.Err(), wire.ErrMalformed, "and Err returns the first failure")
		})
	})

	t.Run("Read", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every byte of a record read whole", func(t *testing.T) {
			t.Parallel()

			b := record()
			d := wire.NewDecoder(b)
			d.Uvarint()
			d.Varint()
			d.Byte()
			d.Bool()
			d.Count()
			d.Bytes()
			d.Text()
			d.Digest()
			assert.Equal(t, d.Read(), len(b), "every byte is read")
		})
	})

	t.Run("Len", func(t *testing.T) {
		t.Parallel()

		t.Run("returns zero for a record read whole", func(t *testing.T) {
			t.Parallel()

			d := wire.NewDecoder(byteAB)
			d.Byte()
			assert.Equal(t, d.Len(), 0, "none is left")
		})
	})

	t.Run("Rest", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the bytes left to read", func(t *testing.T) {
			t.Parallel()

			d := wire.NewDecoder([]byte{1, 2, 3})
			d.Byte()
			assert.Equal(t, d.Rest(), []byte{2, 3}, "the rest is what is left to read")
		})
	})

	t.Run("Skip", func(t *testing.T) {
		t.Parallel()

		t.Run("consumes the bytes another decoder read from the rest", func(t *testing.T) {
			t.Parallel()

			d := wire.NewDecoder([]byte{1, 2, 3})
			d.Byte()
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
		})

		t.Run("leaves nothing to read", func(t *testing.T) {
			t.Parallel()

			d := wire.NewDecoder(record())
			d.Fail(errFound)
			assert.Equal(t, d.Len(), 0, "the decode ends")
		})
	})

	reads := []struct {
		method string
		ok     []byte
		want   any
		read   func(d *wire.Decoder) any
		bad    []byte
		fault  string
	}{
		{
			method: "Uvarint", ok: uvarint300, want: uint64(300),
			read: func(d *wire.Decoder) any { return d.Uvarint() },
			bad:  overflow, fault: "returns ErrMalformed for an unsigned integer that overflows",
		},
		{
			method: "Varint", ok: varintMinus, want: int64(-7),
			read: func(d *wire.Decoder) any { return d.Varint() },
			bad:  overflow, fault: "returns ErrMalformed for a signed integer that overflows",
		},
		{
			method: "Byte", ok: byteAB, want: byte(0xab),
			read: func(d *wire.Decoder) any { return d.Byte() },
			bad:  nil, fault: "returns ErrMalformed for a byte past the end",
		},
		{
			method: "Bool", ok: boolTrue, want: true,
			read: func(d *wire.Decoder) any { return d.Bool() },
			bad:  []byte{2}, fault: "returns ErrMalformed for a bool other than zero and one",
		},
		{
			method: "Count", ok: countTwo, want: 2,
			read: func(d *wire.Decoder) any { return d.Count() },
			bad:  []byte{5, 0}, fault: "returns ErrMalformed for a count larger than the bytes left",
		},
		{
			method: "Bytes", ok: bytesAB, want: []byte("ab"),
			read: func(d *wire.Decoder) any { return d.Bytes() },
			bad:  []byte{3, 'a'}, fault: "returns ErrMalformed for bytes longer than the bytes left",
		},
		{
			method: "Text", ok: textCD, want: "cd",
			read: func(d *wire.Decoder) any { return d.Text() },
			bad:  []byte{3, 'a'}, fault: "returns ErrMalformed for a text longer than the bytes left",
		},
		{
			method: "Digest", ok: digestX[:], want: digestX,
			read: func(d *wire.Decoder) any { return d.Digest() },
			bad:  []byte{1, 2, 3}, fault: "returns ErrMalformed for a digest past the end",
		},
	}
	for _, tt := range reads {
		t.Run(tt.method, func(t *testing.T) {
			t.Parallel()

			t.Run("returns the encoded value", func(t *testing.T) {
				t.Parallel()

				d := wire.NewDecoder(tt.ok)
				assert.Equal(t, tt.read(&d), tt.want, "the value its encoding states")
			})

			t.Run(tt.fault, func(t *testing.T) {
				t.Parallel()

				d := wire.NewDecoder(tt.bad)
				tt.read(&d)
				assert.ErrorIs(t, d.Err(), wire.ErrMalformed, "the encoding is malformed")
			})
		})
	}
}

// Every read but a text allocates nothing, and so does a caller's
// failure. A text allocates its string. The ordinary run, which runs no
// benchmark, checks these ceilings here. The check runs alone, because
// AllocsPerRun refuses to run beside parallel tests.
func TestDecoderAllocs(t *testing.T) {
	b := record()
	assert.MaxAllocs(t, func() {
		d := wire.NewDecoder(b)
		d.Uvarint()
		d.Varint()
		d.Byte()
		d.Bool()
		d.Count()
		d.Bytes()
		_ = d.Rest()
		d.Skip(len(textCD))
		if d.Digest() != digestX || d.Len() != 0 || d.Err() != nil || d.Read() != len(b) {
			t.Fatal("the record read back wrong")
		}
	}, 0, "every read of the record but its text allocates nothing")
	text := textCD
	var got string
	assert.MaxAllocs(t, func() {
		d := wire.NewDecoder(text)
		got = d.Text()
	}, 1, "Text allocates the string it returns")
	assert.Equal(t, got, "cd", "Text returns the text")
	assert.MaxAllocs(t, func() {
		d := wire.NewDecoder(text)
		d.Fail(errFound)
		if !errors.Is(d.Err(), errFound) {
			t.Fatal("Fail kept another failure")
		}
	}, 0, "Fail allocates nothing")
}

// BenchmarkDecoder measures each read over one value's encoding, and
// the decoder's own questions. Every read is free of allocation but the
// text, which allocates its string once.
func BenchmarkDecoder(b *testing.B) {
	b.Run("NewDecoder", func(b *testing.B) {
		rec := record()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var d wire.Decoder
		for c.Loop() {
			d = wire.NewDecoder(rec)
		}
		assert.Equal(b, d.Len(), len(rec), "the decoder has every byte left")
	})

	b.Run("Uvarint", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got uint64
		for c.Loop() {
			d := wire.NewDecoder(uvarint300)
			got = d.Uvarint()
		}
		assert.Equal(b, got, uint64(300), "the unsigned integer")
	})

	b.Run("Varint", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int64
		for c.Loop() {
			d := wire.NewDecoder(varintMinus)
			got = d.Varint()
		}
		assert.Equal(b, got, int64(-7), "the signed integer")
	})

	b.Run("Byte", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got byte
		for c.Loop() {
			d := wire.NewDecoder(byteAB)
			got = d.Byte()
		}
		assert.Equal(b, got, byte(0xab), "the byte")
	})

	b.Run("Bool", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			d := wire.NewDecoder(boolTrue)
			got = d.Bool()
		}
		assert.True(b, got, "the bool")
	})

	b.Run("Count", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			d := wire.NewDecoder(countTwo)
			got = d.Count()
		}
		assert.Equal(b, got, 2, "the count")
	})

	b.Run("Bytes", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got []byte
		for c.Loop() {
			d := wire.NewDecoder(bytesAB)
			got = d.Bytes()
		}
		assert.Equal(b, got, []byte("ab"), "the byte string")
	})

	b.Run("Text", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got string
		for c.Loop() {
			d := wire.NewDecoder(textCD)
			got = d.Text()
		}
		assert.Equal(b, got, "cd", "the text")
	})

	b.Run("Digest", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got [sha256.Size]byte
		for c.Loop() {
			d := wire.NewDecoder(digestX[:])
			got = d.Digest()
		}
		assert.Equal(b, got, digestX, "the digest")
	})

	b.Run("Skip", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var d wire.Decoder
		for c.Loop() {
			d = wire.NewDecoder(textCD)
			d.Skip(1)
		}
		assert.Equal(b, d.Read(), 1, "the skipped byte counts as read")
	})

	b.Run("Fail", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var d wire.Decoder
		for c.Loop() {
			d = wire.NewDecoder(textCD)
			d.Fail(errFound)
		}
		assert.ErrorIs(b, d.Err(), errFound, "the caller's failure")
	})

	rec := wire.NewDecoder(record())
	rec.Byte()

	b.Run("Err", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got error
		for c.Loop() {
			got = rec.Err()
		}
		assert.NoError(b, got, "no read failed")
	})

	b.Run("Read", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got = rec.Read()
		}
		assert.Equal(b, got, 1, "the one byte read")
	})

	b.Run("Len", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got = rec.Len()
		}
		assert.Equal(b, got, len(record())-1, "every byte after the first")
	})

	b.Run("Rest", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got []byte
		for c.Loop() {
			got = rec.Rest()
		}
		assert.Length(b, got, len(record())-1, "the bytes left to read")
	})
}

// record is one encoding of every value a decoder reads, in order: an
// unsigned integer, a signed one, a byte, a bool, a count, a byte
// string, a text and a digest.
func record() []byte {
	b := binary.AppendUvarint(nil, 300)
	b = binary.AppendVarint(b, -7)
	b = append(b, 0xab)
	b = wire.AppendBool(b, true)
	b = binary.AppendUvarint(b, 2)
	b = wire.AppendBytes(b, []byte("ab"))
	b = wire.AppendText(b, "cd")
	return append(b, digestX[:]...)
}
