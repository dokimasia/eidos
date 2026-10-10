// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/workspace"
)

// names is a flag that collects its values in order, such as --plan.
type names []string

var _ flag.Value = (*names)(nil)

// String returns the values, separated by commas.
func (n *names) String() string { return strings.Join(*n, ",") }

// Set appends the value s. It returns no error.
func (n *names) Set(s string) error {
	*n = append(*n, s)
	return nil
}

// runCommand returns the command run, which runs each workspace once.
func runCommand(compose Compose) *kernel {
	return &kernel{
		name:     "run",
		synopsis: "Generates the output of each workspace and commits it.",
		form:     "[<pattern>...]",
		compose:  compose,
		define: func(fs *flag.FlagSet) runner {
			var (
				plans                          names
				dry, check, drift, adopt, cold bool
			)
			fs.Var(&plans, "plan",
				"run this `plan` and the plans that it depends on, and repeat the flag for more plans")
			fs.BoolVar(&dry, "dry-run", false, "run every phase and commit nothing")
			fs.BoolVar(&check, "check", false,
				"report each file that the run would change as an error, and commit nothing")
			fs.BoolVar(&drift, "overwrite-drift", false, "write over a generated file that was edited since its stamp")
			fs.BoolVar(&adopt, "adopt", false, "write over a file without the frame of the brand")
			fs.BoolVar(&cold, "cold", false, "run without the state that the last run recorded")
			return func(ctx context.Context, x *invocation) int {
				return x.runEach(ctx, workspace.Input{
					Plans: plans, Dry: dry, Check: check, OverwriteDrift: drift, Adopt: adopt, Cold: cold,
				})
			}
		},
	}
}

// runEach runs each workspace of the invocation once with the input in,
// and renders each report. A pattern is relative to the working directory,
// and each workspace runs with the patterns inside its root. With patterns,
// a workspace without one of them does not run.
//
// runEach returns [StatusUsage] for a config error, for a pattern outside
// the root of every workspace and for an input that a run refuses. It
// returns [StatusFailed] when a run fails, and [StatusOK] otherwise.
func (x *invocation) runEach(ctx context.Context, in workspace.Input) int {
	found, err := open(x.stdio, x.flags, x.k.compose)
	if err != nil {
		x.r.Error(err)
		return StatusUsage
	}
	patterns, err := assign(found.dir, found.members, x.args)
	if err != nil {
		return x.usage(err)
	}
	status := StatusOK
	for i, m := range found.members {
		if len(x.args) > 0 && len(patterns[i]) == 0 {
			continue
		}
		if m.Name != "" {
			x.r.Workspace(m)
		}
		in.Patterns = patterns[i]
		status = max(status, x.runOnce(ctx, m, in))
	}
	return status
}

// runOnce runs the workspace of m with the input in, and renders the
// report. It sets the tree, the stores, the strict mode and the caller of
// the input.
//
// runOnce returns [StatusUsage] for an input that the run refuses, and
// [StatusFailed] for an error of the stores or of the run.
func (x *invocation) runOnce(ctx context.Context, m Member, in workspace.Input) int {
	stores, err := m.Workspace.Stores(x.stdio.Getenv)
	if err != nil {
		x.r.Error(err)
		return StatusFailed
	}
	in.Tree, in.Stores, in.Strict, in.Caller = os.DirFS(m.Root), stores, x.flags.Strict, x.caller()
	report, err := m.Workspace.Run(ctx, in)
	if report == nil {
		return x.usage(err)
	}
	x.render(report)
	if err == nil {
		return StatusOK
	}
	if failure := unreported(err); failure != nil {
		x.r.Error(failure)
	}
	return StatusFailed
}

// render renders the findings of a run, the changes and the outcome of
// each plan, the removals of the sweep and the counts of the suppressed
// findings.
func (x *invocation) render(report *workspace.Report) {
	for d := range report.Sink.All() {
		x.r.Report(d)
	}
	for _, p := range report.Plans {
		for _, c := range p.Changes {
			x.r.file(p.Name, c, changed(c))
		}
		for _, c := range p.Refused {
			x.r.file(p.Name, c, refused(c))
		}
		for _, c := range p.Withheld {
			if c.Action != output.ActionUnchanged {
				x.r.file(p.Name, c, actionWithheld)
			}
		}
		x.r.outcome(p.Name, p.Status)
	}
	for _, w := range report.Swept {
		x.r.file("", output.Change{Path: w.Path, Action: w.Action, Found: output.FoundIntact}, actionStale)
	}
	x.r.suppressed(report.Suppressed)
}

// changed returns the action of a change that a plan committed or
// prepared. A removal that kept a file edited since its stamp is drifted.
func changed(c output.Change) action {
	switch {
	case c.Action == output.ActionCreated:
		return actionCreate
	case c.Action == output.ActionUpdated:
		return actionUpdate
	case c.Action == output.ActionDeleted:
		return actionStale
	case c.Found == output.FoundDrifted:
		return actionDrifted
	default:
		return actionUnchanged
	}
}

// refused returns the action of a write that a plan refused. The action is
// drifted for a file edited since its stamp, and foreign for a file without
// the frame of the brand.
func refused(c output.Change) action {
	if c.Found == output.FoundDrifted {
		return actionDrifted
	}
	return actionForeign
}

// assign returns the patterns of each member. A pattern of args is relative
// to the working directory dir. assign gives each member the patterns
// inside its root, relative to the root and with slashes, such as svc/store
// or svc/....
//
// Error modes: assign returns an error for a pattern outside the root of
// every member.
func assign(dir string, members []Member, args []string) ([][]string, error) {
	out := make([][]string, len(members))
	for _, arg := range args {
		at := resolve(dir, arg)
		placed := false
		for i, m := range members {
			if !inside(m.Root, at) {
				continue
			}
			rel, _ := filepath.Rel(m.Root, at)
			out[i] = append(out[i], filepath.ToSlash(rel))
			placed = true
		}
		if !placed {
			return nil, fmt.Errorf("the pattern %s is outside the root of every workspace", arg)
		}
	}
	return out, nil
}

// unreported returns the error of a run without [workspace.ErrRunFailed],
// and nil when err joins no other error. A run returns ErrRunFailed for an
// Error finding of its report, and the output renders those findings
// already. unreported keeps every other error that err joins, at any depth
// of the joins.
func unreported(err error) error {
	joined, isJoin := err.(interface{ Unwrap() []error })
	if !isJoin {
		if errors.Is(err, workspace.ErrRunFailed) {
			return nil
		}
		return err
	}
	errs := joined.Unwrap()
	rest := make([]error, 0, len(errs))
	for _, e := range errs {
		rest = append(rest, unreported(e))
	}
	return errors.Join(rest...)
}
