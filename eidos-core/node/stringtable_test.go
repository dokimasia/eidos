// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package node_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/node"
)

// tableBytes is the encoding of a table that numbers "ab" and then
// "c": the count, then each string's length and bytes. Every region
// in a generation and in the memo starts with these bytes, so they are
// pinned.
var tableBytes = []byte{2, 2, 'a', 'b', 1, 'c'}

// A region's strings decode against the region's own table, so the
// table's numbering and its encoding are the contract every encoded
// declaration depends on.
func TestStringTable(t *testing.T) {
	t.Parallel()

	t.Run("Add", func(t *testing.T) {
		t.Parallel()

		t.Run("returns zero for the empty string without storing it", func(t *testing.T) {
			t.Parallel()

			var table node.StringTable
			assert.Equal(t, table.Add(""), uint64(0), "the empty string is number zero")
			assert.Equal(t, table.Len(), 0, "and takes no place in the table")
		})

		t.Run("numbers each new string from one in the order it arrives", func(t *testing.T) {
			t.Parallel()

			var table node.StringTable
			assert.Equal(t, table.Add("ab"), uint64(1), "the first string is number one")
			assert.Equal(t, table.Add("c"), uint64(2), "the second is number two")
		})

		t.Run("returns the number an earlier call returned for a repeat", func(t *testing.T) {
			t.Parallel()

			var table node.StringTable
			table.Add("ab")
			table.Add("c")
			assert.Equal(t, table.Add("ab"), uint64(1), "a repeat keeps its number")
			assert.Equal(t, table.Len(), 2, "and adds nothing")
		})
	})

	t.Run("At", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the string a number names", func(t *testing.T) {
			t.Parallel()

			var table node.StringTable
			table.Add("ab")
			got, held := table.At(1)
			assert.True(t, held, "number one is in the table")
			assert.Equal(t, got, "ab", "and names the first string")
		})

		t.Run("returns the empty string for zero", func(t *testing.T) {
			t.Parallel()

			var table node.StringTable
			got, held := table.At(0)
			assert.True(t, held, "number zero is in every table")
			assert.Equal(t, got, "", "and names the empty string")
		})

		t.Run("reports false for a number past the table", func(t *testing.T) {
			t.Parallel()

			var table node.StringTable
			table.Add("ab")
			_, held := table.At(2)
			assert.False(t, held, "a table of one string has no number two")
		})
	})

	t.Run("Reset", func(t *testing.T) {
		t.Parallel()

		t.Run("numbers strings from one again", func(t *testing.T) {
			t.Parallel()

			var table node.StringTable
			table.Add("ab")
			table.Reset()
			assert.Equal(t, table.Len(), 0, "a reset table is empty")
			assert.Equal(t, table.Add("c"), uint64(1), "and numbers its next string one")
		})
	})

	t.Run("AppendBinary", func(t *testing.T) {
		t.Parallel()

		t.Run("appends the count before each string's length-prefixed bytes", func(t *testing.T) {
			t.Parallel()

			var table node.StringTable
			table.Add("ab")
			table.Add("c")
			got, err := table.AppendBinary([]byte{9})
			assert.NoError(t, err, "a table always encodes")
			assert.Equal(t, got, append([]byte{9}, tableBytes...), "behind what dst already contains")
		})
	})

	t.Run("DecodeStringTable", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the table AppendBinary wrote", func(t *testing.T) {
			t.Parallel()

			table, read, err := node.DecodeStringTable(tableBytes)
			assert.NoError(t, err, "the pinned table decodes")
			assert.Equal(t, read, len(tableBytes), "whole")
			first, _ := table.At(1)
			second, _ := table.At(2)
			assert.Equal(t, []string{first, second}, []string{"ab", "c"}, "with its strings in number order")
		})

		t.Run("returns the count of the bytes it read", func(t *testing.T) {
			t.Parallel()

			_, read, err := node.DecodeStringTable(append(tableBytes, 7, 7))
			assert.NoError(t, err, "a table followed by other bytes decodes")
			assert.Equal(t, read, len(tableBytes), "and reads the table alone")
		})

		tests := []struct {
			name string
			give []byte
		}{
			{name: "returns ErrMalformed for no bytes at all", give: nil},
			{name: "returns ErrMalformed for a count larger than the bytes left", give: []byte{3, 0}},
			{name: "returns ErrMalformed for a string longer than the bytes left", give: []byte{1, 5, 'a'}},
			{name: "returns ErrMalformed for a table that ends before its last length", give: []byte{2, 1, 'a'}},
			{name: "returns ErrMalformed for a count that overflows", give: []byte{
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01,
			}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, _, err := node.DecodeStringTable(tt.give)
				assert.ErrorIs(t, err, node.ErrMalformed, "a malformed table does not decode")
			})
		}
	})
}

// TestStringTableAllocs checks the allocation contract: a read and a
// repeat allocate nothing, a reset table refills without allocating,
// an encoding into a buffer with room allocates nothing, and a decode
// allocates the table, its list and one string. Each count of the codec
// keeps the first error of its calls, which cmp.Or returns without
// allocating. The check runs alone, because the count includes every
// goroutine's allocations.
func TestStringTableAllocs(t *testing.T) {
	var table node.StringTable
	table.Add("ab")
	table.Add("c")
	dst := make([]byte, 0, len(tableBytes))

	var number uint64
	assert.MaxAllocs(t, func() { number = table.Add("ab") }, 0, "Add of a repeat allocates nothing")
	assert.Equal(t, number, uint64(1), "Add returns a repeat's own number")
	var s string
	assert.MaxAllocs(t, func() { s, _ = table.At(2) }, 0, "At allocates nothing")
	assert.Equal(t, s, "c", "At returns the string of the number")
	var n int
	assert.MaxAllocs(t, func() { n = table.Len() }, 0, "Len allocates nothing")
	assert.Equal(t, n, 2, "Len counts both strings")
	var reused node.StringTable
	assert.MaxAllocs(t, func() {
		reused.Reset()
		reused.Add("ab")
		reused.Add("c")
	}, 0, "a table refilled after Reset allocates nothing")
	assert.Equal(t, reused.Len(), 2, "the refilled table contains both strings")
	var (
		got []byte
		err error
	)
	assert.MaxAllocs(t, func() {
		var aerr error
		got, aerr = table.AppendBinary(dst[:0])
		err = cmp.Or(err, aerr)
	}, 0, "AppendBinary into a buffer with room allocates nothing")
	assert.NoError(t, err, "the table encodes")
	assert.Equal(t, got, tableBytes, "AppendBinary writes the table")
	var read int
	assert.MaxAllocs(t, func() {
		var derr error
		_, read, derr = node.DecodeStringTable(tableBytes)
		err = cmp.Or(err, derr)
	}, decodeTableAllocs, "DecodeStringTable allocates the table, its list and one string")
	assert.NoError(t, err, "the table decodes")
	assert.Equal(t, read, len(tableBytes), "DecodeStringTable reads the whole table")
}

// BenchmarkStringTable measures the table's operations under their
// allocation ceilings: a repeat, a read, a count, a reset and refill,
// an encoding into a buffer with room, and a decode.
func BenchmarkStringTable(b *testing.B) {
	var table node.StringTable
	table.Add("ab")
	table.Add("c")

	b.Run("Add", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got uint64
		for c.Loop() {
			got = table.Add("ab")
		}
		assert.Equal(b, got, uint64(1), "Add returns a repeat's own number")
	})

	b.Run("At", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got, _ = table.At(2)
		}
		assert.Equal(b, got, "c", "At returns the string of the number")
	})

	b.Run("Len", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got = table.Len()
		}
		assert.Equal(b, got, 2, "Len counts both strings")
	})

	b.Run("Reset", func(b *testing.B) {
		b.Run("a table of two strings refilled", func(b *testing.B) {
			var reused node.StringTable
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			for c.Loop() {
				reused.Reset()
				reused.Add("ab")
				reused.Add("c")
			}
			assert.Equal(b, reused.Len(), 2, "the refilled table contains both strings")
		})
	})

	b.Run("AppendBinary", func(b *testing.B) {
		dst := make([]byte, 0, len(tableBytes))
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got []byte
		for c.Loop() {
			got, _ = table.AppendBinary(dst[:0])
		}
		assert.Equal(b, got, tableBytes, "AppendBinary writes the table")
	})

	b.Run("DecodeStringTable", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(decodeTableAllocs)
		defer c.End()
		var read int
		for c.Loop() {
			_, read, _ = node.DecodeStringTable(tableBytes)
		}
		assert.Equal(b, read, len(tableBytes), "DecodeStringTable reads the whole table")
	})
}

// decodeTableAllocs is the ceiling of one table decode: the table, its
// list and the one string every decoded string is a substring of.
const decodeTableAllocs = 3
