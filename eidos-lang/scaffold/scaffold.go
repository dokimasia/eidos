// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package scaffold spells the parts of the neutral statement
// vocabulary every C-family target writes identically. A name is
// a name and a call is fn(a, b) in Go, TypeScript, Java and Rust
// alike, so the expression writer lives here once; statements
// differ per target and stay in each satellite.
//
// # Dependency position
//
// lang/scaffold imports core/emit and the Go stdlib. Like every
// helper package here, it pulls no grammar machinery.
package scaffold

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/core/emit"
)

// Expr writes one expression of the neutral vocabulary into b.
//
// A name spells itself: the vocabulary admits only locally
// resolving names, so no target qualifies one. A call spells the
// applied expression, an open parenthesis, its arguments joined
// with commas, and a close. An empty name, a call applying no
// function, and a kind nothing declares each return an error
// naming what is missing, and write nothing for it.
func Expr(b *strings.Builder, e emit.Expr) error {
	switch e.Kind {
	case emit.ExprName:
		if e.Name == "" {
			return fmt.Errorf("scaffold: a name expression spells nothing")
		}
		b.WriteString(e.Name)
		return nil
	case emit.ExprCall:
		return call(b, e)
	default:
		return fmt.Errorf("scaffold: no spelling for the %s expression", e.Kind)
	}
}

// call writes an application of one expression to its arguments.
func call(b *strings.Builder, e emit.Expr) error {
	if e.Fn == nil {
		return fmt.Errorf("scaffold: a call applies no function")
	}
	if err := Expr(b, *e.Fn); err != nil {
		return err
	}
	b.WriteByte('(')
	for i, arg := range e.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		if err := Expr(b, arg); err != nil {
			return err
		}
	}
	b.WriteByte(')')
	return nil
}
