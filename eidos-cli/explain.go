// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The events of explain.
const (
	eventExplain       = "explain"
	eventExplainFile   = "explain.file"
	eventExplainClaim  = "explain.claim"
	eventExplainRecord = "explain.record"
)

// generationOf is the JSON form of the generation that an explanation
// reads. Recorded is a time in RFC 3339 form.
type generationOf struct {
	Generation string `json:"generation"`
	Recorded   string `json:"recorded"`
}

// matchOf is the JSON form of the match of an invocation. Subject is empty
// for a match without a subject.
type matchOf struct {
	Plugin   string `json:"plugin"`
	Rule     int    `json:"rule"`
	Subject  string `json:"subject,omitempty"`
	Instance int    `json:"instance,omitempty"`
}

// unitOf is the JSON form of the key unit of a group.
type unitOf struct {
	Plugin  string `json:"plugin"`
	Tag     string `json:"tag,omitempty"`
	Package string `json:"package"`
	Key     string `json:"key"`
}

// readOf is the JSON form of one read of a record that Explain identified.
type readOf struct {
	Grain     string `json:"grain"`
	Subject   string `json:"subject,omitempty"`
	Key       string `json:"key,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Directive string `json:"directive,omitempty"`
	Plan      string `json:"plan,omitempty"`
}

// pointRead is the JSON form of one read from which a claim derives. It is
// a read of a fact when Key is set, and a read of a declaration otherwise.
type pointRead struct {
	Subject string `json:"subject"`
	Key     string `json:"key,omitempty"`
}

// explainedFile is the JSON form of one generated file of an explanation.
type explainedFile struct {
	Path         string    `json:"path"`
	Plan         string    `json:"plan"`
	Hash         string    `json:"hash"`
	Plugins      []string  `json:"plugins"`
	Sources      []string  `json:"sources"`
	Contributors []matchOf `json:"contributors"`
	Findings     []finding `json:"findings,omitempty"`
}

// explainedClaim is the JSON form of one claim on a fact. Pos is empty for
// the stamp of a plugin, which has no carrier.
type explainedClaim struct {
	Key       string      `json:"key"`
	Subject   string      `json:"subject"`
	Value     any         `json:"value"`
	Winner    bool        `json:"winner"`
	Authority string      `json:"authority"`
	Bucket    int         `json:"bucket"`
	Plugin    string      `json:"plugin"`
	Pos       string      `json:"pos,omitempty"`
	Derived   []pointRead `json:"derived,omitempty"`
}

// explainedRecord is the JSON form of the record of one execution. Match is
// set for an invocation, and Group for a group.
type explainedRecord struct {
	Kind         string    `json:"kind"`
	Plan         string    `json:"plan,omitempty"`
	Match        *matchOf  `json:"match,omitempty"`
	Check        string    `json:"check,omitempty"`
	Group        *unitOf   `json:"group,omitempty"`
	Subject      string    `json:"subject,omitempty"`
	Findings     []finding `json:"findings,omitempty"`
	Reads        []readOf  `json:"reads,omitempty"`
	Unidentified int       `json:"unidentified"`
}

// explainCommand returns the command explain, which prints what the live
// generation records about one target.
func explainCommand(compose Compose) *kernel {
	return &kernel{
		name:     "explain",
		synopsis: "Prints what the last run recorded about a file, a declaration, a fact or a finding.",
		form:     "<target>",
		compose:  compose,
		define: func(*flag.FlagSet) runner {
			return func(ctx context.Context, x *invocation) int { return x.explain(ctx) }
		},
	}
}

// explain explains the one target of the invocation in each workspace. A
// path and the file of a position are relative to the working directory,
// and explain rewrites them relative to the root of each workspace. explain
// skips a workspace whose root does not contain them.
//
// explain returns [StatusUsage] for these errors:
//
//   - a config error
//   - a number of arguments other than one
//   - a target that does not parse
//   - a path outside the root of every workspace
//
// It returns [StatusFailed] when Explain returns an error, such as
// [workspace.ErrNoGeneration].
func (x *invocation) explain(ctx context.Context) int {
	if len(x.args) != 1 {
		return x.usage(fmt.Errorf("the command takes one target, and has %d arguments", len(x.args)))
	}
	found, err := open(x.stdio, x.flags, x.k.compose)
	if err != nil {
		x.r.Error(err)
		return StatusUsage
	}
	status, placed := StatusOK, false
	for _, m := range found.members {
		t, err := m.Workspace.ParseTarget(x.args[0])
		if err != nil {
			return x.usage(err)
		}
		if !relocate(&t, found.dir, m.Root) {
			continue
		}
		placed = true
		if m.Name != "" {
			x.r.Workspace(m)
		}
		explanation, err := m.Workspace.Explain(ctx, t)
		if err != nil {
			x.r.Error(err)
			status = StatusFailed
			continue
		}
		x.explanation(explanation)
	}
	if !placed {
		return x.usage(fmt.Errorf("the target %s is outside the root of every workspace", x.args[0]))
	}
	return status
}

// explanation renders the generation of an explanation, then each file,
// each claim and each record.
func (x *invocation) explanation(e *workspace.Explanation) {
	recorded := e.Recorded.UTC().Format(time.RFC3339)
	x.r.Event(eventExplain, &generationOf{Generation: e.Generation, Recorded: recorded},
		"generation "+e.Generation+", recorded "+recorded)
	for _, f := range e.Files {
		out := &explainedFile{
			Path: f.Entry.Path, Plan: f.Entry.Plan, Hash: f.Entry.Hash, Sources: f.Entry.Sources,
			Contributors: make([]matchOf, 0, len(f.Contributors)), Findings: findingsOf(f.Findings),
		}
		for _, p := range f.Entry.Plugins {
			out.Plugins = append(out.Plugins, string(p))
		}
		lines := []string{
			"file " + out.Path + ": " + fields("plan", out.Plan, "plugins", strings.Join(out.Plugins, " ")),
		}
		for _, c := range f.Contributors {
			m := matchOf{
				Plugin: string(c.Plugin), Rule: int(c.Rule), Subject: identityOf(c.Subject), Instance: c.Instance,
			}
			out.Contributors = append(out.Contributors, m)
			lines = append(lines, "  contributor "+matchText(&m))
		}
		for _, s := range out.Sources {
			lines = append(lines, "  source "+s)
		}
		x.r.Event(eventExplainFile, out, strings.Join(append(lines, findingsText(out.Findings)...), "\n"))
	}
	for _, c := range e.Claims {
		out := &explainedClaim{
			Key: string(c.Key), Subject: c.Claim.Subject.String(), Value: c.Value, Winner: c.Winner,
			Authority: c.Claim.Authority.String(), Bucket: c.Claim.Bucket, Plugin: string(c.Claim.Plugin),
			Pos: placeOf(c.Claim.Pos),
		}
		value := fmt.Sprint(out.Value)
		if out.Value == nil {
			value = "dropped"
		}
		lines := []string{fmt.Sprintf("claim %s on %s: %s", out.Key, out.Subject, fields(
			"value", value, "winner", strconv.FormatBool(out.Winner), "authority", out.Authority,
			"plugin", out.Plugin, "bucket", strconv.Itoa(out.Bucket), "at", out.Pos,
		))}
		for _, r := range c.Claim.Derived {
			read := pointRead{Subject: r.Subject.String(), Key: string(r.Key)}
			out.Derived = append(out.Derived, read)
			lines = append(lines, "  derived from "+fields("subject", read.Subject, "key", read.Key))
		}
		x.r.Event(eventExplainClaim, out, strings.Join(lines, "\n"))
	}
	for _, r := range e.Records {
		x.record(r)
	}
}

// record renders the record of one execution.
func (x *invocation) record(r workspace.ExplainedRecord) {
	out := &explainedRecord{
		Kind: r.Kind.String(), Plan: r.Plan, Check: string(r.Check), Subject: identityOf(r.Subject),
		Findings: findingsOf(r.Findings), Unidentified: r.Unidentified,
	}
	if m := r.Match; m.Plugin != "" {
		out.Match = &matchOf{
			Plugin: string(m.Plugin), Rule: int(m.Rule), Subject: identityOf(m.Subject), Instance: m.Instance,
		}
	}
	if r.Group.Plugin != "" {
		out.Group = &unitOf{
			Plugin: string(r.Group.Plugin), Tag: r.Group.Tag, Package: r.Group.Pkg.String(), Key: r.Group.Key,
		}
	}
	group := ""
	if out.Group != nil {
		group = out.Group.Key
	}
	lines := []string{fmt.Sprintf("record %s: %s; %d reads, %d unidentified", out.Kind, fields(
		"plan", out.Plan, "match", matchText(out.Match), "check", out.Check, "group", group, "subject", out.Subject,
	), len(r.Reads), r.Unidentified)}
	for _, read := range r.Reads {
		kind := ""
		if read.Grain == workspace.ReadKind {
			kind = read.Kind.String()
		}
		of := readOf{
			Grain: read.Grain.String(), Subject: identityOf(read.Subject), Key: string(read.Key),
			Kind: kind, Directive: string(read.Directive), Plan: read.Plan,
		}
		out.Reads = append(out.Reads, of)
		line := "  read " + of.Grain
		if name := cmp.Or(of.Key, of.Kind, of.Directive, of.Plan); name != "" {
			line += " " + name
		}
		if of.Subject != "" {
			line += " of " + of.Subject
		}
		lines = append(lines, line)
	}
	x.r.Event(eventExplainRecord, out, strings.Join(append(lines, findingsText(out.Findings)...), "\n"))
}

// relocate rewrites the path of t and the file of its position from the
// working directory dir to the root root, with slashes. It reports whether
// root contains them. A target without a path and without a position
// belongs to every root.
func relocate(t *workspace.Target, dir, root string) bool {
	for _, p := range []*string{&t.Path, &t.At.File} {
		if *p == "" {
			continue
		}
		at := resolve(dir, *p)
		if !inside(root, at) {
			return false
		}
		rel, _ := filepath.Rel(root, at)
		*p = filepath.ToSlash(rel)
	}
	return true
}

// matchText returns the text of the match of an invocation, and the empty
// string for no match.
func matchText(m *matchOf) string {
	if m == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%s rule %d %s", m.Plugin, m.Rule, m.Subject))
}

// findingsOf returns the JSON form of the findings of an explanation.
func findingsOf(findings []diag.Diag) []finding {
	out := make([]finding, 0, len(findings))
	for _, d := range findings {
		out = append(out, findingOf(d))
	}
	return out
}

// findingsText returns one indented line for each finding of an
// explanation.
func findingsText(findings []finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, fmt.Sprintf("  %s: %s %s: %s (%s)", f.Pos, f.Severity, f.Code, f.Msg, f.Origin))
	}
	return out
}

// fields joins the label and the value of each pair whose value is not
// empty, as in "plan mirrors, check lister". pairs alternates labels and
// values.
func fields(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] != "" {
			parts = append(parts, pairs[i]+" "+pairs[i+1])
		}
	}
	return strings.Join(parts, ", ")
}

// identityOf returns the text of an identity, and the empty string for the
// zero identity.
func identityOf(id symbol.Identity) string {
	if id.IsZero() {
		return ""
	}
	return id.String()
}

// placeOf returns the text of a position, and the empty string for the zero
// position.
func placeOf(p position.Pos) string {
	if p.IsZero() {
		return ""
	}
	return spot(p)
}
