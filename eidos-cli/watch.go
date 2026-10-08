// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
	"time"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/workspace"
)

// watch pauses for defaultInterval between two passes by default. A pause
// shorter than minInterval is a usage error.
const (
	defaultInterval = time.Second
	minInterval     = 100 * time.Millisecond
)

// pass is the result of one pass of watch over one workspace. It has the
// findings of the run, and the text of the run error that no finding
// reports.
type pass struct {
	findings []diag.Diag
	failure  string
}

// same reports whether two passes saw the same findings and the same error.
// It compares two findings with [diag.Diag.Compare] and by their severity.
func (p pass) same(o pass) bool {
	return p.failure == o.failure && slices.EqualFunc(p.findings, o.findings, func(a, b diag.Diag) bool {
		return a.Compare(b) == 0 && a.Severity == b.Severity
	})
}

// watchCommand returns the command watch, which runs each workspace in a
// loop until an interrupt.
func watchCommand(compose Compose) *kernel {
	return &kernel{
		name:     "watch",
		synopsis: "Runs each workspace again after each change, until an interrupt.",
		form:     "[<pattern>...]",
		compose:  compose,
		define: func(fs *flag.FlagSet) runner {
			var interval time.Duration
			fs.DurationVar(&interval, "interval", defaultInterval, "the `pause` between two runs, at least 100ms")
			return func(ctx context.Context, x *invocation) int { return x.watch(ctx, interval) }
		},
	}
}

// watch runs each workspace in passes until ctx ends, and pauses for
// interval between two passes. A warm run over an unchanged tree changes
// nothing, so it finds a change at the cost of one stat of each file. A
// pass renders the report of a run only when the run changed a file, or
// when its findings or its error differ from the findings or the error of
// the previous pass over the workspace. watch renders the StateLocked
// finding of a run once, until a run takes the lock again.
//
// watch returns [StatusOK] when ctx ends, also during a run. It returns
// [StatusFailed] for an error of the stores, and [StatusUsage] for these
// errors:
//
//   - an interval below 100ms
//   - a config error
//   - a pattern outside the root of every workspace
//   - a pattern that a run refuses
func (x *invocation) watch(ctx context.Context, interval time.Duration) int {
	if interval < minInterval {
		return x.usage(fmt.Errorf("the interval %s is shorter than %s", interval, minInterval))
	}
	found, err := open(x.stdio, x.flags, x.k.compose)
	if err != nil {
		x.r.Error(err)
		return StatusUsage
	}
	patterns, err := assign(found.dir, found.members, x.args)
	if err != nil {
		return x.usage(err)
	}
	previous := make([]pass, len(found.members))
	for {
		for i, m := range found.members {
			if len(x.args) > 0 && len(patterns[i]) == 0 {
				continue
			}
			stores, err := m.Workspace.Stores(x.stdio.Getenv)
			if err != nil {
				x.r.Error(err)
				return StatusFailed
			}
			report, err := m.Workspace.Run(ctx, workspace.Input{
				Tree: os.DirFS(m.Root), Stores: stores, Patterns: patterns[i],
				Strict: x.flags.Strict, Caller: x.caller(),
			})
			if ctx.Err() != nil {
				return StatusOK
			}
			if report == nil {
				return x.usage(err)
			}
			failure := unreported(err)
			now := pass{findings: slices.Collect(report.Sink.All())}
			if failure != nil {
				now.failure = failure.Error()
			}
			if changes(report) || !now.same(previous[i]) {
				if m.Name != "" {
					x.r.Workspace(m)
				}
				x.render(report)
				if failure != nil {
					x.r.Error(failure)
				}
			}
			previous[i] = now
		}
		select {
		case <-ctx.Done():
			return StatusOK
		case <-time.After(interval):
		}
	}
}

// changes reports whether a report has a change other than unchanged, a
// refused write or a removal of the sweep.
func changes(report *workspace.Report) bool {
	for _, p := range report.Plans {
		if len(p.Refused) > 0 || slices.ContainsFunc(p.Changes, func(c output.Change) bool {
			return changed(c) != actionUnchanged
		}) {
			return true
		}
	}
	return len(report.Swept) > 0
}
