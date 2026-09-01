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

// indent is one level of TypeScript indentation: two spaces, the
// convention the ecosystem's formatters settle on.
const indent = "  "

// Scaffold spells one statement of the neutral vocabulary as
// TypeScript. Every statement closes with a semicolon, and a
// multi-name assignment destructures, because a delegate
// returning several values arrives as a tuple.
//
// It records no import: a scaffolding name resolves locally by
// construction, and a name needing qualification is a type
// spelling rather than a statement. The set stays in the
// signature for a spelling that delegates to a runtime helper,
// which this one never does. A statement TypeScript has no form for
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
		b.WriteString(";\n")
		return nil
	case emit.StmtGuard:
		return guardStmt(b, s, depth)
	default:
		return fmt.Errorf("typescript: no spelling for the %s statement", s.Kind)
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
	b.WriteString(";\n")
	return nil
}

// assignStmt writes an assignment. A declaration binds with
// const, and several names destructure the value as a tuple.
func assignStmt(b *strings.Builder, s emit.Stmt) error {
	if len(s.Names) == 0 {
		return fmt.Errorf("typescript: an assignment binds no name")
	}
	targets := s.Names[0]
	if len(s.Names) > 1 {
		targets = "[" + strings.Join(s.Names, ", ") + "]"
	}
	if s.Declare {
		b.WriteString("const ")
	}
	b.WriteString(targets)
	b.WriteString(" = ")
	if err := scaffold.Expr(b, s.Value); err != nil {
		return err
	}
	b.WriteString(";\n")
	return nil
}

// guardStmt writes a failure guard: the truthiness check, because
// a failure bound to a name is non-null exactly when it happened.
func guardStmt(b *strings.Builder, s emit.Stmt, depth int) error {
	if s.Name == "" {
		return fmt.Errorf("typescript: a guard names no value to test")
	}
	b.WriteString("if (")
	b.WriteString(s.Name)
	b.WriteString(") {\n")
	for _, then := range s.Then {
		if err := statement(b, then, depth+1); err != nil {
			return err
		}
	}
	b.WriteString(strings.Repeat(indent, depth))
	b.WriteString("}\n")
	return nil
}
