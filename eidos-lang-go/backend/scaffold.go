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

// indent is one level of Go indentation. Bodies are written at
// one level and gofmt settles the rest, so this is the depth a
// statement starts at rather than a claim about the final text.
const indent = "\t"

// Scaffold spells one statement of the neutral vocabulary as Go.
//
// A scaffolding name resolves locally by construction, so no name
// records an import. A carried value does: its references and its
// callees name packages the writing file may not import yet, and
// each spelling records into the set. A statement or a value Go
// has no form for returns an error, and the render skips that
// declaration rather than the file.
func Scaffold(s emit.Stmt, set *render.ImportSet) ([]byte, error) {
	var b strings.Builder
	if err := statement(&b, s, 1, target{set: set}); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// statement writes one statement at depth levels of indentation.
func statement(b *strings.Builder, s emit.Stmt, depth int, t target) error {
	b.WriteString(strings.Repeat(indent, depth))
	switch s.Kind {
	case emit.StmtReturn:
		return returnStmt(b, s, t)
	case emit.StmtAssign:
		return assignStmt(b, s, t)
	case emit.StmtExpr:
		if err := scaffold.Expr(b, s.Value, t); err != nil {
			return err
		}
		b.WriteByte('\n')
		return nil
	case emit.StmtGuard:
		return guardStmt(b, s, depth, t)
	default:
		return fmt.Errorf("golang: no spelling for the %s statement", s.Kind)
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
	b.WriteByte('\n')
	return nil
}

// assignStmt writes an assignment, declaring its names where the
// statement says to.
func assignStmt(b *strings.Builder, s emit.Stmt, t target) error {
	if len(s.Names) == 0 {
		return fmt.Errorf("golang: an assignment binds no name")
	}
	b.WriteString(strings.Join(s.Names, ", "))
	if s.Declare {
		b.WriteString(" := ")
	} else {
		b.WriteString(" = ")
	}
	if err := scaffold.Expr(b, s.Value, t); err != nil {
		return err
	}
	b.WriteByte('\n')
	return nil
}

// guardStmt writes a failure guard. Go propagates a failure by
// hand, so the guard is the nil comparison every caller writes.
func guardStmt(b *strings.Builder, s emit.Stmt, depth int, t target) error {
	if s.Name == "" {
		return fmt.Errorf("golang: a guard names no value to test")
	}
	b.WriteString("if ")
	b.WriteString(s.Name)
	b.WriteString(" != nil {\n")
	for _, then := range s.Then {
		if err := statement(b, then, depth+1, t); err != nil {
			return err
		}
	}
	b.WriteString(strings.Repeat(indent, depth))
	b.WriteString("}\n")
	return nil
}
