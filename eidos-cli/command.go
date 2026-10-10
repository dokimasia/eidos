// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"

	"go.dokimi.dev/eidos/core/workspace"
)

// A command returns one of these statuses.
const (
	// StatusOK is the status of a command that succeeded.
	StatusOK = 0
	// StatusFailed is the status of a command whose findings or errors
	// failed it.
	StatusFailed = 1
	// StatusUsage is the status of a usage error or a config error.
	StatusUsage = 64
)

// Compose returns a new composition with the brand, the frontends, the
// plugins and the plans of the binary. A command calls it once for each
// workspace, because a plugin instance belongs to one workspace.
type Compose func() *workspace.Builder

// Command is one command of a binary. A host selects a command by its name,
// passes it the arguments after the name, and exits with the status that
// Run returns.
type Command interface {
	// Name returns the name of the command on the command line.
	Name() string
	// Synopsis returns the line that a list of commands prints beside the
	// name.
	Synopsis() string
	// Usage returns the help text of the command. The first line is the
	// form of the command, such as "run [flags] [<pattern>...]".
	Usage() string
	// Run parses the flags and the arguments of the command from args, runs
	// the command and renders its output to stdio. It returns StatusOK,
	// StatusFailed or StatusUsage. For -h or --help in args, Run prints
	// Usage to stdio.Stdout and returns StatusOK.
	Run(ctx context.Context, stdio IO, args []string) int
}

// ExitError is a status that a command returned after it rendered its
// output. A host whose commands return errors exits with Code and prints
// nothing.
type ExitError struct {
	Code int
}

// Error returns the empty string, because the command rendered its failure.
func (ExitError) Error() string { return "" }

// ExitCode returns Code. A host such as urfave/cli exits with the code of
// an error that has this method.
func (e ExitError) ExitCode() int { return e.Code }

// UsageError is an error in the invocation of a command, such as an unknown
// flag. [Renderer.Error] renders it as an error event with the field usage
// set to true.
type UsageError struct {
	Err error
}

// Error returns the text of Err.
func (e *UsageError) Error() string { return e.Err.Error() }

// Unwrap returns Err.
func (e *UsageError) Unwrap() error { return e.Err }

// Exit returns nil for StatusOK, and an [ExitError] with status for any
// other status.
func Exit(status int) error {
	if status == StatusOK {
		return nil
	}
	return ExitError{Code: status}
}
