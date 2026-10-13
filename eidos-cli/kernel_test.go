// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
)

// The names of the kernel commands.
const (
	cmdRun     = "run"
	cmdPlan    = "plan"
	cmdExplain = "explain"
	cmdPrune   = "prune"
	cmdDoctor  = "doctor"
	cmdWatch   = "watch"
	cmdVersion = "version"
	cmdSchema  = "schema"
)

// The flags that the cases pass to the commands.
const (
	jsonFlag     = "--format=json"
	verboseFlag  = "--verbose"
	quietFlag    = "--quiet"
	strictFlag   = "--strict"
	noColorFlag  = "--no-color"
	debugFlag    = "--flag-debug"
	chdirFlag    = "-C"
	configFlag   = "--config"
	helpFlag     = "-h"
	longHelpFlag = "--help"
	bogusFlag    = "--bogus"
	endFlag      = "--"
	planFlag     = "--plan"
	dryRunFlag   = "--dry-run"
	stdoutFlag   = "--stdout"
	checkFlag    = "--check"
	coldFlag     = "--cold"
	driftFlag    = "--overwrite-drift"
	adoptFlag    = "--adopt"
	intervalFlag = "--interval"
)

// These constants are the events of JSON output that the cases read.
const (
	eventStart              = "start"
	eventError              = "error"
	eventDiag               = "diag"
	eventFile               = "file"
	eventOutcome            = "outcome"
	eventSummary            = "summary"
	eventComponent          = "component"
	eventPlan               = "plan"
	eventVersion            = "version"
	eventSchema             = "schema"
	eventExplain            = "explain"
	eventExplainDeclaration = "explain.declaration"
	eventExplainFile        = "explain.file"
	eventExplainClaim       = "explain.claim"
	eventExplainExecution   = "explain.execution"
)

// appHelp is the help text of the apps of the cases. groupName and
// groupHelp are the name and the help text of the group that a case mounts
// the kernel commands under, and missingDir is a directory that a case
// does not create.
const (
	appHelp    = "Generates the code of the acme platform."
	groupName  = "gen"
	groupHelp  = "Generates the code."
	missingDir = "missing"
)

// kernelNames contains the names of the kernel commands, in the order of
// the help of an app that New returns.
var kernelNames = []string{cmdRun, cmdPlan, cmdExplain, cmdPrune, cmdDoctor, cmdWatch, cmdVersion, cmdSchema}

// header is the field that every event of JSON output starts with.
type header struct {
	Event string `json:"event"`
}

// startEvent is the start event of JSON output, with the fields that the
// cases read.
type startEvent struct {
	Command string `json:"command"`
	Brand   string `json:"brand"`
}

// errorEvent is an error event of JSON output.
type errorEvent struct {
	Msg    string `json:"msg"`
	Usage  bool   `json:"usage"`
	Member string `json:"member"`
}

// summaryEvent is the summary event of JSON output, with the fields that
// the cases read.
type summaryEvent struct {
	Status     int            `json:"status"`
	Suppressed map[string]int `json:"suppressed"`
}

// findingEvent is a diag event of JSON output, or a finding of an explained
// execution.
type findingEvent struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Pos      string `json:"pos"`
	Msg      string `json:"msg"`
	Origin   string `json:"origin"`
	Member   string `json:"member"`
}

// Every kernel command reads its options in the same way, renders its own
// usage errors and returns its own status.
func TestKernel(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the help of the command to standard output for -h", func(t *testing.T) {
			t.Parallel()

			status, stdout, stderr := invoke(t, compose, cmdPlan, t.TempDir(), helpFlag)
			expect.Equal(t, status, cli.StatusOK, "help is no failure")
			expect.HasPrefix(t, stdout, "Usage: acme plan [options]\n\n"+
				"Prints the composition of each workspace without running it.\n",
				"the command writes its usage line and its help text")
			expect.Empty(t, stderr, "help writes no error")
		})

		t.Run("writes the help for --help after a positional argument", func(t *testing.T) {
			t.Parallel()

			status, stdout, _ := invoke(t, compose, cmdRun, t.TempDir(), "svc", longHelpFlag)
			expect.Equal(t, status, cli.StatusOK, "help is no failure")
			expect.HasPrefix(t, stdout, "Usage: acme run [options] [PATTERNS...]\n", "run writes its usage line")
		})

		t.Run("returns StatusUsage for an unknown flag", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdPlan, t.TempDir(), bogusFlag)
			expect.Equal(t, status, cli.StatusUsage, "an unknown flag is a usage error")
			expect.Equal(t, stderr, "error: cli: plan: unknown flag --bogus\n",
				"text output writes the error alone to standard error")
		})

		t.Run("suggests the flag of a misspelled name", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdRun, t.TempDir(), "--plna", planName)
			expect.Equal(t, status, cli.StatusUsage, "a misspelled flag is a usage error")
			expect.Equal(t, stderr, "error: cli: run: unknown flag --plna (did you mean --plan?)\n",
				"the error suggests the flag")
		})

		t.Run("returns StatusUsage for --quiet with --verbose", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdPlan, t.TempDir(), quietFlag, verboseFlag)
			expect.Equal(t, status, cli.StatusUsage, "the two flags exclude each other")
			expect.Equal(t, stderr, "error: cli: plan: the flags --quiet and --verbose exclude each other\n",
				"the error names both flags")
		})

		t.Run("returns StatusUsage for an argument of a command that takes none", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdPlan, t.TempDir(), "svc", "api")
			expect.Equal(t, status, cli.StatusUsage, "an argument of plan is a usage error")
			expect.Equal(t, stderr, "error: cli: plan: unexpected argument \"svc\"\n", "the error names the argument")
		})

		t.Run("writes a usage error as an error event in JSON output", func(t *testing.T) {
			t.Parallel()

			status, stdout, stderr := invoke(t, compose, cmdPlan, t.TempDir(), jsonFlag, bogusFlag)
			expect.Equal(t, status, cli.StatusUsage, "an unknown flag is a usage error")
			expect.Empty(t, stderr, "JSON output writes nothing to standard error")
			expect.Equal(t, decoded[header](t, stdout, ""),
				[]header{{Event: eventStart}, {Event: eventError}, {Event: eventSummary}},
				"the output has the start event, the error event and the summary event")
			starts := decoded[startEvent](t, stdout, eventStart)
			assert.Length(t, starts, 1, "the output has one start")
			expect.Equal(t, starts[0], startEvent{Command: cmdPlan, Brand: "acme"},
				"the start has the command of the error and the brand")
			errs := decoded[errorEvent](t, stdout, eventError)
			assert.Length(t, errs, 1, "the output has one error")
			expect.True(t, errs[0].Usage, "the error is a usage error")
			summaries := decoded[summaryEvent](t, stdout, eventSummary)
			assert.Length(t, summaries, 1, "the output has one summary")
			expect.Equal(t, summaries[0].Status, cli.StatusUsage, "the summary has the status")
		})

		t.Run("renders a usage error as text when --format=json follows the fault", func(t *testing.T) {
			t.Parallel()

			_, stdout, stderr := invoke(t, compose, cmdPlan, t.TempDir(), bogusFlag, jsonFlag)
			expect.Empty(t, stdout, "a flag after the fault does not count")
			expect.HasPrefix(t, stderr, "error: ", "the output is text")
		})

		t.Run("reads a flag after the positional arguments", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, compose, cmdRun, stored(t, nil), "svc", jsonFlag)
			assert.HasPrefix(t, stdout, `{"event":"start"`, "the flag after the pattern selects JSON output")
		})

		t.Run("takes each argument after -- as positional", func(t *testing.T) {
			t.Parallel()

			status, stdout, _ := invoke(t, compose, cmdRun, stored(t, nil), dryRunFlag, endFlag, jsonFlag)
			expect.Equal(t, status, cli.StatusOK, "the argument after -- is a pattern")
			expect.NotContains(t, stdout, `"event"`, "the output is text")
		})

		t.Run("takes -- after a flag that takes a value as the value", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdRun, stored(t, nil), planFlag, endFlag, "svc")
			expect.Equal(t, status, cli.StatusUsage, "the run refuses the plan --")
			expect.Contains(t, stderr, `the plan "--"`, "-- is the value of --plan")
		})

		t.Run("takes -- after a flag with an equals sign as the end of the flags", func(t *testing.T) {
			t.Parallel()

			status, _, _ := invoke(t, compose, cmdRun, stored(t, nil), planFlag+"="+planName, endFlag, "svc")
			assert.Equal(t, status, cli.StatusOK, "-- ends the flags, and svc is a pattern")
		})

		t.Run("renders the error of IOOf in an app without the hook of Prepare", func(t *testing.T) {
			t.Parallel()

			app := gocli.New[cli.Global]("acme", appHelp)
			cli.Mount(app.Command[struct{}](groupName, groupHelp, nil), compose)
			missing := filepath.Join(t.TempDir(), missingDir)
			status, _, stderr := execute(t, app, compose, chdirFlag, missing, groupName, cmdPlan)
			expect.Equal(t, status, cli.StatusUsage, "a working directory that does not exist is a usage error")
			expect.HasPrefix(t, stderr, "error: cli: find the working directory: ", "the handler renders the error")
		})
	})
}

// invoke runs the kernel command name of an app that New returns over c,
// in the working directory dir with args, under the context of the test and
// an empty environment. It returns the status of the command and what the
// command wrote to standard output and to standard error.
func invoke(t *testing.T, c cli.Compose, name, dir string, args ...string) (int, string, string) {
	t.Helper()

	return execute(t, cli.New(appHelp, c), c, slices.Concat([]string{chdirFlag, dir, name}, args)...)
}

// execute runs the command line args of app over c with Run, under the
// context of the test, with the files of the process and an empty
// environment. It returns the status of the command and what the command
// wrote to standard output and to standard error.
func execute(t *testing.T, app *gocli.App[cli.Global], c cli.Compose, args ...string) (int, string, string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	env := &gocli.Env{Open: gocli.OS().Open, Stdout: &stdout, Stderr: &stderr, Args: args}
	status := cli.Run(t.Context(), app, c, env)
	return status, stdout.String(), stderr.String()
}

// decoded decodes each event of JSON output whose field event is name into
// a T, in the order of the output. An empty name selects every event. It
// fails the test for a line that is not a JSON object.
func decoded[T any](t *testing.T, out, name string) []T {
	t.Helper()

	var events []T
	for line := range strings.Lines(out) {
		var h header
		assert.NoError(t, json.Unmarshal([]byte(line), &h), "each line of the output is a JSON object")
		if name != "" && h.Event != name {
			continue
		}
		var event T
		assert.NoError(t, json.Unmarshal([]byte(line), &event), "the event decodes")
		events = append(events, event)
	}
	return events
}
