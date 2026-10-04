// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/meta"
)

// A claim's envelope decides its rank, so its zero value and the order
// of its places are contract.
func TestClaim(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		var c meta.Claim
		assert.True(t, c.Subject.IsZero(), "a zero claim is about nothing")
		assert.Equal(t, c.Authority, meta.AuthorityPlugin,
			"and speaks with the least authority")
		assert.True(t, c.Pos.IsZero(), "from no carrier")
		assert.Nil(t, c.Derived, "derived from nothing")
	})

	t.Run("Compare", func(t *testing.T) {
		t.Parallel()

		place := meta.Order{Rule: 1, Subject: subject, Instance: 1}
		tests := []struct {
			name string
			give meta.Order
			want int
		}{
			{name: "returns zero for one place", give: place, want: 0},
			{
				name: "returns a negative number for an earlier rule",
				give: meta.Order{Rule: 0, Subject: subject, Instance: 9},
				want: -1,
			},
			{
				name: "returns a negative number for an earlier subject in one rule",
				give: meta.Order{Rule: 1, Subject: sibling, Instance: 9},
				want: -1,
			},
			{
				name: "returns a positive number for a later instance on one subject",
				give: meta.Order{Rule: 1, Subject: subject, Instance: 2},
				want: 1,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := tt.give.Compare(place)
				assert.Equal(t, sign(got), tt.want, "the places compare by rule, then subject, then instance")
			})
		}
	})
}

// Two places compare without allocating in the ordinary run, which runs
// no benchmark.
func TestClaimZeroAlloc(t *testing.T) {
	earlier, later := places()
	var got int
	assert.MaxAllocs(t, func() { got = earlier.Compare(later) }, 0, "Compare allocates nothing")
	assert.Equal(t, sign(got), -1, "Compare orders the earlier subject first")
}

// BenchmarkClaim measures the comparison of two places, which ranking
// two claims of one plugin makes.
func BenchmarkClaim(b *testing.B) {
	b.Run("Compare", func(b *testing.B) {
		earlier, later := places()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got = earlier.Compare(later)
		}
		assert.Equal(b, sign(got), -1, "Compare orders the earlier subject first")
	})
}

// places returns two places in one rule: one on the sibling subject and
// one on the fixture subject, which sorts after it.
func places() (earlier, later meta.Order) {
	return meta.Order{Rule: 1, Subject: sibling, Instance: 2},
		meta.Order{Rule: 1, Subject: subject, Instance: 1}
}

// sign returns -1, 0 or 1 as n is negative, zero or positive.
func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}
