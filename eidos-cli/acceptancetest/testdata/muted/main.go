// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command muted runs the kernel commands, and discards what a command
// writes to standard error.
package main

import (
	"context"
	"io"
	"os"

	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest/testdata/fixture"
)

func main() {
	env := gocli.OS()
	env.Stderr = io.Discard
	os.Exit(cli.Run(context.Background(), fixture.App(fixture.Compose), fixture.Compose, env))
}
