// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// A plugin that journals no invocation is withdrawn by the facts it
// claimed, so what the store returns as one plugin's claims of the run
// is contract.
func TestClaimed(t *testing.T) {
	t.Parallel()

	t.Run("Compare", func(t *testing.T) {
		t.Parallel()

		t.Run("orders facts by subject then by key", func(t *testing.T) {
			t.Parallel()

			other := subject
			other.Name = "Cache"
			refs := []meta.FactRef{
				{Subject: subject, Key: "shape.role"},
				{Subject: other, Key: "shape.role"},
				{Subject: subject, Key: "shape.comparable"},
			}
			assert.True(t, refs[1].Compare(refs[0]) < 0, "the earlier subject sorts first")
			assert.True(t, refs[2].Compare(refs[0]) < 0, "on one subject, the earlier key sorts first")
			same := refs[0]
			assert.Equal(t, refs[0].Compare(same), 0, "a fact compares equal to its copy")
		})
	})

	t.Run("ClaimedBy", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each fact the plugin claimed in the run once in order", func(t *testing.T) {
			t.Parallel()

			_, f, role, flag := fixture(t)
			other := subject
			other.Name = "Cache"
			onOther := by("alpha", 1)
			onOther.Subject = other
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "alpha stamps the role")
			assert.NoError(t, meta.Stamp(f, flag, true, by("alpha", 2)), "alpha stamps the flag")
			assert.NoError(t, meta.Stamp(f, role, "reader", onOther), "alpha stamps another subject")
			assert.NoError(t, meta.Stamp(f, role, "writer", by("beta", 1)), "beta stamps the role")

			assert.Equal(t, f.ClaimedBy("alpha"), []meta.FactRef{
				{Subject: other, Key: "shape.role"},
				{Subject: subject, Key: "shape.comparable"},
				{Subject: subject, Key: "shape.role"},
			}, "alpha's facts, by subject then key")
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
			src := &recordedSource{claims: map[symbol.Identity][]meta.StoredClaim{
				subject: {{Key: "shape.role", Claim: by("alpha", 1), Value: "writer"}},
			}}
			f := meta.Restore(r, src)
			_, held := meta.Get(f, subject, role)
			assert.True(t, held, "the restored claim reads present")
			assert.Empty(t, f.ClaimedBy("alpha"), "and is not the run's claim")
		})
	})
}
