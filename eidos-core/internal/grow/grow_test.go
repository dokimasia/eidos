// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package grow_test

import (
	"math"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/grow"
)

// least is the capacity the cases ask an empty buffer to take.
const least = 64

// The ceilings of Room: none where the slice has the room, and the copy
// where it does not.
const (
	roomFitsAllocs  = 0
	roomGrowsAllocs = 1
)

// A buffer that Room grows doubles, so its total allocation is less than
// twice its final size.
func TestGrow(t *testing.T) {
	t.Parallel()

	t.Run("Room", func(t *testing.T) {
		t.Parallel()

		t.Run("returns s itself where its capacity has the room", func(t *testing.T) {
			t.Parallel()

			s := make([]int, 2, 8)
			got := grow.Room(s, 6, least)
			assert.Equal(t, cap(got), 8, "the capacity is unchanged")
			assert.Equal(t, got, s, "and the storage is the same", assert.ByIdentity())
		})

		t.Run("returns a copy of at least the least capacity for an empty slice", func(t *testing.T) {
			t.Parallel()

			got := grow.Room([]int(nil), 1, least)
			assert.InRange(t, cap(got), least, math.Inf(1), "the first growth takes the least capacity")
			assert.Empty(t, got, "and the length is zero")
		})

		t.Run("returns a copy of at least twice the length of a full slice", func(t *testing.T) {
			t.Parallel()

			s := make([]int, 100)
			got := grow.Room(s, 1, least)
			assert.InRange(t, cap(got), 200, math.Inf(1), "the capacity at least doubles")
		})

		t.Run("returns a copy with room for n where n exceeds the length", func(t *testing.T) {
			t.Parallel()

			s := make([]int, 100)
			got := grow.Room(s, 500, least)
			assert.InRange(t, cap(got), 600, math.Inf(1), "the capacity covers the length and n")
		})

		t.Run("returns a copy that keeps the elements and the length", func(t *testing.T) {
			t.Parallel()

			s := []int{1, 2, 3}
			got := grow.Room(s, 1, least)
			assert.Equal(t, got, []int{1, 2, 3}, "the elements and the length remain")
		})

		t.Run("returns s for a negative n", func(t *testing.T) {
			t.Parallel()

			s := make([]int, 4)
			got := grow.Room(s, -1, least)
			assert.Equal(t, cap(got), 4, "a negative n asks for no room")
		})
	})
}

// Room allocates nothing where the slice has the room and one copy where
// it does not, in the ordinary run, which runs no benchmark. The check
// runs alone, because the count includes every goroutine's allocations.
func TestGrowAllocs(t *testing.T) {
	roomy := make([]int, 0, least)
	var got []int
	assert.MaxAllocs(t, func() { got = grow.Room(roomy, 1, least) }, roomFitsAllocs,
		"Room allocates nothing for a slice with room")
	assert.Equal(t, got, roomy, "the slice keeps its storage", assert.ByIdentity())
	full := make([]int, least)
	assert.MaxAllocs(t, func() { got = grow.Room(full, 1, least) }, roomGrowsAllocs,
		"Room allocates the copy of a full slice")
	assert.InRange(t, cap(got), 2*least, math.Inf(1), "the copy at least doubles")
}

// BenchmarkGrow measures Room over a slice with room and over a full
// one.
func BenchmarkGrow(b *testing.B) {
	b.Run("Room", func(b *testing.B) {
		b.Run("a slice with room", func(b *testing.B) {
			s := make([]int, 0, least)
			c := bench.Start(b).MaxAllocs(roomFitsAllocs)
			defer c.End()
			var got []int
			for c.Loop() {
				got = grow.Room(s, 1, least)
			}
			assert.Equal(b, cap(got), least, "the slice keeps its storage")
		})

		b.Run("a full slice", func(b *testing.B) {
			s := make([]int, least)
			c := bench.Start(b).MaxAllocs(roomGrowsAllocs)
			defer c.End()
			var got []int
			for c.Loop() {
				got = grow.Room(s, 1, least)
			}
			assert.InRange(b, cap(got), 2*least, math.Inf(1), "the copy at least doubles")
		})
	})
}
