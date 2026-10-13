// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/reference"
)

// help is the help text of the app.
const help = "Generates the code of a workspace with the reference plugins."

// main runs the command of the command line over the composition of the
// reference plugins, and exits the process with the status of the command.
func main() {
	cli.Main(cli.New(help, reference.Compose), reference.Compose)
}
