// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/position"
)

// Text output writes these ANSI escape codes on a terminal.
const (
	red    = "\x1b[31m"
	yellow = "\x1b[33m"
	cyan   = "\x1b[36m"
	plain  = "\x1b[0m"
)

// A summary of a command without files and plans has these counts.
const (
	noFiles = `"files":{"create":0,"update":0,"unchanged":0,"stale":0,"drifted":0,"foreign":0,"withheld":0}`
	noPlans = `"plans":{"committed":0,"failed":0,"cancelled":0,"prepared":0,"skipped":0}`
)

// member is the member of a list that the cases of a list render.
var member = cli.Member{Name: "platform"}

// finding is an Error with a column and a related position.
var finding = diag.Diag{
	Code:     frontendtest.ScriptedBadFile,
	Severity: diag.SeverityError,
	Pos:      position.Pos{File: "svc/store.zz", Line: 3, Col: 2},
	Msg:      "Store has no spelling",
	Origin:   "mirror",
	Related:  []position.Pos{{File: "svc/api.zz", Line: 1}},
}

// A renderer writes text for a person, or one JSON event per line.
func TestRenderer(t *testing.T) {
	t.Parallel()

	code := frontendtest.ScriptedBadFile.String()
	text := cli.Flags{Format: cli.FormatText}
	jsonOutput := cli.Flags{Format: cli.FormatJSON}

	t.Run("Begin", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the start event for JSON output", func(t *testing.T) {
			t.Parallel()

			_, stdout, stderr := rendered(t, jsonOutput, nil, false)
			var got struct {
				Event, Schema, Command, Brand string
				Binary                        struct{ Path, Version string }
			}
			assert.NoError(t, json.Unmarshal(stdout.Bytes(), &got), "the start event is one JSON object")
			info, ok := debug.ReadBuildInfo()
			assert.True(t, ok, "the test binary has build information")
			assert.Equal(t, got.Event, "start", "the first event is start")
			assert.Equal(t, got.Schema, "1.0", "the event has the version of the schema")
			assert.Equal(t, got.Command, "run", "the event has the name of the command")
			assert.Equal(t, got.Brand, string(brand), "the event has the brand of the composition")
			assert.Equal(t, got.Binary.Path, info.Main.Path, "the event has the path of the main module")
			assert.Equal(t, got.Binary.Version, info.Main.Version, "the event has the version of the main module")
			assert.Empty(t, stderr.String(), "JSON output writes nothing to standard error")
		})

		t.Run("writes nothing for text output", func(t *testing.T) {
			t.Parallel()

			_, stdout, stderr := rendered(t, text, nil, false)
			assert.Empty(t, stdout.String(), "text output has no start line")
			assert.Empty(t, stderr.String(), "text output has no start line")
		})

		t.Run("panics for an IO without an output stream", func(t *testing.T) {
			t.Parallel()

			stdio := cli.IO{Stderr: io.Discard, Dir: t.TempDir(), Getenv: os.Getenv}
			assert.Panics(t, func() { cli.Begin(stdio, text, compose, "run") }, "the IO needs both streams")
		})

		t.Run("panics for an IO without Getenv", func(t *testing.T) {
			t.Parallel()

			stdio := cli.IO{Stdout: io.Discard, Stderr: io.Discard, Dir: t.TempDir()}
			assert.Panics(t, func() { cli.Begin(stdio, text, compose, "run") }, "the IO needs Getenv")
		})
	})

	t.Run("Report", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the finding and its related positions to standard error", func(t *testing.T) {
			t.Parallel()

			r, stdout, stderr := rendered(t, text, nil, false)
			r.Report(finding)
			assert.Equal(t, stderr.String(),
				"svc/store.zz:3:2: error "+code+": Store has no spelling (mirror)\n    related: svc/api.zz:1\n",
				"the finding has one line and each related position one more")
			assert.Empty(t, stdout.String(), "a finding is no result of the command")
		})

		positions := []struct {
			name string
			give position.Pos
			want string
		}{
			{
				name: "leaves out a column of 0",
				give: position.Pos{File: "svc/store.zz", Line: 3},
				want: "svc/store.zz:3",
			},
			{name: "leaves out a line of 0", give: position.Pos{File: ".acme/lock"}, want: ".acme/lock"},
		}
		for _, tt := range positions {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r, _, stderr := rendered(t, text, nil, false)
				d := finding
				d.Pos, d.Related = tt.give, nil
				r.Report(d)
				assert.Equal(t, stderr.String(), tt.want+": error "+code+": Store has no spelling (mirror)\n",
					"the position has no part of 0")
			})
		}

		colours := []struct {
			name     string
			severity diag.Severity
			want     string
		}{
			{name: "colours an Error red on a terminal", severity: diag.SeverityError, want: red + "error" + plain},
			{
				name:     "colours a Warning yellow on a terminal",
				severity: diag.SeverityWarning,
				want:     yellow + "warning" + plain,
			},
			{name: "colours an Info cyan on a terminal", severity: diag.SeverityInfo, want: cyan + "info" + plain},
		}
		for _, tt := range colours {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r, _, stderr := rendered(t, cli.Flags{Format: cli.FormatText, Verbose: true}, nil, true)
				d := finding
				d.Severity, d.Related = tt.severity, nil
				r.Report(d)
				assert.Equal(t, stderr.String(),
					"svc/store.zz:3:2: "+tt.want+" "+code+": Store has no spelling (mirror)\n",
					"the severity has its colour")
			})
		}

		plainOutputs := []struct {
			name string
			flag cli.Flags
			env  map[string]string
		}{
			{name: "writes no colour under --no-color", flag: cli.Flags{Format: cli.FormatText, NoColor: true}},
			{name: "writes no colour when NO_COLOR is set", flag: text, env: map[string]string{"NO_COLOR": "1"}},
		}
		for _, tt := range plainOutputs {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r, _, stderr := rendered(t, tt.flag, tt.env, true)
				r.Report(finding)
				assert.NotContains(t, stderr.String(), "\x1b[", "the output has no escape code")
			})
		}

		t.Run("writes no Info without --verbose", func(t *testing.T) {
			t.Parallel()

			r, _, stderr := rendered(t, text, nil, false)
			d := finding
			d.Severity = diag.SeverityInfo
			r.Report(d)
			assert.Empty(t, stderr.String(), "an Info renders under --verbose alone")
		})

		t.Run("writes an Info under --verbose", func(t *testing.T) {
			t.Parallel()

			r, _, stderr := rendered(t, cli.Flags{Format: cli.FormatText, Verbose: true}, nil, false)
			d := finding
			d.Severity, d.Related = diag.SeverityInfo, nil
			r.Report(d)
			assert.Equal(t, stderr.String(), "svc/store.zz:3:2: info "+code+": Store has no spelling (mirror)\n",
				"the Info renders")
		})

		t.Run("puts the name of the member before each position in text output", func(t *testing.T) {
			t.Parallel()

			r, _, stderr := rendered(t, text, nil, false)
			r.Workspace(member)
			r.Report(finding)
			assert.Equal(t, stderr.String(),
				"platform/svc/store.zz:3:2: error "+code+": Store has no spelling (mirror)\n"+
					"    related: platform/svc/api.zz:1\n",
				"each position is relative to the directory of the list")
		})

		t.Run("writes a diag event for JSON output", func(t *testing.T) {
			t.Parallel()

			r, stdout, stderr := rendered(t, jsonOutput, nil, false)
			r.Report(finding)
			assert.Equal(t, after(stdout),
				`{"event":"diag","code":"`+code+`","severity":"error","pos":"svc/store.zz:3:2",`+
					`"msg":"Store has no spelling","origin":"mirror","related":["svc/api.zz:1"]}`+"\n",
				"the event has the fields of the finding")
			assert.Empty(t, stderr.String(), "JSON output writes nothing to standard error")
		})

		t.Run("writes a diag event without related positions", func(t *testing.T) {
			t.Parallel()

			r, stdout, _ := rendered(t, jsonOutput, nil, false)
			d := finding
			d.Related = nil
			r.Report(d)
			assert.NotContains(t, after(stdout), "related", "the event has no field related")
		})

		t.Run("writes the member into a diag event", func(t *testing.T) {
			t.Parallel()

			r, stdout, _ := rendered(t, jsonOutput, nil, false)
			r.Workspace(member)
			r.Report(finding)
			assert.HasSuffix(t, after(stdout), `,"workspace":"platform"}`+"\n", "the event names the member")
		})
	})

	t.Run("Event", func(t *testing.T) {
		t.Parallel()

		type file struct {
			Path string `json:"path"`
		}

		t.Run("writes the text to standard output for text output", func(t *testing.T) {
			t.Parallel()

			r, stdout, _ := rendered(t, text, nil, false)
			r.Event("file", file{Path: "svc/mirror.txt"}, "create svc/mirror.txt")
			assert.Equal(t, stdout.String(), "create svc/mirror.txt\n", "the text is one line")
		})

		t.Run("writes nothing for empty text", func(t *testing.T) {
			t.Parallel()

			r, stdout, _ := rendered(t, text, nil, false)
			r.Event("file", file{Path: "svc/mirror.txt"}, "")
			assert.Empty(t, stdout.String(), "the event has no text")
		})

		t.Run("writes the name of the event before the fields of the value", func(t *testing.T) {
			t.Parallel()

			r, stdout, _ := rendered(t, jsonOutput, nil, false)
			r.Event("file", file{Path: "svc/mirror.txt"}, "create svc/mirror.txt")
			assert.Equal(t, after(stdout), `{"event":"file","path":"svc/mirror.txt"}`+"\n", "the event has the fields")
		})

		t.Run("writes only the name of the event for a value without fields", func(t *testing.T) {
			t.Parallel()

			r, stdout, _ := rendered(t, jsonOutput, nil, false)
			r.Event("tick", struct{}{}, "")
			assert.Equal(t, after(stdout), `{"event":"tick"}`+"\n", "the event has no other field")
		})

		t.Run("writes the member after the name of the event", func(t *testing.T) {
			t.Parallel()

			r, stdout, _ := rendered(t, jsonOutput, nil, false)
			r.Workspace(member)
			r.Event("file", file{Path: "svc/mirror.txt"}, "")
			assert.Equal(t, after(stdout), `{"event":"file","workspace":"platform","path":"svc/mirror.txt"}`+"\n",
				"the event names the member")
		})

		t.Run("panics for a value that is not a JSON object", func(t *testing.T) {
			t.Parallel()

			r, _, _ := rendered(t, jsonOutput, nil, false)
			assert.Panics(t, func() { r.Event("file", "svc/mirror.txt", "") }, "a string is no event")
		})

		t.Run("panics for a value that does not encode", func(t *testing.T) {
			t.Parallel()

			r, _, _ := rendered(t, jsonOutput, nil, false)
			assert.Panics(t, func() { r.Event("file", map[string]any{"done": make(chan int)}, "") },
				"a channel does not encode")
		})
	})

	t.Run("Error", func(t *testing.T) {
		t.Parallel()

		joined := errors.Join(errors.New("cli_test: the first fault"), errors.New("cli_test: the second fault"))

		t.Run("writes each line of the error to standard error", func(t *testing.T) {
			t.Parallel()

			r, stdout, stderr := rendered(t, text, nil, false)
			r.Error(joined)
			assert.Equal(t, stderr.String(), "error: cli_test: the first fault\nerror: cli_test: the second fault\n",
				"each joined error has its own line")
			assert.Empty(t, stdout.String(), "an error is no result of the command")
		})

		t.Run("skips an empty line of the error", func(t *testing.T) {
			t.Parallel()

			r, _, stderr := rendered(t, text, nil, false)
			r.Error(errors.New("cli_test: the first fault\n\ncli_test: the second fault"))
			assert.Equal(t, stderr.String(), "error: cli_test: the first fault\nerror: cli_test: the second fault\n",
				"the empty line renders nothing")
		})

		t.Run("colours the word error on a terminal", func(t *testing.T) {
			t.Parallel()

			r, _, stderr := rendered(t, text, nil, true)
			r.Error(errors.New("cli_test: the fault"))
			assert.Equal(t, stderr.String(), red+"error"+plain+": cli_test: the fault\n", "the word is red")
		})

		t.Run("writes an error event for each line of the error", func(t *testing.T) {
			t.Parallel()

			r, stdout, stderr := rendered(t, jsonOutput, nil, false)
			r.Error(joined)
			assert.Equal(t, after(stdout),
				`{"event":"error","msg":"cli_test: the first fault","usage":false}`+"\n"+
					`{"event":"error","msg":"cli_test: the second fault","usage":false}`+"\n",
				"each joined error has its own event")
			assert.Empty(t, stderr.String(), "JSON output writes nothing to standard error")
		})

		t.Run("writes an error event with usage set for a UsageError", func(t *testing.T) {
			t.Parallel()

			r, stdout, _ := rendered(t, jsonOutput, nil, false)
			r.Error(&cli.UsageError{Err: errors.New("flag provided but not defined: -x")})
			assert.Equal(t, after(stdout),
				`{"event":"error","msg":"flag provided but not defined: -x","usage":true}`+"\n",
				"the event marks a usage error")
		})

		t.Run("writes the member into an error event", func(t *testing.T) {
			t.Parallel()

			r, stdout, _ := rendered(t, jsonOutput, nil, false)
			r.Workspace(member)
			r.Error(errors.New("cli_test: the fault"))
			assert.Equal(t, after(stdout),
				`{"event":"error","msg":"cli_test: the fault","usage":false,"workspace":"platform"}`+"\n",
				"the event names the member")
		})
	})

	t.Run("End", func(t *testing.T) {
		t.Parallel()

		warning, info := finding, finding
		warning.Severity, info.Severity = diag.SeverityWarning, diag.SeverityInfo

		t.Run("writes the summary event with the count of each severity", func(t *testing.T) {
			t.Parallel()

			r, stdout, _ := rendered(t, jsonOutput, nil, false)
			r.Report(finding)
			r.Report(warning)
			r.Report(info)
			r.End(cli.StatusFailed)
			assert.HasSuffix(t, stdout.String(),
				`{"event":"summary","status":1,`+noFiles+`,`+noPlans+`,"errors":1,"warnings":1,"infos":1}`+"\n",
				"the summary counts the Info that it did not render")
		})

		t.Run("writes the counts of each member of a list", func(t *testing.T) {
			t.Parallel()

			r, stdout, _ := rendered(t, jsonOutput, nil, false)
			r.Workspace(cli.Member{Name: "platform"})
			r.Report(finding)
			r.Workspace(cli.Member{Name: "tools/gen"})
			r.Report(warning)
			r.End(cli.StatusFailed)
			assert.HasSuffix(t, stdout.String(), `{"event":"summary","status":1,`+noFiles+`,`+noPlans+
				`,"errors":1,"warnings":1,"infos":0,"workspaces":[`+
				`{"workspace":"platform",`+noFiles+`,`+noPlans+`,"errors":1,"warnings":0,"infos":0},`+
				`{"workspace":"tools/gen",`+noFiles+`,`+noPlans+`,"errors":0,"warnings":1,"infos":0}]}`+"\n",
				"the summary has the totals and the counts of each member")
		})

		t.Run("continues the counts of a member that the output starts again", func(t *testing.T) {
			t.Parallel()

			r, stdout, _ := rendered(t, jsonOutput, nil, false)
			r.Workspace(cli.Member{Name: "platform"})
			r.Report(finding)
			r.Workspace(cli.Member{Name: "platform"})
			r.Report(finding)
			r.End(cli.StatusFailed)
			assert.HasSuffix(t, stdout.String(), `"workspaces":[{"workspace":"platform",`+noFiles+`,`+noPlans+
				`,"errors":2,"warnings":0,"infos":0}]}`+"\n",
				"the summary has one count of the member")
		})

		t.Run("writes nothing for text output", func(t *testing.T) {
			t.Parallel()

			r, stdout, stderr := rendered(t, text, nil, false)
			r.End(cli.StatusOK)
			assert.Empty(t, stdout.String(), "text output has no summary")
			assert.Empty(t, stderr.String(), "text output has no summary")
		})
	})
}

// rendered begins the output of the command run with the flags f, the
// environment env and the terminal flag terminal. It returns the renderer
// and the buffers of standard output and standard error.
func rendered(
	t *testing.T, f cli.Flags, env map[string]string, terminal bool,
) (*cli.Renderer, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	stdio := cli.IO{
		Stdout:   &stdout,
		Stderr:   &stderr,
		Dir:      t.TempDir(),
		Getenv:   func(key string) string { return env[key] },
		Terminal: terminal,
	}
	return cli.Begin(stdio, f, compose, "run"), &stdout, &stderr
}

// after returns the output after the start event.
func after(stdout *bytes.Buffer) string {
	_, rest, _ := strings.Cut(stdout.String(), "\n")
	return rest
}
