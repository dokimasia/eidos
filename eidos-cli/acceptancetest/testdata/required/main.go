// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command required mounts the kernel commands. It searches the working
// directory and each parent directory for the config file of the brand past
// every version control marker. It passes the first file that it finds to
// each command through --config. Without a file it exits 64, as a
// composition with a required option exits without a config file.
package main

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest/testdata/fixture"
)

// configFlag is the flag that passes a config file to a command.
const configFlag = "--config"

func main() {
	stdio := fixture.ProcessIO()
	file, found := fixture.ConfigAbove(stdio.Dir)
	if !found {
		fmt.Fprintln(stdio.Stderr, errors.New("required: no directory has the config file of the brand"))
		os.Exit(cli.StatusUsage)
	}
	args := os.Args[1:]
	if len(args) > 0 {
		args = slices.Concat(args[:1], []string{configFlag, file}, args[1:])
	}
	os.Exit(fixture.Dispatch(stdio, fixture.Commands(fixture.Compose), args))
}
