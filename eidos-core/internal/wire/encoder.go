// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package wire

import "encoding/binary"

// AppendBool appends a bool as one byte, one for true and zero for
// false: the encoding [Decoder.Bool] reads.
func AppendBool(dst []byte, b bool) []byte {
	if b {
		return append(dst, 1)
	}
	return append(dst, 0)
}

// AppendBytes appends b's length as a uvarint and then b: the encoding
// [Decoder.Bytes] reads.
func AppendBytes(dst, b []byte) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(b)))
	return append(dst, b...)
}

// AppendText appends s's length as a uvarint and then s: the encoding
// [Decoder.Text] reads.
func AppendText(dst []byte, s string) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(s)))
	return append(dst, s...)
}
