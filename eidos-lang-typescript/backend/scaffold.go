// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
)

// grammar is TypeScript's statement syntax, indented two spaces, the
// convention the ecosystem's formatters settle on. Every statement
// ends with a semicolon, and a declaring assignment binds with const.
// Several names destructure as an array, because a delegate returning
// several values returns a tuple. A guard is the truthiness check,
// because a failure bound to a name is non-null exactly when it
// happened.
var grammar = scaffold.Grammar{
	Lang:    string(typescript.Lang),
	Indent:  "  ",
	End:     ";",
	Declare: "const ",
	Tuple:   func(names string) string { return "[" + names + "]" },
	Guard:   func(name string) string { return "if (" + name + ")" },
}

// Scaffold spells one statement of the neutral vocabulary as
// TypeScript, through [grammar] and a target that records imports
// into set.
//
// A scaffolding name resolves locally by construction, so no name
// records an import. A value in a statement does: its references and its
// callees name modules the writing file may not import yet, and each
// spelling records into the set. A statement or a value TypeScript
// has no form for returns an error, and the render skips that
// declaration and keeps the file.
//
// Scaffold allocates what [scaffold.Scaffold] allocates: the buffer a
// statement writes into, one allocation for a statement of up to 64
// bytes, and the spelling of each value it contains. An assignment of
// more than one name allocates the comma-joined names and their array
// pattern, and a guard allocates its truthiness check.
func Scaffold(s emit.Stmt, set *render.ImportSet) ([]byte, error) {
	return scaffold.Scaffold(grammar, s, target{set: set})
}
