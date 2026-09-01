// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// The index is dispatch's view: which subjects presently carry a
// key, cached between transitions.
func TestFactIndex(t *testing.T) {
	t.Parallel()

	t.Run("ByKey", func(t *testing.T) {
		t.Parallel()

		t.Run("enumerates present subjects in identity order", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			other := subject
			other.Name = "Cache"
			second := by("shape", 2)
			second.Subject = other
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the first stamps")
			assert.NoError(t, meta.Stamp(f, role, "reader", second), "and the second")

			got := slices.Collect(f.ByKey(role.ID()))
			assert.Equal(t, got, []symbol.Identity{second.Subject, subject},
				"the index returns every present subject, in identity order")
		})

		t.Run("a drop on an absent subject leaves the index alone", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)),
				"one subject stamps present")
			other := subject
			other.Name = "Cache"
			drop := by("defaults", 1)
			drop.Authority = meta.AuthorityDirective
			drop.Subject = other
			assert.NoError(t, f.DropKey(role.ID(), drop),
				"a drop arrives on a subject never stamped")

			assert.Equal(t, slices.Collect(f.ByKey(role.ID())), []symbol.Identity{subject},
				"the present subject stays, and the absent one stays absent")
		})

		t.Run("enumerates nothing for a key never stamped", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.Empty(t, slices.Collect(f.ByKey(role.ID())),
				"a key with no presence enumerates nothing")
		})

		t.Run("a dropped fact leaves the index", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the stamp arrives")
			assert.Length(t, slices.Collect(f.ByKey(role.ID())), 1, "and is indexed")

			drop := by("defaults", 1)
			drop.Authority = meta.AuthorityDirective
			assert.NoError(t, f.DropKey(role.ID(), drop), "the drop arrives")
			assert.Empty(t, slices.Collect(f.ByKey(role.ID())),
				"a subject whose winner is a drop is no match for a gated rule")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			other := subject
			other.Name = "Cache"
			second := by("shape", 2)
			second.Subject = other
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the first stamps")
			assert.NoError(t, meta.Stamp(f, role, "reader", second), "and the second")

			seen := 0
			for range f.ByKey(role.ID()) {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the enumeration stops when the range stops")
		})
	})
}
