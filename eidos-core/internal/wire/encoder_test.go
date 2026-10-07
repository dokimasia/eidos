// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/wire"
)

// The bytes each helper appends are the record format every generation
// and memo entry stores, so each is pinned.
func TestEncoder(t *testing.T) {
	t.Parallel()

	t.Run("AppendBool", func(t *testing.T) {
		t.Parallel()

		t.Run("appends one for true", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, wire.AppendBool([]byte{9}, true), []byte{9, 1}, "true is one byte of one")
		})

		t.Run("appends zero for false", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, wire.AppendBool(nil, false), []byte{0}, "false is one byte of zero")
		})
	})

	t.Run("AppendBytes", func(t *testing.T) {
		t.Parallel()

		t.Run("appends the length and then the bytes", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, wire.AppendBytes([]byte{9}, []byte("ab")), []byte{9, 2, 'a', 'b'},
				"the length is a uvarint in front of the bytes")
		})
	})

	t.Run("AppendText", func(t *testing.T) {
		t.Parallel()

		t.Run("appends the length and then the text", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, wire.AppendText(nil, "abc"), []byte{3, 'a', 'b', 'c'},
				"the length is a uvarint in front of the text")
		})
	})
}

// TestEncoderZeroAlloc checks that every helper appends into a buffer
// with room without allocating. The check runs alone, because the count
// includes every goroutine's allocations.
func TestEncoderZeroAlloc(t *testing.T) {
	dst := make([]byte, 0, 16)
	assert.MaxAllocs(t, func() { dst = wire.AppendBool(dst[:0], true) }, 0, "AppendBool allocates nothing")
	assert.Length(t, dst, 1, "AppendBool appends one byte")
	assert.MaxAllocs(t, func() { dst = wire.AppendBytes(dst[:0], []byte("ab")) }, 0,
		"AppendBytes allocates nothing")
	assert.Length(t, dst, 3, "AppendBytes appends the length and the bytes")
	assert.MaxAllocs(t, func() { dst = wire.AppendText(dst[:0], "ab") }, 0, "AppendText allocates nothing")
	assert.Length(t, dst, 3, "AppendText appends the length and the text")
}

// BenchmarkEncoder measures each helper into a buffer with room, under
// a ceiling of no allocation.
func BenchmarkEncoder(b *testing.B) {
	dst := make([]byte, 0, 16)

	b.Run("AppendBool", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		for c.Loop() {
			dst = wire.AppendBool(dst[:0], true)
		}
		assert.Length(b, dst, 1, "AppendBool appends one byte")
	})

	b.Run("AppendBytes", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		for c.Loop() {
			dst = wire.AppendBytes(dst[:0], []byte("ab"))
		}
		assert.Length(b, dst, 3, "AppendBytes appends the length and the bytes")
	})

	b.Run("AppendText", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		for c.Loop() {
			dst = wire.AppendText(dst[:0], "ab")
		}
		assert.Length(b, dst, 3, "AppendText appends the length and the text")
	})
}
