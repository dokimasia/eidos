// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command banner runs the kernel commands, and writes a banner line to
// standard output before each command runs.
package main

import (
	"fmt"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest/testdata/fixture"
)

func main() {
	fmt.Println("acme 1.0, the generator of the acme platform")
	cli.Main(fixture.App(fixture.Compose), fixture.Compose)
}
