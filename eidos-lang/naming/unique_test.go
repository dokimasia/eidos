// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/naming"
)

// The candidates of the cases, and the names that a number follows.
const (
	firstCandidate  = "r"
	secondCandidate = "re"
	lastCandidate   = "recv"
	numberedZero    = "recv0"
	numberedOne     = "recv1"
	numberedTwo     = "recv2"
)

// Unique returns the first free candidate, and numbers the last candidate
// where every candidate is taken.
func TestUnique(t *testing.T) {
	t.Parallel()

	t.Run("Unique", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			giveTaken map[string]bool
			want      string
		}{
			{
				name:      "returns the first candidate that taken does not contain",
				giveTaken: map[string]bool{secondCandidate: true},
				want:      firstCandidate,
			},
			{
				name:      "returns the first candidate for a nil set",
				giveTaken: nil,
				want:      firstCandidate,
			},
			{
				name:      "returns a later candidate where taken contains the candidates before it",
				giveTaken: map[string]bool{firstCandidate: true},
				want:      secondCandidate,
			},
			{
				name:      "returns the last candidate followed by 0 where taken contains every candidate",
				giveTaken: map[string]bool{firstCandidate: true, secondCandidate: true, lastCandidate: true},
				want:      numberedZero,
			},
			{
				name: "returns the last candidate followed by the smallest number that gives a free name",
				giveTaken: map[string]bool{
					firstCandidate: true, secondCandidate: true, lastCandidate: true,
					numberedZero: true, numberedOne: true,
				},
				want: numberedTwo,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := naming.Unique(tt.giveTaken, firstCandidate, secondCandidate, lastCandidate)
				assert.Equal(t, got, tt.want, "the unique name")
			})
		}

		t.Run("returns a numbered name longer than the scratch of the probe", func(t *testing.T) {
			t.Parallel()

			long := string(make([]byte, 100))
			got := naming.Unique(map[string]bool{long: true}, long)
			assert.Equal(t, got, long+"0", "the long candidate followed by 0")
		})

		t.Run("panics without a candidate", func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() { naming.Unique(map[string]bool{}) }, "Unique without a candidate")
		})
	})
}

// A free candidate costs no allocation, and a numbered name costs the one
// allocation of its text. The ordinary run does not run a benchmark, so
// this test checks both ceilings.
func TestUniqueAllocs(t *testing.T) {
	free := map[string]bool{secondCandidate: true}
	full := map[string]bool{firstCandidate: true, secondCandidate: true, lastCandidate: true, numberedZero: true}
	candidates := []string{firstCandidate, secondCandidate, lastCandidate}
	var got string
	assert.MaxAllocs(t, func() { got = naming.Unique(free, candidates...) }, 0,
		"Unique allocates nothing for a free candidate")
	assert.Equal(t, got, firstCandidate, "Unique returns the free candidate")
	assert.MaxAllocs(t, func() { got = naming.Unique(full, candidates...) }, 1,
		"Unique allocates the text of a numbered name alone")
	assert.Equal(t, got, numberedOne, "Unique returns the numbered name")
}

// BenchmarkUnique measures the choice of a name that a backend makes for
// each receiver and each unnamed parameter.
func BenchmarkUnique(b *testing.B) {
	candidates := []string{firstCandidate, secondCandidate, lastCandidate}

	b.Run("Unique", func(b *testing.B) {
		b.Run("free", func(b *testing.B) {
			taken := map[string]bool{secondCandidate: true}
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got string
			for c.Loop() {
				got = naming.Unique(taken, candidates...)
			}
			assert.Equal(b, got, firstCandidate, "Unique returns the free candidate")
		})

		b.Run("numbered", func(b *testing.B) {
			taken := map[string]bool{
				firstCandidate:  true,
				secondCandidate: true,
				lastCandidate:   true,
				numberedZero:    true,
			}
			c := bench.Start(b).MaxAllocs(1)
			defer c.End()
			var got string
			for c.Loop() {
				got = naming.Unique(taken, candidates...)
			}
			assert.Equal(b, got, numberedOne, "Unique returns the numbered name")
		})
	})
}
