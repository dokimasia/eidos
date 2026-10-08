// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// endFlags is the argument that ends the flags of a command.
const endFlags = "--"

// runner runs a kernel command once its flags have parsed, and returns the
// status of the command.
type runner func(ctx context.Context, x *invocation) int

// kernel is one of the seven kernel commands. Every kernel command parses
// its flags and arguments in the same way, renders through a [Renderer]
// and returns its status.
type kernel struct {
	name     string
	synopsis string
	// form is the form of the positional arguments of the command, such as
	// "[<pattern>...]". It is empty for a command without positional
	// arguments, and such a command returns a usage error for one.
	form    string
	compose Compose
	// define defines the flags of the command on fs, and returns the
	// function that runs the command once fs has parsed.
	define func(fs *flag.FlagSet) runner
}

var _ Command = (*kernel)(nil)

// Kernels returns the seven kernel commands over compose, in the order run,
// plan, explain, prune, doctor, watch and version. Each call returns new
// values. The Run method of a command is safe for concurrent use when
// compose is.
func Kernels(compose Compose) []Command {
	return []Command{
		runCommand(compose), planCommand(compose), explainCommand(compose), pruneCommand(compose),
		doctorCommand(compose), watchCommand(compose), versionCommand(compose),
	}
}

// Name returns the name of the command.
func (k *kernel) Name() string { return k.name }

// Synopsis returns the line that a list of commands prints beside the name.
func (k *kernel) Synopsis() string { return k.synopsis }

// Usage returns the help text of the command. The first line is the form of
// the command. The synopsis follows as the second paragraph, and then each
// flag with its description.
func (k *kernel) Usage() string {
	var f Flags
	fs, _ := k.flagSet(&f)
	var b strings.Builder
	fmt.Fprintf(&b, "usage: %s [flags]", k.name)
	if k.form != "" {
		fmt.Fprintf(&b, " %s", k.form)
	}
	fmt.Fprintf(&b, "\n\n%s\n\nflags:\n", k.synopsis)
	fs.VisitAll(func(fl *flag.Flag) {
		value, usage := flag.UnquoteUsage(fl)
		if value != "" {
			value = " <" + value + ">"
		}
		fmt.Fprintf(&b, "  --%s%s\n        %s\n", fl.Name, value, usage)
	})
	return b.String()
}

// Run parses the flags and the arguments of the command from args, runs the
// command and renders its output to stdio. Flags can come before, between
// and after the positional arguments, and the argument -- ends the flags.
// For -h or --help in args, Run prints Usage to stdio.Stdout and returns
// [StatusOK]. For an unknown flag, a malformed value or a positional
// argument of a command that takes none, Run renders a usage error and
// returns [StatusUsage].
//
// Run panics unless stdio has both output streams, Getenv and an absolute
// Dir.
func (k *kernel) Run(ctx context.Context, stdio IO, args []string) int {
	stdio.check()
	var f Flags
	fs, run := k.flagSet(&f)
	positional, err := parse(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(stdio.Stdout, k.Usage())
		return StatusOK
	}
	if err == nil && k.form == "" && len(positional) > 0 {
		err = fmt.Errorf("the command takes no arguments, and has %s", strings.Join(positional, " "))
	}
	x := &invocation{k: k, stdio: stdio, flags: f, args: positional, raw: args, r: Begin(stdio, f, k.compose, k.name)}
	status := StatusUsage
	if err != nil {
		x.usage(err)
	} else {
		status = run(ctx, x)
	}
	x.r.End(status)
	return status
}

// flagSet returns a flag set with the shared flags and the flags of the
// command, and the function that runs the command. The shared flags set the
// fields of f. The flag set discards its own output, because the command
// renders its own errors.
func (k *kernel) flagSet(f *Flags) (*flag.FlagSet, runner) {
	fs := flag.NewFlagSet(k.name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	f.Register(fs)
	return fs, k.define(fs)
}

// invocation is one call of a kernel command.
type invocation struct {
	k     *kernel
	stdio IO
	flags Flags
	// args are the positional arguments, and raw are the arguments that the
	// host passed to the command.
	args []string
	raw  []string
	r    *Renderer
}

// usage renders err as a usage error of the command and returns
// [StatusUsage]. Text output also writes the help text of the command to
// standard error.
func (x *invocation) usage(err error) int {
	x.r.Error(&UsageError{Err: fmt.Errorf("cli: %s: %w", x.k.name, err)})
	if x.flags.Format != FormatJSON {
		fmt.Fprint(x.stdio.Stderr, x.k.Usage())
	}
	return StatusUsage
}

// caller returns the name of the command and its arguments, joined by
// spaces. A run records the result as the caller in the holder record of
// its lock.
func (x *invocation) caller() string {
	return strings.Join(append([]string{x.k.name}, x.raw...), " ")
}

// parse parses args with fs and returns the positional arguments in order.
// fs.Parse stops at the first argument that is not a flag, so parse moves
// that argument to the positional arguments and parses the rest again. The
// argument -- ends the flags, and every argument after it is positional.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if ended(fs, args[:len(args)-len(rest)]) {
			return append(positional, rest...), nil
		}
		if len(rest) == 0 {
			return positional, nil
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

// ended reports whether the arguments that one fs.Parse consumed end with
// the -- that ends the flags, and not with the value of a flag. It walks the
// arguments in order, because a flag that takes a value and has no equals
// sign takes the next argument as its value.
func ended(fs *flag.FlagSet, parsed []string) bool {
	for i := 0; i < len(parsed); i++ {
		if parsed[i] == endFlags {
			return true
		}
		if takesValue(fs, parsed[i]) {
			i++
		}
	}
	return false
}

// takesValue reports whether the flag arg takes the next argument as its
// value: arg has no equals sign, and the flag is not a boolean flag. fs has
// parsed arg, so fs defines the flag.
func takesValue(fs *flag.FlagSet, arg string) bool {
	name := strings.TrimLeft(arg, "-")
	if strings.Contains(name, "=") {
		return false
	}
	b, isBool := fs.Lookup(name).Value.(interface{ IsBoolFlag() bool })
	return !isBool || !b.IsBoolFlag()
}
