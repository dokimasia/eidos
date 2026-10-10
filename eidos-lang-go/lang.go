// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Lang is the source language of every loaded Go declaration, and
// the language the Go rules project for.
const Lang symbol.Lang = "golang"

// CodePrefix opens every diagnostic code the satellite registers:
// GOLANG-0001 is the first.
const CodePrefix diag.Prefix = "GOLANG"

// Target names the rendering target a plan resolves to select the
// Go backend.
const Target plugin.Target = "golang"

// Name is the backend's plugin identity: what its findings are
// reported under and what a composition schedules it by.
const Name plugin.ID = "golang"

// Extension is the suffix of every Go file.
const Extension = ".go"

// Version is the backend's behavior version, folded into the
// composition fingerprint: bump it with any change to the rendered
// output.
const Version = "0.8.0"

// FrontendVersion is the frontend's behavior version, folded into
// every unit key the frontend builds: bump it with any change to
// the graph a parse produces.
const FrontendVersion = "0.8.0"

// Syntax returns Go's comment forms, declared once and shared: the
// frontend strips comments with them, and the output contract
// writes the generated-file frame through them. Directives is true,
// because the go:build family is Go's own convention, the open
// tool:name form gofmt preserves. It allocates the two lists, so no
// caller shares another's.
func Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{
		Line:       []string{"//"},
		Blocks:     []plugin.CommentBlock{{Open: "/*", Close: "*/"}},
		Directives: true,
	}
}
