// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package typescript

import "go.dokimi.dev/eidos/sdk/plugin"

// Target names the rendering target a plan resolves to reach the
// TypeScript backend.
const Target plugin.Target = "typescript"

// Name is the backend's plugin identity: what its findings are
// reported under and what a composition schedules it by.
const Name plugin.ID = "typescript"

// Extension is the suffix every TypeScript file carries.
const Extension = ".ts"

// Syntax is TypeScript's comment forms, declared once and shared:
// the frontend strips comments with it, and the output contract
// writes the generated-file frame through it. The doc-block form
// carries the star gutter TSDoc continuation lines use.
func Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{
		Line: []string{"//"},
		Blocks: []plugin.CommentBlock{
			{Open: "/*", Close: "*/"},
			{Open: "/**", Close: "*/", Gutter: "*"},
		},
	}
}
