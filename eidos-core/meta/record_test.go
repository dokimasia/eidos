// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
)

// claimsAllocs is a range over the claims on one fact whose values are
// scalars and whose claims record no provenance: the list of views.
const claimsAllocs = 1

// The record is what attribution walks: every claim, winner marked,
// drops included.
func TestRecord(t *testing.T) {
	t.Parallel()

	t.Run("Claims", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every claim in rank order", func(t *testing.T) {
			t.Parallel()

			f, role := contested(t)
			values := make([]any, 0, 2)
			for view := range f.Claims(subject, role.ID()) {
				values = append(values, view.Value)
			}
			assert.Equal(t, values, []any{"declared", "inferred"},
				"the directive's claim ranks before the plugin's")
		})

		t.Run("marks the first claim as the winner", func(t *testing.T) {
			t.Parallel()

			f, role := contested(t)
			got := slices.Collect(f.Claims(subject, role.ID()))
			assert.Length(t, got, 2, "both claims remain visible")
			assert.True(t, got[0].Won, "the first claim won")
			assert.False(t, got[1].Won, "the losing claim is marked as lost")
		})

		t.Run("returns a drop as a claim with a nil value", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the stamp arrives")
			assert.NoError(t, f.DropKey(role.ID(), dropBy("defaults", 1)), "the drop arrives")

			got := slices.Collect(f.Claims(subject, role.ID()))
			assert.Length(t, got, 2, "the drop is a claim like any other")
			assert.True(t, got[0].Won, "the drop won")
			assert.Nil(t, got[0].Value, "a deletion somebody authored has no value")
		})

		t.Run("returns a group drop that covers the key", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the stamp arrives")
			assert.NoError(t, f.DropGroup("shape.writer", dropBy("defaults", 1)), "the group drop arrives")

			got := slices.Collect(f.Claims(subject, role.ID()))
			assert.Length(t, got, 2, "the group drop is listed beside the stamp")
			assert.Equal(t, got[0].Claim.Plugin, diag.Origin("defaults"), "and outranks it")
		})

		t.Run("returns the provenance of a claim", func(t *testing.T) {
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

		t.Run("returns a copy of a list value", func(t *testing.T) {
			t.Parallel()

			f, tags := listed(t)
			for view := range f.Claims(subject, tags.ID()) {
				values, _ := view.Value.([]string)
				values[0] = "edited"
			}
			got, _ := meta.Get(f, subject, tags)
			assert.Equal(t, got, []string{"a", "b"}, "the value keeps what was stamped")
		})

		t.Run("returns a copy of the provenance", func(t *testing.T) {
			t.Parallel()

			f, tags := listed(t)
			for view := range f.Claims(subject, tags.ID()) {
				view.Claim.Derived[0].Key = "edited"
			}
			again := slices.Collect(f.Claims(subject, tags.ID()))
			assert.Equal(t, again[0].Claim.Derived[0].Key, meta.KeyName("shape.comparable"),
				"the provenance keeps what was stamped")
		})

		t.Run("returns nothing for a fact never claimed", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.Empty(t, slices.Collect(f.Claims(subject, role.ID())),
				"no claim, no record")
		})
	})
}

// A range over the record allocates its list of views alone in the
// ordinary run, which runs no benchmark. The check runs alone, because
// AllocsPerRun counts every goroutine's allocations and refuses to run
// beside parallel tests.
func TestRecordAllocs(t *testing.T) {
	f, role := contested(t)
	views := 0
	assert.MaxAllocs(t, func() {
		views = 0
		for range f.Claims(subject, role.ID()) {
			views++
		}
	}, claimsAllocs, "a range over Claims allocates the list of views")
	assert.Equal(t, views, 2, "Claims enumerates both claims")

	assert.MaxAllocs(t, func() {
		views = 0
		for range f.Claims(sibling, role.ID()) {
			views++
		}
	}, 0, "a range over Claims allocates nothing for a fact never claimed")
	assert.Equal(t, views, 0, "Claims enumerates nothing for a fact never claimed")
}

// BenchmarkRecord measures a range over the record of one fact, the walk
// attribution and a name override make.
func BenchmarkRecord(b *testing.B) {
	f, role := contested(b)

	b.Run("Claims/a fact with two claims", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(claimsAllocs)
		defer c.End()
		views := 0
		for c.Loop() {
			views = 0
			for range f.Claims(subject, role.ID()) {
				views++
			}
		}
		assert.Equal(b, views, 2, "Claims enumerates both claims")
	})

	b.Run("Claims/a fact never claimed", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		views := 0
		for c.Loop() {
			views = 0
			for range f.Claims(sibling, role.ID()) {
				views++
			}
		}
		assert.Equal(b, views, 0, "Claims enumerates nothing for a fact never claimed")
	})
}

// contested returns a store in which the shape plugin and the defaults
// directive both claim the role on the fixture subject, and the role's
// handle.
func contested(tb assert.TB) (*meta.Facts, meta.Key[string]) {
	tb.Helper()

	_, f, role, _ := fixture(tb)
	assert.NoError(tb, meta.Stamp(f, role, "inferred", by("shape", 1)), "the plugin claims")
	directive := by("defaults", 1)
	directive.Authority = meta.AuthorityDirective
	assert.NoError(tb, meta.Stamp(f, role, "declared", directive), "the directive claims over it")
	return f, role
}

// listed returns a store with a list key registered before the store was
// built, one claim of a list value on the fixture subject with its
// provenance, and the key's handle.
func listed(tb assert.TB) (*meta.Facts, meta.Key[[]string]) {
	tb.Helper()

	r := claimed(tb)
	tags, err := meta.Register[[]string](r, meta.KeySpec{Name: "shape.tags", Doc: "the classified tags"})
	assert.NoError(tb, err, "the list key registers")
	f := meta.NewFacts(r)
	claim := by("shape", 1)
	claim.Derived = []meta.Read{{Subject: subject, Key: "shape.comparable"}}
	assert.NoError(tb, meta.Stamp(f, tags, []string{"a", "b"}, claim), "the claim stamps")
	return f, tags
}
