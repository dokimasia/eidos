// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command main is the binary of the acceptance fixture of Go that mounts
// the kernel commands through cli.Main, beside the crash command.
package main

import (
	"go.dokimi.dev/eidos/cli"
	golang "go.dokimi.dev/eidos/conformance/lang/go"
)

func main() {
	cli.Main(golang.ComposeAcceptance, golang.Crash{})
}
