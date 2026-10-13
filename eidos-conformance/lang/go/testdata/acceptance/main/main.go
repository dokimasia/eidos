// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command main is the binary of the acceptance fixture of Go. Its app has
// the kernel commands at its root, beside the crash command.
package main

import (
	"go.dokimi.dev/eidos/cli"
	golang "go.dokimi.dev/eidos/conformance/lang/go"
)

// help is the help text of the app.
const help = "Runs the acceptance fixture of Go."

func main() {
	app := cli.New(help, golang.ComposeAcceptance)
	app.Command(golang.CrashName, golang.CrashHelp, golang.Crash)
	cli.Main(app, golang.ComposeAcceptance)
}
