// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package fixture declares the composition, the panicking command and the
// dispatcher of the hosts that the tests of the acceptance kit build. A
// host is a command line that mounts the kernel commands. One host meets
// the contract, and each other host breaks one rule of it.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"time"

	"go.dokimi.dev/eidos/cli"
	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// Brand is the brand of the composition of every host.
const Brand output.Brand = "acme"

// CrashName is the name of the command that panics.
const CrashName = "crash"

// The composition renders each struct of the scripted language into the
// text file of its package, through one plan.
const (
	target   plugin.Target = "text"
	planName               = "mirrors"
)

// Crash is a command that panics, which every host mounts beside the kernel
// commands.
type Crash struct{}

// Name returns the name of the command.
func (Crash) Name() string { return CrashName }

// Synopsis returns the synopsis of the command.
func (Crash) Synopsis() string { return "Panics." }

// Usage returns the help text of the command.
func (Crash) Usage() string { return "usage: crash\n" }

// Run panics.
func (Crash) Run(context.Context, cli.IO, []string) int {
	panic("fixture: the crash command panics")
}

// Compose returns the composition of the hosts: the scripted frontend and
// one plan that mirrors each struct into the file mirror.txt of its
// package.
func Compose() *workspace.Builder {
	return composed(func(name string) string { return "For" + name })
}

// ComposeClock returns the composition of Compose with another mirror. The
// mirror puts the time of the run into the name of each struct, so no two
// runs write the same bytes.
func ComposeClock() *workspace.Builder {
	return composed(func(name string) string {
		return "For" + name + strconv.FormatInt(time.Now().UnixNano(), 10)
	})
}

// Commands returns the kernel commands over compose and the crash command.
func Commands(compose cli.Compose) []cli.Command {
	return append(cli.Kernels(compose), Crash{})
}

// ProcessIO returns the IO of the process, and exits the process with
// status 1 when the working directory does not resolve.
func ProcessIO() cli.IO {
	stdio, err := cli.ProcessIO()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.StatusFailed)
	}
	return stdio
}

// Dispatch runs the command of commands whose name is the first argument,
// with the arguments after it, and returns its status. It returns 64 for a
// missing name and for the name of no command.
func Dispatch(stdio cli.IO, commands []cli.Command, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(stdio.Stderr, errors.New("fixture: the command line has no command"))
		return cli.StatusUsage
	}
	i := slices.IndexFunc(commands, func(c cli.Command) bool { return c.Name() == args[0] })
	if i < 0 {
		fmt.Fprintf(stdio.Stderr, "fixture: %s is not a command\n", args[0])
		return cli.StatusUsage
	}
	return commands[i].Run(context.Background(), stdio, args[1:])
}

// composed returns the composition of the hosts. Its mirror gives the
// mirror of each struct the name that name returns for the struct.
func composed(name func(string) string) *workspace.Builder {
	mirror := eidos.NewPlugin("mirror").
		Output(plugin.Output{Per: plugin.PerPackage, Word: "mirror"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			e.PackageFile().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: name(m.Struct.Name)})
			return nil
		})).
		Build().(plugin.Generator)
	coverage := map[symbol.Fact]render.Verdict{}
	for _, f := range symbol.Facts() {
		coverage[f] = render.Renders
	}
	printer := backend.New("printer", target, plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{symbol.KindStruct: "type {{.Name}} struct{}\n"}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return nil, errors.New("fixture: the printer writes no statements")
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Coverage(render.Coverage{Facts: coverage}).
		Build()
	return workspace.New().
		Brand(Brand).
		Frontends(frontendtest.NewScripted()).
		Targets(target).
		Plans(workspace.Plan{Name: planName, Generators: []plugin.Generator{mirror}, Backend: printer})
}
