// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile

import (
	"encoding/binary"
	"math"
	"unicode/utf16"
)

// The constant pool's tags (§4.4).
const (
	tagUtf8               = 1
	tagInteger            = 3
	tagFloat              = 4
	tagLong               = 5
	tagDouble             = 6
	tagClass              = 7
	tagString             = 8
	tagFieldref           = 9
	tagMethodref          = 10
	tagInterfaceMethodref = 11
	tagNameAndType        = 12
	tagMethodHandle       = 15
	tagMethodType         = 16
	tagDynamic            = 17
	tagInvokeDynamic      = 18
	tagModule             = 19
	tagPackage            = 20
)

// pool is a class file's constant pool: each constant's tag and the
// bytes that follow the tag, at the constant's index. A Utf8 constant's
// bytes are its string's, without the length before them. Index 0 and
// the index after a long or a double have no constant, and tag 0.
type pool struct {
	tags []uint8
	data [][]byte
}

// reader decodes one class file: the decoder over its bytes, and the
// constant pool its items index once the pool is read. A reader over one
// attribute's bytes shares its class file's pool.
type reader struct {
	decoder
	pool pool
}

// readPool reads the constant pool (§4.4). It fails the reader for an
// unknown tag, and for a long or a double at the last index, whose
// second slot is past the pool.
func (r *reader) readPool() {
	count := r.u2()
	p := pool{tags: make([]uint8, count), data: make([][]byte, count)}
	for i := uint16(1); i < count && r.err == nil; i++ {
		tag := r.u1()
		var size uint32
		switch tag {
		case tagUtf8:
			size = uint32(r.u2())
		case tagClass, tagString, tagMethodType, tagModule, tagPackage:
			size = 2
		case tagMethodHandle:
			size = 3
		case tagInteger, tagFloat, tagFieldref, tagMethodref, tagInterfaceMethodref, tagNameAndType, tagDynamic,
			tagInvokeDynamic:
			size = 4
		case tagLong, tagDouble:
			size = 8
		default:
			r.fail("constant %d has the unknown tag %d", i, tag)
			return
		}
		p.data[i], _ = r.take(size)
		p.tags[i] = tag
		if tag == tagLong || tag == tagDouble {
			if i+1 == count {
				r.fail("the eight-byte constant %d is at the pool's last index", i)
				return
			}
			i++
		}
	}
	r.pool = p
}

// entry returns the bytes of the constant at an index, and fails the
// reader for an index that names no constant of the tag.
func (r *reader) entry(i uint16, tag uint8) []byte {
	if int(i) >= len(r.pool.tags) || r.pool.tags[i] != tag {
		r.fail("constant %d is not of tag %d", i, tag)
		return nil
	}
	return r.pool.data[i]
}

// units returns the UTF-16 code units of the Utf8 constant at an index,
// and fails the reader for bytes that are not modified UTF-8.
func (r *reader) units(i uint16) []uint16 {
	u, ok := decodeMUTF8(r.entry(i, tagUtf8))
	if !ok {
		r.fail("constant %d is not modified UTF-8", i)
	}
	return u
}

// utf8 returns the Utf8 constant at an index as a string, an unpaired
// surrogate as U+FFFD.
func (r *reader) utf8(i uint16) string {
	if b := r.entry(i, tagUtf8); ascii(b) {
		return string(b)
	}
	return string(utf16.Decode(r.units(i)))
}

// optionalUtf8 returns the Utf8 constant at an index, and empty for index
// 0, which an item that may name nothing spells.
func (r *reader) optionalUtf8(i uint16) string {
	if i == 0 {
		return ""
	}
	return r.utf8(i)
}

// class returns what the Class constant at an index names: a binary
// name in internal form, or an array type's descriptor.
func (r *reader) class(i uint16) string {
	b := r.entry(i, tagClass)
	if len(b) < 2 {
		return ""
	}
	return r.utf8(binary.BigEndian.Uint16(b))
}

// optionalClass returns the Class constant at an index, and empty for
// index 0.
func (r *reader) optionalClass(i uint16) string {
	if i == 0 {
		return ""
	}
	return r.class(i)
}

// integer returns the Integer constant at an index.
func (r *reader) integer(i uint16) int32 {
	b := r.entry(i, tagInteger)
	if len(b) < 4 {
		return 0
	}
	return int32(binary.BigEndian.Uint32(b))
}

// long returns the Long constant at an index.
func (r *reader) long(i uint16) int64 {
	b := r.entry(i, tagLong)
	if len(b) < 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(b))
}

// float returns the Float constant at an index.
func (r *reader) float(i uint16) float32 {
	b := r.entry(i, tagFloat)
	if len(b) < 4 {
		return 0
	}
	return math.Float32frombits(binary.BigEndian.Uint32(b))
}

// double returns the Double constant at an index.
func (r *reader) double(i uint16) float64 {
	b := r.entry(i, tagDouble)
	if len(b) < 8 {
		return 0
	}
	return math.Float64frombits(binary.BigEndian.Uint64(b))
}

// str returns the UTF-16 code units of the String constant at an index.
func (r *reader) str(i uint16) []uint16 {
	b := r.entry(i, tagString)
	if len(b) < 2 {
		return nil
	}
	return r.units(binary.BigEndian.Uint16(b))
}
