// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The modules of the forms of the typing specification: typing, whose
// backport typing_extensions the scope reads as typing, the abstract
// collections, the concrete collections, dataclasses and enum.
const (
	typingModule      = "typing"
	typingExtensions  = "typing_extensions"
	abcModule         = "collections/abc"
	collectionsModule = "collections"
	dataclassesMod    = "dataclasses"
	enumModule        = "enum"
)

// The names of the forms of the typing specification that the lowering of
// a type reads, and the forms that are types without arguments.
const (
	optionalForm      = "Optional"
	unionForm         = "Union"
	callableForm      = "Callable"
	tupleAlias        = "Tuple"
	tupleBuiltin      = "tuple"
	annotatedForm     = "Annotated"
	literalForm       = "Literal"
	classVarForm      = "ClassVar"
	finalForm         = "Final"
	requiredForm      = "Required"
	notRequiredForm   = "NotRequired"
	readOnlyForm      = "ReadOnly"
	initVarForm       = "InitVar"
	noneName          = "None"
	unionMark         = "|"
	anyForm           = "Any"
	literalStringForm = "LiteralString"
	neverForm         = "Never"
	noReturnForm      = "NoReturn"
)

// The prefixes of the snippet that parses a string annotation as the
// annotation of an assignment: the snippet of an annotation of one line,
// and the snippet of an annotation of several lines, in parentheses.
const (
	snippet          = "_: "
	parenthesized    = "_: ("
	closeParenthesis = ")"
)

// shift maps the positions of the tree of a string annotation to the
// positions of its content in the file: the file, the line and the column
// of the first byte of the content, and the bytes of the snippet before the
// content on its first line.
type shift struct {
	file   string
	line   int
	col    int
	prefix int
}

// annotation is the lowered annotation of a declaration: its type, the
// qualifiers that wrap the type, the metadata of an Annotated that wraps
// it, and its position. typeAlias reports the annotation TypeAlias, which
// declares an alias and no type. nested reports that the type has an
// Annotated inside another type, and whole is then the whole annotation as
// written.
type annotation struct {
	ref         *node.TypeRef
	classVar    bool
	final       bool
	required    bool
	notRequired bool
	readOnly    bool
	initVar     bool
	typeAlias   bool
	nested      bool
	metadata    []string
	whole       string
	pos         position.Pos
}

// annotationOf lowers the annotation of a declaration. The qualifiers
// ClassVar, Final, Required, NotRequired and ReadOnly, InitVar of
// dataclasses and Annotated wrap the type in any order, each with the type
// as its first argument. A bare Final or ClassVar states no type. A string
// annotation parses as the annotation that it contains, qualifiers
// included.
func (l *lowering) annotationOf(n treesitter.Node) annotation {
	a := annotation{pos: l.pos(n)}
	l.annotated = false
	l.unwrap(l.inner(n), &a)
	if a.nested = l.annotated; a.nested {
		a.whole = n.Compact()
	}
	return a
}

// unwrap lowers the type of an annotation into a, through the qualifiers
// and the strings that wrap it. A name reuses the spelling that the check of
// the qualifiers compacts, so it compacts once.
func (l *lowering) unwrap(n treesitter.Node, a *annotation) {
	if n.Kind() == l.v.stringNode {
		l.inString(n, func(inner treesitter.Node) { l.unwrap(l.inner(inner), a) },
			func() { a.ref = l.named(n, n.Compact()) })
		return
	}
	value, args := l.generic(n)
	plain := value.IsZero()
	if plain {
		value = n
	}
	spelling := value.Compact()
	module, name := l.scope.canonical(spelling)
	var flag *bool
	switch {
	case module == typingModule && name == typeAliasName:
		a.typeAlias = true
		return
	case module == typingModule && name == classVarForm:
		flag = &a.classVar
	case module == typingModule && name == finalForm:
		flag = &a.final
	case module == typingModule && name == requiredForm:
		flag = &a.required
	case module == typingModule && name == notRequiredForm:
		flag = &a.notRequired
	case module == typingModule && name == readOnlyForm:
		flag = &a.readOnly
	case module == dataclassesMod && name == initVarForm:
		flag = &a.initVar
	case module == typingModule && name == annotatedForm && len(args) > 0:
		for _, m := range args[1:] {
			a.metadata = append(a.metadata, m.Compact())
		}
		l.unwrap(l.inner(args[0]), a)
		return
	case plain && (n.Kind() == l.v.identifier || n.Kind() == l.v.attribute || n.Kind() == l.v.memberType):
		a.ref = l.named(n, spelling)
		return
	default:
		a.ref = l.typeRef(n)
		return
	}
	*flag = true
	if len(args) > 0 {
		l.unwrap(l.inner(args[0]), a)
	}
}

// typeRef lowers one type. The spelling of a name is the tokens of its
// source without the whitespace between them, so a reformatted signature
// has one identity. typeRef sets the structural forms that the syntax
// states, and leaves every other type Named with its spelling:
//
//   - A name and a dotted name are Named. A name that an import binds
//     records the module of the import as its package. None is Named.
//   - A subscript of a generic class is Named with its arguments in Args
//     and the spelling of the class.
//   - X | None, Optional[X] and Union[X, None] are Optional of X. Every other
//     union is a Union of its members, and a nested union flattens.
//   - Callable[[A, B], R] is a Func of the parameters and then the result,
//     and a result None is no result. A Callable whose parameters are no
//     list is Named.
//   - tuple[A, B] and Tuple[A, B] are an immutable Tuple, tuple[()] has no
//     member, and tuple[A, ...] is an immutable List of A.
//   - A string annotation is the type that it contains, at the position of
//     its content.
//   - Annotated[T, m] is the type T. The metadata of an Annotated inside
//     another type is in the stamp of the declaration.
//   - Literal[...] is Named with its values as written in Args.
//
// typeRef unwraps the node type and parentheses. It returns nil for the
// zero Node.
func (l *lowering) typeRef(n treesitter.Node) *node.TypeRef {
	if n.IsZero() {
		return nil
	}
	n = l.inner(n)
	switch n.Kind() {
	case l.v.stringNode:
		var ref *node.TypeRef
		l.inString(n, func(inner treesitter.Node) { ref = l.typeRef(inner) }, func() { ref = l.named(n, n.Compact()) })
		return ref
	case l.v.binaryOperator, l.v.unionType:
		if n.Kind() == l.v.unionType || n.Child(l.v.fieldOperator).Text() == unionMark {
			return l.union(n, nil)
		}
	case l.v.subscript, l.v.genericType:
		return l.genericRef(n)
	case l.v.none:
		return &node.TypeRef{Spelling: noneName, Pos: l.pos(n)}
	case l.v.identifier, l.v.attribute, l.v.memberType:
		return l.named(n, n.Compact())
	}
	return &node.TypeRef{Spelling: n.Compact(), Pos: l.pos(n)}
}

// genericRef lowers the subscript of a generic class, and the special forms
// of typing among them.
func (l *lowering) genericRef(n treesitter.Node) *node.TypeRef {
	value, args := l.generic(n)
	spelling := value.Compact()
	module, name := l.scope.canonical(spelling)
	switch {
	case module == typingModule && name == optionalForm && len(args) == 1:
		return plugin.Composite(n.Compact(), l.pos(n), symbol.FormOptional, l.typeRef(args[0]))
	case module == typingModule && name == unionForm:
		return l.union(n, args)
	case (module == typingModule || module == abcModule) && name == callableForm && len(args) == 2 &&
		l.inner(args[0]).Kind() == l.v.list:
		var params []*node.TypeRef
		for p := range l.inner(args[0]).Content() {
			params = append(params, l.typeRef(p))
		}
		var results []*node.TypeRef
		if result := l.inner(args[1]); result.Kind() != l.v.none {
			results = append(results, l.typeRef(result))
		}
		return plugin.FuncType(n.Compact(), l.pos(n), params, results)
	case module == typingModule && name == tupleAlias, module == builtinsModule && name == tupleBuiltin:
		return l.tuple(n, args)
	case module == typingModule && name == annotatedForm && len(args) > 0:
		l.annotated = true
		return l.typeRef(args[0])
	case module == typingModule && name == literalForm:
		ref := l.named(value, spelling)
		ref.Pos = l.pos(n)
		for _, v := range args {
			ref.Args = append(ref.Args, &node.TypeRef{Spelling: v.Compact(), Pos: l.pos(v)})
		}
		return ref
	}
	ref := l.named(value, spelling)
	ref.Pos = l.pos(n)
	for _, a := range args {
		ref.Args = append(ref.Args, l.typeRef(a))
	}
	return ref
}

// tuple lowers the subscript of tuple: an immutable List of the first
// argument where the second is an ellipsis, and an immutable Tuple of the
// arguments otherwise. The empty tuple () as the one argument is a Tuple
// without members.
func (l *lowering) tuple(n treesitter.Node, args []treesitter.Node) *node.TypeRef {
	if len(args) == 2 && l.inner(args[1]).Kind() == l.v.ellipsis {
		list := plugin.Composite(n.Compact(), l.pos(n), symbol.FormList, l.typeRef(args[0]))
		list.Mutability = symbol.MutabilityImmutable
		return list
	}
	elems := make([]*node.TypeRef, 0, len(args))
	if len(args) != 1 || l.inner(args[0]).Kind() != l.v.tuple || !firstNamed(l.inner(args[0])).IsZero() {
		for _, a := range args {
			elems = append(elems, l.typeRef(a))
		}
	}
	t := plugin.Composite(n.Compact(), l.pos(n), symbol.FormTuple, elems...)
	t.Mutability = symbol.MutabilityImmutable
	return t
}

// union lowers a union: the members of X | Y where args is nil, and the
// arguments of Union[X, Y] otherwise. A union of one member and None is
// Optional of the member, and Absent lists the None members. Every other
// union is a Union of its members, in order, with each nested union
// flattened into it.
func (l *lowering) union(n treesitter.Node, args []treesitter.Node) *node.TypeRef {
	var members []treesitter.Node
	if args == nil {
		l.flatten(n, &members)
	}
	for _, a := range args {
		l.flatten(a, &members)
	}
	var kept, absent []*node.TypeRef
	all := make([]*node.TypeRef, 0, len(members))
	for _, m := range members {
		ref := l.typeRef(m)
		all = append(all, ref)
		if m.Kind() == l.v.none {
			absent = append(absent, ref)
			continue
		}
		kept = append(kept, ref)
	}
	if len(kept) != 1 || len(absent) == 0 {
		return plugin.Composite(n.Compact(), l.pos(n), symbol.FormUnion, all...)
	}
	optional := plugin.Composite(n.Compact(), l.pos(n), symbol.FormOptional, kept[0])
	optional.Absent = absent
	return optional
}

// flatten appends the members of a union to out: the operands of each |,
// and the arguments of a nested Union, in their order.
func (l *lowering) flatten(n treesitter.Node, out *[]treesitter.Node) {
	n = l.inner(n)
	switch n.Kind() {
	case l.v.unionType, l.v.binaryOperator:
		if n.Kind() == l.v.unionType || n.Child(l.v.fieldOperator).Text() == unionMark {
			for operand := range n.Content() {
				l.flatten(operand, out)
			}
			return
		}
	case l.v.subscript, l.v.genericType:
		value, args := l.generic(n)
		if module, name := l.scope.canonical(value.Compact()); module == typingModule && name == unionForm {
			for _, a := range args {
				l.flatten(a, out)
			}
			return
		}
	}
	*out = append(*out, n)
}

// generic returns the class and the arguments of a subscript or a generic
// type, and the zero Node and no arguments for any other node.
func (l *lowering) generic(n treesitter.Node) (treesitter.Node, []treesitter.Node) {
	var args []treesitter.Node
	switch n.Kind() {
	case l.v.subscript:
		for a := range n.Children(l.v.fieldSubscript) {
			args = append(args, a)
		}
		return n.Child(l.v.fieldValue), args
	case l.v.genericType:
		for a := range n.FirstOf(l.v.typeParameter).Content() {
			args = append(args, a)
		}
		return firstNamed(n), args
	default:
		return treesitter.Node{}, nil
	}
}

// named returns the Named reference of a name at the position of n, with
// the module of the import that binds its first name as its package.
func (l *lowering) named(n treesitter.Node, spelling string) *node.TypeRef {
	return &node.TypeRef{Spelling: spelling, Pos: l.pos(n), Package: l.scope.packageOf(spelling)}
}

// inString lowers a string annotation: it parses the content of the string
// as an annotation, and calls lower with the type of the parse, at the
// positions of the content. It calls verbatim for a string that is no
// constant string, or that does not parse, which it reports under
// [UnparsedFile].
func (l *lowering) inString(n treesitter.Node, lower func(inner treesitter.Node), verbatim func()) {
	value, constant := python.Unquote(n.Text())
	if !constant {
		verbatim()
		return
	}
	at := l.pos(n)
	start := n.FirstOf(l.v.stringStart)
	at.Col += start.End().Col - start.Pos().Col
	if !l.parseAt(value, at, lower) {
		l.u.Warnf(UnparsedFile, at, "the string annotation %s does not parse, and its type keeps the string", n.Text())
		verbatim()
	}
}

// parseAt parses the text of a type, whose first byte is at the position at
// in the file, and calls lower with the type of the parse at the positions
// of the text. It reports false for an empty text, for a text that does not
// parse as a type, and for a parse that finds the context of the lowering
// ended, and calls nothing for them.
func (l *lowering) parseAt(text string, at position.Pos, lower func(typed treesitter.Node)) bool {
	if text == "" {
		return false
	}
	source, prefix := snippet+text, len(snippet)
	if strings.Contains(text, "\n") {
		source, prefix = parenthesized+text+closeParenthesis, len(parenthesized)
	}
	lowered := false
	l.parsed([]byte(source), func(tree *treesitter.Tree) {
		typed := firstNamed(tree.Root()).Child(l.v.fieldType)
		if tree.Problems() != nil || typed.IsZero() {
			return
		}
		outer := l.shift
		l.shift = &shift{file: at.File, line: at.Line, col: at.Col, prefix: prefix}
		lower(typed)
		l.shift = outer
		lowered = true
	})
	return lowered
}

// pos returns the position of a node in the file: its own position, or the
// position that the shift of the string annotation under lowering maps it
// to.
func (l *lowering) pos(n treesitter.Node) position.Pos {
	p := n.Pos()
	if l.shift == nil {
		return p
	}
	if p.Line == 1 {
		p.Col = l.shift.col + p.Col - 1 - l.shift.prefix
	}
	p.Line = l.shift.line + p.Line - 1
	p.File = l.shift.file
	return p
}

// inner returns the node that a type node and parentheses wrap, and the
// node itself for any other node.
func (l *lowering) inner(n treesitter.Node) treesitter.Node {
	for n.Kind() == l.v.typeNode || n.Kind() == l.v.parenthesizedExpression {
		n = firstNamed(n)
	}
	return n
}
