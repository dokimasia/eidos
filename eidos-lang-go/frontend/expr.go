// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go/ast"
	"go/token"
	"strconv"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
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
	// comments is the file's whole comment list, for the trailing
	// comment a declaration's closing brace shares a line with,
	// which the parser attaches to nothing.
	comments []*ast.CommentGroup
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

// typeRef lowers one type expression: the verbatim spelling, the
// structure Go's grammar states, and its children in the form's
// fixed order. An explicit generic instantiation keeps the bare
// name in the spelling with its arguments split out, so the target
// spells its own brackets. A pointer is Optional, a slice a List, a
// sized array an Array with its literal length, a map a Map, a
// channel a Stream whose direction stays in the spelling, a
// function type a Func with its parameters then its results, and
// an inline struct or interface body Inline. A variadic parameter
// inside a function type is the list it is.
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
	case *ast.StarExpr:
		return l.structural(e, symbol.FormOptional, t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return l.structural(e, symbol.FormList, t.Elt)
		}
		ref := l.structural(e, symbol.FormArray, t.Elt)
		ref.Length = arrayLength(t.Len)
		return ref
	case *ast.Ellipsis:
		return l.structural(e, symbol.FormList, t.Elt)
	case *ast.MapType:
		return l.structural(e, symbol.FormMap, t.Key, t.Value)
	case *ast.ChanType:
		return l.structural(e, symbol.FormStream, t.Value)
	case *ast.FuncType:
		return l.funcRef(e, t)
	case *ast.StructType, *ast.InterfaceType:
		return &node.TypeRef{Spelling: l.spelling(e), Pos: l.at(e.Pos()), Form: symbol.FormInline}
	default:
		return &node.TypeRef{Spelling: l.spelling(e), Pos: l.at(e.Pos())}
	}
}

// structural lowers a composite whose children are the given
// expressions, in order.
func (l *lowered) structural(e ast.Expr, form symbol.TypeForm, children ...ast.Expr) *node.TypeRef {
	elems := make([]*node.TypeRef, 0, len(children))
	for _, child := range children {
		elems = append(elems, l.typeRef(child))
	}
	return &node.TypeRef{Spelling: l.spelling(e), Pos: l.at(e.Pos()), Form: form, Elems: elems}
}

// funcRef lowers a function type: its parameters then its results
// as children, the split recorded where the results begin.
func (l *lowered) funcRef(e ast.Expr, t *ast.FuncType) *node.TypeRef {
	ref := &node.TypeRef{Spelling: l.spelling(e), Pos: l.at(e.Pos()), Form: symbol.FormFunc}
	ref.Elems = append(ref.Elems, l.fieldTypes(t.Params)...)
	ref.Split = len(ref.Elems)
	ref.Elems = append(ref.Elems, l.fieldTypes(t.Results)...)
	return ref
}

// fieldTypes lowers a field list's types, one per bound name and
// one for an unnamed entry, the way a signature reads.
func (l *lowered) fieldTypes(fields *ast.FieldList) []*node.TypeRef {
	if fields == nil {
		return nil
	}
	var out []*node.TypeRef
	for _, field := range fields.List {
		count := max(len(field.Names), 1)
		for range count {
			out = append(out, l.typeRef(field.Type))
		}
	}
	return out
}

// arrayLength reads a literal array length, and 0 for a length
// the spelling keeps as an expression, an ellipsis included.
func arrayLength(e ast.Expr) int {
	lit, is := e.(*ast.BasicLit)
	if !is || lit.Kind != token.INT {
		return 0
	}
	n, err := strconv.Atoi(lit.Value)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// trailing returns the comment group opening on the line a
// declaration ends on, after its end and before the limit where
// one is given: the trailing comment of a function or method,
// which go/ast leaves free, or of a parameter inside its list,
// whose closing parenthesis is the limit. A group another
// declaration consumed is not a candidate. Nil when none.
func (l *lowered) trailing(end, limit token.Pos) *ast.CommentGroup {
	if !end.IsValid() {
		return nil
	}
	line := l.file.Line(end)
	for _, group := range l.comments {
		if group.Pos() <= end || l.consumed[group] {
			continue
		}
		if limit.IsValid() && group.Pos() >= limit {
			return nil
		}
		at := l.file.Line(group.Pos())
		if at > line {
			return nil
		}
		if at == line {
			return group
		}
	}
	return nil
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
