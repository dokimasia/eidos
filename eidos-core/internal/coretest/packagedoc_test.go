// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package coretest_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/eidos/core/internal/coretest"
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
			if err != nil {
				t.Fatalf("StatesDependencyPosition: unexpected error: %v", err)
			}
			if !states {
				t.Fatal("StatesDependencyPosition(\".\") = false, want true: " +
					"this package states its own")
			}
		})

		t.Run("answers false for a package that states none", func(t *testing.T) {
			t.Parallel()

			silent := filepath.Join("testdata", "silent")
			states, err := coretest.StatesDependencyPosition(silent)
			if err != nil {
				t.Fatalf("StatesDependencyPosition: unexpected error: %v", err)
			}
			if states {
				t.Fatal("a package comment without the heading answered true, " +
					"so the assertion would pass for every package")
			}
		})

		t.Run("answers an error for a directory holding no Go source", func(t *testing.T) {
			t.Parallel()

			if _, err := coretest.StatesDependencyPosition("testdata"); err == nil {
				t.Fatal("StatesDependencyPosition of a sourceless directory: " +
					"error = nil, want non-nil")
			}
		})
	})

	t.Run("AssertDependencyPosition", func(t *testing.T) {
		t.Parallel()

		t.Run("passes for a package that states one", func(t *testing.T) {
			t.Parallel()
			coretest.AssertDependencyPosition(t)
		})
	})
}
