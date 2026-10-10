// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package main_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/internal/coretest"
)

// generateTimeout bounds a wrapper run. A generator that hangs fails
// its own case.
const generateTimeout = 2 * time.Minute

// emptyModule is a go.mod for a module without a schema.
const emptyModule = "module example.test/empty\n\ngo 1.27.0\n"

// The cases run the wrapper as a process, the way `go generate` runs
// it. The model package's own cases test what [model.Regenerate]
// generates. These cases check the exit status and the stream a
// refusal is written to.
func TestMain(t *testing.T) {
	t.Parallel()

	bin := wrapper(t)

	t.Run("regenerates from inside the module", func(t *testing.T) {
		t.Parallel()

		// The run writes into a copy of a fixture module, so no test
		// run writes the committed models the mirror guard compares.
		root := coretest.CopyTree(t, filepath.Join("testdata", "module"))
		out, err := runFrom(t, bin, filepath.Join(root, "symbol", "schema"))
		assert.NoError(t, err, "the wrapper regenerates from inside the module: "+out)
		assert.Empty(t, out, "and writes nothing on success")
		files.IsFile(t, filepath.Join(root, "emit", "kinds.gen.go"), "and the models arrive in the copied module")
	})

	t.Run("reports a module without a schema", func(t *testing.T) {
		t.Parallel()

		dir := files.Workspace(t, files.Tree{"go.mod": files.Text(emptyModule)})
		out, err := runFrom(t, bin, dir)
		assert.HasError(t, err, "a module without a schema is reported, not generated into")
		assert.Contains(t, out, "model: load schema",
			"and the refusal is written to the standard error the caller reads")
	})
}

// wrapper builds the command and returns the binary, so a case can
// run it from a directory of its choosing the way `go generate`
// runs it from the package it annotates.
func wrapper(t *testing.T) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), generateTimeout)
	defer cancel()

	bin := filepath.Join(t.TempDir(), "wrapper")
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
	out, err := build.CombinedOutput()
	assert.NoError(t, err, "the wrapper builds: "+string(out))
	return bin
}

// runFrom runs the wrapper with dir as its working directory and
// returns what it wrote and how it exited.
func runFrom(t *testing.T, bin, dir string) (string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), generateTimeout)
	defer cancel()

	run := exec.CommandContext(ctx, bin)
	run.Dir = dir
	out, err := run.CombinedOutput()
	return string(out), err
}
