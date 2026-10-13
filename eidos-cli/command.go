// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
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
// workspace, because a plugin instance belongs to one workspace. [Open]
// sets the output and the ledger of each composition to the disk below the
// workspace root, so an output or a ledger that compose sets has no effect
// under a command.
type Compose func() *workspace.Builder

// ExitError is a status that a handler returned after it rendered its
// output. [Run] exits with Code and renders nothing.
type ExitError struct {
	Code int
}

// Error returns the empty string, because the handler rendered its failure.
func (ExitError) Error() string { return "" }

// ExitCode returns Code. A program that runs an app without Run exits with
// the code of an error that has this method.
func (e ExitError) ExitCode() int { return e.Code }

// UsageError is an error in the invocation of a command, such as an unknown
// flag. [Renderer.Error] renders it as an error event with the field usage
// set to true, and [Run] exits with [StatusUsage] for an error of a handler
// that wraps it.
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
