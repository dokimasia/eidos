// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package store_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The fact grain's compile-time contract: one artifact's read set
// records declaration reads and fact reads into one place.
var _ meta.Recorder = (*store.ReadSet)(nil)

// enumerate ranges every kind through the reader, which is what
// records a set-membership edge for each.
func enumerate(r *store.Reader, kinds ...symbol.Kind) {
	for _, kind := range kinds {
		for range r.ByKind(kind) { // ranging is what records the edge
		}
	}
}

// A read set is what one derived artifact read. Its grain is the
// contract: edges deduplicate, and both grains count.
func TestReadSet(t *testing.T) {
	t.Parallel()

	t.Run("Identities", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a set that read nothing", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, slices.Collect(store.NewReadSet().Identities()),
				"a set that read nothing holds nothing")
		})

		t.Run("records one edge for a declaration read twice", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))
			for range 3 {
				r.Lookup(decl.ID)
			}

			assert.Length(t, slices.Collect(reads.Identities()), 1,
				"a declaration read three times records one edge")
		})

		t.Run("returns one order however the reads arrived", func(t *testing.T) {
			t.Parallel()

			alpha, omega := coretest.Struct(coretest.StorePath, "Alpha"), coretest.Struct(coretest.StorePath, "Omega")

			forward, forwardReads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, alpha, omega))
			forward.Lookup(alpha.ID)
			forward.Lookup(omega.ID)

			backward, backwardReads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, alpha, omega))
			backward.Lookup(omega.ID)
			backward.Lookup(alpha.ID)

			assert.Equal(t,
				slices.Collect(forwardReads.Identities()),
				slices.Collect(backwardReads.Identities()),
				"the order is the set's own, not the read order")
		})
	})

	t.Run("Kinds", func(t *testing.T) {
		t.Parallel()

		t.Run("records one edge for a kind enumerated twice", func(t *testing.T) {
			t.Parallel()

			r, reads := coretest.Reading(t, nil,
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")))
			enumerate(r, symbol.KindStruct, symbol.KindStruct)

			assert.Length(t, slices.Collect(reads.Kinds()), 1,
				"a kind enumerated twice records one edge")
		})

		t.Run("returns one order however the enumerations arrived", func(t *testing.T) {
			t.Parallel()

			forward, forwardReads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			enumerate(forward, symbol.KindStruct, symbol.KindFile, symbol.KindPackage)

			backward, backwardReads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			enumerate(backward, symbol.KindPackage, symbol.KindFile, symbol.KindStruct)

			assert.Equal(t,
				slices.Collect(forwardReads.Kinds()),
				slices.Collect(backwardReads.Kinds()),
				"the order is the set's own, not the enumeration order")
		})
	})

	t.Run("Reset", func(t *testing.T) {
		t.Parallel()

		t.Run("drops every grain and keeps recording", func(t *testing.T) {
			t.Parallel()

			s := store.NewReadSet()
			id := coretest.Struct(coretest.StorePath, "Store").ID
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath,
				coretest.Struct(coretest.StorePath, "Store")))
			r, err := g.Reader(s, nil)
			assert.NoError(t, err, "the tracked handle mints")
			r.Lookup(id)
			for range r.ByKind(symbol.KindStruct) {
				break
			}
			for range r.ByDirective("stub") {
				break
			}
			s.RecordFact(id, "shape.role")
			assert.Equal(t, s.Len(), 4, "all four grains recorded")

			s.Reset()
			assert.Equal(t, s.Len(), 0, "a reset set holds no edges")
			s.RecordFact(id, "shape.role")
			assert.Equal(t, s.Len(), 1, "and records again after the reset")
		})
	})

	t.Run("Facts", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing for a set that read none", func(t *testing.T) {
			t.Parallel()

			count := 0
			for range store.NewReadSet().Facts() {
				count++
			}
			assert.Equal(t, count, 0, "a set that read no facts holds none")
		})

		t.Run("records at (subject, key) and deduplicates", func(t *testing.T) {
			t.Parallel()

			s := store.NewReadSet()
			id := coretest.Struct(coretest.StorePath, "Store").ID
			for range 3 {
				s.RecordFact(id, "shape.role")
			}
			s.RecordFact(id, "shape.comparable")

			var got []meta.KeyName
			for subject, key := range s.Facts() {
				assert.Equal(t, subject, id, "every edge names its subject")
				got = append(got, key)
			}
			assert.Equal(t, got, []meta.KeyName{"shape.comparable", "shape.role"},
				"edges deduplicate and return in subject then key order")
		})

		t.Run("returns one order however the reads arrived", func(t *testing.T) {
			t.Parallel()

			one := coretest.Struct(coretest.StorePath, "Alpha").ID
			other := coretest.Struct(coretest.StorePath, "Omega").ID

			forward, backward := store.NewReadSet(), store.NewReadSet()
			forward.RecordFact(one, "shape.role")
			forward.RecordFact(other, "shape.role")
			backward.RecordFact(other, "shape.role")
			backward.RecordFact(one, "shape.role")

			collect := func(s *store.ReadSet) []symbol.Identity {
				var out []symbol.Identity
				for subject := range s.Facts() {
					out = append(out, subject)
				}
				return out
			}
			assert.Equal(t, collect(forward), collect(backward),
				"the order is the set's own, not the read order")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			s := store.NewReadSet()
			s.RecordFact(coretest.Struct(coretest.StorePath, "Store").ID, "shape.role")
			s.RecordFact(coretest.Struct(coretest.StorePath, "Cache").ID, "shape.role")

			seen := 0
			for range s.Facts() {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the iteration stops when the range stops")
		})
	})

	t.Run("Len", func(t *testing.T) {
		t.Parallel()

		t.Run("returns zero for a set that read nothing", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, store.NewReadSet().Len(), 0,
				"a set that read nothing counts nothing")
		})

		t.Run("counts both grains", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))
			r.Lookup(decl.ID)
			enumerate(r, symbol.KindFile)
			reads.RecordFact(decl.ID, "shape.role")

			// One identity from the lookup, one from the file the
			// enumeration reached, one kind edge, and one fact edge.
			assert.Equal(t, reads.Len(), 4, "Len counts all three grains")
		})
	})
}
