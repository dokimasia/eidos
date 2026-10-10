// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// members lowers a type body's members into their host. An initializer
// block declares nothing, because it is part of a body.
func (l *lowering) members(body treesitter.Node, h *host) {
	for child := range body.NamedChildren() {
		if !l.comment(child) && !l.member(child, h) {
			l.refuse(child, "an initializer block")
		}
	}
}

// member lowers one member declaration into its host, and reports
// whether the node declares a member: a field, a method, a constructor,
// an annotation type element or a nested type, and a compact source
// file's field, whose grammar node is a local variable declaration. A
// type an enum declares has no place in the model.
func (l *lowering) member(n treesitter.Node, h *host) bool {
	switch k := n.Kind(); {
	case k == l.v.fieldDeclaration, k == l.v.constantDeclaration, k == l.v.localVariableDeclaration:
		l.fields(n, h)
	case k == l.v.methodDeclaration:
		l.method(n, h)
	case k == l.v.constructorDeclaration, k == l.v.compactConstructorDeclaration:
		l.constructor(n, h)
	case k == l.v.annotationTypeElementDeclaration:
		l.element(n, h)
	case l.typeDeclaration(k) && h.types == nil:
		l.unmodeled(n)
	case l.typeDeclaration(k):
		if decl := l.typeDecl(n, h.kind); decl != nil {
			*h.types = append(*h.types, decl)
		}
	default:
		return false
	}
	return true
}

// memberVisibility returns the visibility a member takes where its
// modifiers state none: public in an interface, and package-visible
// anywhere else.
func memberVisibility(h *host) symbol.Visibility {
	if h.kind == hostInterface {
		return symbol.VisibilityPublic
	}
	return symbol.VisibilityPackage
}

// fields lowers a field declaration into one field per declarator, each
// with the declaration's type, its dimensions, and the declarator's
// initializer verbatim. A static field is at the type level and a final
// one immutable, and an interface's fields are both. The declaration's
// comments document every field it declares, and its carriers attach to
// each.
func (l *lowering) fields(n treesitter.Node, h *host) {
	m := l.modifiersOf(n)
	vis := m.visibility(memberVisibility(h))
	if !l.kept(vis) {
		l.skip(n)
		return
	}
	parts, comment := l.declParts(n)
	level, mutability := symbol.LevelInstance, symbol.MutabilityMutable
	if h.kind == hostInterface || m.has(modStatic) {
		level = symbol.LevelType
	}
	if h.kind == hostInterface || m.has(modFinal) {
		mutability = symbol.MutabilityImmutable
	}
	for d := range n.Children(l.v.fieldDeclarator) {
		name := d.Child(l.v.fieldName)
		f := &node.Field{
			Name: name.Text(), Pos: name.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
			Level: level, Mutability: mutability, Value: d.Child(l.v.fieldValue).Text(),
			Type: l.dimensioned(n.Child(l.v.fieldType), d.Child(l.v.fieldDimensions)), Annotations: m.annotations,
		}
		l.attach(f, parts.Carriers, m.sugars)
		*h.fields = append(*h.fields, f)
	}
}

// method lowers a method declaration: static at the type level, final
// and abstract where its modifiers state them, with its type parameters,
// its parameters, its result and the exceptions it throws. An interface's
// method without a body is abstract, and one its modifiers mark default
// has a default. A void method has no result.
func (l *lowering) method(n treesitter.Node, h *host) {
	m := l.modifiersOf(n)
	vis := m.visibility(memberVisibility(h))
	if !l.kept(vis) {
		l.skip(n)
		return
	}
	parts, comment := l.declParts(n)
	name := n.Child(l.v.fieldName)
	params, receiver := l.params(n.Child(l.v.fieldParameters))
	md := &node.Method{
		Name: name.Text(), Pos: name.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
		Final: m.has(modFinal), Abstract: m.has(modAbstract), TypeParams: l.typeParams(n),
		Receiver: receiver, Params: params, Throws: l.typesOf(l.firstOf(n, l.v.throws)), Annotations: m.annotations,
	}
	if m.has(modStatic) {
		md.Level = symbol.LevelType
	}
	if h.kind == hostInterface {
		md.Abstract = n.Child(l.v.fieldBody).IsZero()
		md.HasDefault = m.has(modDefault)
	}
	if typ := n.Child(l.v.fieldType); typ.Kind() != l.v.voidType {
		ref := l.dimensioned(typ, n.Child(l.v.fieldDimensions))
		md.Returns = []*node.Return{{Pos: ref.Pos, Type: ref}}
	}
	l.attach(md, parts.Carriers, m.sugars)
	*h.methods = append(*h.methods, md)
}

// constructor lowers a constructor as a method that constructs, named
// after its type, as Java spells it: its type parameters, its parameters
// and the exceptions it throws. A record's compact constructor takes the
// record's components as its parameters. An enum's constructor is
// private where it states no access, and any other package-visible.
func (l *lowering) constructor(n treesitter.Node, h *host) {
	m := l.modifiersOf(n)
	def := symbol.VisibilityPackage
	if h.kind == hostEnum {
		def = symbol.VisibilityPrivate
	}
	vis := m.visibility(def)
	if !l.kept(vis) {
		l.skip(n)
		return
	}
	parts, comment := l.declParts(n)
	list := n.Child(l.v.fieldParameters)
	if n.Kind() == l.v.compactConstructorDeclaration {
		list = h.components
	}
	params, _ := l.params(list)
	name := n.Child(l.v.fieldName)
	md := &node.Method{
		Name: name.Text(), Pos: name.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
		Constructs: true, TypeParams: l.typeParams(n), Params: params,
		Throws: l.typesOf(l.firstOf(n, l.v.throws)), Annotations: m.annotations,
	}
	l.attach(md, parts.Carriers, m.sugars)
	*h.methods = append(*h.methods, md)
}

// element lowers an annotation type's element as an abstract public
// method whose result is the element's type.
func (l *lowering) element(n treesitter.Node, h *host) {
	parts, comment := l.declParts(n)
	name := n.Child(l.v.fieldName)
	ref := l.dimensioned(n.Child(l.v.fieldType), n.Child(l.v.fieldDimensions))
	m := l.modifiersOf(n)
	md := &node.Method{
		Name: name.Text(), Pos: name.Pos(), Doc: parts.Docs, Comment: comment,
		Visibility: symbol.VisibilityPublic, Abstract: true, Returns: []*node.Return{{Pos: ref.Pos, Type: ref}},
		Annotations: m.annotations,
	}
	l.attach(md, parts.Carriers, m.sugars)
	*h.methods = append(*h.methods, md)
}

// params lowers a parameter list: each parameter's name, its type with
// its dimensions and its annotations, and a variadic one as positionally
// variadic, whose type is the element it collects. An explicit receiver
// is returned apart, positioned at its start, and the grammar reads an
// annotated one as a parameter named this. The carriers of a parameter's
// comments are refused.
func (l *lowering) params(list treesitter.Node) (params []*node.Param, receiver *node.Param) {
	for p := range list.NamedChildren() {
		switch p.Kind() {
		case l.v.formalParameter:
			name := p.Child(l.v.fieldName)
			param := &node.Param{
				Name: name.Text(), Pos: name.Pos(), Annotations: l.modifiersOf(p).annotations,
				Type: l.dimensioned(p.Child(l.v.fieldType), p.Child(l.v.fieldDimensions)),
			}
			if param.Name == receiverName {
				param.Pos, receiver = p.Pos(), param
				break
			}
			params = append(params, param)
		case l.v.spreadParameter:
			name := l.firstOf(p, l.v.variableDeclarator).Child(l.v.fieldName)
			params = append(params, &node.Param{
				Name: name.Text(), Pos: name.Pos(), Variadic: symbol.VariadicPositional,
				Type: l.typeRef(l.typeIn(p)), Annotations: l.modifiersOf(p).annotations,
			})
		case l.v.receiverParameter:
			receiver = &node.Param{Name: receiverName, Pos: p.Pos(), Type: l.typeRef(l.typeIn(p))}
		default:
			continue
		}
		l.refuse(p, "a parameter")
	}
	return params, receiver
}

// typeIn returns the type a spread or a receiver parameter states, which
// the grammar gives no field: its first named child that is no modifier
// list or comment, because the type precedes its declarator and a
// receiver's this.
func (l *lowering) typeIn(p treesitter.Node) treesitter.Node {
	return firstWhere(p, func(child treesitter.Node) bool {
		return child.Kind() != l.v.modifiers && !l.comment(child)
	})
}

// typeParams lowers a declaration's type parameters: each one's name and
// the bounds its extends clause states.
func (l *lowering) typeParams(n treesitter.Node) []*node.TypeParam {
	var out []*node.TypeParam
	for tp := range n.Child(l.v.fieldTypeParameters).NamedChildren() {
		if tp.Kind() != l.v.typeParameter {
			continue
		}
		name := l.firstOf(tp, l.v.typeIdentifier)
		p := &node.TypeParam{Name: name.Text(), Pos: name.Pos()}
		for _, b := range l.children(l.firstOf(tp, l.v.typeBound)) {
			p.Bounds = append(p.Bounds, l.typeRef(b))
		}
		out = append(out, p)
	}
	return out
}

// typesOf lowers the types a clause lists: a superclass, an
// implements, extends or permits clause, or a throws clause. A clause
// states its types directly or through a type list.
func (l *lowering) typesOf(clause treesitter.Node) []*node.TypeRef {
	if list := l.firstOf(clause, l.v.typeList); !list.IsZero() {
		clause = list
	}
	children := l.children(clause)
	if len(children) == 0 {
		return nil
	}
	out := make([]*node.TypeRef, 0, len(children))
	for _, t := range children {
		out = append(out, l.typeRef(t))
	}
	return out
}

// unmodeled reports a type an enum declares under [UnmodeledItem], at
// its name, because an enum has no list of nested types. It refuses the
// type's carriers and passes the sweep over the type.
func (l *lowering) unmodeled(n treesitter.Node) {
	name := n.Child(l.v.fieldName)
	l.u.Infof(UnmodeledItem, name.Pos(), "%s is not in the model, because an enum has no list of nested types",
		name.Text())
	l.refuse(n, "a type an enum declares")
	l.taken[n.Pos()] = true
}
