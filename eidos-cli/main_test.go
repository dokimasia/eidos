// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/cli"
)

// mainEnv makes the test binary run Main in place of its tests when it has
// the value runMain. A case then runs Main as a process, with arguments, a
// working directory and an exit status. go test sets GOCOVERDIR for a test
// binary built with coverage. The process writes its coverage counters there
// when it exits, and the test binary merges them into its coverage profile.
const (
	mainEnv = "EIDOS_CLI_TEST_MAIN"
	runMain = "1"
)

// graceShare is the share of the time left before the deadline of a test
// that a process of the test does not get: the test kills the process at
// the deadline less the time left divided by graceShare.
const graceShare = 20

// The command line of the process cases: the extra command of the binary,
// the command name that lists the commands, and a name of no command.
const (
	echoName  = "echo"
	helpName  = "help"
	ghostName = "ghost"
)

// goneDir is the working directory that the test removes before Main runs,
// and dirMode its mode.
const (
	goneDir             = "gone"
	dirMode fs.FileMode = 0o755
)

// extra is a command of a binary beside the kernel commands. It writes its
// arguments to standard output as one line, and returns StatusOK.
type extra struct {
	name string
}

// Name returns the name of the command.
func (e extra) Name() string { return e.name }

// Synopsis returns the synopsis of the command.
func (extra) Synopsis() string { return "Writes its arguments." }

// Usage returns the help text of the command.
func (e extra) Usage() string { return "usage: " + e.name + " [arguments]\n" }

// Run writes args to standard output as one line, and returns StatusOK.
func (extra) Run(_ context.Context, stdio cli.IO, args []string) int {
	fmt.Fprintln(stdio.Stdout, strings.Join(args, " "))
	return cli.StatusOK
}

// TestMain runs Main in place of the tests when mainEnv has the value
// runMain, and the tests otherwise. The process reads its standard input to
// the end before Main runs, so a case can change the surroundings of the
// process first.
func TestMain(m *testing.M) {
	if os.Getenv(mainEnv) == runMain {
		_, _ = io.Copy(io.Discard, os.Stdin)
		cli.Main(compose, extra{name: echoName})
	}
	os.Exit(m.Run())
}

// TestMainProcess runs Main as a process of the test binary, one process at
// a time. Each process writes the coverage meta-data file of the binary into
// the one GOCOVERDIR under a temporary name that Go 1.27 makes from the time
// in nanoseconds alone. When two processes exit in the same nanosecond, both
// write one temporary file, and the rename of one of them fails.
func TestMainProcess(t *testing.T) {
	self, err := os.Executable()
	assert.NoError(t, err, "the test binary has a path")
	binary := filepath.Base(self)
	listed := "usage: " + binary + " [flags] <command> [flags] [arguments]\n\ncommands:\n"

	t.Run("Main", func(t *testing.T) {
		tests := []struct {
			name       string
			args       []string
			wantStatus int
			// wantStdout and wantStderr are the beginnings of what the process
			// writes to standard output and to standard error.
			wantStdout string
			wantStderr string
		}{
			{
				name:       "exits with StatusUsage without a command",
				wantStatus: cli.StatusUsage,
				wantStderr: "error: cli: the command line of " + binary + " has no command\n" + listed,
			},
			{
				name:       "writes the list of commands to standard output for -h",
				args:       []string{helpFlag},
				wantStatus: cli.StatusOK,
				wantStdout: listed,
			},
			{
				name:       "writes the list of commands to standard output for help",
				args:       []string{helpName},
				wantStatus: cli.StatusOK,
				wantStdout: listed,
			},
			{
				name:       "writes the usage of the command after help",
				args:       []string{helpName, cmdRun},
				wantStatus: cli.StatusOK,
				wantStdout: "usage: run [flags] [<pattern>...]\n",
			},
			{
				name:       "exits with StatusUsage for help with two names",
				args:       []string{helpName, cmdRun, cmdPlan},
				wantStatus: cli.StatusUsage,
				wantStderr: "error: cli: help takes the name of one command, and has run plan\n" + listed,
			},
			{
				name:       "exits with StatusUsage for help with the name of no command",
				args:       []string{helpName, ghostName},
				wantStatus: cli.StatusUsage,
				wantStderr: "error: cli: help takes the name of one command, and has ghost\n",
			},
			{
				name:       "exits with StatusUsage for a command that the binary does not have",
				args:       []string{ghostName},
				wantStatus: cli.StatusUsage,
				wantStderr: "error: cli: ghost is not a command of " + binary + "\n" + listed,
			},
			{
				name:       "exits with StatusUsage for a flag before the command that no command defines",
				args:       []string{bogusFlag, cmdRun},
				wantStatus: cli.StatusUsage,
				wantStderr: "error: cli: flag provided but not defined: -bogus\n",
			},
			{
				name:       "writes a usage error before the command as JSON under --format=json",
				args:       []string{jsonFlag, ghostName},
				wantStatus: cli.StatusUsage,
				wantStdout: `{"event":"start","schema":"1.0","command":"","brand":"acme",`,
			},
			{
				name:       "passes the flags before the name of a command ahead of its arguments",
				args:       []string{jsonFlag, echoName, "a"},
				wantStatus: cli.StatusOK,
				wantStdout: jsonFlag + " a\n",
			},
			{
				name:       "runs the kernel command after the flags",
				args:       []string{cmdPlan},
				wantStatus: cli.StatusOK,
				wantStdout: "frontend fakefront 1\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cmd := process(t, self, t.TempDir(), tt.args...)
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				if err := cmd.Run(); err != nil {
					_, exited := errors.AsType[*exec.ExitError](err)
					assert.True(t, exited, "the process exits with a status: "+err.Error())
				}
				expect.Equal(t, cmd.ProcessState.ExitCode(), tt.wantStatus, "Main exits with the status of the command")
				expect.HasPrefix(t, stdout.String(), tt.wantStdout, "Main writes the output of the command")
				expect.HasPrefix(t, stderr.String(), tt.wantStderr, "Main writes the errors of the command")
			})
		}

		t.Run("exits with StatusFailed for a working directory that does not exist", func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("Windows does not remove the working directory of a process")
			}
			dir := filepath.Join(t.TempDir(), goneDir)
			assert.NoError(t, os.Mkdir(dir, dirMode), "the test creates the working directory")
			cmd := process(t, self, dir, cmdPlan)
			stdin, err := cmd.StdinPipe()
			assert.NoError(t, err, "the test opens the standard input of the process")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			assert.NoError(t, cmd.Start(), "the process starts")
			assert.NoError(t, os.Remove(dir), "the test removes the working directory")
			assert.NoError(t, stdin.Close(), "the test lets the process run")
			err = cmd.Wait()
			_, exited := errors.AsType[*exec.ExitError](err)
			assert.True(t, exited, "the process exits with a status")
			expect.Equal(t, cmd.ProcessState.ExitCode(), cli.StatusFailed, "the missing directory fails Main")
			expect.HasPrefix(t, stderr.String(), "cli: find the working directory: ",
				"Main writes the error of ProcessIO")
		})

		panics := []struct {
			name  string
			extra []cli.Command
			want  string
		}{
			{
				name:  "panics for an extra command with an empty name",
				extra: []cli.Command{extra{}},
				want:  "cli: a command has an empty name",
			},
			{
				name:  "panics for an extra command with the name of a kernel command",
				extra: []cli.Command{extra{name: cmdRun}},
				want:  `cli: two commands have the name "run"`,
			},
			{
				name:  "panics for two extra commands with one name",
				extra: []cli.Command{extra{name: echoName}, extra{name: echoName}},
				want:  `cli: two commands have the name "echo"`,
			},
		}
		for _, tt := range panics {
			t.Run(tt.name, func(t *testing.T) {
				got := assert.Panics(t, func() { cli.Main(compose, tt.extra...) },
					"the commands are a defect of the binary")
				assert.Equal(t, got, any(tt.want), "the panic names the defect")
			})
		}
	})
}

// process returns the command that runs binary as a process of Main in dir
// with args. The test kills the process shortly before its deadline, so a
// process that hangs fails the test and stops before the test binary exits.
func process(t *testing.T, binary, dir string, args ...string) *exec.Cmd {
	t.Helper()

	ctx := t.Context()
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline.Add(-time.Until(deadline)/graceShare))
		t.Cleanup(cancel)
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), mainEnv+"="+runMain, "PWD="+dir)
	return cmd
}
