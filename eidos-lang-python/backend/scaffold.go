// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
)

// grammar is the statement syntax of Python: four spaces of indentation, no
// end of a statement, a declaring assignment without a keyword, and the
// names of an assignment of two or more values separated by commas, as
// a, b = pair() unpacks a tuple. Python raises its failures, so a guard
// without statements spells nothing and the scaffold refuses a guard with
// statements.
var grammar = scaffold.Grammar{
	Lang:   string(python.Lang),
	Indent: python.Indent,
	Tuple:  func(names string) string { return names },
}

// Scaffold spells one statement of the neutral vocabulary as Python, through
// [grammar] and a target that imports into set. A value of the statement
// records the imports of its references and its callees into set, and
// spells a type parameter of the callable of the body, or of its host, by
// the name of its type variable of the module. A statement or a value that
// Python has no form for returns an error, and the render skips that
// declaration and keeps the file.
//
// Scaffold allocates what [scaffold.Scaffold] allocates and the spelling of
// each value of the statement.
func Scaffold(st emit.Stmt, set *render.ImportSet) ([]byte, error) {
	return scaffold.Scaffold(grammar, st, target{set: set})
}
