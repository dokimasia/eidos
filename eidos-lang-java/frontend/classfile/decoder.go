// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile

import (
	"encoding/binary"
	"fmt"
)

// decoder reads a class file's items in order: big-endian unsigned
// integers of one, two and four bytes, and runs of bytes. A read past the
// end returns zero and sets the decoder's error, which keeps the first
// fault, and the caller checks the error once per structure.
type decoder struct {
	data []byte
	err  error
}

// u1 returns the next byte.
func (d *decoder) u1() uint8 {
	b, ok := d.take(1)
	if !ok {
		return 0
	}
	return b[0]
}

// u2 returns the next two bytes as a big-endian integer.
func (d *decoder) u2() uint16 {
	b, ok := d.take(2)
	if !ok {
		return 0
	}
	return binary.BigEndian.Uint16(b)
}

// u4 returns the next four bytes as a big-endian integer.
func (d *decoder) u4() uint32 {
	b, ok := d.take(4)
	if !ok {
		return 0
	}
	return binary.BigEndian.Uint32(b)
}

// take returns the next n bytes and advances past them, and reports
// false where fewer than n bytes are left. The bytes alias the class
// file's.
func (d *decoder) take(n uint32) ([]byte, bool) {
	if uint64(n) > uint64(len(d.data)) {
		d.fail("%d bytes are left where %d are read", len(d.data), n)
		return nil, false
	}
	b := d.data[:n]
	d.data = d.data[n:]
	return b, true
}

// fail records the decoder's first error, which wraps [ErrMalformed].
func (d *decoder) fail(format string, args ...any) {
	if d.err == nil {
		d.err = fmt.Errorf("%w: %s", ErrMalformed, fmt.Sprintf(format, args...))
	}
}
