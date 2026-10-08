// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command remapped mounts the kernel commands, and exits with status 1 for
// every status other than 0, so a usage error exits 1 in place of 64.
package main

import (
	"os"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest/testdata/fixture"
)

func main() {
	status := fixture.Dispatch(fixture.ProcessIO(), fixture.Commands(fixture.Compose), os.Args[1:])
	if status != cli.StatusOK {
		status = cli.StatusFailed
	}
	os.Exit(status)
}
