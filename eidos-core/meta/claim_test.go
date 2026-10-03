// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/meta"
)

func TestClaim(t *testing.T) {
	t.Parallel()

	t.Run("Authority", func(t *testing.T) {
		t.Parallel()

		t.Run("orders plugin below directive below manual", func(t *testing.T) {
			t.Parallel()

			assert.True(t, meta.AuthorityPlugin < meta.AuthorityDirective,
				"a human override at the source beats any inference")
			assert.True(t, meta.AuthorityDirective < meta.AuthorityManual,
				"and consumer tooling outranks even the directives it rewrites")
		})

		t.Run("the zero value is the plugin authority", func(t *testing.T) {
			t.Parallel()

			var got meta.Authority
			assert.Equal(t, got, meta.AuthorityPlugin,
				"a claim that returned no authority claims the least")
		})
	})

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

		other := subject
		other.Name = "Cache"
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
				give: meta.Order{Rule: 1, Subject: other, Instance: 9},
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
