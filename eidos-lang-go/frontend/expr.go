// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go/ast"
	"go/token"
	"sort"
	"strconv"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// lowered is the state of the lowering of one file. It has the position
// table of the file, its bytes for verbatim spellings, the spelling intern
// of the unit, the comment groups that a declaration took and the import
// scope of the file. The sweep refuses the carriers of the comment groups
// that no declaration took. A qualified reference records the package that
// its qualifier binds in the import scope.
type lowered struct {
	file     *token.File
	src      []byte
	intern   map[string]string
	consumed map[*ast.CommentGroup]bool
	bindings golang.Bindings
	// comments is every comment group of the file. The parser attaches the
	// trailing comment on the line of a closing brace to no node, so this
	// list is the only one that has it.
	comments []*ast.CommentGroup
	// underlyings are the shape stamps of the defined types. They wait
	// until the enum promotion decides which declaration replaces each
	// type.
	underlyings []pendingUnderlying
	// std reports whether the file is in the standard library. A file of
	// the standard library imports the packages that it vendors from
	// vendor/.
	std bool
}

// at returns the position of a token position of the file.
func (l *lowered) at(p token.Pos) position.Pos {
	if !p.IsValid() {
		return position.Pos{File: l.file.Name()}
	}
	pos := l.file.Position(p)
	return position.Pos{File: pos.Filename, Line: pos.Line, Col: pos.Column}
}

// offset returns the byte offset of a token position, and -1 for a
// position outside the file. A recovered parse can synthesize such a
// position.
func (l *lowered) offset(p token.Pos) int {
	base := token.Pos(l.file.Base())
	if p < base || p > base+token.Pos(l.file.Size()) {
		return -1
	}
	return l.file.Offset(p)
}

// spelling returns the verbatim source text of an expression. It interns
// the text, so a spelling that the unit repeats is one string.
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

// typeRef lowers one type expression to a reference with its verbatim
// spelling, the structure that the grammar of Go states, and its children
// in the fixed order of the form. It returns nil for a nil expression.
//
//   - An explicit instantiation has the bare name as its spelling and its
//     arguments in Args, so a target writes its own brackets.
//   - A pointer is Optional, and a slice is a List.
//   - A sized array is an Array with its literal length.
//   - A map is a Map, and a channel is a Stream. The spelling keeps the
//     direction of the channel.
//   - A function type is a Func of its parameters and then its results.
//   - An inline struct or interface body is Inline. [lowered.inlineFields]
//     and [lowered.inlineMethods] record its members, and its children
//     are the types that the body states.
//   - A union constraint is a Union of its terms, and an approximation
//     outside a union is a Union of one term. The spelling keeps the
//     approximation marks, as [lowered.appendTerms] describes.
//   - A variadic parameter inside a function type is a List.
//   - A qualified name records the import path that its qualifier binds,
//     as [lowered.packageOf] reads it.
func (l *lowered) typeRef(e ast.Expr) *node.TypeRef {
	if e == nil {
		return nil
	}
	switch t := e.(type) {
	case *ast.ParenExpr:
		// The reference drops the parentheses: `type P (int)` refers to
		// int.
		return l.typeRef(t.X)
	case *ast.IndexExpr:
		return &node.TypeRef{
			Spelling: l.spelling(t.X),
			Package:  l.packageOf(t.X),
			Pos:      l.at(e.Pos()),
			Args:     []*node.TypeRef{l.typeRef(t.Index)},
		}
	case *ast.IndexListExpr:
		args := make([]*node.TypeRef, 0, len(t.Indices))
		for _, index := range t.Indices {
			args = append(args, l.typeRef(index))
		}
		return &node.TypeRef{
			Spelling: l.spelling(t.X), Package: l.packageOf(t.X), Pos: l.at(e.Pos()), Args: args,
		}
	case *ast.SelectorExpr:
		return &node.TypeRef{Spelling: l.spelling(e), Package: l.packageOf(t), Pos: l.at(e.Pos())}
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
		return plugin.FuncType(l.spelling(e), l.at(e.Pos()), l.fieldTypes(t.Params), l.fieldTypes(t.Results))
	case *ast.StructType:
		fields, children := l.inlineFields(t)
		return &node.TypeRef{
			Spelling: l.spelling(e), Pos: l.at(e.Pos()), Form: symbol.FormInline, Fields: fields, Elems: children,
		}
	case *ast.InterfaceType:
		methods, children := l.inlineMethods(t)
		return &node.TypeRef{
			Spelling: l.spelling(e), Pos: l.at(e.Pos()), Form: symbol.FormInline, Methods: methods, Elems: children,
		}
	case *ast.BinaryExpr, *ast.UnaryExpr:
		return plugin.Composite(l.spelling(e), l.at(e.Pos()), symbol.FormUnion, l.appendTerms(nil, e)...)
	default:
		return &node.TypeRef{Spelling: l.spelling(e), Pos: l.at(e.Pos())}
	}
}

// appendTerms appends the terms of a constraint element to out, in the
// order of the source. go/parser writes a union as a binary expression of
// the operator | and an approximation as a unary expression of ~, and no
// other operator in a type. A union of unions and a term in parentheses
// flatten into their terms. The term of an approximation is the type
// after its mark, and the spelling of the element keeps the mark.
func (l *lowered) appendTerms(out []*node.TypeRef, e ast.Expr) []*node.TypeRef {
	switch t := e.(type) {
	case *ast.ParenExpr:
		return l.appendTerms(out, t.X)
	case *ast.BinaryExpr:
		return l.appendTerms(l.appendTerms(out, t.X), t.Y)
	case *ast.UnaryExpr:
		return append(out, l.typeRef(t.X))
	default:
		return append(out, l.typeRef(e))
	}
}

// inlineFields lowers the named fields of an inline struct body, and the
// type of each entry of the body as a child of the reference, in order.
// A field has no identity and no comment, and otherwise lowers as the
// field of a declared struct. The sweep refuses the carriers in its
// comments. A blank field binds nothing, and an embedded field is not a
// field of the model, so the fields leave both out. The children and the
// spelling of the reference keep them. Each field of an entry has a copy
// of the child of the entry, which [node.CloneRef] returns, because a node
// of the graph has one place.
func (l *lowered) inlineFields(t *ast.StructType) (fields []*node.Field, children []*node.TypeRef) {
	for _, field := range t.Fields.List {
		typ := l.typeRef(field.Type)
		children = append(children, typ)
		for _, name := range field.Names {
			if !named(name) {
				continue
			}
			fields = append(fields, &node.Field{
				Name: name.Name, Pos: l.at(name.Pos()), Visibility: visibilityOf(name.Name),
				Type: node.CloneRef(typ), Tag: tagOf(field.Tag),
			})
		}
	}
	return fields, children
}

// inlineMethods lowers the methods of an inline interface body, and the
// types that the body states as the children of the reference, in the
// order of its elements. A method is abstract and has no identity and no
// comment, and otherwise lowers as the method of a declared interface.
// The sweep refuses the carriers in its comments. The methods leave out an
// embedded interface, a constraint element and a blank method. The
// children have the type of each embed and each constraint element, and
// copies of the parameter types and the result types of each method, a
// blank method included. [node.CloneRef] returns each copy, because a node
// of the graph has one place.
func (l *lowered) inlineMethods(t *ast.InterfaceType) (methods []*node.Method, children []*node.TypeRef) {
	for _, member := range t.Methods.List {
		sig, is := member.Type.(*ast.FuncType)
		if !is || len(member.Names) == 0 {
			children = append(children, l.typeRef(member.Type))
			continue
		}
		name := member.Names[0].Name
		m := &node.Method{
			Name: name, Pos: l.at(member.Pos()), Visibility: visibilityOf(name), Abstract: true,
		}
		for _, field := range sig.Params.List {
			m.Params = l.appendParams(m.Params, field, "")
		}
		if sig.Results != nil {
			for _, field := range sig.Results.List {
				m.Returns = l.appendReturns(m.Returns, field, "")
			}
		}
		for _, p := range m.Params {
			children = append(children, node.CloneRef(p.Type))
		}
		for _, r := range m.Returns {
			children = append(children, node.CloneRef(r.Type))
		}
		if named(member.Names[0]) {
			methods = append(methods, m)
		}
	}
	return methods, children
}

// packageOf returns the import path that [golang.Bindings.Import] returns
// for the qualifier of a qualified type name.
// It returns the empty string for an unqualified name, which the package
// of the file or a dot import declares. It also returns the empty string
// for a qualifier that no import binds, such as the qualifier of a package
// whose clause declares a name other than its assumed name.
func (l *lowered) packageOf(e ast.Expr) string {
	sel, qualified := e.(*ast.SelectorExpr)
	if !qualified {
		return ""
	}
	qualifier, is := sel.X.(*ast.Ident)
	if !is {
		return ""
	}
	path, _ := l.bindings.Import(qualifier.Name)
	return path
}

// structural lowers a composite with the expressions as its children, in
// order. A child that the source does not state is an empty reference, as
// [plugin.Composite] builds it.
func (l *lowered) structural(e ast.Expr, form symbol.TypeForm, children ...ast.Expr) *node.TypeRef {
	elems := make([]*node.TypeRef, 0, len(children))
	for _, child := range children {
		elems = append(elems, l.typeRef(child))
	}
	return plugin.Composite(l.spelling(e), l.at(e.Pos()), form, elems...)
}

// fieldTypes lowers the types of a field list, one for each bound name
// and one for an unnamed entry, in the order of the signature.
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

// arrayLength returns the length of an array whose length is an integer
// literal. The literal can have any form of an integer literal of Go.
// arrayLength returns 0 for a length that the spelling keeps as an
// expression, such as an ellipsis.
func arrayLength(e ast.Expr) int {
	lit, is := e.(*ast.BasicLit)
	if !is || lit.Kind != token.INT {
		return 0
	}
	n, err := strconv.ParseInt(lit.Value, 0, strconv.IntSize)
	if err != nil || n < 0 {
		return 0
	}
	return int(n)
}

// trailing returns the comment group that opens on the last line of a
// declaration, after end and before limit where limit is valid. Such a
// group is the trailing comment of a function or a method, which go/ast
// attaches to nothing. It is also the trailing comment of a parameter,
// whose list ends at limit with its closing parenthesis. A group that
// another declaration took is not a candidate, and trailing returns nil
// where no group is. The comments of the file are in source order, so the
// search starts at the first group after end.
func (l *lowered) trailing(end, limit token.Pos) *ast.CommentGroup {
	if !end.IsValid() {
		return nil
	}
	line := l.file.Line(end)
	first := sort.Search(len(l.comments), func(i int) bool { return l.comments[i].Pos() > end })
	for _, group := range l.comments[first:] {
		if l.consumed[group] {
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

// leading returns the comment group that ends on the line above start and
// opens after after, where after is valid. Such a group is the leading
// comment of a parameter or a result in a list with one member per line,
// which go/parser attaches to nothing. after is the end of the member
// before, or the opening parenthesis of the list. A group that another
// declaration took is not a candidate, and leading returns nil where no
// group is.
func (l *lowered) leading(after, start token.Pos) *ast.CommentGroup {
	if !after.IsValid() {
		return nil
	}
	above := l.file.Line(start) - 1
	first := sort.Search(len(l.comments), func(i int) bool { return l.comments[i].Pos() > after })
	for _, group := range l.comments[first:] {
		if group.Pos() >= start {
			return nil
		}
		if !l.consumed[group] && l.file.Line(group.End()) == above {
			return group
		}
	}
	return nil
}

// bareName returns the name of the type of a receiver expression without
// its pointer and its instantiation, because the owner of a method is the
// name of the type.
func (l *lowered) bareName(e ast.Expr) string {
	e = undecorated(e)
	if id, is := e.(*ast.Ident); is {
		return id.Name
	}
	return l.spelling(e)
}

// undecorated returns the expression of a type without its pointer, its
// parentheses and its instantiation.
func undecorated(e ast.Expr) ast.Expr {
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
		default:
			return e
		}
	}
}
