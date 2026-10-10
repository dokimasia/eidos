// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"io"
	"os"
	"runtime"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/cli"
)

// The IO of a process has the streams, the working directory and the
// environment of the process.
func TestIO(t *testing.T) {
	t.Parallel()

	t.Run("ProcessIO", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the streams, the working directory and the environment of the process", func(t *testing.T) {
			t.Parallel()

			got, err := cli.ProcessIO()
			assert.NoError(t, err, "the process has a working directory")
			dir, err := os.Getwd()
			assert.NoError(t, err, "the test reads the working directory")
			info, err := os.Stderr.Stat()
			expect.Equal(t, got.Stdout, io.Writer(os.Stdout), "the IO writes to standard output", assert.ByIdentity())
			expect.Equal(t, got.Stderr, io.Writer(os.Stderr), "the IO writes to standard error", assert.ByIdentity())
			expect.Equal(t, got.Dir, dir, "the IO has the working directory")
			expect.Equal(t, got.Getenv("PATH"), os.Getenv("PATH"), "the IO reads the environment")
			expect.Equal(t, got.Terminal, err == nil && info.Mode()&os.ModeCharDevice != 0,
				"the IO is a terminal when standard error is a character device")
		})
	})
}

// ProcessIO returns an error when the working directory of the process does
// not exist. The case changes the working directory of the process, so it
// runs alone.
func TestIOProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not remove the working directory of a process")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	assert.NoError(t, os.Remove(dir), "the test removes the working directory")
	_, err := cli.ProcessIO()
	assert.HasError(t, err, "ProcessIO returns the error of os.Getwd")
}
