// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package coretest_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/symbol"
)

// The rule every package is held to is written once here, so the
// case that proves it bites lives here too.
func TestPackageDoc(t *testing.T) {
	t.Parallel()

	t.Run("StatesDependencyPosition", func(t *testing.T) {
		t.Parallel()

		t.Run("answers true for a package that states one", func(t *testing.T) {
			t.Parallel()

			states, err := coretest.StatesDependencyPosition(".")
			assert.NoError(t, err, "this package parses")
			assert.True(t, states, "and states its own dependency position")
		})

		t.Run("answers false for a package that states none", func(t *testing.T) {
			t.Parallel()

			silent := filepath.Join("testdata", "silent")
			states, err := coretest.StatesDependencyPosition(silent)
			assert.NoError(t, err, "the silent package parses")
			assert.False(t, states,
				"a package comment without the heading does not count, "+
					"or the assertion would pass for every package")
		})

		t.Run("answers an error for a directory holding no Go source", func(t *testing.T) {
			t.Parallel()

			_, err := coretest.StatesDependencyPosition("testdata")
			assert.HasError(t, err, "a directory holding no Go source is an error, not a false")
		})
	})

	t.Run("AssertDependencyPosition", func(t *testing.T) {
		t.Parallel()

		t.Run("passes for a package that states one", func(t *testing.T) {
			t.Parallel()
			coretest.AssertDependencyPosition(t)
		})
	})

	t.Run("Rejects", func(t *testing.T) {
		t.Parallel()

		t.Run("Frozen refuses a fixture that would not load", func(t *testing.T) {
			t.Parallel()

			pkg := coretest.Package(coretest.StorePath)
			got := assert.Rejects(t, "a duplicate package fails the fixture",
				func(tb assert.TB) { coretest.Frozen(tb, pkg, pkg) })
			assert.Contains(t, got, "fixture",
				"and fails for the reason the helper is about")
		})

		t.Run("Reading refuses what Frozen refuses", func(t *testing.T) {
			t.Parallel()

			pkg := coretest.Package(coretest.StorePath)
			assert.Rejects(t, "the reader fixture carries the load check",
				func(tb assert.TB) { coretest.Reading(tb, nil, pkg, pkg) })
		})

		t.Run("Names refuses a symbol that names no declaration", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a foreign symbol fails the read-back",
				func(tb assert.TB) { coretest.Names(tb, []symbol.Symbol{foreign{}}) })
			assert.Contains(t, got, "names a declaration",
				"and says what the traversal was supposed to yield")
		})
	})
}
