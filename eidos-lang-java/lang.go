// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package java

import "go.dokimi.dev/eidos/core/plugin"

// Target names the rendering target a plan resolves to reach the
// Java backend.
const Target plugin.Target = "java"

// Name is the backend's plugin identity: what its findings are
// reported under and what a composition schedules it by.
const Name plugin.ID = "java"

// Extension is the suffix every Java file carries.
const Extension = ".java"

// Syntax is Java's comment forms, declared once and shared: the
// frontend strips comments with it, and the output contract
// writes the generated-file frame through it. The doc-block form
// carries the star gutter Javadoc continuation lines use.
func Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{
		Line: []string{"//"},
		Blocks: []plugin.CommentBlock{
			{Open: "/*", Close: "*/"},
			{Open: "/**", Close: "*/", Gutter: "*"},
		},
	}
}
