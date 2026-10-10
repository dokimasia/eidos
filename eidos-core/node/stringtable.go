// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"encoding/binary"
	"fmt"
)

// StringTable numbers the strings of one encoded region, so the
// encoding writes each distinct string once and refers to it by its
// number. A region decodes against its own table and no other, so a
// region read from a generation and one read from the memo decode the
// same way.
//
// The empty string is number zero and is never stored. Every other
// string is numbered from one, in the order [StringTable.Add] first
// met it.
//
// The zero StringTable is empty and ready to use.
//
// # Concurrency
//
// A StringTable is not safe for concurrent use. A decoded table is not
// mutated by a read, so any number of goroutines decode against it at
// once.
//
// # Allocation contract
//
// [StringTable.Add] allocates only when the table grows its list or its
// index. [StringTable.Reset] keeps both, so a table reused across
// regions allocates only to grow. [DecodeStringTable] allocates three
// times: the table, its list, and one string that every decoded string
// is a substring of.
type StringTable struct {
	strings []string
	index   map[string]uint64
}

// DecodeStringTable decodes a table [StringTable.AppendBinary] wrote at
// the start of b, and returns it with the number of bytes it read. It
// reads the table twice: once to check every length against the bytes
// left, and once to slice each string out of one copy of the table's
// bytes.
//
// Error modes: an error wrapping [ErrMalformed] where b ends inside the
// table, or where the count or a string's length exceeds the bytes
// left.
func DecodeStringTable(b []byte) (*StringTable, int, error) {
	count, read := binary.Uvarint(b)
	if read <= 0 || count > uint64(len(b)-read) {
		return nil, 0, fmt.Errorf("node: %w: the string table's count does not fit its %d bytes",
			ErrMalformed, len(b))
	}
	end := read
	for range count {
		length, n := binary.Uvarint(b[end:])
		if n <= 0 || length > uint64(len(b)-end-n) {
			return nil, 0, fmt.Errorf("node: %w: a string of the table does not fit the %d bytes left",
				ErrMalformed, len(b)-end)
		}
		end += n + int(length)
	}

	whole := string(b[read:end])
	t := &StringTable{strings: make([]string, 0, count)}
	at := 0
	for range count {
		length, n := binary.Uvarint(b[read+at:])
		at += n
		t.strings = append(t.strings, whole[at:at+int(length)])
		at += int(length)
	}
	return t, end, nil
}

// Add returns the number of s, adding s where the table does not yet
// contain it. The empty string is number zero.
func (t *StringTable) Add(s string) uint64 {
	if s == "" {
		return 0
	}
	if i, held := t.index[s]; held {
		return i
	}
	if t.index == nil {
		t.index = make(map[string]uint64)
	}
	t.strings = append(t.strings, s)
	i := uint64(len(t.strings))
	t.index[s] = i
	return i
}

// At returns the string numbered i, and false for a number the table
// does not contain.
func (t *StringTable) At(i uint64) (string, bool) {
	switch {
	case i == 0:
		return "", true
	case i > uint64(len(t.strings)):
		return "", false
	default:
		return t.strings[i-1], true
	}
}

// Len returns how many strings the table contains, the empty string not
// counted.
func (t *StringTable) Len() int { return len(t.strings) }

// Reset empties the table and keeps its storage, so the next region's
// strings number from one again.
func (t *StringTable) Reset() {
	clear(t.strings)
	t.strings = t.strings[:0]
	clear(t.index)
}

// AppendBinary appends the table's encoding to dst: the count, then
// each string's length and bytes, in number order. It implements
// [encoding.BinaryAppender] and returns no error.
func (t *StringTable) AppendBinary(dst []byte) ([]byte, error) {
	dst = binary.AppendUvarint(dst, uint64(len(t.strings)))
	for _, s := range t.strings {
		dst = binary.AppendUvarint(dst, uint64(len(s)))
		dst = append(dst, s...)
	}
	return dst, nil
}
