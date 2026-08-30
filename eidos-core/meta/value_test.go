// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package meta_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/meta"
)

// The vocabulary's runtime behaviour: values copy at the boundary
// and compare per term.
func TestValue(t *testing.T) {
	t.Parallel()

	t.Run("answers absent through a foreign handle of another type", func(t *testing.T) {
		t.Parallel()

		// A handle belongs to the registry that answered it. One
		// from another composition can collide on the dense id with
		// a different value type; the read refuses rather than
		// answering the wrong type.
		_, f, role, _ := fixture(t)
		assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)),
			"the string fact stamps")

		foreignRegistry := claimed(t)
		foreign, err := meta.Register[int64](foreignRegistry, meta.KeySpec{
			Name: "shape.role", Doc: "the same id in another composition",
		})
		assert.NoError(t, err, "the foreign registry hands out the same dense id")
		assert.Equal(t, foreign.ID(), role.ID(), "which is the collision under test")

		_, held := meta.Get(f, subject, foreign)
		assert.False(t, held, "a type the key does not hold reads absent")
	})

	t.Run("an identical slice re-stamp changes nothing", func(t *testing.T) {
		t.Parallel()

		r := claimed(t)
		tags, err := meta.Register[[]string](r, meta.KeySpec{
			Name: "gen.tags", Doc: "the declared tags",
		})
		assert.NoError(t, err, "the slice key registers")
		f := meta.NewFacts(r)
		assert.NoError(t, meta.Stamp(f, tags, []string{"a", "b"}, by("gen", 1)),
			"the slice stamps")
		assert.NoError(t, meta.Stamp(f, tags, []string{"a", "b"}, by("gen", 1)),
			"and re-stamps")

		assert.Length(t, slices.Collect(f.Claims(subject, tags.ID())), 1,
			"element-wise equality dedupes the repeat")
	})

	t.Run("copies a slice value out", func(t *testing.T) {
		t.Parallel()

		r := claimed(t)
		tags, err := meta.Register[[]string](r, meta.KeySpec{
			Name: "gen.tags", Doc: "the declared tags",
		})
		assert.NoError(t, err, "the slice key registers")
		f := meta.NewFacts(r)
		assert.NoError(t, meta.Stamp(f, tags, []string{"a", "b"}, by("gen", 1)),
			"the slice stamps")

		got, _ := meta.Get(f, subject, tags)
		got[0] = "mutated"
		again, _ := meta.Get(f, subject, tags)
		assert.Equal(t, again, []string{"a", "b"},
			"a caller cannot reach into a bag")
	})
}
