// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/toolchain"
)

// The binary that the run cases start, which go test puts on the PATH of
// every test, the command that prints a setting, and the settings that the
// cases read.
const (
	goBinary   = "go"
	envCommand = "env"
	goosVar    = "GOOS"
	goflagsVar = "GOFLAGS"
	gomodVar   = "GOMOD"
	modFlag    = "-mod=mod"
)

// The module file that the case of the working directory writes, and its
// content.
const (
	modFile = "go.mod"
	modBody = "module example.test/run\n"
)

// The names of two binaries that no PATH contains, and a command that the
// go binary does not know.
const (
	missingBinary  = "eidos-toolchain-no-such-binary"
	missingSecond  = "eidos-toolchain-no-such-second"
	unknownCommand = "eidos-no-such-command"
)

// TestRun checks that Run starts a binary in its directory and environment,
// and that LookPaths finds a binary on PATH.
func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the output of the binary", func(t *testing.T) {
			t.Parallel()

			out, err := toolchain.Run(t.Context(), t.TempDir(), nil, goBinary, envCommand, goosVar)
			assert.NoError(t, err, "go env runs")
			assert.Equal(t, strings.TrimSpace(out), runtime.GOOS, "the output is the value of GOOS")
		})

		t.Run("sets the variables of env on top of the environment", func(t *testing.T) {
			t.Parallel()

			env := []string{goflagsVar + "=" + modFlag}
			out, err := toolchain.Run(t.Context(), t.TempDir(), env, goBinary, envCommand, goflagsVar)
			assert.NoError(t, err, "go env runs")
			assert.Equal(t, strings.TrimSpace(out), modFlag, "the binary reads the variable of env")
		})

		t.Run("runs the binary in dir", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{modFile: files.Text(modBody)})
			out, err := toolchain.Run(t.Context(), dir, nil, goBinary, envCommand, gomodVar)
			assert.NoError(t, err, "go env runs")
			got, err := os.Stat(strings.TrimSpace(out))
			assert.NoError(t, err, "GOMOD is the path of a file")
			want, err := os.Stat(filepath.Join(dir, modFile))
			assert.NoError(t, err, "the module file of dir exists")
			assert.True(t, os.SameFile(got, want), "GOMOD is the module file of dir")
		})

		t.Run("returns an error that contains the output of a binary that fails", func(t *testing.T) {
			t.Parallel()

			out, err := toolchain.Run(t.Context(), t.TempDir(), nil, goBinary, unknownCommand)
			exit := assert.ErrorAs[*exec.ExitError](t, err, "the error wraps the exit of go")
			assert.NotEqual(t, exit.ExitCode(), 0, "go exits with an error")
			assert.Contains(t, out, unknownCommand, "go reports the command that it does not know")
			assert.Contains(t, err.Error(), out, "the error contains the output of go")
		})

		t.Run("returns an error that wraps ErrNotFound for a binary that is not on PATH", func(t *testing.T) {
			t.Parallel()

			_, err := toolchain.Run(t.Context(), t.TempDir(), nil, missingBinary)
			assert.ErrorIs(t, err, exec.ErrNotFound, "the error wraps the error of os/exec")
		})

		t.Run("returns the error of the context for a context that ended", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, err := toolchain.Run(ctx, dir, nil, goBinary, envCommand, goosVar)
				return err
			}, "an ended context starts no binary")
		})
	})

	t.Run("LookPaths", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for binaries on PATH", func(t *testing.T) {
			t.Parallel()

			found, why := toolchain.LookPaths(goBinary)
			assert.True(t, found, "go is on PATH")
			assert.Empty(t, why, "LookPaths returns no reason for a binary on PATH")
		})

		t.Run("returns the first binary that is not on PATH", func(t *testing.T) {
			t.Parallel()

			found, why := toolchain.LookPaths(goBinary, missingBinary, missingSecond)
			assert.False(t, found, "LookPaths reports false for a missing binary")
			assert.HasPrefix(t, why, missingBinary+" is not on PATH", "the reason starts with the first missing binary")
		})

		t.Run("reports true for no binaries", func(t *testing.T) {
			t.Parallel()

			found, why := toolchain.LookPaths()
			assert.True(t, found, "no binary is missing")
			assert.Empty(t, why, "LookPaths returns no reason")
		})
	})
}

// TestRunEnv checks the cases that set a variable of the process, which run
// apart from the parallel cases of TestRun.
func TestRunEnv(t *testing.T) {
	t.Run("Run", func(t *testing.T) {
		t.Run("runs the binary in the environment of the process for a nil env", func(t *testing.T) {
			t.Setenv(goflagsVar, modFlag)
			out, err := toolchain.Run(t.Context(), t.TempDir(), nil, goBinary, envCommand, goflagsVar)
			assert.NoError(t, err, "go env runs")
			assert.Equal(t, strings.TrimSpace(out), modFlag, "the binary reads the variable of the process")
		})
	})
}
