// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"context"

	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
)

// The name and the help text of the command [Crash] in the binaries of the
// acceptance fixture.
const (
	CrashName = "crash"
	CrashHelp = "Panics, so that the acceptance suite checks the status of a panic."
)

// Crash is the handler of a command that panics. The binaries of the
// acceptance fixture add it to their apps under CrashName, and the
// acceptance suite runs it to check that a panic exits with status 2.
func Crash(context.Context, *gocli.Run[cli.Global, struct{}]) error {
	panic("golang: the crash command panics")
}

// ComposeAcceptance returns the composition of the binaries of the
// acceptance fixture. The composition has the Go frontend and rules, the
// plans that [WorkspacePlans] returns and the check [Stubbed]. It has no
// sink and no ledger, because the command line sets them for each
// workspace.
func ComposeAcceptance() *workspace.Builder {
	return workspace.New().
		Brand(Brand).
		Frontends(gofrontend.New(nil)).
		Rules(gorules.New()).
		Targets(golang.Target).
		Plans(WorkspacePlans()...).
		Checks(Stubbed{})
}
