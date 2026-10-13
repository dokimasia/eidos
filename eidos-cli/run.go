// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
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

// The flags of run that a check of --stdout names.
const (
	stdoutFlag = "stdout"
	dryRunFlag = "dry-run"
	checkFlag  = "check"
)

// runCommand returns the command run, which runs each workspace once.
func runCommand(compose Compose) *kernel {
	return &kernel{
		name:     "run",
		synopsis: "Generates the output of each workspace and commits it.",
		form:     "[<pattern>...]",
		compose:  compose,
		define: func(fs *flag.FlagSet) runner {
			var (
				plans                                  names
				dry, check, drift, adopt, cold, stdout bool
			)
			fs.Var(&plans, "plan",
				"run this `plan` and the plans that it depends on, and repeat the flag for more plans")
			fs.BoolVar(&dry, dryRunFlag, false, "run every phase and commit nothing")
			fs.BoolVar(&check, checkFlag, false,
				"report each file that the run would change as an error, and commit nothing")
			fs.BoolVar(&drift, "overwrite-drift", false, "write over a generated file that was edited since its stamp")
			fs.BoolVar(&adopt, "adopt", false, "write over a file without the frame of the brand")
			fs.BoolVar(&cold, "cold", false, "run without the state that the last run recorded")
			fs.BoolVar(&stdout, stdoutFlag, false,
				"write the files to standard output in place of the workspace, and record nothing")
			return func(ctx context.Context, x *invocation) int {
				if stdout {
					switch {
					case x.flags.Format == FormatJSON:
						return x.usage(fmt.Errorf("the flags --%s and --%s=%s exclude each other",
							stdoutFlag, formatFlag, FormatJSON))
					case dry:
						return x.usage(fmt.Errorf("the flags --%s and --%s exclude each other", stdoutFlag, dryRunFlag))
					case check:
						return x.usage(fmt.Errorf("the flags --%s and --%s exclude each other", stdoutFlag, checkFlag))
					}
					x.stream = x.stdio.Stdout
				}
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
// a workspace without one of them does not run. Each workspace writes its
// files to the stream of the invocation where it has one.
//
// runEach returns [StatusUsage] for a config error, for a pattern outside
// the root of every workspace and for an input that a run refuses. It
// returns [StatusFailed] when a run fails, and [StatusOK] otherwise.
func (x *invocation) runEach(ctx context.Context, in workspace.Input) int {
	found, err := open(x.stdio, x.flags, x.k.compose, x.stream)
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
			x.r.Member(m)
		}
		in.Patterns = patterns[i]
		status = max(status, x.runOnce(ctx, m, in))
	}
	return status
}

// runOnce runs the workspace of m with the input in, and renders the
// report.
//
// runOnce returns [StatusUsage] for an input that the run refuses, and
// [StatusFailed] for an error of the stores or of the run.
func (x *invocation) runOnce(ctx context.Context, m Member, in workspace.Input) int {
	in, err := x.input(m, in)
	if err != nil {
		x.r.Error(err)
		return StatusFailed
	}
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

// input completes the input of a run of the workspace of m. It sets the
// tree at the member's root, the stores, the strict mode and the caller.
// run, watch and doctor run each workspace with it.
//
// Error modes: input returns the error of the stores.
func (x *invocation) input(m Member, in workspace.Input) (workspace.Input, error) {
	stores, err := m.Workspace.Stores(x.stdio.Getenv)
	if err != nil {
		return in, err
	}
	in.Tree, in.Stores, in.Strict, in.Caller = os.DirFS(m.Root), stores, x.flags.Strict, x.caller()
	return in, nil
}

// render renders the findings of a run and the counts of the suppressed
// findings, then the changes and the outcome of each plan and the removals
// of the sweep. A run that writes to the stream of the invocation renders
// no change, outcome or removal, because the stream contains the files.
func (x *invocation) render(report *workspace.Report) {
	for d := range report.Sink.All() {
		x.r.Report(d)
	}
	x.r.suppressed(report.Suppressed)
	if x.stream != nil {
		return
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
			rel, isBelow := below(m.Root, at)
			if !isBelow {
				continue
			}
			out[i] = append(out[i], rel)
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
