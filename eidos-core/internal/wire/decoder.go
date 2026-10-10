// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// ErrMalformed is the class of every decoding failure: the bytes end
// inside a value, an integer overflows or does not fit the type that the
// caller reads, a bool is neither zero nor one, or a length exceeds the
// bytes left. A caller tests for it with [errors.Is].
var ErrMalformed = errors.New("wire: malformed encoding")

// Decoder reads values from the front of a byte slice in sequence,
// counting the bytes it read. Its first failure ends the decode: every
// later read returns the zero value, and [Decoder.Err] returns the
// failure.
//
// The zero Decoder reads nothing and fails on its first read.
//
// # Concurrency
//
// A Decoder is not safe for concurrent use.
//
// # Allocation contract
//
// A read allocates nothing, except [Decoder.Text], which allocates the
// string it returns, and a failure, which allocates its error once.
type Decoder struct {
	rest []byte
	read int
	err  error
}

// NewDecoder returns a decoder over b.
func NewDecoder(b []byte) Decoder { return Decoder{rest: b} }

// Err returns the decoder's first failure, nil while every read
// succeeded.
func (d *Decoder) Err() error { return d.err }

// Read returns how many bytes the decoder consumed.
func (d *Decoder) Read() int { return d.read }

// Len returns how many bytes are left to read.
func (d *Decoder) Len() int { return len(d.rest) }

// Rest returns the bytes left to read, a window of the decoder's input,
// for a value another decoder reads. [Decoder.Skip] then consumes what
// that decoder read.
func (d *Decoder) Rest() []byte { return d.rest }

// Skip consumes n bytes another decoder read from [Decoder.Rest], and
// fails for more bytes than are left.
func (d *Decoder) Skip(n int) {
	if n < 0 || n > len(d.rest) {
		d.fail("a value read elsewhere does not fit the bytes left")
		return
	}
	d.advance(n)
}

// Fail records a failure the caller found in a value it decoded, such
// as a kind the caller does not know, and ends the decode. The first
// failure recorded is the one [Decoder.Err] returns.
func (d *Decoder) Fail(err error) {
	if d.err == nil {
		d.err = err
	}
	d.rest = nil
}

// Uvarint reads an unsigned integer.
func (d *Decoder) Uvarint() uint64 {
	u, n := binary.Uvarint(d.rest)
	if n <= 0 {
		d.fail("an unsigned integer ends or overflows")
		return 0
	}
	d.advance(n)
	return u
}

// Enum reads the value of an enumeration, an unsigned integer that fits
// a byte, and fails for a larger value.
func (d *Decoder) Enum() uint8 {
	u := d.Uvarint()
	if u > math.MaxUint8 {
		d.fail("an enumeration does not fit a byte")
		return 0
	}
	return uint8(u)
}

// Int reads an unsigned integer that fits an int, such as a count or an
// offset, and fails for a larger value.
func (d *Decoder) Int() int {
	u := d.Uvarint()
	if u > math.MaxInt {
		d.fail("an unsigned integer does not fit an int")
		return 0
	}
	return int(u)
}

// Int64 reads an unsigned integer that fits an int64, such as the offset
// of a region in a segment, and fails for a larger value.
func (d *Decoder) Int64() int64 {
	u := d.Uvarint()
	if u > math.MaxInt64 {
		d.fail("an unsigned integer does not fit an int64")
		return 0
	}
	return int64(u)
}

// Varint reads a signed integer.
func (d *Decoder) Varint() int64 {
	i, n := binary.Varint(d.rest)
	if n <= 0 {
		d.fail("an integer ends or overflows")
		return 0
	}
	d.advance(n)
	return i
}

// Byte reads one byte.
func (d *Decoder) Byte() byte {
	if len(d.rest) == 0 {
		d.fail("the encoding ends")
		return 0
	}
	b := d.rest[0]
	d.advance(1)
	return b
}

// Bool reads a bool, and fails for a byte other than zero and one, so
// every value has one encoding.
func (d *Decoder) Bool() bool {
	switch d.Byte() {
	case 0:
		return false
	case 1:
		return true
	default:
		d.fail("a bool is neither zero nor one")
		return false
	}
}

// Count reads a list's length, and fails for a length larger than the
// bytes left: every element takes at least one byte, so a larger length
// is a corrupt encoding and never a long list.
func (d *Decoder) Count() int {
	n := d.Int()
	if n > len(d.rest) {
		d.fail("a list does not fit the bytes left")
		return 0
	}
	return n
}

// Bytes reads a length and that many bytes, and returns them as a
// window of the decoder's input, which the caller copies to keep.
func (d *Decoder) Bytes() []byte {
	n := d.Count()
	b := d.rest[:n:n]
	d.advance(n)
	return b
}

// Text reads a length and that many bytes as a string.
func (d *Decoder) Text() string { return string(d.Bytes()) }

// Digest reads a SHA-256 digest: its 32 bytes, with no length.
func (d *Decoder) Digest() [sha256.Size]byte {
	var out [sha256.Size]byte
	if len(d.rest) < sha256.Size {
		d.fail("a digest does not fit the bytes left")
		return out
	}
	copy(out[:], d.rest)
	d.advance(sha256.Size)
	return out
}

// advance consumes n bytes the caller read.
func (d *Decoder) advance(n int) {
	d.rest = d.rest[n:]
	d.read += n
}

// fail records a malformed value at the current byte.
func (d *Decoder) fail(what string) {
	d.Fail(fmt.Errorf("%w: %s at byte %d", ErrMalformed, what, d.read))
}
