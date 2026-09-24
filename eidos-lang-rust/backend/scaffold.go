// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
)

// grammar is Rust's statement syntax, indented four spaces, which is
// what rustfmt writes. Every statement ends with a semicolon, so a
// return is the explicit form: a scaffolding statement cannot know
// it is last, which a tail expression requires. A declaring
// assignment binds with let, several names destructure as a tuple,
// and a guard is the is_err test a caller with a Result writes when
// it handles the failure itself.
var grammar = scaffold.Grammar{
	Lang:    string(rust.Lang),
	Indent:  "    ",
	End:     ";",
	Declare: "let ",
	Tuple:   func(names string) string { return "(" + names + ")" },
	Guard:   func(name string) string { return "if " + name + ".is_err()" },
}

// Scaffold spells one statement of the neutral vocabulary as Rust,
// through [grammar] and a target that records imports into set.
//
// A scaffolding name resolves locally by construction, so no name
// records an import. A value in a statement does: its references and its
// callees name modules the writing file may not import yet, and each
// spelling records into the set. A statement or a value Rust has no
// form for returns an error, and the render skips that declaration
// and keeps the file.
func Scaffold(s emit.Stmt, set *render.ImportSet) ([]byte, error) {
	return scaffold.Scaffold(grammar, s, target{set: set})
}
