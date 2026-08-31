// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package meta_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
)

// The record is what attribution walks: every claim, winner
// marked, drops included.
func TestRecord(t *testing.T) {
	t.Parallel()

	t.Run("Claims", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every claim in rank order, winner first", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "inferred", by("shape", 1)),
				"the plugin claims")
			directive := by("defaults", 1)
			directive.Authority = meta.AuthorityDirective
			assert.NoError(t, meta.Stamp(f, role, "declared", directive),
				"and the directive claims over it")

			got := slices.Collect(f.Claims(subject, role.ID()))
			assert.Length(t, got, 2, "both claims stay visible")
			assert.True(t, got[0].Won, "the first is the winner")
			assert.Equal(t, got[0].Value, any("declared"), "the directive's value")
			assert.False(t, got[1].Won, "the losing write stays visible instead of mysterious")
			assert.Equal(t, got[1].Value, any("inferred"), "with its value")
		})

		t.Run("shows a drop as a claim carrying no value", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the stamp arrives")
			drop := by("defaults", 1)
			drop.Authority = meta.AuthorityDirective
			assert.NoError(t, f.DropKey(role.ID(), drop), "and the drop over it")

			got := slices.Collect(f.Claims(subject, role.ID()))
			assert.Length(t, got, 2, "the drop is a claim like any other")
			assert.True(t, got[0].Won, "and it won")
			assert.Nil(t, got[0].Value, "a deletion somebody authored, not a fact nobody wrote")
		})

		t.Run("carries the provenance the claim arrived with", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			claim := by("shape", 1)
			claim.Derived = []meta.Read{{Subject: subject, Key: "shape.comparable"}}
			assert.NoError(t, meta.Stamp(f, role, "writer", claim), "the claim stamps")

			got := slices.Collect(f.Claims(subject, role.ID()))
			assert.Length(t, got, 1, "and is recorded")
			assert.Equal(t, got[0].Claim.Plugin, diag.Origin("shape"), "who wrote it")
			assert.Equal(t, got[0].Claim.Pos, carrier, "where it was authored")
			assert.Equal(t, got[0].Claim.Derived, claim.Derived, "and what produced it")
		})

		t.Run("returns nothing for a fact never claimed", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.Empty(t, slices.Collect(f.Claims(subject, role.ID())),
				"no claim, no record")
		})
	})
}
