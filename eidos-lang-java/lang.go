// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package java

import (
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Lang is the source language of every Java declaration, and the
// language the value target spells for.
const Lang symbol.Lang = "java"

// Target names the rendering target a plan resolves to reach the
// Java backend.
const Target plugin.Target = "java"

// Name is the backend's plugin identity: what its findings are
// reported under and what a composition schedules it by.
const Name plugin.ID = "java"

// Extension is the suffix of every Java file.
const Extension = ".java"

// Version is the backend's behavior version, folded into the
// composition fingerprint: bump it with any change to the rendered
// output.
const Version = "0.3.0"

// Syntax returns Java's comment forms, declared once and shared: the
// line form, the block form, and the doc-block form with the star
// gutter Javadoc continuation lines use. The output contract writes
// the generated-file header through them.
func Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{
		Line: []string{"//"},
		Blocks: []plugin.CommentBlock{
			{Open: "/*", Close: "*/"},
			{Open: "/**", Close: "*/", Gutter: "*"},
		},
	}
}
