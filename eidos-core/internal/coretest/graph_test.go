// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package coretest_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/symbol"
)

// The graph fixtures decide what a case starts from, so what they
// hand back has to be the state the case claims to be testing.
func TestGraph(t *testing.T) {
	t.Parallel()

	t.Run("Frozen", func(t *testing.T) {
		t.Parallel()

		t.Run("answers a sealed graph", func(t *testing.T) {
			t.Parallel()

			if !coretest.Frozen(t).Frozen() {
				t.Fatal("Frozen() answered an unsealed graph, so every read would answer nothing")
			}
		})

		t.Run("holds the packages it was given", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, decl))

			if _, held := g.Lookup(decl.Identity()); !held {
				t.Fatalf("Lookup(%v) = false, want the declaration", decl.Identity())
			}
		})
	})

	t.Run("Reading", func(t *testing.T) {
		t.Parallel()

		t.Run("answers a reader over the packages it was given", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))

			if _, held := r.Lookup(decl.Identity()); !held {
				t.Fatalf("Lookup(%v) = false, want the declaration", decl.Identity())
			}
		})

		t.Run("answers a read set holding nothing yet", func(t *testing.T) {
			t.Parallel()

			_, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			if got := reads.Len(); got != 0 {
				t.Fatalf("Len() = %d, want 0: the fixture recorded a read of its own", got)
			}
		})

		t.Run("carries the scope it was given", func(t *testing.T) {
			t.Parallel()

			hidden := coretest.Struct(coretest.CachePath, "Cache")
			r, _ := coretest.Reading(t,
				func(pkg symbol.Identity) bool { return pkg.Package == coretest.StorePath },
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
				coretest.Package(coretest.CachePath, hidden))

			if got := len(slices.Collect(r.ByKind(symbol.KindStruct))); got != 1 {
				t.Fatalf("ByKind answered %d declarations, want 1: the scope was dropped", got)
			}
		})
	})
}
