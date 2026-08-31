// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/lang/scaffold"
)

// indent is one level of Go indentation. Bodies are written at
// one level and gofmt settles the rest, so this is the depth a
// statement starts at rather than a claim about the final text.
const indent = "\t"

// Scaffold spells one statement of the neutral vocabulary as Go.
//
// It records no import: a scaffolding name resolves locally by
// construction, and a name needing qualification is a type
// spelling rather than a statement. The set stays in the
// signature for a spelling that delegates to a runtime helper,
// which this one never does. A statement Go has no form for
// returns an error, and the render skips that declaration rather
// than the file.
func Scaffold(s emit.Stmt, _ *render.ImportSet) ([]byte, error) {
	var b strings.Builder
	if err := statement(&b, s, 1); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// statement writes one statement at depth levels of indentation.
func statement(b *strings.Builder, s emit.Stmt, depth int) error {
	b.WriteString(strings.Repeat(indent, depth))
	switch s.Kind {
	case emit.StmtReturn:
		return returnStmt(b, s)
	case emit.StmtAssign:
		return assignStmt(b, s)
	case emit.StmtExpr:
		if err := scaffold.Expr(b, s.Value); err != nil {
			return err
		}
		b.WriteByte('\n')
		return nil
	case emit.StmtGuard:
		return guardStmt(b, s, depth)
	default:
		return fmt.Errorf("golang: no spelling for the %s statement", s.Kind)
	}
}

// returnStmt writes a return, bare where it carries no value.
func returnStmt(b *strings.Builder, s emit.Stmt) error {
	b.WriteString("return")
	if s.Value.Kind != 0 {
		b.WriteByte(' ')
		if err := scaffold.Expr(b, s.Value); err != nil {
			return err
		}
	}
	b.WriteByte('\n')
	return nil
}

// assignStmt writes an assignment, declaring its names where the
// statement says to.
func assignStmt(b *strings.Builder, s emit.Stmt) error {
	if len(s.Names) == 0 {
		return fmt.Errorf("golang: an assignment binds no name")
	}
	b.WriteString(strings.Join(s.Names, ", "))
	if s.Declare {
		b.WriteString(" := ")
	} else {
		b.WriteString(" = ")
	}
	if err := scaffold.Expr(b, s.Value); err != nil {
		return err
	}
	b.WriteByte('\n')
	return nil
}

// guardStmt writes a failure guard. Go propagates a failure by
// hand, so the guard is the nil comparison every caller writes.
func guardStmt(b *strings.Builder, s emit.Stmt, depth int) error {
	if s.Name == "" {
		return fmt.Errorf("golang: a guard names no value to test")
	}
	b.WriteString("if ")
	b.WriteString(s.Name)
	b.WriteString(" != nil {\n")
	for _, then := range s.Then {
		if err := statement(b, then, depth+1); err != nil {
			return err
		}
	}
	b.WriteString(strings.Repeat(indent, depth))
	b.WriteString("}\n")
	return nil
}
