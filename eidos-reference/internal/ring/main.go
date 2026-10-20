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
// failed. statusFailed reports a module that failed or a run that could not
// test the modules. statusUsage reports a command line or a checkout that
// does not read. A Go panic exits with status 2. No status of the ring has
// that value.
const (
	statusOK     = 0
	statusFailed = 1
	statusUsage  = 64
)

// The name of the command, the flag of the root of the checkout, and the
// default and the usage of the flag.
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
	earlierWord     = "earlier"
)

// scratchPattern is the pattern of the name of the scratch directory of a
// run. The directory is in the temporary directory of the system.
const scratchPattern = "eidos-ring-*"

// main runs the ring with the working directory, the arguments and the
// environment of the process. It exits the process with the status of the
// run. The first interrupt or termination signal cancels the run. A working
// directory that does not resolve ends the process with statusUsage.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	status := statusUsage
	if dir, err := os.Getwd(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", commandName, err)
	} else {
		status = run(ctx, dir, os.Args[1:], os.Environ(), os.Stdout, os.Stderr)
	}
	stop()
	os.Exit(status)
}

// run runs the ring under ctx over the checkout of the flag -root in args.
// It returns the exit status. A relative root resolves against the absolute
// working directory dir. env is the environment of the go command and of
// git.
//
// run reads the checkout and queries the module proxy for the latest version
// of each module of the ring. It tests each published version from the tree
// of its release tag against the kernel and the SDK facade of the checkout. A
// published version that requires the kernel or the facade on an earlier
// compatibility line than the checkout releases is not tested. run reports it
// as earlier, and the report does not fail the run.
// It writes the line of each module to stdout in the order of the module
// paths. It writes the errors of the command line and of the checkout to
// stderr. The tree of each release has a directory of its own in the scratch
// directory of the run. run removes the scratch directory before it returns.
func run(ctx context.Context, dir string, args, env []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet(commandName, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String(rootFlag, rootDefault, rootUsage)
	if err := flags.Parse(args); err != nil {
		return statusUsage
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "%s: the command does not take the arguments %s\n", commandName,
			strings.Join(flags.Args(), " "))
		return statusUsage
	}
	c, err := read(dir, *root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return statusUsage
	}
	scratch, err := os.MkdirTemp("", scratchPattern)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", commandName, err)
		return statusFailed
	}
	// The scratch directory contains the trees of the releases alone. A
	// directory that a run leaves behind does not change a later run.
	defer func() { _ = os.RemoveAll(scratch) }()

	status := statusOK
	for i, m := range c.ring {
		version, refusal, err := latest(ctx, env, scratch, m.path)
		unserved := slices.ContainsFunc(notServed, func(line string) bool { return strings.Contains(refusal, line) })
		switch {
		case err != nil:
			fmt.Fprintf(stdout, "%s %s: %v\n", failWord, m.path, err)
			status = statusFailed
		case unserved:
			fmt.Fprintf(stdout, "%s %s: %s\n", unpublishedWord, m.path, refusal)
		case refusal != "":
			fmt.Fprintf(stdout, "%s %s: %s\n", failWord, m.path, refusal)
			status = statusFailed
		case module.IsPseudoVersion(version):
			fmt.Fprintf(stdout,
				"%s %s: the module proxy returns the pseudo-version %s for a module without a release\n",
				unpublishedWord, m.path, version)
		default:
			reason, err := earlier(ctx, env, c, m, version)
			if err == nil && reason != "" {
				fmt.Fprintf(stdout, "%s %s@%s: %s\n", earlierWord, m.path, version, reason)
				continue
			}
			var out []byte
			if err == nil {
				out, err = test(ctx, env, c, filepath.Join(scratch, strconv.Itoa(i)), m, version)
			}
			if err != nil {
				fmt.Fprintf(stdout, "%s %s@%s: %v\n%s", failWord, m.path, version, err, out)
				status = statusFailed
				continue
			}
			fmt.Fprintf(stdout, "%s %s@%s\n", okWord, m.path, version)
		}
	}
	return status
}
