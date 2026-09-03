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

// indent is one level of Rust indentation: four spaces, which is
// what rustfmt writes.
const indent = "    "

// Scaffold spells one statement of the neutral vocabulary as
// Rust. A declaration binds with let, several names destructure
// as a tuple, and a guard tests the bound value for failure the
// way a caller holding a Result writes it.
//
// A scaffolding name resolves locally by construction, so no name
// records an import. A carried value does: its references and its
// callees name packages the writing file may not import yet, and
// each spelling records into the set. A statement Rust has no form for
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
		b.WriteString(";\n")
		return nil
	case emit.StmtGuard:
		return guardStmt(b, s, depth, t)
	default:
		return fmt.Errorf("rust: no spelling for the %s statement", s.Kind)
	}
}

// returnStmt writes a return, bare where it carries no value. The
// explicit form is spelled rather than a tail expression, because
// a scaffolding statement cannot know it is last.
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

// assignStmt writes an assignment. A declaration binds with let,
// and several names destructure the value as a tuple.
func assignStmt(b *strings.Builder, s emit.Stmt, t target) error {
	if len(s.Names) == 0 {
		return fmt.Errorf("rust: an assignment binds no name")
	}
	targets := s.Names[0]
	if len(s.Names) > 1 {
		targets = "(" + strings.Join(s.Names, ", ") + ")"
	}
	if s.Declare {
		b.WriteString("let ")
	}
	b.WriteString(targets)
	b.WriteString(" = ")
	if err := scaffold.Expr(b, s.Value, t); err != nil {
		return err
	}
	b.WriteString(";\n")
	return nil
}

// guardStmt writes a failure guard: the is_err test on the bound
// value, which is what a caller holding a Result writes when it
// handles the failure rather than propagating it.
func guardStmt(b *strings.Builder, s emit.Stmt, depth int, t target) error {
	if s.Name == "" {
		return fmt.Errorf("rust: a guard names no value to test")
	}
	b.WriteString("if ")
	b.WriteString(s.Name)
	b.WriteString(".is_err() {\n")
	for _, then := range s.Then {
		if err := statement(b, then, depth+1, t); err != nil {
			return err
		}
	}
	b.WriteString(strings.Repeat(indent, depth))
	b.WriteString("}\n")
	return nil
}
