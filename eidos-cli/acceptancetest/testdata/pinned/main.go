// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command pinned mounts the kernel commands, and passes each command the
// config file of the working directory through --config, so no command
// searches the parent directories for a config file.
package main

import (
	"os"
	"slices"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest/testdata/fixture"
)

// configFlag is the flag that the host passes.
const configFlag = "--config"

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		args = slices.Concat(args[:1], []string{configFlag, cli.ConfigName(fixture.Brand)}, args[1:])
	}
	os.Exit(fixture.Dispatch(fixture.ProcessIO(), fixture.Commands(fixture.Compose), args))
}
