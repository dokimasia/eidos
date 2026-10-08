// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command muted mounts the kernel commands, and discards what a command
// writes to standard error.
package main

import (
	"io"
	"os"

	"go.dokimi.dev/eidos/cli/acceptancetest/testdata/fixture"
)

func main() {
	stdio := fixture.ProcessIO()
	stdio.Stderr = io.Discard
	os.Exit(fixture.Dispatch(stdio, fixture.Commands(fixture.Compose), os.Args[1:]))
}
