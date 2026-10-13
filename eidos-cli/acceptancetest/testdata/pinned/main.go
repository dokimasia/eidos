// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command pinned runs the kernel commands, and passes each command the
// config file of the working directory through --config, so no command
// searches the parent directories for a config file.
package main

import (
	"context"
	"os"
	"slices"

	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest/testdata/fixture"
)

// configFlag is the flag that the binary passes.
const configFlag = "--config"

func main() {
	env := gocli.OS()
	env.Args = slices.Concat([]string{configFlag, cli.ConfigName(fixture.Brand)}, env.Args)
	os.Exit(cli.Run(context.Background(), fixture.App(fixture.Compose), fixture.Compose, env))
}
