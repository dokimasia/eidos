// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command host mounts the kernel commands and the crash command through
// cli.Main, and meets the contract that the acceptance kit checks.
package main

import (
	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest/testdata/fixture"
)

func main() {
	cli.Main(fixture.Compose, fixture.Crash{})
}
