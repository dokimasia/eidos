// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The rank of a claim follows from its envelope, so the zero value of a
// claim and the order of its places are contract.
func TestClaim(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		var c meta.Claim
		expect.Equal(t, c.Subject, symbol.Identity{}, "a zero claim is about nothing")
		expect.Equal(t, c.Authority, meta.AuthorityPlugin, "a zero claim has the lowest authority")
		expect.Equal(t, c.Pos, position.Pos{}, "a zero claim has no carrier position")
		expect.Nil(t, c.Derived, "a zero claim records no reads")
	})

	t.Run("Authority", func(t *testing.T) {
		t.Parallel()

		t.Run("String", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give meta.Authority
				want string
			}{
				{name: "returns plugin for AuthorityPlugin", give: meta.AuthorityPlugin, want: "plugin"},
				{name: "returns directive for AuthorityDirective", give: meta.AuthorityDirective, want: "directive"},
				{name: "returns manual for AuthorityManual", give: meta.AuthorityManual, want: "manual"},
				{
					name: "returns the number of an undeclared authority",
					give: meta.AuthorityManual + 1,
					want: "Authority(3)",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.String(), tt.want, "String returns the name of the authority")
				})
			}
		})
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

	t.Run("LoadClaim", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a plugin claim at the index of the stamp", func(t *testing.T) {
			t.Parallel()

			stamp := loadStamp()
			assert.Equal(t, meta.LoadClaim(subject, 2, stamp), meta.Claim{
				Subject:   subject,
				Authority: meta.AuthorityPlugin,
				Plugin:    stamp.Origin,
				Order:     meta.Order{Subject: subject, Instance: 2},
				Pos:       stamp.Pos,
			}, "the claim has plugin authority, the origin of the stamp, its index and its position")
		})
	})
}

// Compare and LoadClaim allocate nothing. The ordinary run, which runs no
// benchmark, checks both ceilings here.
func TestClaimAllocs(t *testing.T) {
	earlier, later := places()
	var got int
	assert.MaxAllocs(t, func() { got = earlier.Compare(later) }, 0, "Compare allocates nothing")
	assert.Equal(t, sign(got), -1, "Compare orders the earlier subject first")

	stamp := loadStamp()
	var claim meta.Claim
	assert.MaxAllocs(t, func() { claim = meta.LoadClaim(subject, 2, stamp) }, 0, "LoadClaim allocates nothing")
	assert.Equal(t, claim.Order.Instance, 2, "LoadClaim orders the claim at the index of the stamp")
}

// BenchmarkClaim measures the comparison of two places and the claim of a
// load stamp. The rank of two claims of one plugin compares their places,
// and a run builds a claim for every stamp of the load.
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

	b.Run("LoadClaim", func(b *testing.B) {
		stamp := loadStamp()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got meta.Claim
		for c.Loop() {
			got = meta.LoadClaim(subject, 2, stamp)
		}
		assert.Equal(b, got.Order.Instance, 2, "LoadClaim orders the claim at the index of the stamp")
	})
}

// loadStamp returns a classification stamp of the fixture subject, as a
// frontend records it in a load.
func loadStamp() meta.RawStamp {
	return meta.RawStamp{
		Key:    "fake.testFile",
		Value:  true,
		Pos:    position.Pos{File: "svc/a_test.zz", Line: 3, Col: 1},
		Origin: "fakefront",
	}
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
