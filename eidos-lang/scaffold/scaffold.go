// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"bytes"
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/sdk/emit"
)

// The tokens every C-family target spells a statement with.
const (
	returnKeyword = "return"
	assignOp      = " = "
	nameSep       = ", "
	blockOpen     = " {\n"
	blockClose    = "}\n"
)

// Grammar is one C-family target's statement syntax: the tokens in
// which the scaffolding statements differ between targets. [Scaffold]
// writes every statement through it, so a target states its tokens
// once and never the walk.
type Grammar struct {
	// Lang names the target in every refusal.
	Lang string
	// Indent is one level of indentation.
	Indent string
	// End closes a return, an assignment and an expression statement
	// before the line break: ";" for a target whose statements take
	// one, empty for one whose line break ends them.
	End string
	// Declare opens a declaring assignment, such as "let ".
	Declare string
	// DeclareOp binds the names of a declaring assignment to the
	// value, such as " := ". Empty means " = ", the operator of every
	// plain assignment.
	DeclareOp string
	// Tuple spells the names of an assignment binding more than one,
	// passed comma-joined, such as "[res, err]". Nil means the target
	// binds one name per assignment and refuses more than one.
	Tuple func(names string) string
	// Guard spells the condition that opens a failure guard on the
	// named value, such as "if err != nil". Nil means the target's
	// failures throw: a guard without actions spells nothing, because
	// the throw propagates the failure, and a guard with actions is
	// refused.
	Guard func(name string) string
}

// Scaffold spells one statement of the neutral vocabulary in grammar
// g, at one level of indentation, with t spelling the values its
// expressions contain.
//
// A statement or an expression the grammar has no form for returns
// an error, and the render skips that declaration and keeps the
// file. The returned bytes are the buffer's own: Scaffold keeps no
// reference to them.
func Scaffold(g Grammar, s emit.Stmt, t Target) ([]byte, error) {
	var b bytes.Buffer
	if err := statement(&b, g, s, 1, t); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// statement writes one statement at depth levels of indentation.
func statement(b *bytes.Buffer, g Grammar, s emit.Stmt, depth int, t Target) error {
	switch s.Kind {
	case emit.StmtReturn:
		b.WriteString(strings.Repeat(g.Indent, depth))
		b.WriteString(returnKeyword)
		if s.Value.Kind != 0 {
			b.WriteByte(' ')
			if err := Expr(b, s.Value, t); err != nil {
				return err
			}
		}
	case emit.StmtAssign:
		if err := assignment(b, g, s, depth, t); err != nil {
			return err
		}
	case emit.StmtExpr:
		b.WriteString(strings.Repeat(g.Indent, depth))
		if err := Expr(b, s.Value, t); err != nil {
			return err
		}
	case emit.StmtGuard:
		return guard(b, g, s, depth, t)
	default:
		return fmt.Errorf("%s: no spelling for the %s statement", g.Lang, s.Kind)
	}
	b.WriteString(g.End)
	b.WriteByte('\n')
	return nil
}

// assignment writes an assignment without its end: the declaring
// token where the statement declares, one name or the grammar's
// tuple of more than one, the operator and the value.
func assignment(b *bytes.Buffer, g Grammar, s emit.Stmt, depth int, t Target) error {
	var names string
	switch {
	case len(s.Names) == 0:
		return fmt.Errorf("%s: an assignment binds no name", g.Lang)
	case len(s.Names) == 1:
		names = s.Names[0]
	case g.Tuple == nil:
		return fmt.Errorf("%s: an assignment binds %d names, and %s binds one per assignment",
			g.Lang, len(s.Names), g.Lang)
	default:
		names = g.Tuple(strings.Join(s.Names, nameSep))
	}
	b.WriteString(strings.Repeat(g.Indent, depth))
	op := assignOp
	if s.Declare {
		b.WriteString(g.Declare)
		if g.DeclareOp != "" {
			op = g.DeclareOp
		}
	}
	b.WriteString(names)
	b.WriteString(op)
	return Expr(b, s.Value, t)
}

// guard writes a failure guard: the grammar's condition on the named
// value, and the guard's actions one level deeper inside a block.
func guard(b *bytes.Buffer, g Grammar, s emit.Stmt, depth int, t Target) error {
	if g.Guard == nil {
		if len(s.Then) > 0 {
			return fmt.Errorf("%s: a guard states %d statements, and %s throws its failures, "+
				"so no value is left to test", g.Lang, len(s.Then), g.Lang)
		}
		return nil
	}
	if s.Name == "" {
		return fmt.Errorf("%s: a guard names no value to test", g.Lang)
	}
	b.WriteString(strings.Repeat(g.Indent, depth))
	b.WriteString(g.Guard(s.Name))
	b.WriteString(blockOpen)
	for _, then := range s.Then {
		if err := statement(b, g, then, depth+1, t); err != nil {
			return err
		}
	}
	b.WriteString(strings.Repeat(g.Indent, depth))
	b.WriteString(blockClose)
	return nil
}

// Expr writes one expression of the neutral vocabulary into b,
// through the target that spells the values it contains.
//
// A name spells as itself, because the vocabulary admits only
// locally resolving names and no target qualifies one. A call spells
// the applied expression, an open parenthesis, its arguments joined
// with commas, and a close. A value goes to the target, which spells
// its tree and records the imports its references need. An empty
// name, a call applying no function, a value expression without a
// value, and a kind nothing declares each return an error naming
// what is missing.
func Expr(b *bytes.Buffer, e emit.Expr, t Target) error {
	switch e.Kind {
	case emit.ExprName:
		if e.Name == "" {
			return fmt.Errorf("scaffold: a name expression spells nothing")
		}
		b.WriteString(e.Name)
		return nil
	case emit.ExprCall:
		return call(b, e, t)
	case emit.ExprValue:
		return value(b, e, t)
	default:
		return fmt.Errorf("scaffold: no spelling for the %s expression", e.Kind)
	}
}

// value writes the value an expression contains through the target.
func value(b *bytes.Buffer, e emit.Expr, t Target) error {
	if e.Val == nil {
		return fmt.Errorf("scaffold: a value expression carries no value")
	}
	if t == nil {
		return fmt.Errorf("scaffold: a value expression needs a target to spell it")
	}
	spelled, err := Value(t, *e.Val)
	if err != nil {
		return err
	}
	b.WriteString(spelled)
	return nil
}

// call writes an application of one expression to its arguments.
func call(b *bytes.Buffer, e emit.Expr, t Target) error {
	if e.Fn == nil {
		return fmt.Errorf("scaffold: a call applies no function")
	}
	if err := Expr(b, *e.Fn, t); err != nil {
		return err
	}
	b.WriteByte('(')
	for i, arg := range e.Args {
		if i > 0 {
			b.WriteString(nameSep)
		}
		if err := Expr(b, arg, t); err != nil {
			return err
		}
	}
	b.WriteByte(')')
	return nil
}
