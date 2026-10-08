// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"flag"
	"os"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/workspace"
)

// doctored contains the codes whose findings doctor renders at every
// severity, Infos included. A run reports these codes for a sealed state
// that the next run ignores, for a diag directive that does not remove a
// finding and for a deprecated directive.
var doctored = map[diag.Code]bool{
	workspace.ColdState:           true,
	workspace.UnusedSuppression:   true,
	directive.DeprecatedDirective: true,
}

// doctorCommand returns the command doctor, which checks the config, the
// sealed state and the directives of each workspace. The command writes
// nothing but the lock file.
func doctorCommand(compose Compose) *kernel {
	return &kernel{
		name:     "doctor",
		synopsis: "Checks the config, the sealed state and the directives of each workspace.",
		compose:  compose,
		define: func(*flag.FlagSet) runner {
			return func(ctx context.Context, x *invocation) int { return x.doctor(ctx) }
		},
	}
}

// doctor reports the config errors of the workspaces, and makes a dry run of
// each workspace that builds. It renders every Error and Warning of a dry
// run, and the findings of the codes that doctored contains at every
// severity. The dry run reports a ColdState Info for an executable that does
// not read, and for a generation of another format, composition or
// executable.
//
// doctor returns [StatusFailed] when it reports a config error, an error of
// a run or an Error finding, and [StatusOK] otherwise.
func (x *invocation) doctor(ctx context.Context) int {
	found, err := open(x.stdio, x.flags, x.k.compose)
	status := StatusOK
	if err != nil {
		x.r.Error(err)
		status = StatusFailed
	}
	for _, m := range found.members {
		if m.Name != "" {
			x.r.Workspace(m)
		}
		stores, err := m.Workspace.Stores(x.stdio.Getenv)
		if err != nil {
			x.r.Error(err)
			status = StatusFailed
			continue
		}
		// A dry run over a tree without plans or patterns has no input that
		// the run refuses, so the report is never nil.
		report, err := m.Workspace.Run(ctx, workspace.Input{
			Tree: os.DirFS(m.Root), Stores: stores, Dry: true, Strict: x.flags.Strict, Caller: x.caller(),
		})
		for d := range report.Sink.All() {
			x.r.report(d, doctored[d.Code])
		}
		if err != nil {
			status = StatusFailed
		}
		if failure := unreported(err); failure != nil {
			x.r.Error(failure)
		}
	}
	return status
}
