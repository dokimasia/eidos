// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang

import "go.dokimi.dev/eidos/sdk/plugin"

// Target names the rendering target a plan resolves to reach the
// Go backend.
const Target plugin.Target = "golang"

// Name is the backend's plugin identity: what its findings are
// reported under and what a composition schedules it by.
const Name plugin.ID = "golang"

// Extension is the suffix every Go file carries.
const Extension = ".go"

// Syntax is Go's comment forms, declared once and shared: the
// frontend strips comments with it, and the output contract writes
// the generated-file frame through it.
func Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{
		Line:   []string{"//"},
		Blocks: []plugin.CommentBlock{{Open: "/*", Close: "*/"}},
	}
}
