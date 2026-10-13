// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The keywords that modify a declaration or a member.
const (
	keywordAsync    = "async"
	keywordConst    = "const"
	keywordStatic   = "static"
	keywordReadonly = "readonly"
	keywordAbstract = "abstract"
	keywordGet      = "get"
	keywordSet      = "set"
	keywordOptional = "?"
	keywordDefinite = "!"
)

// declaration lowers one declaration into a container: a class, an
// interface, an enum, a type alias, a function or overload signature, or
// the variables of a let, const or var statement. outermost is the
// statement that wraps it, an export or a declare, whose comments belong
// to the declaration. A declaration in an ambient context is stamped
// typescript.ambient. A declaration that the signature depth leaves out
// takes its comments with it. The implementation of a function after its
// overload signatures declares nothing at any depth. Neither does a
// declaration whose name does not parse, nor a statement, so a carrier on
// one of them is refused.
func (l *lowering) declaration(n treesitter.Node, c container, exported bool, outermost treesitter.Node,
	overloaded map[string]bool,
) {
	k := n.Kind()
	if k == l.v.lexicalDeclaration || k == l.v.variableDeclaration {
		l.variables(n, c, exported, outermost)
		return
	}
	function := k == l.v.functionDeclaration || k == l.v.generatorFunctionDeclaration ||
		k == l.v.functionSignature || k == l.v.functionExpression || k == l.v.generatorFunction
	if !function && !l.typeDeclaration(k) {
		l.refuse(outermost, "a statement")
		return
	}
	name := l.nameText(n.Child(l.v.fieldName))
	if name == "" && l.anonymousDefault(n) {
		name = typescript.DefaultExport
	}
	switch {
	case name == "":
		l.refuse(outermost, "a declaration whose name does not parse")
		return
	case k != l.v.functionSignature && function && overloaded[name]:
		l.refuse(outermost, "an overloaded function's implementation")
		return
	case l.u.Depth() == plugin.DepthSignatures && !l.visible(c, exported, name):
		l.skip(outermost)
		return
	}
	vis := l.visibility(c, exported, name)
	parts, comment := l.declParts(outermost)
	var decl symbol.Symbol
	switch k {
	case l.v.classDeclaration, l.v.abstractClassDeclaration, l.v.class:
		decl = l.class(n, name, vis, parts, comment, outermost)
	case l.v.interfaceDeclaration:
		decl = l.iface(n, name, vis, parts, comment, c)
	case l.v.enumDeclaration:
		decl = l.enum(n, name, vis, parts, comment)
	case l.v.typeAliasDeclaration:
		decl = l.alias(n, name, vis, parts, comment)
	default:
		decl = l.function(n, name, vis, parts, comment)
	}
	if decl == nil {
		return
	}
	c.file.Decls = append(c.file.Decls, decl)
	if c.ambient {
		l.mark(decl, typescript.AmbientKey, n.Pos())
	}
	l.u.AttachCarriers(decl, parts.Carriers, BadCarrier)
}

// typeDeclaration reports whether a kind declares a type: a class, an
// interface, an enum or a type alias.
func (l *lowering) typeDeclaration(k treesitter.Kind) bool {
	return k == l.v.classDeclaration || k == l.v.abstractClassDeclaration || k == l.v.class ||
		k == l.v.interfaceDeclaration || k == l.v.enumDeclaration || k == l.v.typeAliasDeclaration
}

// refuse reports the carriers of the comments of a node under
// [UnaddressedCarrier]. The node is a subject that the model cannot
// address, and what describes it.
func (l *lowering) refuse(outermost treesitter.Node, what string) {
	parts, _ := l.declParts(outermost)
	l.u.RefuseCarriers(UnaddressedCarrier, parts.Carriers, what)
}

// class lowers a class: abstract where the declaration states it, its type
// parameters, its superclass and interfaces, its decorators, and its
// members.
func (l *lowering) class(n treesitter.Node, name string, vis symbol.Visibility, parts plugin.CommentParts,
	comment string, outermost treesitter.Node,
) *node.Struct {
	st := &node.Struct{
		Name: name, Pos: l.namePos(n), Doc: parts.Docs, Comment: comment, Visibility: vis,
		Abstract:   n.Kind() == l.v.abstractClassDeclaration,
		TypeParams: l.typeParams(n.Child(l.v.fieldTypeParameters)),
	}
	decorators := l.decoratorsOf(n)
	if outermost.Kind() == l.v.exportStatement {
		decorators = append(l.decoratorsOf(outermost), decorators...)
	}
	st.Annotations = l.decorate(st, decorators)
	if heritage := n.FirstOf(l.v.classHeritage); !heritage.IsZero() {
		for clause := range heritage.NamedChildren() {
			switch clause.Kind() {
			case l.v.extendsClause:
				st.Extends = append(st.Extends, l.heritage(clause)...)
			case l.v.implementsClause:
				st.Implements = append(st.Implements, l.typeList(clause)...)
			}
		}
	}
	l.classBody(n.Child(l.v.fieldBody), st)
	return st
}

// heritage lowers the superclass of an extends clause: the expression that
// it names, with the type arguments that the clause states.
func (l *lowering) heritage(clause treesitter.Node) []*node.TypeRef {
	var out []*node.TypeRef
	for value := range clause.Children(l.v.fieldValue) {
		ref := &node.TypeRef{Spelling: value.Compact(), Pos: value.Pos(), Package: l.importOf(value.Compact())}
		ref.Args = l.typeArgs(clause.Child(l.v.fieldTypeArguments))
		out = append(out, ref)
	}
	return out
}

// typeList lowers each type that a clause lists, in order.
func (l *lowering) typeList(clause treesitter.Node) []*node.TypeRef {
	var out []*node.TypeRef
	for t := range clause.Content() {
		out = append(out, l.typeRef(t))
	}
	return out
}

// iface lowers an interface: its type parameters, the interfaces it
// extends, and its members. A second declaration of an interface in one
// package of one file merges its members into the first, as TypeScript
// merges them, and declares nothing of its own.
func (l *lowering) iface(n treesitter.Node, name string, vis symbol.Visibility, parts plugin.CommentParts,
	comment string, c container,
) symbol.Symbol {
	it := &node.Interface{
		Name: name, Pos: l.namePos(n), Doc: parts.Docs, Comment: comment, Visibility: vis,
		TypeParams: l.typeParams(n.Child(l.v.fieldTypeParameters)),
	}
	if clause := n.FirstOf(l.v.extendsTypeClause); !clause.IsZero() {
		for t := range clause.Children(l.v.fieldType) {
			it.Extends = append(it.Extends, l.typeRef(t))
		}
	}
	calls, _ := l.objectMembers(n.Child(l.v.fieldBody), &it.Fields, &it.Methods)
	for _, prior := range c.file.Decls {
		if first, merges := prior.(*node.Interface); merges && first.Name == name {
			first.Fields = append(first.Fields, it.Fields...)
			first.Methods = append(first.Methods, it.Methods...)
			first.Extends = append(first.Extends, it.Extends...)
			l.stampCalls(first, calls, n)
			l.u.AttachCarriers(first, parts.Carriers, BadCarrier)
			return nil
		}
	}
	l.stampCalls(it, calls, n)
	return it
}

// stampCalls stamps the call signatures of a callable object type on
// the declaration that declares it, each as written, because the model
// has no member for a callable object.
func (l *lowering) stampCalls(subject symbol.Symbol, calls []string, at treesitter.Node) {
	if len(calls) > 0 {
		l.u.Graph().Stamp(subject, meta.RawStamp{Key: typescript.CallSignatureKey, Value: calls, Pos: at.Pos()})
	}
}

// mark stamps a key true on a subject, at the position of the source that
// states the mark. A member of an inline body has no identity that a stamp
// can address, so mark stamps nothing on one.
func (l *lowering) mark(subject symbol.Symbol, key meta.KeyName, at position.Pos) {
	if l.inline > 0 {
		return
	}
	l.u.Graph().Stamp(subject, meta.RawStamp{Key: key, Value: true, Pos: at})
}

// attach attaches the carriers of a member to it. A member of an inline
// body has no identity that a directive can address, so attach refuses its
// carriers under [UnaddressedCarrier].
func (l *lowering) attach(member symbol.Symbol, carriers []plugin.Carrier) {
	if l.inline > 0 {
		l.u.RefuseCarriers(UnaddressedCarrier, carriers, "a member of an inline object type")
		return
	}
	l.u.AttachCarriers(member, carriers, BadCarrier)
}

// enum lowers an enum, which is const where the declaration states it.
// Each member is a variant, with its value verbatim where the member
// assigns one.
func (l *lowering) enum(n treesitter.Node, name string, vis symbol.Visibility, parts plugin.CommentParts,
	comment string,
) *node.Enum {
	e := &node.Enum{
		Name: name, Pos: l.namePos(n), Doc: parts.Docs, Comment: comment, Visibility: vis,
		Const: n.HasToken(keywordConst),
	}
	for member := range n.Child(l.v.fieldBody).Content() {
		var variant *node.EnumVariant
		if member.Kind() == l.v.enumAssignment {
			name := member.Child(l.v.fieldName)
			variant = &node.EnumVariant{
				Name: l.nameText(name), Pos: name.Pos(), Value: member.Child(l.v.fieldValue).Text(),
			}
		} else {
			variant = &node.EnumVariant{Name: l.nameText(member), Pos: member.Pos()}
		}
		vparts, vcomment := l.declParts(member)
		variant.Doc, variant.Comment = vparts.Docs, vcomment
		e.Variants = append(e.Variants, variant)
		l.u.AttachCarriers(variant, vparts.Carriers, BadCarrier)
	}
	return e
}

// alias lowers a type alias: its type parameters and the type it names.
// An object type with call signatures stamps them on the alias. A
// conditional type stamps its spelling under typescript.conditionalType,
// because the model keeps it as a named reference.
func (l *lowering) alias(n treesitter.Node, name string, vis symbol.Visibility, parts plugin.CommentParts,
	comment string,
) *node.Alias {
	value := n.Child(l.v.fieldValue)
	a := &node.Alias{
		Name: name, Pos: l.namePos(n), Doc: parts.Docs, Comment: comment, Visibility: vis,
		TypeParams: l.typeParams(n.Child(l.v.fieldTypeParameters)),
		Target:     l.typeRef(value),
	}
	switch value.Kind() {
	case l.v.objectType:
		l.stampCalls(a, l.callSignatures(value), value)
	case l.v.conditionalType:
		stamp := meta.RawStamp{Key: typescript.ConditionalTypeKey, Value: a.Target.Spelling, Pos: value.Pos()}
		l.u.Graph().Stamp(a, stamp)
	}
	return a
}

// function lowers a function or one overload signature with its type
// parameters, its parameters and its result, which [lowering.result]
// lowers. The function is async when the declaration has the async
// keyword or returns a promise. A generator function, which has a *, is
// stamped typescript.generator.
func (l *lowering) function(n treesitter.Node, name string, vis symbol.Visibility, parts plugin.CommentParts,
	comment string,
) *node.Function {
	params, _ := l.params(n.Child(l.v.fieldParameters))
	returns, promised := l.result(n.Child(l.v.fieldReturnType), true, l.namePos(n))
	fn := &node.Function{
		Name: name, Pos: l.namePos(n), Doc: parts.Docs, Comment: comment, Visibility: vis,
		Async:      promised || n.HasToken(keywordAsync),
		TypeParams: l.typeParams(n.Child(l.v.fieldTypeParameters)),
		Params:     params,
		Returns:    returns,
	}
	if n.HasToken(keywordStar) {
		l.mark(fn, typescript.GeneratorKey, n.Pos())
	}
	return fn
}

// variables lowers a let, const or var statement. Each name that const
// binds is a constant, and each name that let or var binds is a mutable
// variable, with its stated type and its initializer verbatim. In an
// ambient context each is stamped typescript.ambient. Each takes the
// documentation and the trailing comment of the statement, and the first
// takes its carriers. A destructuring pattern binds no single name and
// declares nothing, so a carrier on a statement of patterns alone is
// refused.
func (l *lowering) variables(n treesitter.Node, c container, exported bool, outermost treesitter.Node) {
	constant := n.HasToken(keywordConst)
	var names []treesitter.Node
	for declarator := range n.NamedChildren() {
		name := declarator.Child(l.v.fieldName)
		if declarator.Kind() != l.v.variableDeclarator || name.Kind() != l.v.identifier {
			continue
		}
		if l.u.Depth() == plugin.DepthSignatures && !l.visible(c, exported, name.Text()) {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 && l.u.Depth() == plugin.DepthSignatures {
		l.skip(outermost)
		return
	}
	if len(names) == 0 {
		l.refuse(outermost, "a destructuring pattern")
		return
	}
	parts, comment := l.declParts(outermost)
	var first symbol.Symbol
	for _, name := range names {
		declarator := name.Parent()
		vis := l.visibility(c, exported, name.Text())
		typ := l.typeRef(declarator.Child(l.v.fieldType))
		value := declarator.Child(l.v.fieldValue).Text()
		var decl symbol.Symbol
		if constant {
			decl = &node.Constant{
				Name: name.Text(), Pos: name.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
				Type: typ, Value: value,
			}
		} else {
			decl = &node.Variable{
				Name: name.Text(), Pos: name.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
				Mutability: symbol.MutabilityMutable, Type: typ, Value: value,
			}
		}
		if first == nil {
			first = decl
		}
		c.file.Decls = append(c.file.Decls, decl)
		if c.ambient {
			l.mark(decl, typescript.AmbientKey, name.Pos())
		}
	}
	l.u.AttachCarriers(first, parts.Carriers, BadCarrier)
}

// namePos returns the position of the name of a declaration, and the
// position of the declaration for one without a name.
func (l *lowering) namePos(n treesitter.Node) position.Pos {
	if name := n.Child(l.v.fieldName); !name.IsZero() {
		return name.Pos()
	}
	return n.Pos()
}
