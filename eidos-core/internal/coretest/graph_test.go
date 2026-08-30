// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package coretest_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

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

			assert.True(t, coretest.Frozen(t).Frozen(),
				"the fixture graph arrives sealed, or every read would answer nothing")
		})

		t.Run("holds the packages it was given", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, decl))

			_, held := g.Lookup(decl.Identity())
			assert.True(t, held, "the fixture graph holds what it was given")
		})
	})

	t.Run("Reading", func(t *testing.T) {
		t.Parallel()

		t.Run("answers a reader over the packages it was given", func(t *testing.T) {
			t.Parallel()

			decl := coretest.Struct(coretest.StorePath, "Store")
			r, _ := coretest.Reading(t, nil, coretest.Package(coretest.StorePath, decl))

			_, held := r.Lookup(decl.Identity())
			assert.True(t, held, "the fixture reader answers what it was given")
		})

		t.Run("answers a read set holding nothing yet", func(t *testing.T) {
			t.Parallel()

			_, reads := coretest.Reading(t, nil, coretest.Package(coretest.StorePath))
			assert.Equal(t, reads.Len(), 0,
				"the fixture records no reads of its own")
		})

		t.Run("carries the scope it was given", func(t *testing.T) {
			t.Parallel()

			hidden := coretest.Struct(coretest.CachePath, "Cache")
			r, _ := coretest.Reading(t,
				func(pkg symbol.Identity) bool { return pkg.Package == coretest.StorePath },
				coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Store")),
				coretest.Package(coretest.CachePath, hidden))

			assert.Length(t, slices.Collect(r.ByKind(symbol.KindStruct)), 1,
				"the fixture carries the scope it was given")
		})
	})
}
