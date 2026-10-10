// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
)

// CrashName is the name of [Crash] in the binaries of the acceptance
// fixture.
const CrashName = "crash"

// The go command, the arguments that [CompileAcceptance] passes to it, and
// the variable that turns off the workspace of the go command.
const (
	goCommand = "go"
	goBuild   = "build"
	allPkgs   = "./..."
	noWork    = "GOWORK=off"
)

// Crash is a command that panics. The binaries of the acceptance fixture
// mount it, and the acceptance suite runs it to check that a panic exits
// with status 2. The zero value is the command.
type Crash struct{}

// Name returns the name of the command, crash.
func (Crash) Name() string { return CrashName }

// Synopsis returns the synopsis of the command.
func (Crash) Synopsis() string {
	return "Panics, so that the acceptance suite checks the status of a panic."
}

// Usage returns the help text of the command.
func (Crash) Usage() string { return "usage: crash\n" }

// Run panics.
func (Crash) Run(context.Context, cli.IO, []string) int {
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

// CompileAcceptance is the Compile of the acceptance fixture. It runs go
// build over every package of the module in root, outside the workspace of
// the go command.
//
// Error modes: an error with the output of the go command, for a module
// that does not compile.
func CompileAcceptance(ctx context.Context, root string) error {
	cmd := exec.CommandContext(ctx, goCommand, goBuild, allPkgs)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), noWork)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("golang: compile the output in %s: %w\n%s", root, err, out)
	}
	return nil
}
