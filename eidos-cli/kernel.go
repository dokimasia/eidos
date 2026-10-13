// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"io"

	gocli "go.stealthscale.io/cli"
)

// kernels contains the handlers of the eight kernel commands over one
// composition. A handler is safe for concurrent use when compose is.
type kernels struct {
	compose Compose
}

// invocation is one run of a kernel command.
type invocation struct {
	compose Compose
	stdio   IO
	global  Global
	// command is the path of the command, such as run or gen run. A run
	// records it as the caller in the holder record of its lock.
	command string
	// patterns are the patterns of run and watch, and empty for the other
	// commands.
	patterns []string
	r        *Renderer
	// stream is the writer of the files of run --stdout, and nil for a run
	// that writes to the disk.
	stream io.Writer
}

// execute runs body as the handler of a kernel command, and returns [Exit]
// of the status of body. It begins the output with [IOOf] of run and the
// global options of run, and ends the output with the status. patterns are
// the patterns of the command. execute renders a usage error in place of
// body when the IO of run does not resolve, and for --quiet with --verbose.
func execute[C any](
	ctx context.Context, compose Compose, run *gocli.Run[Global, C], patterns []string,
	body func(ctx context.Context, x *invocation) int,
) error {
	stdio, err := IOOf(run)
	x := &invocation{
		compose: compose, stdio: stdio, global: *run.Global(), command: run.Command(), patterns: patterns,
	}
	x.r = Begin(stdio, x.global, compose, x.command)
	status := StatusUsage
	switch {
	case err != nil:
		x.r.Error(err)
	case x.global.Quiet && x.global.Verbose:
		x.usage(fmt.Errorf("the flags --%s and --%s exclude each other", quietFlag, verboseFlag))
	default:
		status = body(ctx, x)
	}
	x.r.End(status)
	return Exit(status)
}

// usage renders err as a usage error of the command and returns
// [StatusUsage].
func (x *invocation) usage(err error) int {
	x.r.Error(&UsageError{Err: fmt.Errorf("cli: %s: %w", x.command, err)})
	return StatusUsage
}
