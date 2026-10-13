// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	gocli "go.stealthscale.io/cli"
)

// Main runs app over the Env of the process through [Run], under a context
// that the first interrupt or termination signal cancels. It exits the
// process with the status that Run returns, and never returns. Main panics
// for each defect of the binary that Run panics for.
func Main(app *gocli.App[Global], compose Compose) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	status := Run(ctx, app, compose, gocli.OS())
	stop()
	os.Exit(status)
}

// Run runs the command line of env with app under ctx, and returns the exit
// status of the command:
//
//   - [StatusOK] when the run succeeds, and for the help of -h or --help.
//   - the code of an [ExitError] of a handler, which rendered its output.
//   - [StatusUsage] for an error of the command line, the environment or
//     the config file that go-cli finds, and for an error of a handler that
//     wraps a [*UsageError].
//   - [StatusFailed] for any other error of a handler.
//
// Run renders the error of each status other than an ExitError through
// [Begin], [Renderer.Error] and [Renderer.End], in the format and the
// colour of the global options that the run read before its fault. The
// start event of a usage error from go-cli has the path of the command of
// the error, and the start event of any other error has no command. compose
// sets the brand of the start event.
//
// Run panics for an error of the run that wraps gocli.ErrDeclaration or
// gocli.ErrEnv, such as two commands with one name or an env without
// Stdout. Each is a defect of the binary.
func Run(ctx context.Context, app *gocli.App[Global], compose Compose, env *gocli.Env) int {
	r, err := app.ExecuteRenderer(ctx, env)
	if err == nil {
		return StatusOK
	}
	if exit, ok := errors.AsType[ExitError](err); ok {
		return exit.Code
	}
	if errors.Is(err, gocli.ErrDeclaration) || errors.Is(err, gocli.ErrEnv) {
		panic(err)
	}
	// A run without Prepare has another renderer, and renders in the zero
	// format, which is text.
	selected, _ := r.(appRenderer)
	var environ []string
	if env.Environ != nil {
		environ = env.Environ()
	}
	stdio := IO{Stdout: env.Stdout, Stderr: env.Stderr, Getenv: getenv(environ), Terminal: terminal(env.Stderr)}
	command, status := "", StatusFailed
	if e, ok := errors.AsType[*gocli.Error](err); ok {
		command, status, err = e.Command, StatusUsage, &UsageError{Err: err}
	} else if _, ok := errors.AsType[*UsageError](err); ok {
		status = StatusUsage
	}
	out := Begin(stdio, selected.global, compose, command)
	out.Error(err)
	out.End(status)
	return status
}
