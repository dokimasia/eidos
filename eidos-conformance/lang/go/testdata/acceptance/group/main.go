// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command group is a binary of the acceptance fixture of Go. Its app mounts
// the kernel commands as subcommands of the group gen, as in acme gen run,
// and has the crash command at its root, as in acme crash.
package main

import (
	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
	golang "go.dokimi.dev/eidos/conformance/lang/go"
)

// The help text of the app, and the name and the help text of the group of
// the kernel commands.
const (
	help      = "Runs the acceptance fixture of Go below a group."
	group     = "gen"
	groupHelp = "Generates the code of the fixture."
)

func main() {
	app := gocli.New[cli.Global](string(golang.Brand), help)
	cli.Prepare(app, golang.ComposeAcceptance)
	cli.Mount(app.Command[struct{}](group, groupHelp, nil), golang.ComposeAcceptance)
	app.Command(golang.CrashName, golang.CrashHelp, golang.Crash)
	cli.Main(app, golang.ComposeAcceptance)
}
