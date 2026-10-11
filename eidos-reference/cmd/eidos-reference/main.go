// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/reference"
)

// main runs the command of the command line over the composition of the
// reference plugins, and exits the process with the status of the command.
func main() {
	cli.Main(reference.Compose)
}
