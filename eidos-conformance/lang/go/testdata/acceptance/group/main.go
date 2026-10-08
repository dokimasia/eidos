// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command group is a binary of the acceptance fixture of Go. Its dispatcher
// mounts the kernel commands under the group gen, as in acme gen run, and
// the crash command at the root, as in acme crash. The dispatcher uses the
// standard library alone.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"go.dokimi.dev/eidos/cli"
	golang "go.dokimi.dev/eidos/conformance/lang/go"
)

// group is the name of the group of the kernel commands.
const group = "gen"

func main() {
	stdio, err := cli.ProcessIO()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.StatusFailed)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	status := dispatch(ctx, stdio, os.Args[1:])
	stop()
	os.Exit(status)
}

// dispatch runs the crash command for crash, and the kernel command whose
// name follows gen with the arguments after the name. It returns the status
// of the command, and 64 for any other command line.
func dispatch(ctx context.Context, stdio cli.IO, args []string) int {
	if len(args) > 0 && args[0] == golang.CrashName {
		return golang.Crash{}.Run(ctx, stdio, args[1:])
	}
	if len(args) < 2 || args[0] != group {
		fmt.Fprintln(stdio.Stderr, "usage: group "+group+" <command> [flags] [arguments]")
		return cli.StatusUsage
	}
	commands := cli.Kernels(golang.ComposeAcceptance)
	i := slices.IndexFunc(commands, func(c cli.Command) bool { return c.Name() == args[1] })
	if i < 0 {
		fmt.Fprintf(stdio.Stderr, "group: %s is not a command of %s\n", args[1], group)
		return cli.StatusUsage
	}
	return commands[i].Run(ctx, stdio, args[2:])
}
