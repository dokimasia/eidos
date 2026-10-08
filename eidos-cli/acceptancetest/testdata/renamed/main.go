// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command renamed mounts the kernel command run under the name generate,
// and every other kernel command under its own name.
package main

import (
	"os"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest/testdata/fixture"
)

// generateName is the name that the host gives the kernel command run.
const generateName = "generate"

// renamed is a kernel command under another name.
type renamed struct {
	cli.Command
	name string
}

// Name returns the other name.
func (r renamed) Name() string { return r.name }

func main() {
	commands := fixture.Commands(fixture.Compose)
	commands[0] = renamed{Command: commands[0], name: generateName}
	os.Exit(fixture.Dispatch(fixture.ProcessIO(), commands, os.Args[1:]))
}
