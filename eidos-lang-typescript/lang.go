// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Lang is the source language of every TypeScript declaration, and
// the language the value target spells for.
const Lang symbol.Lang = "typescript"

// CodePrefix opens every diagnostic code the satellite registers:
// TYPESCRIPT-0001 is the first.
const CodePrefix diag.Prefix = "TYPESCRIPT"

// Target names the rendering target a plan resolves to select the
// TypeScript backend.
const Target plugin.Target = "typescript"

// Name is the backend's plugin identity: what its findings are
// reported under and what a composition schedules it by.
const Name plugin.ID = "typescript"

// Extension is the suffix of every TypeScript file.
const Extension = ".ts"

// Version is the backend's behavior version, folded into the
// composition fingerprint: bump it with any change to the rendered
// output.
const Version = "0.5.0"

// FrontendVersion is the frontend's behavior version, folded into
// every unit key the frontend builds beside the grammar's version:
// bump it with any change to the graph a parse produces.
const FrontendVersion = "0.2.0"

// Syntax returns TypeScript's comment forms, declared once and
// shared: the line form, the block form, and the doc-block form with
// the star gutter TSDoc continuation lines use. The output contract
// writes the generated-file header through them.
func Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{
		Line: []string{"//"},
		Blocks: []plugin.CommentBlock{
			{Open: "/*", Close: "*/"},
			{Open: "/**", Close: "*/", Gutter: "*"},
		},
	}
}
