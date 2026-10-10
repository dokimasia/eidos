// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
)

// grammar is Go's statement syntax. A line break ends a statement, a
// declaring assignment binds with :=, several names bind as a list,
// and a guard is the nil comparison every caller writes, because Go
// propagates a failure by hand. Bodies are written one tab deep, and
// gofmt settles the final indentation.
var grammar = scaffold.Grammar{
	Lang:      string(golang.Lang),
	Indent:    "\t",
	DeclareOp: " := ",
	Tuple:     func(names string) string { return names },
	Guard:     func(name string) string { return "if " + name + " != nil" },
}

// Scaffold spells one statement of the neutral vocabulary as Go,
// through [grammar] and a target that records imports into set.
//
// A scaffolding name resolves locally by construction, so no name
// records an import. A value in a statement does: its references and its
// callees name packages the writing file may not import yet, and
// each spelling records into the set. A statement or a value Go has
// no form for returns an error, and the render skips that
// declaration and keeps the file.
//
// Scaffold allocates what [scaffold.Scaffold] allocates: the buffer a
// statement writes into, one allocation for a statement of up to 64
// bytes, and the spelling of each value it contains. An assignment of
// more than one name allocates the comma-joined names, and a guard
// allocates its nil comparison.
func Scaffold(s emit.Stmt, set *render.ImportSet) ([]byte, error) {
	return scaffold.Scaffold(grammar, s, target{set: set})
}
