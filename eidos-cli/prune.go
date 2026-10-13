// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"

	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/core/workspace"
)

// The name and the help text of the command prune.
const (
	pruneName = "prune"
	pruneHelp = "Removes the generated files that no plan produces."
)

// pruneOptions are the options of prune.
type pruneOptions struct {
	DryRun bool `help:"List the removals and commit nothing."`
}

// prune is the handler of prune, which removes the stale output of each
// workspace. A prune runs every plan and withholds every write, so it
// commits only the removal of stale files.
func (k kernels) prune(ctx context.Context, run *gocli.Run[Global, pruneOptions]) error {
	dry := run.Config().DryRun
	return execute(ctx, k.compose, run, nil, func(ctx context.Context, x *invocation) int {
		return x.runEach(ctx, workspace.Input{Prune: true, Dry: dry})
	})
}
