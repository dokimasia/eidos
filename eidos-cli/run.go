// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/workspace"
)

// The name and the help text of the command run.
const (
	runName = "run"
	runHelp = "Generates the output of each workspace and commits it."
)

// The flags of run that a check of --stdout names.
const (
	stdoutFlag = "stdout"
	dryRunFlag = "dry-run"
	checkFlag  = "check"
)

// runOptions are the options of run.
type runOptions struct {
	// Plans are the plans that the run runs with the plans that they depend
	// on. Each repeated flag adds its values, and --plan web,cli adds two.
	Plans []string `help:"Run this plan and the plans that it depends on." name:"plan"`

	DryRun         bool `help:"Run every phase and commit nothing."`
	Check          bool `help:"Commit nothing and report each file that the run would change as an error."`
	OverwriteDrift bool `help:"Write over a generated file that was edited since its stamp."`
	Adopt          bool `help:"Write over a file without the frame of the brand."`
	Cold           bool `help:"Run without the state that the last run recorded."`
	Stdout         bool `help:"Record nothing and write the files to standard output."`
	// Patterns are the patterns of the run, and empty for a run without
	// patterns.
	Patterns []string `arg:"" default:"" help:"Patterns of the packages to run. Each is relative to the working directory."`
}

// run is the handler of run, which runs each workspace once. It refuses
// --stdout with --format=json, --dry-run or --check.
func (k kernels) run(ctx context.Context, run *gocli.Run[Global, runOptions]) error {
	o := run.Config()
	return execute(ctx, k.compose, run, o.Patterns, func(ctx context.Context, x *invocation) int {
		if o.Stdout {
			switch {
			case x.global.Format == FormatJSON:
				return x.usage(fmt.Errorf("the flags --%s and --%s=%s exclude each other",
					stdoutFlag, formatFlag, FormatJSON))
			case o.DryRun:
				return x.usage(fmt.Errorf("the flags --%s and --%s exclude each other", stdoutFlag, dryRunFlag))
			case o.Check:
				return x.usage(fmt.Errorf("the flags --%s and --%s exclude each other", stdoutFlag, checkFlag))
			}
			x.stream = x.stdio.Stdout
		}
		return x.runEach(ctx, workspace.Input{
			Plans: o.Plans, Dry: o.DryRun, Check: o.Check, OverwriteDrift: o.OverwriteDrift, Adopt: o.Adopt,
			Cold: o.Cold,
		})
	})
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
	found, err := open(x.stdio, x.global, x.compose, x.stream)
	if err != nil {
		x.r.Error(err)
		return StatusUsage
	}
	patterns, err := assign(found.dir, found.members, x.patterns)
	if err != nil {
		return x.usage(err)
	}
	status := StatusOK
	for i, m := range found.members {
		if len(x.patterns) > 0 && len(patterns[i]) == 0 {
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
// [StatusFailed] for an error of the run, such as the error of a store
// locator.
func (x *invocation) runOnce(ctx context.Context, m Member, in workspace.Input) int {
	report, err := m.Workspace.Run(ctx, x.input(m, in))
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
// tree at the member's root, the environment of the store locators, the
// strict mode and the path of the command as the caller. run, watch and
// doctor run each workspace with it. The load of the run calls the locator
// of a frontend only when the tree has an input file of the frontend.
func (x *invocation) input(m Member, in workspace.Input) workspace.Input {
	in.Tree, in.Getenv, in.Strict, in.Caller = os.DirFS(m.Root), x.stdio.Getenv, x.global.Strict, x.command
	return in
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
