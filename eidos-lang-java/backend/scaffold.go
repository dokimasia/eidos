// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
)

// indent is one level of Java indentation: four spaces, the
// convention the ecosystem's formatters settle on.
const indent = "    "

// Scaffold spells one statement of the neutral vocabulary as
// Java, and refuses what Java cannot say. An assignment binding
// several names has no Java form, because nothing destructures: a
// delegate's second result arrives thrown, not returned. A guard
// spells nothing at all, for the same reason from the other side:
// a thrown failure propagates natively, so the guard is the
// propagation and the propagation is silence.
//
// A scaffolding name resolves locally by construction, so no name
// records an import. A carried value does: its references and its
// callees name packages the writing file may not import yet, and
// each spelling records into the set. A statement Java has no form for
// returns an error, and the render skips that declaration rather
// than the file.
func Scaffold(s emit.Stmt, set *render.ImportSet) ([]byte, error) {
	var b strings.Builder
	if err := statement(&b, s, 1, target{set: set}); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// statement writes one statement at depth levels of indentation.
func statement(b *strings.Builder, s emit.Stmt, depth int, t target) error {
	switch s.Kind {
	case emit.StmtReturn:
		b.WriteString(strings.Repeat(indent, depth))
		return returnStmt(b, s, t)
	case emit.StmtAssign:
		return assignStmt(b, s, depth, t)
	case emit.StmtExpr:
		b.WriteString(strings.Repeat(indent, depth))
		if err := scaffold.Expr(b, s.Value, t); err != nil {
			return err
		}
		b.WriteString(";\n")
		return nil
	case emit.StmtGuard:
		// A thrown failure propagates on its own, so a bare guard
		// renders as that propagation, which is nothing at all. A
		// guard carrying its own actions has no Java spelling —
		// there is no failure value to test — and dropping the
		// actions would narrow silently, so it refuses.
		if len(s.Then) > 0 {
			return fmt.Errorf(
				"java: a guard's actions have no spelling where failures throw, "+
					"and this one states %d statements", len(s.Then),
			)
		}
		return nil
	default:
		return fmt.Errorf("java: no spelling for the %s statement", s.Kind)
	}
}

// returnStmt writes a return, bare where it carries no value.
func returnStmt(b *strings.Builder, s emit.Stmt, t target) error {
	b.WriteString("return")
	if s.Value.Kind != 0 {
		b.WriteByte(' ')
		if err := scaffold.Expr(b, s.Value, t); err != nil {
			return err
		}
	}
	b.WriteString(";\n")
	return nil
}

// assignStmt writes an assignment. A declaration binds with var;
// several names are refused, because Java destructures nothing.
func assignStmt(b *strings.Builder, s emit.Stmt, depth int, t target) error {
	if len(s.Names) == 0 {
		return fmt.Errorf("java: an assignment binds no name")
	}
	if len(s.Names) > 1 {
		return fmt.Errorf(
			"java: an assignment binds %d names, and Java destructures none: "+
				"a delegate's second result arrives thrown, not returned",
			len(s.Names),
		)
	}
	b.WriteString(strings.Repeat(indent, depth))
	if s.Declare {
		b.WriteString("var ")
	}
	b.WriteString(s.Names[0])
	b.WriteString(" = ")
	if err := scaffold.Expr(b, s.Value, t); err != nil {
		return err
	}
	b.WriteString(";\n")
	return nil
}
