// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command recovering mounts the kernel commands. It recovers the panic of a
// command and exits with status 1 in place of status 2.
package main

import (
	"fmt"
	"os"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest/testdata/fixture"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "recovering: the command failed:", r)
			os.Exit(cli.StatusFailed)
		}
	}()
	os.Exit(fixture.Dispatch(fixture.ProcessIO(), fixture.Commands(fixture.Compose), os.Args[1:]))
}
