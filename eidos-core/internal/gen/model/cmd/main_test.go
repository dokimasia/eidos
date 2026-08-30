// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package main_test

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// generateTimeout bounds a wrapper run, so a hung generator fails
// the case rather than the suite.
const generateTimeout = 2 * time.Minute

// The wrapper is driven as a process, because that is how anyone
// runs it: `go generate` executes it. What it writes is the mirror
// guard's subject; what is left here is that it finds its module
// and reports when it cannot.
func TestMain(t *testing.T) {
	t.Parallel()

	t.Run("regenerates from inside the module", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), generateTimeout)
		defer cancel()

		run := exec.CommandContext(ctx, "go", "run", ".")
		run.Dir = "."
		if out, err := run.CombinedOutput(); err != nil {
			t.Fatalf("go run .: %v\n%s", err, out)
		}
	})

	t.Run("reports a directory outside any module", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), generateTimeout)
		defer cancel()

		run := exec.CommandContext(ctx, "go", "run", ".")
		run.Dir = t.TempDir()
		if err := run.Run(); err == nil {
			t.Fatal("go run . outside a module: error = nil, want non-nil")
		}
	})
}
