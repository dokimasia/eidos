// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go/ast"
	"go/token"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
)

// lowered carries what one file's lowering reads everywhere: the
// file set for positions and the raw bytes for verbatim spellings.
type lowered struct {
	fset *token.FileSet
	src  []byte
}

// at converts one token position.
func (l *lowered) at(p token.Pos) position.Pos {
	pos := l.fset.Position(p)
	return position.Pos{File: pos.Filename, Line: pos.Line, Col: pos.Column}
}

// spelling returns an expression's verbatim source text.
func (l *lowered) spelling(e ast.Expr) string {
	from := l.fset.Position(e.Pos()).Offset
	to := l.fset.Position(e.End()).Offset
	if from < 0 || to > len(l.src) || from >= to {
		return ""
	}
	return string(l.src[from:to])
}

// typeRef lowers one type expression: the verbatim spelling, with
// arguments split out for an explicit generic instantiation so the
// reference holds the bare name and the target spells its own
// brackets.
func (l *lowered) typeRef(e ast.Expr) *node.TypeRef {
	if e == nil {
		return nil
	}
	switch t := e.(type) {
	case *ast.IndexExpr:
		return &node.TypeRef{
			Spelling: l.spelling(t.X),
			Pos:      l.at(e.Pos()),
			Args:     []*node.TypeRef{l.typeRef(t.Index)},
		}
	case *ast.IndexListExpr:
		args := make([]*node.TypeRef, 0, len(t.Indices))
		for _, index := range t.Indices {
			args = append(args, l.typeRef(index))
		}
		return &node.TypeRef{Spelling: l.spelling(t.X), Pos: l.at(e.Pos()), Args: args}
	default:
		return &node.TypeRef{Spelling: l.spelling(e), Pos: l.at(e.Pos())}
	}
}

// bareName returns the named type under a receiver expression:
// pointer and instantiation unwrapped, because a method's owner is
// the type's name, never its decoration.
func (l *lowered) bareName(e ast.Expr) string {
	for {
		switch t := e.(type) {
		case *ast.StarExpr:
			e = t.X
		case *ast.IndexExpr:
			e = t.X
		case *ast.IndexListExpr:
			e = t.X
		case *ast.Ident:
			return t.Name
		default:
			return l.spelling(e)
		}
	}
}
