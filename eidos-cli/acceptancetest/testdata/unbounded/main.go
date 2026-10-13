// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command unbounded runs the kernel commands. It searches the working
// directory and each parent directory for the config file of the brand past
// every version control marker. It passes the first file that it finds to
// each command through --config.
package main

import (
	"context"
	"fmt"
	"os"
	"slices"

	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest/testdata/fixture"
)

// configFlag is the flag that passes a config file to a command.
const configFlag = "--config"

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.StatusFailed)
	}
	env := gocli.OS()
	if file, found := fixture.ConfigAbove(dir); found {
		env.Args = slices.Concat([]string{configFlag, file}, env.Args)
	}
	os.Exit(cli.Run(context.Background(), fixture.App(fixture.Compose), fixture.Compose, env))
}
