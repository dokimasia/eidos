// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"flag"

	"go.dokimi.dev/eidos/core/workspace"
)

// pruneCommand returns the command prune, which removes the stale output
// of each workspace. A prune runs every plan and withholds every write, so
// it commits only the removal of stale files.
func pruneCommand(compose Compose) *kernel {
	return &kernel{
		name:     "prune",
		synopsis: "Removes the generated files that no plan produces.",
		compose:  compose,
		define: func(fs *flag.FlagSet) runner {
			var dry bool
			fs.BoolVar(&dry, "dry-run", false, "list the removals and commit nothing")
			return func(ctx context.Context, x *invocation) int {
				return x.runEach(ctx, workspace.Input{Prune: true, Dry: dry})
			}
		},
	}
}
