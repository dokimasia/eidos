// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command required runs the kernel commands. It searches the working
// directory and each parent directory for the config file of the brand past
// every version control marker. It passes the first file that it finds to
// each command through --config. Without a file it exits 64, as a
// composition with a required option exits without a config file.
package main

import (
	"context"
	"errors"
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
	file, found := fixture.ConfigAbove(dir)
	if !found {
		fmt.Fprintln(os.Stderr, errors.New("required: no directory has the config file of the brand"))
		os.Exit(cli.StatusUsage)
	}
	env := gocli.OS()
	env.Args = slices.Concat([]string{configFlag, file}, env.Args)
	os.Exit(cli.Run(context.Background(), fixture.App(fixture.Compose), fixture.Compose, env))
}
