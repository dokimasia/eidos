// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package store_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

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

		t.Run("answers nothing for a set that read nothing", func(t *testing.T) {
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

		t.Run("answers one order however the reads arrived", func(t *testing.T) {
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

		t.Run("answers one order however the enumerations arrived", func(t *testing.T) {
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

	t.Run("Len", func(t *testing.T) {
		t.Parallel()

		t.Run("answers zero for a set that read nothing", func(t *testing.T) {
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

			// One identity from the lookup, one from the file the
			// enumeration reached, and one kind edge.
			assert.Equal(t, reads.Len(), 3, "Len counts both grains")
		})
	})
}
