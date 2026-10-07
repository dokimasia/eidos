// Copyright ThesmOS B.V. 2026
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

// otherModule is a go.mod naming a module this wrapper does not
// generate from.
const otherModule = "module example.test/other\n\ngo 1.27.0\n"

// The cases run the wrapper as a process, the way `go generate` runs
// it. The facade package's own cases test what [facade.Regenerate]
// generates. These cases check the exit status and the stream a
// refusal is written to.
func TestMain(t *testing.T) {
	t.Parallel()

	bin := wrapper(t)

	t.Run("regenerates from inside the kernel", func(t *testing.T) {
		t.Parallel()

		// The run writes into a copy of the mini kernel, so no test
		// run writes the committed facade the mirror guard compares.
		root := coretest.CopyTree(t, filepath.Join("..", "testdata", "mini"))
		out, err := runFrom(t, bin, filepath.Join(root, "eidos-core"))
		assert.NoError(t, err, "the wrapper regenerates from inside the kernel: "+out)
		assert.Empty(t, out, "and writes nothing on success")
		files.IsFile(t, filepath.Join(root, "eidos-sdk", "facade.gen.go"),
			"and the facade arrives beside the copied kernel")
	})

	t.Run("reports a module that is not the kernel", func(t *testing.T) {
		t.Parallel()

		dir := files.Workspace(t, files.Tree{"go.mod": files.Text(otherModule)})
		out, err := runFrom(t, bin, dir)
		assert.HasError(t, err,
			"a module that is not the kernel is reported, not generated into")
		assert.Contains(t, out, "example.test/other",
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
