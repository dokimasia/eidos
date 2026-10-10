// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

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
)

// The flags that the cases pass to the commands.
const (
	jsonFlag     = "--format=json"
	verboseFlag  = "--verbose"
	strictFlag   = "--strict"
	helpFlag     = "-h"
	longHelpFlag = "--help"
	bogusFlag    = "--bogus"
	endFlag      = "--"
	planFlag     = "--plan"
	dryRunFlag   = "--dry-run"
	checkFlag    = "--check"
	coldFlag     = "--cold"
	driftFlag    = "--overwrite-drift"
	adoptFlag    = "--adopt"
	intervalFlag = "--interval"
)

// The events of JSON output that the cases read.
const (
	eventStart         = "start"
	eventError         = "error"
	eventDiag          = "diag"
	eventFile          = "file"
	eventOutcome       = "outcome"
	eventSummary       = "summary"
	eventComponent     = "component"
	eventPlan          = "plan"
	eventVersion       = "version"
	eventExplain       = "explain"
	eventExplainFile   = "explain.file"
	eventExplainClaim  = "explain.claim"
	eventExplainRecord = "explain.record"
)

// kernelNames contains the names of the kernel commands, in the order of
// Kernels.
var kernelNames = []string{cmdRun, cmdPlan, cmdExplain, cmdPrune, cmdDoctor, cmdWatch, cmdVersion}

// header is the field that every event of JSON output starts with.
type header struct {
	Event string `json:"event"`
}

// errorEvent is an error event of JSON output.
type errorEvent struct {
	Msg       string `json:"msg"`
	Usage     bool   `json:"usage"`
	Workspace string `json:"workspace"`
}

// summaryEvent is the summary event of JSON output, with the fields that
// the cases read.
type summaryEvent struct {
	Status     int            `json:"status"`
	Suppressed map[string]int `json:"suppressed"`
}

// findingEvent is a diag event of JSON output, or a finding of an explained
// record.
type findingEvent struct {
	Code      string `json:"code"`
	Severity  string `json:"severity"`
	Pos       string `json:"pos"`
	Msg       string `json:"msg"`
	Origin    string `json:"origin"`
	Workspace string `json:"workspace"`
}

// Every kernel command parses its flags and its arguments in the same way,
// and returns its own status.
func TestKernel(t *testing.T) {
	t.Parallel()

	t.Run("Kernels", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the seven kernel commands in order", func(t *testing.T) {
			t.Parallel()

			kernels := cli.Kernels(compose)
			got := make([]string, 0, len(kernels))
			for _, c := range kernels {
				got = append(got, c.Name())
			}
			assert.Equal(t, got, kernelNames, "the commands have the kernel names")
		})

		t.Run("returns a synopsis of one sentence for each command", func(t *testing.T) {
			t.Parallel()

			for _, c := range cli.Kernels(compose) {
				expect.HasSuffix(t, c.Synopsis(), ".", "the synopsis of "+c.Name()+" ends with a full stop")
				expect.NotContains(t, c.Synopsis(), "\n", "the synopsis of "+c.Name()+" has one line")
			}
		})
	})

	t.Run("Usage", func(t *testing.T) {
		t.Parallel()

		usage := command(t, compose, cmdRun).Usage()

		t.Run("returns the form of the command above its synopsis", func(t *testing.T) {
			t.Parallel()

			assert.HasPrefix(t, usage, "usage: run [flags] [<pattern>...]\n\n"+
				"Generates the output of each workspace and commits it.\n\nflags:\n",
				"the usage starts with the form and the synopsis")
		})

		t.Run("lists a flag that takes a value with the name of the value", func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, usage, "  --plan <plan>\n"+
				"        run this plan and the plans that it depends on, and repeat the flag for more plans\n",
				"the flag has the name of its value and its description")
		})

		t.Run("lists a boolean flag without a value", func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, usage, "  --dry-run\n        run every phase and commit nothing\n",
				"the flag has its description")
		})

		t.Run("lists the shared flags", func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, usage, "  --format <format>\n        the output format, text or json\n",
				"the usage has the flags of every command")
		})

		t.Run("returns a form without arguments for a command that takes none", func(t *testing.T) {
			t.Parallel()

			assert.HasPrefix(t, command(t, compose, cmdPlan).Usage(), "usage: plan [flags]\n\n",
				"plan takes no arguments")
		})
	})

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the usage to standard output for -h", func(t *testing.T) {
			t.Parallel()

			status, stdout, stderr := invoke(t, compose, cmdPlan, t.TempDir(), helpFlag)
			expect.Equal(t, status, cli.StatusOK, "help is no failure")
			expect.Equal(t, stdout, command(t, compose, cmdPlan).Usage(), "the command writes its usage")
			expect.Empty(t, stderr, "help writes no error")
		})

		t.Run("writes the usage for --help after a positional argument", func(t *testing.T) {
			t.Parallel()

			status, stdout, _ := invoke(t, compose, cmdRun, t.TempDir(), "svc", longHelpFlag)
			expect.Equal(t, status, cli.StatusOK, "help is no failure")
			expect.HasPrefix(t, stdout, "usage: run ", "the command writes its usage")
		})

		t.Run("returns StatusUsage for an unknown flag", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdPlan, t.TempDir(), bogusFlag)
			expect.Equal(t, status, cli.StatusUsage, "an unknown flag is a usage error")
			expect.HasPrefix(t, stderr, "error: cli: plan: flag provided but not defined: -bogus\nusage: plan [flags]",
				"text output writes the error and the usage to standard error")
		})

		t.Run("returns StatusUsage for an argument of a command that takes none", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdPlan, t.TempDir(), "svc", "api")
			expect.Equal(t, status, cli.StatusUsage, "an argument of plan is a usage error")
			expect.HasPrefix(t, stderr, "error: cli: plan: the command takes no arguments, and has svc api\n",
				"the error lists the arguments")
		})

		t.Run("writes a usage error as an error event in JSON output", func(t *testing.T) {
			t.Parallel()

			status, stdout, stderr := invoke(t, compose, cmdPlan, t.TempDir(), jsonFlag, bogusFlag)
			expect.Equal(t, status, cli.StatusUsage, "an unknown flag is a usage error")
			expect.Empty(t, stderr, "JSON output writes nothing to standard error")
			expect.Equal(t, decoded[header](t, stdout, ""),
				[]header{{Event: eventStart}, {Event: eventError}, {Event: eventSummary}},
				"the output has the start event, the error event and the summary event")
			errs := decoded[errorEvent](t, stdout, eventError)
			assert.Length(t, errs, 1, "the output has one error")
			expect.True(t, errs[0].Usage, "the error is a usage error")
			summaries := decoded[summaryEvent](t, stdout, eventSummary)
			assert.Length(t, summaries, 1, "the output has one summary")
			expect.Equal(t, summaries[0].Status, cli.StatusUsage, "the summary has the status")
		})

		t.Run("parses a flag after the positional arguments", func(t *testing.T) {
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

		t.Run("panics for an IO without an output stream", func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() { command(t, compose, cmdPlan).Run(t.Context(), cli.IO{}, nil) },
				"an IO without streams is a defect of the host")
		})
	})
}

// command returns the kernel command of c whose name is name.
func command(t *testing.T, c cli.Compose, name string) cli.Command {
	t.Helper()

	commands := cli.Kernels(c)
	i := slices.IndexFunc(commands, func(k cli.Command) bool { return k.Name() == name })
	assert.NotEqual(t, i, -1, "the kernel has the command "+name)
	return commands[i]
}

// invoke runs the kernel command of c whose name is name, in the working
// directory dir with args, under the context of the test and an empty
// environment. It returns the status of the command and what the command
// wrote to standard output and to standard error.
func invoke(t *testing.T, c cli.Compose, name, dir string, args ...string) (int, string, string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	stdio := cli.IO{Stdout: &stdout, Stderr: &stderr, Dir: dir, Getenv: func(string) string { return "" }}
	status := command(t, c, name).Run(t.Context(), stdio, args)
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
