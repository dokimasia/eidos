// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package store_test

import (
	"slices"
	"testing"

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

			if got := slices.Collect(store.NewReadSet().Identities()); len(got) != 0 {
				t.Fatalf("Identities() = %v, want none", got)
			}
		})

		t.Run("records one edge for a declaration read twice", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))
			for range 3 {
				r.Lookup(decl.ID)
			}

			if got := slices.Collect(reads.Identities()); len(got) != 1 {
				t.Fatalf("Identities() = %v, want one edge: edges deduplicate", got)
			}
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

			one := slices.Collect(forwardReads.Identities())
			other := slices.Collect(backwardReads.Identities())
			if !slices.Equal(one, other) {
				t.Fatalf("Identities() answered %v and %v: the order follows the read order",
					one, other)
			}
		})
	})

	t.Run("Kinds", func(t *testing.T) {
		t.Parallel()

		t.Run("records one edge for a kind enumerated twice", func(t *testing.T) {
			t.Parallel()

			r, reads := coretest.Reading(t, nil,
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")))
			enumerate(r, symbol.KindStruct, symbol.KindStruct)

			if got := slices.Collect(reads.Kinds()); len(got) != 1 {
				t.Fatalf("Kinds() = %v, want one edge: edges deduplicate", got)
			}
		})

		t.Run("answers one order however the enumerations arrived", func(t *testing.T) {
			t.Parallel()

			forward, forwardReads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			enumerate(forward, symbol.KindStruct, symbol.KindFile, symbol.KindPackage)

			backward, backwardReads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			enumerate(backward, symbol.KindPackage, symbol.KindFile, symbol.KindStruct)

			one := slices.Collect(forwardReads.Kinds())
			other := slices.Collect(backwardReads.Kinds())
			if !slices.Equal(one, other) {
				t.Fatalf("Kinds() answered %v and %v: the order follows the read order",
					one, other)
			}
		})
	})

	t.Run("Len", func(t *testing.T) {
		t.Parallel()

		t.Run("answers zero for a set that read nothing", func(t *testing.T) {
			t.Parallel()

			if got := store.NewReadSet().Len(); got != 0 {
				t.Fatalf("Len() = %d, want 0", got)
			}
		})

		t.Run("counts both grains", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))
			r.Lookup(decl.ID)
			enumerate(r, symbol.KindFile)

			// One identity from the lookup, one from the file the
			// enumeration reached, and one kind edge.
			if got := reads.Len(); got != 3 {
				t.Fatalf("Len() = %d, want 3: %v and %v",
					got, slices.Collect(reads.Identities()), slices.Collect(reads.Kinds()))
			}
		})
	})
}
