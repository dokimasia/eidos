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
	"slices"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/core/workspace"
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

// The commands of the binary beside the kernel commands, with their help
// texts, and a name of no command.
const (
	echoName  = "echo"
	echoHelp  = "Writes its words."
	failName  = "fail"
	failHelp  = "Returns the error of its kind."
	ghostName = "ghost"
)

// The kinds of error that the command fail returns, the status of its
// ExitError, and the texts of its errors.
const (
	exitKind    = "exit"
	usageKind   = "usage"
	plainKind   = "plain"
	exitCode    = 3
	usageText   = "cli_test: the words are missing"
	plainText   = "cli_test: the command failed"
	invalidName = "acme-"
)

// goneDir is the working directory that the test removes before Main runs,
// and dirMode its mode.
const (
	goneDir             = "gone"
	dirMode fs.FileMode = 0o755
)

// echoOptions are the options of the command echo.
type echoOptions struct {
	Words []string `arg:"" default:"" help:"Words to write."`
}

// failOptions are the options of the command fail.
type failOptions struct {
	Kind string `arg:"" help:"Kind of the error: exit, usage or plain."`
}

// TestMain runs Main in place of the tests when mainEnv has the value
// runMain, and the tests otherwise. The process reads its standard input to
// the end before Main runs, so a case can change the surroundings of the
// process first.
func TestMain(m *testing.M) {
	if os.Getenv(mainEnv) == runMain {
		_, _ = io.Copy(io.Discard, os.Stdin)
		cli.Main(binary(), compose)
	}
	os.Exit(m.Run())
}

// TestMainProcess runs Main as a process of the test binary, one process at
// a time, and Run in the test process. Each process writes the coverage
// meta-data file of the binary into the one GOCOVERDIR under a temporary
// name that Go 1.27 makes from the time in nanoseconds alone. When two
// processes exit in the same nanosecond, both write one temporary file, and
// the rename of one of them fails.
func TestMainProcess(t *testing.T) {
	self, err := os.Executable()
	assert.NoError(t, err, "the test binary has a path")

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
				name:       "exits with the status of the command",
				args:       []string{cmdPlan},
				wantStatus: cli.StatusOK,
				wantStdout: "frontend fakefront 1\n",
			},
			{
				name:       "exits with StatusUsage without a command",
				wantStatus: cli.StatusUsage,
				wantStderr: "Usage: acme <command> [options]\n",
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

		t.Run("exits with StatusUsage for a working directory that does not exist", func(t *testing.T) {
			if runtime.GOOS == windowsOS {
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
			expect.Equal(t, cmd.ProcessState.ExitCode(), cli.StatusUsage, "the missing directory is a usage error")
			expect.HasPrefix(t, stderr.String(), "error: cli: find the working directory: ",
				"Main writes the error of the search for the config file")
		})
	})

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			args       []string
			wantStatus int
			// wantStdout and wantStderr are what the command writes to
			// standard output and to standard error.
			wantStdout string
			wantStderr string
		}{
			{
				name:       "returns StatusUsage for a command that the binary does not have",
				args:       []string{ghostName},
				wantStatus: cli.StatusUsage,
				wantStderr: "error: cli: unknown command \"ghost\"\n",
			},
			{
				name:       "suggests the command of a misspelled name",
				args:       []string{"plna"},
				wantStatus: cli.StatusUsage,
				wantStderr: "error: cli: unknown command \"plna\" (did you mean plan?)\n",
			},
			{
				name:       "returns StatusUsage for a flag before the command that no option declares",
				args:       []string{bogusFlag, cmdRun},
				wantStatus: cli.StatusUsage,
				wantStderr: "error: cli: unknown flag --bogus\n",
			},
			{
				name:       "runs a command of the binary",
				args:       []string{echoName, "a", "b"},
				wantStatus: cli.StatusOK,
				wantStdout: "a b\n",
			},
			{
				name:       "returns the code of an ExitError of a handler and renders nothing",
				args:       []string{failName, exitKind},
				wantStatus: exitCode,
			},
			{
				name:       "returns StatusUsage for a UsageError of a handler",
				args:       []string{failName, usageKind},
				wantStatus: cli.StatusUsage,
				wantStderr: "error: " + usageText + "\n",
			},
			{
				name:       "returns StatusFailed for another error of a handler",
				args:       []string{failName, plainKind},
				wantStatus: cli.StatusFailed,
				wantStderr: "error: " + plainText + "\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				status, stdout, stderr := execute(t, binary(), compose, tt.args...)
				expect.Equal(t, status, tt.wantStatus, "Run returns the status of the command")
				expect.Equal(t, stdout, tt.wantStdout, "Run writes the output of the command")
				expect.Equal(t, stderr, tt.wantStderr, "Run writes the errors of the command")
			})
		}

		t.Run("writes the help of the app and a usage error without a command", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := execute(t, binary(), compose)
			expect.Equal(t, status, cli.StatusUsage, "a command line without a command is a usage error")
			expect.That(t, stderr).
				HasPrefix("Usage: acme <command> [options]\n\n"+appHelp+"\n", "go-cli writes the help of the app").
				HasSuffix("\n\nerror: cli: missing command\n", "Run writes the usage error after the help")
		})

		t.Run("lists the kernel commands and the commands of the binary in the help", func(t *testing.T) {
			t.Parallel()

			status, stdout, _ := execute(t, binary(), compose, helpFlag)
			assert.Equal(t, status, cli.StatusOK, "help is no failure")
			for _, name := range slices.Concat(kernelNames, []string{echoName, failName}) {
				expect.Contains(t, stdout, "\n  "+name+" ", "the help lists the command "+name)
			}
		})

		t.Run("writes a usage error before the command as JSON under --format=json", func(t *testing.T) {
			t.Parallel()

			status, stdout, stderr := execute(t, binary(), compose, jsonFlag, ghostName)
			expect.Equal(t, status, cli.StatusUsage, "the unknown command is a usage error")
			expect.HasPrefix(t, stdout, `{"event":"start","schema":"1.0","command":"","brand":"acme",`,
				"the start event has no command")
			expect.Empty(t, stderr, "JSON output writes nothing to standard error")
		})

		t.Run("writes the error of a handler as JSON under --format=json", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := execute(t, binary(), compose, jsonFlag, failName, plainKind)
			errs := decoded[errorEvent](t, stdout, eventError)
			assert.Length(t, errs, 1, "the output has one error")
			expect.Equal(t, errs[0], errorEvent{Msg: plainText}, "the error of the handler is no usage error")
			summaries := decoded[summaryEvent](t, stdout, eventSummary)
			assert.Length(t, summaries, 1, "the output has one summary")
			expect.Equal(t, summaries[0].Status, cli.StatusFailed, "the summary has the status")
		})

		t.Run("renders a failure in text for an app without Prepare", func(t *testing.T) {
			t.Parallel()

			app := gocli.New[cli.Global]("acme", appHelp)
			cli.Mount(app.Command[struct{}](groupName, groupHelp, nil), compose)
			status, stdout, stderr := execute(t, app, compose, jsonFlag, ghostName)
			expect.Equal(t, status, cli.StatusUsage, "the unknown command is a usage error")
			expect.Empty(t, stdout, "the run has no format without the renderer of Prepare")
			expect.Equal(t, stderr, "error: cli: unknown command \"ghost\"\n", "the run renders the failure as text")
		})

		panics := []struct {
			name string
			app  func() *gocli.App[cli.Global]
			env  func(stdout io.Writer) *gocli.Env
			want error
		}{
			{
				name: "panics for a command of the binary with the name of a kernel command",
				app: func() *gocli.App[cli.Global] {
					app := cli.New(appHelp, compose)
					app.Command(cmdRun, echoHelp, echo)
					return app
				},
				env:  func(stdout io.Writer) *gocli.Env { return &gocli.Env{Stdout: stdout, Stderr: io.Discard} },
				want: gocli.ErrDeclaration,
			},
			{
				name: "panics for a brand that is not the name of an app",
				app: func() *gocli.App[cli.Global] {
					return cli.New(appHelp, func() *workspace.Builder { return compose().Brand(invalidName) })
				},
				env:  func(stdout io.Writer) *gocli.Env { return &gocli.Env{Stdout: stdout, Stderr: io.Discard} },
				want: gocli.ErrDeclaration,
			},
			{
				name: "panics for an Env without Stdout",
				app:  binary,
				env:  func(io.Writer) *gocli.Env { return &gocli.Env{Stderr: io.Discard} },
				want: gocli.ErrEnv,
			},
		}
		for _, tt := range panics {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var stdout bytes.Buffer
				got := assert.Panics(t, func() { cli.Run(t.Context(), tt.app(), compose, tt.env(&stdout)) },
					"the declarations and the Env are defects of the binary")
				err, _ := got.(error)
				expect.ErrorIs(t, err, tt.want, "the panic is the error of the run")
				expect.Empty(t, stdout.String(), "Run renders nothing for a defect")
			})
		}
	})
}

// binary returns the app of the binary of the cases: the kernel commands
// over the fixture composition, the command echo and the command fail.
func binary() *gocli.App[cli.Global] {
	app := cli.New(appHelp, compose)
	app.Command(echoName, echoHelp, echo)
	app.Command(failName, failHelp, fail)
	return app
}

// echo is the handler of the command echo. It writes its words to standard
// output as one line.
//
// Error modes: echo returns the error of the write.
func echo(_ context.Context, run *gocli.Run[cli.Global, echoOptions]) error {
	_, err := fmt.Fprintln(run.Stdout(), strings.Join(run.Config().Words, " "))
	return err
}

// fail is the handler of the command fail. It returns an ExitError with
// exitCode for exitKind, a UsageError for usageKind, and an error that wraps
// neither for any other kind.
func fail(_ context.Context, run *gocli.Run[cli.Global, failOptions]) error {
	switch run.Config().Kind {
	case exitKind:
		return cli.Exit(exitCode)
	case usageKind:
		return &cli.UsageError{Err: errors.New(usageText)}
	default:
		return errors.New(plainText)
	}
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
