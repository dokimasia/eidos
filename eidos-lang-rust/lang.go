// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rust

import (
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Lang is the source language of every Rust declaration, and the
// language the value target spells for.
const Lang symbol.Lang = "rust"

// CodePrefix opens every diagnostic code the satellite registers:
// RUST-0001 is the first.
const CodePrefix diag.Prefix = "RUST"

// Target names the rendering target a plan resolves to select the
// Rust backend.
const Target plugin.Target = "rust"

// Name is the backend's plugin identity: what its findings are
// reported under and what a composition schedules it by.
const Name plugin.ID = "rust"

// Extension is the suffix of every Rust file.
const Extension = ".rs"

// Version is the backend's behavior version, folded into the
// composition fingerprint: bump it with any change to the rendered
// output.
const Version = "0.8.0"

// FrontendVersion is the frontend's behavior version, folded into
// every unit key the frontend builds beside the grammar's version:
// bump it with any change to the graph a parse produces.
const FrontendVersion = "0.4.0"

// Syntax returns Rust's comment forms, declared once and shared: the
// plain line form, which is canonical, the outer and inner doc line
// forms, the plain block form, and the outer and inner doc block
// forms, whose continuation lines may open with a star gutter. The
// output contract writes the generated-file header through the line
// form, and the frontend strips every form. It allocates the two
// lists, so no caller shares another's.
func Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{
		Line: []string{"//", "///", "//!"},
		Blocks: []plugin.CommentBlock{
			{Open: "/*", Close: "*/"},
			{Open: "/**", Close: "*/", Gutter: "*"},
			{Open: "/*!", Close: "*/", Gutter: "*"},
		},
	}
}
