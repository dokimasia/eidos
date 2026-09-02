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
// file's position table, the raw bytes for verbatim spellings, the
// unit's spelling intern, and the comment groups a declaration
// consumed, so the sweep can refuse a carrier left floating between
// declarations.
type lowered struct {
	file     *token.File
	src      []byte
	intern   map[string]string
	consumed map[*ast.CommentGroup]bool
	// underlyings defer the defined types' shape stamps until the
	// enum promotion has decided who stands for each type.
	underlyings []pendingUnderlying
}

// at converts one token position through the file's own table.
func (l *lowered) at(p token.Pos) position.Pos {
	if !p.IsValid() {
		return position.Pos{File: l.file.Name()}
	}
	pos := l.file.Position(p)
	return position.Pos{File: pos.Filename, Line: pos.Line, Col: pos.Column}
}

// offset returns a position's byte offset, and -1 for one outside
// the file, which a recovered parse can synthesize.
func (l *lowered) offset(p token.Pos) int {
	base := token.Pos(l.file.Base())
	if p < base || p > base+token.Pos(l.file.Size()) {
		return -1
	}
	return l.file.Offset(p)
}

// spelling returns an expression's verbatim source text, interned
// so a spelling the unit repeats is one string.
func (l *lowered) spelling(e ast.Expr) string {
	from, to := l.offset(e.Pos()), l.offset(e.End())
	if from < 0 || to < 0 || to > len(l.src) || from >= to {
		return ""
	}
	text := l.src[from:to]
	if s, held := l.intern[string(text)]; held {
		return s
	}
	s := string(text)
	l.intern[s] = s
	return s
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
	case *ast.ParenExpr:
		// The parentheses are punctuation the reference must not
		// wear: `type P (int)` names int.
		return l.typeRef(t.X)
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
		case *ast.ParenExpr:
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
