// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/mod/module"
)

// The exit statuses of the ring. statusOK reports a run in which no module
// failed. statusFailed reports a module that failed, or a run that could
// not test the modules. statusUsage reports a command line or a checkout
// that does not read.
const (
	statusOK     = 0
	statusFailed = 1
	statusUsage  = 2
)

// The name of the command, and its flag of the root of the checkout with
// the default and the usage of the flag.
const (
	commandName = "ring"
	rootFlag    = "root"
	rootDefault = "."
	rootUsage   = "the directory of the go.work of the checkout"
)

// The words that open the line of a module in the output.
const (
	okWord          = "ok"
	failWord        = "FAIL"
	unpublishedWord = "unpublished"
)

// scratchPattern is the pattern of the name of the scratch directory of a
// run, in the temporary directory of the system.
const scratchPattern = "eidos-ring-*"

// main runs the ring with the arguments and the environment of the process,
// and exits the process with the status of the run. The first interrupt or
// termination signal cancels the run.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	status := run(ctx, os.Args[1:], os.Environ(), os.Stdout, os.Stderr)
	stop()
	os.Exit(status)
}

// run runs the ring under ctx over the checkout that the flags of args
// name, and returns the exit status. env is the environment of the go
// command.
//
// run reads the checkout and queries the module proxy for the latest version
// of each module of the ring. It tests each published version against the
// kernel and the SDK facade of the checkout. It writes the line of each
// module to stdout in the order of the module paths. It writes the errors
// of the command line and of the checkout to stderr. Each module has a
// directory of its own in the scratch directory of the run. run removes
// the scratch directory before it returns.
func run(ctx context.Context, args, env []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet(commandName, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String(rootFlag, rootDefault, rootUsage)
	if err := flags.Parse(args); err != nil {
		return statusUsage
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "%s: the command takes no arguments, and has %s\n", commandName,
			strings.Join(flags.Args(), " "))
		return statusUsage
	}
	c, err := read(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return statusUsage
	}
	scratch, err := os.MkdirTemp("", scratchPattern)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", commandName, err)
		return statusFailed
	}
	// The scratch directory contains the scratch modules alone, and a
	// directory that a run leaves behind changes no later run.
	defer func() { _ = os.RemoveAll(scratch) }()

	status := statusOK
	for i, mod := range c.ring {
		version, refusal, err := latest(ctx, env, scratch, mod)
		unserved := slices.ContainsFunc(notServed, func(line string) bool { return strings.Contains(refusal, line) })
		switch {
		case err != nil:
			fmt.Fprintf(stdout, "%s %s: %v\n", failWord, mod, err)
			status = statusFailed
		case unserved:
			fmt.Fprintf(stdout, "%s %s: %s\n", unpublishedWord, mod, refusal)
		case refusal != "":
			fmt.Fprintf(stdout, "%s %s: %s\n", failWord, mod, refusal)
			status = statusFailed
		case module.IsPseudoVersion(version):
			fmt.Fprintf(stdout, "%s %s: the module proxy has no release, and returns the pseudo-version %s\n",
				unpublishedWord, mod, version)
		default:
			out, err := test(ctx, env, c, filepath.Join(scratch, strconv.Itoa(i)), mod, version)
			if err != nil {
				fmt.Fprintf(stdout, "%s %s@%s: %v\n%s", failWord, mod, version, err, out)
				status = statusFailed
				continue
			}
			fmt.Fprintf(stdout, "%s %s@%s\n", okWord, mod, version)
		}
	}
	return status
}
