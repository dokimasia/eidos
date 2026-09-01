// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rust

import "go.dokimi.dev/eidos/sdk/plugin"

// Target names the rendering target a plan resolves to reach the
// Rust backend.
const Target plugin.Target = "rust"

// Name is the backend's plugin identity: what its findings are
// reported under and what a composition schedules it by.
const Name plugin.ID = "rust"

// Extension is the suffix every Rust file carries.
const Extension = ".rs"

// Syntax is Rust's comment forms, declared once and shared: the
// frontend strips comments with it, and the output contract
// writes the generated-file frame through it. Rust carries three
// line forms; the plain one is canonical, and the outer and inner
// doc forms follow so the frontend strips all three.
func Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{
		Line:   []string{"//", "///", "//!"},
		Blocks: []plugin.CommentBlock{{Open: "/*", Close: "*/"}},
	}
}
