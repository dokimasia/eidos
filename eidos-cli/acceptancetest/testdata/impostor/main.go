// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command impostor is a go-cli app with a command of its own under the name
// run, in place of the kernel command run.
package main

import (
	"context"

	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest/testdata/fixture"
)

// The name and the help text of the command of the binary.
const (
	runName = "run"
	runHelp = "Runs a command of its own."
)

func main() {
	app := gocli.New[cli.Global](string(fixture.Brand), fixture.Help)
	cli.Prepare(app, fixture.Compose)
	app.Command(runName, runHelp, func(context.Context, *gocli.Run[cli.Global, struct{}]) error { return nil })
	cli.Main(app, fixture.Compose)
}
