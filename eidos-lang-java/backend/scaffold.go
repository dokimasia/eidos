// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
)

// grammar is Java's statement syntax, indented four spaces, the
// convention the ecosystem's formatters settle on. Every statement
// ends with a semicolon, and a declaring assignment binds with var.
// Java's failures throw, so the grammar has neither a tuple nor a
// guard: a delegate's second result arrives thrown, and an
// assignment binding several names is refused. A guard without
// actions spells nothing, because the throw propagates the failure,
// and a guard with actions is refused.
var grammar = scaffold.Grammar{
	Lang:    string(java.Lang),
	Indent:  "    ",
	End:     ";",
	Declare: "var ",
}

// Scaffold spells one statement of the neutral vocabulary as Java,
// through [grammar] and a target that records imports into set.
//
// A scaffolding name resolves locally by construction, so no name
// records an import. A value in a statement does: its references and its
// callees name classes the writing file may not import yet, and each
// spelling records into the set. A statement or a value Java has no
// form for returns an error, and the render skips that declaration
// and keeps the file.
func Scaffold(s emit.Stmt, set *render.ImportSet) ([]byte, error) {
	return scaffold.Scaffold(grammar, s, target{set: set})
}
