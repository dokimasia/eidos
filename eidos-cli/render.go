// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/workspace"
)

// schemaVersion is the version of the JSON events. A minor version adds
// events or fields, and a major version changes or removes them.
const schemaVersion = "1.0"

// A renderer writes the events with these names itself.
const (
	eventStart   = "start"
	eventDiag    = "diag"
	eventError   = "error"
	eventFile    = "file"
	eventOutcome = "outcome"
	eventSummary = "summary"
)

// noColor is the environment variable that turns colour off when it is not
// empty.
const noColor = "NO_COLOR"

// colourEnd is the ANSI escape code that ends a colour.
const colourEnd = "\x1b[0m"

// colours contains the ANSI escape code of the colour of each severity.
var colours = [...]string{
	diag.SeverityError:   "\x1b[31m",
	diag.SeverityWarning: "\x1b[33m",
	diag.SeverityInfo:    "\x1b[36m",
}

// action is what a run did or would do to a path, in the vocabulary of the
// file event.
type action string

// A file event has one of these actions.
const (
	// actionCreate is a write to a path without a file.
	actionCreate action = "create"
	// actionUpdate is a write over the intact output of the brand, or over a
	// drifted or foreign file that the run may write over.
	actionUpdate action = "update"
	// actionUnchanged is a write of the bytes that the path already has, or
	// a removal that left the path as it was.
	actionUnchanged action = "unchanged"
	// actionStale is the removal of the intact output of the brand that no
	// plan produces.
	actionStale action = "stale"
	// actionDrifted is a write or a removal that the run refused, because
	// the file was edited since its stamp.
	actionDrifted action = "drifted"
	// actionForeign is a write that the run refused, because the path has a
	// file without the frame of the brand.
	actionForeign action = "foreign"
	// actionWithheld is a change that a narrowed run or a prune left for a
	// later run.
	actionWithheld action = "withheld"
)

// fileCounts counts the changes of a workspace by action.
type fileCounts struct {
	Create    int `json:"create"`
	Update    int `json:"update"`
	Unchanged int `json:"unchanged"`
	Stale     int `json:"stale"`
	Drifted   int `json:"drifted"`
	Foreign   int `json:"foreign"`
	Withheld  int `json:"withheld"`
}

// add counts one change with action a.
func (c *fileCounts) add(a action) {
	switch a {
	case actionCreate:
		c.Create++
	case actionUpdate:
		c.Update++
	case actionUnchanged:
		c.Unchanged++
	case actionStale:
		c.Stale++
	case actionDrifted:
		c.Drifted++
	case actionForeign:
		c.Foreign++
	case actionWithheld:
		c.Withheld++
	}
}

// planCounts counts the plans of a workspace by status.
type planCounts struct {
	Committed int `json:"committed"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
	Prepared  int `json:"prepared"`
	Skipped   int `json:"skipped"`
}

// add counts one plan with status s.
func (c *planCounts) add(s workspace.PlanStatus) {
	switch s {
	case workspace.PlanCommitted:
		c.Committed++
	case workspace.PlanFailed:
		c.Failed++
	case workspace.PlanCancelled:
		c.Cancelled++
	case workspace.PlanPrepared:
		c.Prepared++
	case workspace.PlanSkipped:
		c.Skipped++
	}
}

// tally counts the files, the plans and the findings of a workspace.
type tally struct {
	Workspace string     `json:"workspace,omitempty"`
	Files     fileCounts `json:"files"`
	Plans     planCounts `json:"plans"`
	Errors    int        `json:"errors"`
	Warnings  int        `json:"warnings"`
	Infos     int        `json:"infos"`
	// Suppressed counts the findings that a diag directive removed, for
	// each code.
	Suppressed map[string]int `json:"suppressed,omitempty"`
}

// add counts one finding of severity s.
func (t *tally) add(s diag.Severity) {
	switch s {
	case diag.SeverityError:
		t.Errors++
	case diag.SeverityWarning:
		t.Warnings++
	default:
		t.Infos++
	}
}

// line returns the summary line of text output, with the count of the
// files of each action and of the plans of each status.
func (t *tally) line() string {
	f, p := t.Files, t.Plans
	return fmt.Sprintf(
		"files: %d create, %d update, %d unchanged, %d stale, %d drifted, %d foreign, %d withheld; "+
			"plans: %d committed, %d failed, %d cancelled, %d prepared, %d skipped",
		f.Create, f.Update, f.Unchanged, f.Stale, f.Drifted, f.Foreign, f.Withheld,
		p.Committed, p.Failed, p.Cancelled, p.Prepared, p.Skipped,
	)
}

// Renderer writes the output of one command in the format of the [Flags]
// that [Begin] received.
//
// Text output writes findings and errors to standard error, and the events
// of a command to standard output. It colours the severities when the
// [IO] is a terminal, unless --no-color or the variable NO_COLOR is set.
// JSON output writes one event per line to standard output, and nothing to
// standard error. Every position in the output is relative to the root of
// its workspace.
//
// # Concurrency
//
// A Renderer is not safe for concurrent use.
type Renderer struct {
	stdio  IO
	flags  Flags
	colour bool
	// member is the name of the member of a list that the output is about,
	// and empty outside a list.
	member string
	// listed is true once the renderer rendered a file or the outcome of a
	// plan, and text output then ends with a summary line.
	listed bool
	// total counts for the whole command, and members counts for each
	// member of a list. current is the index of the tally of the member
	// that the output is about.
	total   tally
	members []tally
	current int
}

// Begin starts the output of the command with the name command, and returns
// its renderer. Under [FormatJSON], Begin writes the start event. The event
// contains the brand that compose sets, and the path and the version of
// the main module of the binary.
//
// Begin panics unless stdio has both output streams, Getenv and an absolute
// Dir.
func Begin(stdio IO, f Flags, compose Compose, command string) *Renderer {
	stdio.check()
	r := &Renderer{
		stdio:  stdio,
		flags:  f,
		colour: stdio.Terminal && !f.NoColor && stdio.Getenv(noColor) == "",
	}
	if f.Format == FormatJSON {
		start := &startEvent{
			Event: eventStart, Schema: schemaVersion, Command: command, Brand: string(compose().BrandName()),
		}
		if info, ok := debug.ReadBuildInfo(); ok {
			start.Binary = binary{Path: info.Main.Path, Version: info.Main.Version}
		}
		r.emit(start)
	}
	return r
}

// Workspace starts the output of the member m of a list. The events that
// follow have the name of m in their field workspace, and the summary
// counts the files, the plans and the findings of m on their own. Text
// output puts the name of m before each path. A second call for one member
// continues its counts.
func (r *Renderer) Workspace(m Member) {
	r.member = m.Name
	r.current = slices.IndexFunc(r.members, func(t tally) bool { return t.Workspace == m.Name })
	if r.current < 0 {
		r.current = len(r.members)
		r.members = append(r.members, tally{Workspace: m.Name})
	}
}

// Report renders one finding, and counts it for the summary. Text output
// writes the finding to standard error as one line, followed by one
// indented line for each related position. JSON output writes a diag
// event. Report renders an Info only under --verbose, and counts it in
// both cases.
func (r *Renderer) Report(d diag.Diag) {
	r.report(d, false)
}

// Event renders one event of a command. JSON output writes the event with
// the field event set to name, the field workspace of a member of a list,
// and then the fields of v. Text output writes text as one line to standard
// output, and nothing when text is empty. The caller keeps the fields event
// and workspace out of v.
//
// Event panics when v does not encode as a JSON object, which is a defect
// of the command.
func (r *Renderer) Event(name string, v any, text string) {
	if r.flags.Format != FormatJSON {
		if text != "" {
			fmt.Fprintln(r.stdio.Stdout, text)
		}
		return
	}
	body, err := json.Marshal(v)
	if err != nil || body[0] != '{' {
		panic(fmt.Sprintf("cli: the event %q does not encode as a JSON object", name))
	}
	// A header has two strings, which always encode.
	line, _ := json.Marshal(&header{Event: name, Workspace: r.member})
	line = line[:len(line)-1]
	if len(body) > len("{}") {
		line = append(append(line, ','), body[1:]...)
	} else {
		line = append(line, '}')
	}
	fmt.Fprintf(r.stdio.Stdout, "%s\n", line)
}

// Error renders a failure without a position, such as a usage error, a
// config error or an I/O error. Error renders each line of the text of err
// on its own, so each error that err joins renders on its own. Text output
// writes the word error and the line to standard error. JSON output writes
// an error event for each line. The field usage of the event is true when
// err wraps a [*UsageError].
func (r *Renderer) Error(err error) {
	_, usage := errors.AsType[*UsageError](err)
	for line := range strings.Lines(err.Error()) {
		msg := strings.TrimSuffix(line, "\n")
		if msg == "" {
			continue
		}
		if r.flags.Format == FormatJSON {
			r.emit(&errorEvent{Event: eventError, Msg: msg, Usage: usage, Workspace: r.member})
			continue
		}
		fmt.Fprintf(r.stdio.Stderr, "%s: %s\n", r.paint(diag.SeverityError), msg)
	}
}

// End ends the output of the command. status is the exit status of the
// command. JSON output writes the summary event with status and with the
// counts of the files, the plans and the findings, in total and for each
// member of a list. Text output writes the summary line of the files and
// the plans, when the command rendered any.
func (r *Renderer) End(status int) {
	if r.flags.Format != FormatJSON {
		if r.listed {
			fmt.Fprintln(r.stdio.Stdout, r.total.line())
		}
		return
	}
	r.emit(&summaryEvent{Event: eventSummary, Status: status, tally: r.total, Workspaces: r.members})
}

// report renders one finding and counts it. It renders an Info only under
// --verbose, unless always is true.
func (r *Renderer) report(d diag.Diag, always bool) {
	r.count(func(t *tally) { t.add(d.Severity) })
	if d.Severity == diag.SeverityInfo && !r.flags.Verbose && !always {
		return
	}
	if r.flags.Format == FormatJSON {
		r.emit(r.diagOf(d))
		return
	}
	fmt.Fprintf(r.stdio.Stderr, "%s: %s %s: %s (%s)\n", r.place(d.Pos), r.paint(d.Severity), d.Code, d.Msg, d.Origin)
	for _, p := range d.Related {
		fmt.Fprintf(r.stdio.Stderr, "    related: %s\n", r.place(p))
	}
}

// diagOf returns the diag event of a finding.
func (r *Renderer) diagOf(d diag.Diag) *diagEvent {
	return &diagEvent{Event: eventDiag, finding: findingOf(d), Workspace: r.member}
}

// file renders one change of a plan with action a, and counts it. plan is
// empty for a removal of the sweep. Text output writes the action and the
// path to standard output, and nothing for an unchanged path.
func (r *Renderer) file(plan string, c output.Change, a action) {
	r.listed = true
	r.count(func(t *tally) { t.Files.add(a) })
	if r.flags.Format == FormatJSON {
		r.emit(&fileEvent{
			Event: eventFile, Path: c.Path, Plan: plan, Action: a, Found: c.Found.String(), Hash: c.Hash,
			Workspace: r.member,
		})
		return
	}
	if a != actionUnchanged {
		fmt.Fprintf(r.stdio.Stdout, "%s %s\n", a, r.place(position.Pos{File: c.Path}))
	}
}

// outcome renders the status of one plan, and counts it. Text output writes
// nothing, and the summary line counts the status.
func (r *Renderer) outcome(plan string, s workspace.PlanStatus) {
	r.listed = true
	r.count(func(t *tally) { t.Plans.add(s) })
	if r.flags.Format == FormatJSON {
		r.emit(&outcomeEvent{Event: eventOutcome, Plan: plan, Status: s.String(), Workspace: r.member})
	}
}

// suppressed counts the findings that the diag directives of a run removed,
// for each code.
func (r *Renderer) suppressed(removed map[diag.Code]int) {
	for code, n := range removed {
		r.count(func(t *tally) {
			if t.Suppressed == nil {
				t.Suppressed = map[string]int{}
			}
			t.Suppressed[code.String()] += n
		})
	}
}

// count applies add to the total and to the tally of the current member of
// a list.
func (r *Renderer) count(add func(t *tally)) {
	add(&r.total)
	if len(r.members) > 0 {
		add(&r.members[r.current])
	}
}

// emit writes the JSON encoding of an event of the renderer as one line to
// standard output. The events of the renderer have only fields that
// encode.
func (r *Renderer) emit(event any) {
	line, _ := json.Marshal(event)
	fmt.Fprintf(r.stdio.Stdout, "%s\n", line)
}

// place returns the text of position p in text output. It puts the name of
// the member before the path, so the path is relative to the directory of
// the list.
func (r *Renderer) place(p position.Pos) string {
	if r.member == "" {
		return spot(p)
	}
	return r.member + "/" + spot(p)
}

// paint returns the name of severity s, in the colour of s when the output
// has colour.
func (r *Renderer) paint(s diag.Severity) string {
	if !r.colour {
		return s.String()
	}
	return colours[s] + s.String() + colourEnd
}

// binary is the main module of the running binary.
type binary struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

// startEvent is the first event of a command.
type startEvent struct {
	Event   string `json:"event"`
	Schema  string `json:"schema"`
	Command string `json:"command"`
	Brand   string `json:"brand"`
	Binary  binary `json:"binary"`
}

// finding is the JSON form of one finding.
type finding struct {
	Code     string   `json:"code"`
	Severity string   `json:"severity"`
	Pos      string   `json:"pos"`
	Msg      string   `json:"msg"`
	Origin   string   `json:"origin"`
	Related  []string `json:"related,omitempty"`
}

// diagEvent is the event of one finding.
type diagEvent struct {
	Event string `json:"event"`
	finding
	Workspace string `json:"workspace,omitempty"`
}

// errorEvent is the event of one line of a failure without a position.
type errorEvent struct {
	Event     string `json:"event"`
	Msg       string `json:"msg"`
	Usage     bool   `json:"usage"`
	Workspace string `json:"workspace,omitempty"`
}

// fileEvent is the event of one change of a file.
type fileEvent struct {
	Event string `json:"event"`
	Path  string `json:"path"`
	// Plan is empty for a removal of the sweep, whose plan the composition
	// no longer declares.
	Plan      string `json:"plan,omitempty"`
	Action    action `json:"action"`
	Found     string `json:"found"`
	Hash      string `json:"hash,omitempty"`
	Workspace string `json:"workspace,omitempty"`
}

// outcomeEvent is the event of the status of one plan.
type outcomeEvent struct {
	Event     string `json:"event"`
	Plan      string `json:"plan"`
	Status    string `json:"status"`
	Workspace string `json:"workspace,omitempty"`
}

// header contains the fields that the event of a command starts with.
type header struct {
	Event     string `json:"event"`
	Workspace string `json:"workspace,omitempty"`
}

// summaryEvent is the last event of a command. Its counts are the totals of
// the command, and Workspaces contains the counts of each member of a list.
type summaryEvent struct {
	Event  string `json:"event"`
	Status int    `json:"status"`
	tally
	Workspaces []tally `json:"workspaces,omitempty"`
}

// findingOf returns the JSON form of the finding d.
func findingOf(d diag.Diag) finding {
	related := make([]string, 0, len(d.Related))
	for _, p := range d.Related {
		related = append(related, spot(p))
	}
	return finding{
		Code: d.Code.String(), Severity: d.Severity.String(), Pos: spot(d.Pos), Msg: d.Msg,
		Origin: string(d.Origin), Related: related,
	}
}

// spot returns the text of position p: the file, the line and the column,
// separated by colons. It leaves out a column of 0, and a line of 0.
func spot(p position.Pos) string {
	if p.Line == 0 {
		return p.File
	}
	s := p.File + ":" + strconv.Itoa(p.Line)
	if p.Col == 0 {
		return s
	}
	return s + ":" + strconv.Itoa(p.Col)
}
