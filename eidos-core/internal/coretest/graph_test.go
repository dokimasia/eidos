// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// fixtureLoads is the contract Frozen states for the packages it
// loads, as its record states it.
const fixtureLoads = "the fixture's packages load: a case whose fixture would not load is not testing what it says it is"

// The graph fixtures decide what a case starts from, so what they
// hand back has to be the state the case claims to be testing.
func TestGraph(t *testing.T) {
	t.Parallel()

	t.Run("Frozen", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a sealed graph", func(t *testing.T) {
			t.Parallel()

			assert.True(t, coretest.Frozen(t).Frozen(),
				"the fixture graph arrives sealed, or every read would return nothing")
		})

		t.Run("contains the packages it was given", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, decl))

			_, held := g.Lookup(decl.Identity())
			assert.True(t, held, "the fixture graph contains what it was given")
		})

		t.Run("fails a fixture that would not load", func(t *testing.T) {
			t.Parallel()

			pkg := coretest.Package(coretest.StorePath)
			got := assert.Rejects(t, "a duplicate package fails the fixture",
				func(tb assert.TB) { coretest.Frozen(tb, pkg, pkg) })
			assert.Equal(t, coretest.Contracts(got), []string{fixtureLoads},
				"and fails for the reason the helper is about")
		})
	})

	t.Run("Dependent", func(t *testing.T) {
		t.Parallel()

		dep := coretest.Struct(coretest.CachePath, "Cache")
		own := coretest.Struct(coretest.StorePath, "Store")
		g := coretest.Dependent(
			[]*node.Package{coretest.Package(coretest.CachePath, dep)},
			coretest.Package(coretest.StorePath, own),
		)

		t.Run("returns a sealed graph", func(t *testing.T) {
			t.Parallel()

			assert.True(t, g.Frozen(), "the fixture graph arrives sealed")
		})

		t.Run("contains the packages it was given", func(t *testing.T) {
			t.Parallel()

			_, depHeld := g.Lookup(dep.Identity())
			_, ownHeld := g.Lookup(own.Identity())
			expect.True(t, depHeld, "the graph contains the package of the dependency round")
			expect.True(t, ownHeld, "the graph contains the package of the partition")
		})

		t.Run("marks the packages of the dependency round", func(t *testing.T) {
			t.Parallel()

			assert.True(t, g.Dependency(dep.Identity()), "a dependency round loaded the cache package")
		})

		t.Run("marks no package of the partition", func(t *testing.T) {
			t.Parallel()

			assert.False(t, g.Dependency(own.Identity()), "the partition loaded the store package")
		})
	})

	t.Run("Reading", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a reader over the packages it was given", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))

			_, held := r.Lookup(decl.Identity())
			assert.True(t, held, "the fixture reader returns what it was given")
		})

		t.Run("returns a read set holding nothing yet", func(t *testing.T) {
			t.Parallel()

			_, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			assert.Equal(t, reads.Len(), 0,
				"the fixture records no reads of its own")
		})

		t.Run("reads within the scope it was given", func(t *testing.T) {
			t.Parallel()

			hidden := coretest.Struct(coretest.CachePath, "Cache")
			r, _ := coretest.Reading(t,
				func(pkg symbol.Identity) bool { return pkg.Package == coretest.StorePath },
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
				coretest.Package(coretest.CachePath, hidden))

			assert.Length(t, slices.Collect(r.ByKind(symbol.KindStruct)), 1,
				"the fixture carries the scope it was given")
		})

		t.Run("fails a fixture that Frozen fails", func(t *testing.T) {
			t.Parallel()

			pkg := coretest.Package(coretest.StorePath)
			got := assert.Rejects(t, "the reader fixture carries the load check",
				func(tb assert.TB) { coretest.Reading(tb, nil, pkg, pkg) })
			assert.Equal(t, coretest.Contracts(got), []string{fixtureLoads},
				"and fails for the reason Frozen is about")
		})
	})
}
