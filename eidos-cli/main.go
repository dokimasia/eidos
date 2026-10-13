// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"text/tabwriter"
)

// helpCommand is the command name for which Main lists the commands, or
// prints the help text of the command that follows it.
const helpCommand = "help"

// Main is the command line of a binary without a framework of its own. It
// runs os.Args through [Run] with [ProcessIO], under a context that the
// first interrupt or termination signal cancels. It exits the process with
// the status that Run returns, and never returns. When ProcessIO fails,
// Main writes the error to standard error and exits with [StatusFailed].
// Main panics for each defect of the binary that Run panics for.
func Main(compose Compose, extra ...Command) {
	stdio, err := ProcessIO()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(StatusFailed)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	status := Run(ctx, stdio, compose, os.Args, extra...)
	stop()
	os.Exit(status)
}

// Run runs the command line args of a binary with stdio under ctx, and
// returns the status of the command. args[0] is the path of the binary, as
// [os.Args] has it, and the rest is [flags] <command> [flags] [arguments].
// Run looks up the command among the kernel commands and the extra
// commands. It passes the flags before the name of the command to the
// command, ahead of the arguments after the name. For help, -h or --help
// before the name, Run writes the list of commands. For help <command>, it
// writes the help text of the command. Both return [StatusOK]. A missing or
// unknown command returns [StatusUsage].
//
// Run panics for each of these defects of the binary:
//
//   - a command line without the path of the binary
//   - an IO without both output streams, without Getenv or with a relative
//     working directory
//   - an extra command with an empty name
//   - an extra command with the name of a kernel command
//   - two extra commands with one name
func Run(ctx context.Context, stdio IO, compose Compose, args []string, extra ...Command) int {
	if len(args) == 0 {
		panic("cli: the command line has no path of the binary")
	}
	stdio.check()
	commands := append(Kernels(compose), extra...)
	named := map[string]bool{}
	for _, c := range commands {
		if c.Name() == "" {
			panic("cli: a command has an empty name")
		}
		if named[c.Name()] {
			panic(fmt.Sprintf("cli: two commands have the name %q", c.Name()))
		}
		named[c.Name()] = true
	}
	return dispatch(ctx, stdio, compose, filepath.Base(args[0]), args[1:], commands)
}

// dispatch looks up the command whose name follows the leading flags of
// args. It runs the command with the leading flags and the arguments after
// the name, and returns its status. binary is the name of the binary in the
// list of commands.
func dispatch(ctx context.Context, stdio IO, compose Compose, binary string, args []string, commands []Command) int {
	var f Flags
	fs := flag.NewFlagSet(binary, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	f.Register(fs)
	err := fs.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		listCommands(stdio.Stdout, binary, commands)
		return StatusOK
	}
	if err != nil {
		return refuse(stdio, f, compose, binary, err, commands)
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return refuse(stdio, f, compose, binary, fmt.Errorf("the command line of %s has no command", binary), commands)
	}
	if rest[0] == helpCommand {
		return help(stdio, f, compose, binary, rest[1:], commands)
	}
	i := slices.IndexFunc(commands, func(c Command) bool { return c.Name() == rest[0] })
	if i < 0 {
		return refuse(stdio, f, compose, binary, fmt.Errorf("%s is not a command of %s", rest[0], binary), commands)
	}
	leading := args[:len(args)-len(rest)]
	return commands[i].Run(ctx, stdio, append(slices.Clone(leading), rest[1:]...))
}

// help prints the list of commands for no name, and the help text of the
// command with the one name in names. It returns [StatusUsage] for more than
// one name and for the name of no command.
func help(stdio IO, f Flags, compose Compose, binary string, names []string, commands []Command) int {
	if len(names) == 0 {
		listCommands(stdio.Stdout, binary, commands)
		return StatusOK
	}
	i := slices.IndexFunc(commands, func(c Command) bool { return c.Name() == names[0] })
	if len(names) > 1 || i < 0 {
		return refuse(stdio, f, compose, binary,
			fmt.Errorf("help takes the name of one command, and has %s", strings.Join(names, " ")), commands)
	}
	fmt.Fprint(stdio.Stdout, commands[i].Usage())
	return StatusOK
}

// refuse renders a usage error of the command line, and returns
// [StatusUsage]. Text output also writes the list of commands to standard
// error.
func refuse(stdio IO, f Flags, compose Compose, binary string, err error, commands []Command) int {
	r := Begin(stdio, f, compose, "")
	r.Error(&UsageError{Err: fmt.Errorf("cli: %w", err)})
	if f.Format != FormatJSON {
		listCommands(stdio.Stderr, binary, commands)
	}
	r.End(StatusUsage)
	return StatusUsage
}

// listCommands writes the form of the command line and each command with its
// synopsis to w.
func listCommands(w io.Writer, binary string, commands []Command) {
	fmt.Fprintf(w, "usage: %s [flags] <command> [flags] [arguments]\n\ncommands:\n", binary)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, c := range commands {
		fmt.Fprintf(tw, "  %s\t%s\n", c.Name(), c.Synopsis())
	}
	// The listing is the output of the command, so a write that fails has no
	// other place to report to.
	_ = tw.Flush()
	fmt.Fprintf(w, "\nRun %s help <command> for the flags of a command.\n", binary)
}
