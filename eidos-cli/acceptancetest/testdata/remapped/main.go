// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command remapped runs the kernel commands, and exits with status 1 for
// every status other than 0, so a usage error exits 1 in place of 64.
package main

import (
	"context"
	"os"

	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest/testdata/fixture"
)

func main() {
	status := cli.Run(context.Background(), fixture.App(fixture.Compose), fixture.Compose, gocli.OS())
	if status != cli.StatusOK {
		status = cli.StatusFailed
	}
	os.Exit(status)
}
