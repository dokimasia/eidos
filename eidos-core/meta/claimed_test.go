// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// claimedFacts is the number of facts one plugin claims in the
// allocation check and the benchmark of ClaimedBy.
const claimedFacts = 1_000

// claimedByAllocs is one list of claimedFacts facts: append grows the
// list eleven times on its way to a thousand entries.
const claimedByAllocs = 11

// A plugin that journals no invocation is withdrawn by the facts it
// claimed, so what the store returns as one plugin's claims of the run
// is contract.
func TestClaimed(t *testing.T) {
	t.Parallel()

	t.Run("Compare", func(t *testing.T) {
		t.Parallel()

		ref := meta.FactRef{Subject: subject, Key: "shape.role"}
		tests := []struct {
			name string
			give meta.FactRef
			want int
		}{
			{name: "returns zero for one fact", give: ref, want: 0},
			{
				name: "returns a negative number for an earlier subject",
				give: meta.FactRef{Subject: sibling, Key: "shape.target"},
				want: -1,
			},
			{
				name: "returns a negative number for an earlier key on one subject",
				give: meta.FactRef{Subject: subject, Key: "shape.comparable"},
				want: -1,
			},
			{
				name: "returns a positive number for a later key on one subject",
				give: meta.FactRef{Subject: subject, Key: "shape.target"},
				want: 1,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := tt.give.Compare(ref)
				assert.Equal(t, sign(got), tt.want, "the facts compare by subject, then key")
			})
		}
	})

	t.Run("ClaimedBy", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the facts the plugin claimed in the run in order", func(t *testing.T) {
			t.Parallel()

			_, f, role, flag := fixture(t)
			onSibling := on(sibling, "alpha", 1)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "alpha stamps the role")
			assert.NoError(t, meta.Stamp(f, flag, true, by("alpha", 2)), "alpha stamps the flag")
			assert.NoError(t, meta.Stamp(f, role, "reader", onSibling), "alpha stamps another subject")
			assert.NoError(t, meta.Stamp(f, role, "writer", by("beta", 1)), "beta stamps the role")

			assert.Equal(t, f.ClaimedBy("alpha"), []meta.FactRef{
				{Subject: sibling, Key: "shape.role"},
				{Subject: subject, Key: "shape.comparable"},
				{Subject: subject, Key: "shape.role"},
			}, "alpha's facts, by subject then key")
		})

		t.Run("returns a fact that another plugin also claimed", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "alpha stamps the role")
			assert.NoError(t, meta.Stamp(f, role, "writer", by("beta", 1)), "beta stamps the role")
			assert.Equal(t, f.ClaimedBy("beta"), []meta.FactRef{{Subject: subject, Key: "shape.role"}},
				"beta's fact")
		})

		t.Run("returns nothing for a plugin that claimed nothing", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "alpha stamps")
			assert.Empty(t, f.ClaimedBy("gamma"), "gamma claimed nothing")
		})

		t.Run("leaves out a claim a recorded source restored", func(t *testing.T) {
			t.Parallel()

			r, _, role, _ := fixture(t)
			src := &recordedSource{
				claims: map[symbol.Identity][]meta.StoredClaim{
					subject: {{Key: "shape.role", Claim: by("alpha", 1), Value: "writer"}},
				},
				present: map[meta.KeyName][]symbol.Identity{"shape.role": {subject}},
			}
			f := meta.Restore(r, src)
			_, held := meta.Get(f, subject, role)
			assert.True(t, held, "the restored claim reads present")
			assert.Empty(t, f.ClaimedBy("alpha"), "and is not the run's claim")
		})
	})
}

// Comparing two facts allocates nothing, and listing one plugin's claims
// allocates the list alone, in the ordinary run, which runs no
// benchmark. The check runs alone, because the count includes every
// goroutine's allocations.
func TestClaimedAllocs(t *testing.T) {
	earlier, later := factRefs()
	var order int
	assert.MaxAllocs(t, func() { order = earlier.Compare(later) }, 0, "Compare allocates nothing")
	assert.Equal(t, sign(order), -1, "Compare orders the earlier subject first")

	f, _, _ := stamped(t, benchIdentities(claimedFacts))
	var got []meta.FactRef
	assert.MaxAllocs(t, func() { got = f.ClaimedBy("shape") }, claimedByAllocs,
		"ClaimedBy allocates the list as it grows")
	assert.Length(t, got, claimedFacts, "ClaimedBy lists every fact the plugin claimed")
	assert.MaxAllocs(t, func() { got = f.ClaimedBy("gamma") }, 0,
		"ClaimedBy allocates nothing for a plugin that claimed nothing")
	assert.Empty(t, got, "ClaimedBy lists nothing for a plugin that claimed nothing")
}

// BenchmarkClaimed measures the comparison of two facts and the list of
// one plugin's claims over a store of a thousand subjects: what a warm
// run asks once per phase call of a plugin that journals no invocation.
func BenchmarkClaimed(b *testing.B) {
	b.Run("Compare", func(b *testing.B) {
		earlier, later := factRefs()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got = earlier.Compare(later)
		}
		assert.Equal(b, sign(got), -1, "Compare orders the earlier subject first")
	})

	f, _, _ := stamped(b, benchIdentities(claimedFacts))

	b.Run("ClaimedBy", func(b *testing.B) {
		b.Run("a plugin that claimed a thousand facts", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(claimedByAllocs)
			defer c.End()
			var got []meta.FactRef
			for c.Loop() {
				got = f.ClaimedBy("shape")
			}
			assert.Length(b, got, claimedFacts, "ClaimedBy lists every fact the plugin claimed")
		})

		b.Run("a plugin that claimed nothing", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got []meta.FactRef
			for c.Loop() {
				got = f.ClaimedBy("gamma")
			}
			assert.Empty(b, got, "ClaimedBy lists nothing for a plugin that claimed nothing")
		})
	})
}

// factRefs returns two facts on one key: one on the sibling subject and
// one on the fixture subject, which sorts after it.
func factRefs() (earlier, later meta.FactRef) {
	return meta.FactRef{Subject: sibling, Key: "shape.role"},
		meta.FactRef{Subject: subject, Key: "shape.role"}
}
