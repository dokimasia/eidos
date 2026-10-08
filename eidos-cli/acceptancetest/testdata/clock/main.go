// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command clock mounts the kernel commands through cli.Main over a
// composition whose mirror puts the time of the run into the name of each
// struct, so a second run rewrites every file.
package main

import (
	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest/testdata/fixture"
)

func main() {
	cli.Main(fixture.ComposeClock, fixture.Crash{})
}
