// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile

// The bit patterns of modified UTF-8 (§4.4.7): the leading bits that mark
// a two-byte and a three-byte sequence, the masks that select them, the
// leading bits of a continuation byte and its mask, and the payload masks
// of each byte form.
const (
	twoByteLead    = 0xC0
	twoByteMask    = 0xE0
	threeByteLead  = 0xE0
	threeByteMask  = 0xF0
	continuation   = 0x80
	continueMask   = 0xC0
	twoBytePayload = 0x1F
	threeBytes     = 0x0F
	sixBits        = 0x3F
	asciiLimit     = 0x80
)

// decodeMUTF8 returns the UTF-16 code units a modified UTF-8 string
// encodes (§4.4.7): one byte for U+0001 to U+007F, two bytes for U+0000
// and U+0080 to U+07FF, three bytes for U+0800 to U+FFFF, and a
// supplementary character as its two surrogates of three bytes each. It
// reports false for a zero byte, a byte from 0xF0 to 0xFF, and a
// sequence that ends early or lacks a continuation byte.
func decodeMUTF8(b []byte) ([]uint16, bool) {
	out := make([]uint16, 0, len(b))
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case c != 0 && c < asciiLimit:
			out = append(out, uint16(c))
			i++
		case c&twoByteMask == twoByteLead && i+1 < len(b) && b[i+1]&continueMask == continuation:
			out = append(out, uint16(c&twoBytePayload)<<6|uint16(b[i+1]&sixBits))
			i += 2
		case c&threeByteMask == threeByteLead && i+2 < len(b) && b[i+1]&continueMask == continuation &&
			b[i+2]&continueMask == continuation:
			out = append(out, uint16(c&threeBytes)<<12|uint16(b[i+1]&sixBits)<<6|uint16(b[i+2]&sixBits))
			i += 3
		default:
			return nil, false
		}
	}
	return out, true
}

// ascii reports whether every byte is from U+0001 to U+007F, which
// modified UTF-8 encodes as the byte itself.
func ascii(b []byte) bool {
	for _, c := range b {
		if c == 0 || c >= asciiLimit {
			return false
		}
	}
	return true
}
