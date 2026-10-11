// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"
)

// mainEnv makes the test binary run main in place of its tests when it has
// the value runMain. A case then runs main as a process, with arguments, an
// environment and an exit status. go test sets GOCOVERDIR for a test binary
// built with coverage. The process writes its coverage counters there when
// it exits, and the test binary merges them into its coverage profile.
const (
	mainEnv = "EIDOS_RING_TEST_MAIN"
	runMain = "1"
)

// graceShare is the share of the time left before the deadline of a test
// that a process of the test does not get: the test kills the process at
// the deadline less the time left divided by graceShare.
const graceShare = 20

// The flag and the argument of the cases of the command line that the
// command does not take.
const (
	unknownFlag = "-unknown"
	extraArg    = "extra"
)

// missingDir is a directory that no case creates, or that a case removes.
const missingDir = "missing"

// windowsOS is the GOOS of Windows, which does not remove the working
// directory of a running process.
const windowsOS = "windows"

// tempEnv are the variables of the temporary directory of the system:
// TMPDIR on Linux and macOS, and TMP and TEMP on Windows.
var tempEnv = []string{"TMPDIR", "TMP", "TEMP"}

// TestMain runs main in place of the tests when mainEnv has the value
// runMain, and the tests otherwise. The process reads its standard input to
// the end before main runs, so a case can change the surroundings of the
// process first.
func TestMain(m *testing.M) {
	if os.Getenv(mainEnv) == runMain {
		_, _ = io.Copy(io.Discard, os.Stdin)
		main()
	}
	os.Exit(m.Run())
}

// TestMainProcess runs main as a process of the test binary, and run in the
// process of the test. Each process writes the coverage meta-data file of
// the binary into GOCOVERDIR under a temporary name that Go 1.27 makes from
// the time in nanoseconds alone, so the cases of main run one process at a
// time. The processes get inputs that end the run before the first query of
// the module proxy.
func TestMainProcess(t *testing.T) {
	self, err := os.Executable()
	assert.NoError(t, err, "the test binary has a path")

	t.Run("main", func(t *testing.T) {
		t.Run("exits with statusUsage for a root without a go.work", func(t *testing.T) {
			cmd := process(t, self, "-"+rootFlag, t.TempDir())
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			_, exited := errors.AsType[*exec.ExitError](cmd.Run())
			assert.True(t, exited, "the process exits with a status")
			expect.Equal(t, cmd.ProcessState.ExitCode(), statusUsage, "main exits with the status of run")
			expect.HasPrefix(t, stderr.String(), commandName+": ", "main writes the error of the checkout")
		})

		t.Run("exits with statusUsage for a working directory that does not exist", func(t *testing.T) {
			if runtime.GOOS == windowsOS {
				t.Skip("Windows does not remove the working directory of a process")
			}
			cmd := process(t, self)
			cmd.Dir = filepath.Join(t.TempDir(), missingDir)
			assert.NoError(t, os.Mkdir(cmd.Dir, dirMode), "the test creates the working directory")
			stdin, err := cmd.StdinPipe()
			assert.NoError(t, err, "the test opens the standard input of the process")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			assert.NoError(t, cmd.Start(), "the process starts")
			assert.NoError(t, os.Remove(cmd.Dir), "the test removes the working directory")
			assert.NoError(t, stdin.Close(), "the test lets the process run")
			_, exited := errors.AsType[*exec.ExitError](cmd.Wait())
			assert.True(t, exited, "the process exits with a status")
			expect.Equal(t, cmd.ProcessState.ExitCode(), statusUsage, "main exits with the status of run")
			expect.HasPrefix(t, stderr.String(), commandName+": ", "main writes the error of the root")
		})

		t.Run("exits with statusFailed for a temporary directory that does not exist", func(t *testing.T) {
			cmd := process(t, self, "-"+rootFlag, files.Workspace(t, checkoutTree(passingName)))
			missing := filepath.Join(t.TempDir(), missingDir)
			for _, name := range tempEnv {
				cmd.Env = append(cmd.Env, name+"="+missing)
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			_, exited := errors.AsType[*exec.ExitError](cmd.Run())
			assert.True(t, exited, "the process exits with a status")
			expect.Equal(t, cmd.ProcessState.ExitCode(), statusFailed, "main exits with the status of run")
			expect.HasPrefix(t, stderr.String(), commandName+": ", "main writes the error of the scratch directory")
		})
	})

	t.Run("run", func(t *testing.T) {
		usage := []struct {
			name string
			args []string
			want string
		}{
			{
				name: "returns statusUsage for a flag that the command does not define",
				args: []string{unknownFlag},
				want: "flag provided but not defined",
			},
			{
				name: "returns statusUsage for an argument",
				args: []string{extraArg},
				want: "the command takes no arguments",
			},
		}
		for _, tt := range usage {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var stdout, stderr bytes.Buffer
				expect.Equal(t, run(t.Context(), t.TempDir(), tt.args, os.Environ(), &stdout, &stderr), statusUsage,
					"run returns statusUsage")
				expect.Contains(t, stderr.String(), tt.want, "run writes the error of the command line")
			})
		}

		t.Run("writes the line of each module of the ring in the order of the module paths", func(t *testing.T) {
			t.Parallel()

			f := proxied(t, ringNames...)
			var stdout, stderr bytes.Buffer
			got := run(t.Context(), t.TempDir(), []string{"-" + rootFlag, f.root}, f.env, &stdout, &stderr)
			expect.Equal(t, got, statusFailed, "run returns statusFailed for the module that fails")
			expect.ContainsInOrder(t, stdout.String(), []string{
				failWord + " " + modulePrefix + brokenName + ": ",
				failWord + " " + modulePrefix + failingName + "@" + release + ": ",
				unpublishedWord + " " + modulePrefix + missingName + ": ",
				okWord + " " + modulePrefix + passingName + "@" + release + "\n",
				unpublishedWord + " " + modulePrefix + pseudoName + ": ",
			}, "run writes the line of each module in the order of the module paths")
			expect.Contains(
				t,
				stdout.String(),
				pseudoVersion,
				"the line of the module without a release names its pseudo-version",
			)
		})

		t.Run("returns statusOK for a ring without a module that fails", func(t *testing.T) {
			t.Parallel()

			f := proxied(t, missingName, passingName, pseudoName)
			var stdout, stderr bytes.Buffer
			got := run(t.Context(), t.TempDir(), []string{"-" + rootFlag, f.root}, f.env, &stdout, &stderr)
			expect.Equal(t, got, statusOK, "run returns statusOK: "+stdout.String()+stderr.String())
		})

		t.Run("writes FAIL for a module whose query its context stops", func(t *testing.T) {
			t.Parallel()

			f := proxied(t, passingName)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			var stdout, stderr bytes.Buffer
			got := run(ctx, t.TempDir(), []string{"-" + rootFlag, f.root}, f.env, &stdout, &stderr)
			expect.Equal(t, got, statusFailed, "run returns statusFailed")
			expect.HasPrefix(t, stdout.String(), failWord+" "+modulePrefix+passingName+": ",
				"run writes FAIL for the module")
		})
	})
}

// process returns the command that runs self, the test binary, as a process
// of main with args. The process inherits the environment of the test. The
// test kills the process shortly before its deadline, so a process that
// hangs fails the test and stops before the test binary exits.
func process(t *testing.T, self string, args ...string) *exec.Cmd {
	t.Helper()

	ctx := t.Context()
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline.Add(-time.Until(deadline)/graceShare))
		t.Cleanup(cancel)
	}
	cmd := exec.CommandContext(ctx, self, args...)
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), mainEnv+"="+runMain)
	return cmd
}
