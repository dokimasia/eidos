// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rust

import (
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Lang is the source language of every Rust declaration, and the
// language the value target spells for.
const Lang symbol.Lang = "rust"

// Target names the rendering target a plan resolves to reach the
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
const Version = "0.3.0"

// Syntax returns Rust's comment forms, declared once and shared: the
// plain line form, which is canonical, the outer and inner doc line
// forms, and the block form. The output contract writes the
// generated-file header through them.
func Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{
		Line:   []string{"//", "///", "//!"},
		Blocks: []plugin.CommentBlock{{Open: "/*", Close: "*/"}},
	}
}
