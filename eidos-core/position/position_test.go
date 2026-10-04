// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package position_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/position"
)

// located is a position whose line and column spell three digits, so
// a spelling that formats each number into a string of its own would
// allocate for it.
var located = position.Pos{File: "svc/store.go", Line: 141, Col: 102}

// A position is the anchor every finding and every declaration is
// reported at, so its spelling, its absence and its order are pinned.
func TestPos(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("renders the position as file:line:col", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, located.String(), "svc/store.go:141:102", "the position's spelling")
		})

		t.Run("renders the zero position with zero numbers", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, position.Pos{}.String(), ":0:0", "the zero position's spelling")
		})

		t.Run("renders a negative number with its sign", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, position.Pos{File: "a.go", Line: -1, Col: -20}.String(), "a.go:-1:-20",
				"the spelling of an out-of-range position")
		})
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for the zero position", func(t *testing.T) {
			t.Parallel()

			assert.True(t, position.Pos{}.IsZero(), "the zero Pos carries no source position")
		})

		t.Run("reports false for a located position", func(t *testing.T) {
			t.Parallel()

			assert.False(t, located.IsZero(), "a located Pos is not the absence marker")
		})
	})

	t.Run("Compare", func(t *testing.T) {
		t.Parallel()

		at := position.Pos{File: "b.go", Line: 5, Col: 3}
		tests := []struct {
			name  string
			other position.Pos
			want  int
		}{
			{name: "returns 0 for an equal position", other: at, want: 0},
			{name: "orders by the file first", other: position.Pos{File: "a.go", Line: 9, Col: 9}, want: 1},
			{name: "orders by the line within a file", other: position.Pos{File: "b.go", Line: 6, Col: 1}, want: -1},
			{name: "orders by the column within a line", other: position.Pos{File: "b.go", Line: 5, Col: 2}, want: 1},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, at.Compare(tt.other), tt.want, tt.name)
				assert.Equal(t, tt.other.Compare(at), -tt.want, "and the order is antisymmetric")
			})
		}
	})
}

// String allocates its one string, and IsZero and Compare nothing, in
// the ordinary run, which runs no benchmark.
func TestPosAllocs(t *testing.T) {
	assert.MaxAllocs(t, func() {
		if located.String() == "" {
			t.Fatal("String returned nothing")
		}
	}, 1, "String allocates the returned string")
	assert.MaxAllocs(t, func() {
		if located.IsZero() {
			t.Fatal("IsZero reported a located position as zero")
		}
	}, 0, "IsZero allocates nothing")
	other := located
	other.Col++
	assert.MaxAllocs(t, func() {
		if located.Compare(other) >= 0 {
			t.Fatal("Compare put the earlier column second")
		}
	}, 0, "Compare allocates nothing")
}

// BenchmarkPos measures the spelling, the absence check and the order
// of a position whose numbers spell three digits.
func BenchmarkPos(b *testing.B) {
	b.Run("String", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got string
		for c.Loop() {
			got = located.String()
		}
		assert.Equal(b, got, "svc/store.go:141:102", "String renders file:line:col")
	})

	b.Run("IsZero", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = located.IsZero()
		}
		assert.False(b, got, "IsZero reports a located position as present")
	})

	b.Run("Compare", func(b *testing.B) {
		other := located
		other.Col++
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got = located.Compare(other)
		}
		assert.Equal(b, got, -1, "Compare puts the earlier column first")
	})
}
