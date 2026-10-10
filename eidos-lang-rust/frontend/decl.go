// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strings"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// variant is one enum variant a cfg predicate keeps in: its node, its
// attributes, and its comments' parts and trailing comment.
type variant struct {
	n       treesitter.Node
	a       attributes
	parts   plugin.CommentParts
	comment string
}

// structItem lowers a struct or a union: its type parameters, its named
// or positional fields, and its attributes. A union is stamped
// rust.union.
func (l *lowering) structItem(n treesitter.Node, c container, a attributes) {
	vis, spelled := l.visibility(n)
	if !l.kept(vis) {
		l.skip(n)
		return
	}
	nameNode := n.Child(l.v.fieldName)
	parts, comment := l.declParts(n, a)
	tps, lifetimes := l.typeParams(n)
	st := &node.Struct{
		Name: nameNode.Text(), Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
		TypeParams: tps, Fields: l.fields(n.Child(l.v.fieldBody), symbol.VisibilityPrivate),
		Annotations: a.annotations,
	}
	l.declare(c, st, parts, spelled, a)
	l.lifetimes(st, lifetimes)
	if n.Kind() == l.v.unionItem {
		l.w.stamp(st, rust.UnionKey, true, st.Pos)
	}
	l.w.addType(c.pkg, st.Name, st)
}

// fields lowers a struct's or a variant's fields: a named list's fields
// by name, and a positional list's unnamed, in order, each with its own
// visibility, def where it states none, its attributes and its
// comments.
func (l *lowering) fields(body treesitter.Node, def symbol.Visibility) []*node.Field {
	var out []*node.Field
	var attrs []treesitter.Node
	var vis treesitter.Node
	for child := range body.NamedChildren() {
		switch k := child.Kind(); {
		case k == l.v.attributeItem:
			attrs = append(attrs, child)
		case l.comment(child):
		case k == l.v.visibilityModifier:
			vis = child
		case k == l.v.fieldDeclaration:
			if f := l.field(child, child.Child(l.v.fieldName), child.Child(l.v.fieldType),
				l.firstOf(child, l.v.visibilityModifier), def, attrs); f != nil {
				out = append(out, f)
			}
			attrs = nil
		default:
			if f := l.field(child, treesitter.Node{}, child, vis, def, attrs); f != nil {
				out = append(out, f)
			}
			attrs, vis = nil, treesitter.Node{}
		}
	}
	return out
}

// field lowers one field, named where name is a field identifier and
// positional otherwise, with the visibility its modifier states and def
// where it states none. It returns nil for a field a cfg predicate keeps
// out, and for one signature depth leaves out.
func (l *lowering) field(n, name, typ, vm treesitter.Node, def symbol.Visibility,
	attrs []treesitter.Node,
) *node.Field {
	vis, spelled := def, ""
	if !vm.IsZero() {
		vis, spelled = l.visibilityOf(vm)
	}
	a := l.attributes(attrs)
	if a.excluded != "" {
		l.excluded = append(l.excluded, a.excluded)
	}
	if a.excluded != "" || !l.kept(vis) {
		l.skip(n)
		return nil
	}
	parts, comment := l.declParts(n, a)
	f := &node.Field{
		Pos: n.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
		Type: l.typeRef(typ), Annotations: a.annotations,
	}
	if !name.IsZero() {
		f.Name, f.Pos = name.Text(), name.Pos()
	}
	l.w.mark(f, parts, spelled, a)
	return f
}

// enumItem lowers an enum: an Enum where no variant states a payload,
// each variant's discriminant verbatim, and a Sum where any does, each
// variant's fields named or positional, because the model splits a
// variant set by payload. A variant's fields are as public as the enum.
func (l *lowering) enumItem(n treesitter.Node, c container, a attributes) {
	vis, spelled := l.visibility(n)
	if !l.kept(vis) {
		l.skip(n)
		return
	}
	nameNode := n.Child(l.v.fieldName)
	parts, comment := l.declParts(n, a)
	tps, lifetimes := l.typeParams(n)
	variants := l.variantsOf(n.Child(l.v.fieldBody))
	var decl symbol.Symbol
	if slices.ContainsFunc(variants, l.payload) {
		sum := &node.Sum{
			Name: nameNode.Text(), Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
			TypeParams: tps, Annotations: a.annotations,
		}
		for _, v := range variants {
			name := v.n.Child(l.v.fieldName)
			sv := &node.SumVariant{
				Name: name.Text(), Pos: name.Pos(), Doc: v.parts.Docs, Comment: v.comment,
				Annotations: v.a.annotations, Fields: l.fields(v.n.Child(l.v.fieldBody), symbol.VisibilityPublic),
			}
			sum.Variants = append(sum.Variants, sv)
			l.w.mark(sv, v.parts, "", v.a)
		}
		decl = sum
	} else {
		e := &node.Enum{
			Name: nameNode.Text(), Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
			Annotations: a.annotations,
		}
		for _, v := range variants {
			name := v.n.Child(l.v.fieldName)
			ev := &node.EnumVariant{
				Name: name.Text(), Pos: name.Pos(), Doc: v.parts.Docs, Comment: v.comment,
				Annotations: v.a.annotations, Value: v.n.Child(l.v.fieldValue).Text(),
			}
			e.Variants = append(e.Variants, ev)
			l.w.mark(ev, v.parts, "", v.a)
		}
		decl = e
	}
	l.declare(c, decl, parts, spelled, a)
	l.lifetimes(decl, lifetimes)
	l.w.addType(c.pkg, nameNode.Text(), decl)
}

// variantsOf reads an enum body's variants with their attributes and
// comments, and leaves out a variant a cfg predicate keeps out, with its
// comments.
func (l *lowering) variantsOf(body treesitter.Node) []variant {
	var out []variant
	var attrs []treesitter.Node
	for v := range body.NamedChildren() {
		switch {
		case v.Kind() == l.v.attributeItem:
			attrs = append(attrs, v)
			continue
		case v.Kind() != l.v.enumVariant:
			continue
		}
		a := l.attributes(attrs)
		attrs = nil
		if a.excluded != "" {
			l.excluded = append(l.excluded, a.excluded)
			l.skip(v)
			continue
		}
		parts, comment := l.declParts(v, a)
		out = append(out, variant{n: v, a: a, parts: parts, comment: comment})
	}
	return out
}

// payload reports whether a variant states a payload, named or
// positional.
func (l *lowering) payload(v variant) bool {
	return !v.n.Child(l.v.fieldBody).IsZero()
}

// traitItem lowers a trait as an Interface: its supertraits as what it
// extends, a method without a body as abstract and one with a body as
// having a default, an associated type as an Alias without a target
// among its types, and an associated constant as a Constant among them.
// Signature depth keeps a public trait's items whole.
func (l *lowering) traitItem(n treesitter.Node, c container, a attributes) {
	vis, spelled := l.visibility(n)
	if !l.kept(vis) {
		l.skip(n)
		return
	}
	nameNode := n.Child(l.v.fieldName)
	parts, comment := l.declParts(n, a)
	tps, lifetimes := l.typeParams(n)
	it := &node.Interface{
		Name: nameNode.Text(), Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
		TypeParams: tps, Extends: l.bounds(n.Child(l.v.fieldBounds)), Annotations: a.annotations,
	}
	var attrs []treesitter.Node
	for item := range n.Child(l.v.fieldBody).NamedChildren() {
		switch {
		case item.Kind() == l.v.attributeItem:
			attrs = append(attrs, item)
			continue
		case l.comment(item):
			continue
		}
		ia := l.attributes(attrs)
		attrs = nil
		if ia.excluded != "" {
			l.excluded = append(l.excluded, ia.excluded)
			l.skip(item)
			continue
		}
		switch item.Kind() {
		case l.v.functionItem, l.v.functionSignatureItem:
			m := l.methodOf(item, ia, symbol.VisibilityPublic)
			m.Abstract = item.Kind() == l.v.functionSignatureItem
			m.HasDefault = !m.Abstract
			it.Methods = append(it.Methods, m)
		case l.v.associatedType, l.v.typeItem:
			it.Types = append(it.Types, l.associated(item, ia))
		case l.v.constItem:
			it.Types = append(it.Types, l.constantOf(item, ia, symbol.VisibilityPublic, ""))
		default:
			l.refuse(item, ia, "an item the model does not contain, such as a macro")
		}
	}
	l.declare(c, it, parts, spelled, a)
	l.lifetimes(it, lifetimes)
}

// associated lowers an associated type as an Alias whose target the
// implementation supplies: nil, or the default the trait states, which
// the grammar parses as a type item.
func (l *lowering) associated(n treesitter.Node, a attributes) *node.Alias {
	nameNode := n.Child(l.v.fieldName)
	parts, comment := l.declParts(n, a)
	tps, _ := l.typeParams(n)
	alias := &node.Alias{
		Name: nameNode.Text(), Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment,
		Visibility: symbol.VisibilityPublic, TypeParams: tps, Target: l.typeRef(n.Child(l.v.fieldType)),
		Annotations: a.annotations,
	}
	l.w.mark(alias, parts, "", a)
	return alias
}

// function lowers a free function, or one an extern block declares.
func (l *lowering) function(n treesitter.Node, c container, a attributes) {
	vis, spelled := l.visibility(n)
	if !l.kept(vis) {
		l.skip(n)
		return
	}
	nameNode := n.Child(l.v.fieldName)
	parts, comment := l.declParts(n, a)
	tps, lifetimes := l.typeParams(n)
	params, _ := l.params(n.Child(l.v.fieldParameters))
	fn := &node.Function{
		Name: nameNode.Text(), Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
		Async: l.async(n), TypeParams: tps, Params: params, Returns: l.returns(n),
		Annotations: a.annotations,
	}
	l.declare(c, fn, parts, spelled, a)
	l.lifetimes(fn, lifetimes)
}

// methodOf builds a method of a trait or an impl: its receiver, the
// type level for one without self, async, its signature and its
// attributes. Its carriers attach, and def is its visibility where the
// item states none.
func (l *lowering) methodOf(n treesitter.Node, a attributes, def symbol.Visibility) *node.Method {
	vis, spelled := def, ""
	if vm := l.firstOf(n, l.v.visibilityModifier); !vm.IsZero() {
		vis, spelled = l.visibilityOf(vm)
	}
	nameNode := n.Child(l.v.fieldName)
	parts, comment := l.declParts(n, a)
	tps, lifetimes := l.typeParams(n)
	params, receiver := l.params(n.Child(l.v.fieldParameters))
	m := &node.Method{
		Name: nameNode.Text(), Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
		Async: l.async(n), Receiver: receiver, TypeParams: tps, Params: params, Returns: l.returns(n),
		Annotations: a.annotations,
	}
	if receiver == nil {
		m.Level = symbol.LevelType
	}
	l.w.mark(m, parts, spelled, a)
	l.lifetimes(m, lifetimes)
	return m
}

// async reports whether a function's modifiers include async.
func (l *lowering) async(n treesitter.Node) bool {
	return l.token(l.firstOf(n, l.v.functionModifiers), keywordAsync)
}

// returns lowers a function's return type into its one result, and
// nothing for a function that states none.
func (l *lowering) returns(n treesitter.Node) []*node.Return {
	rt := n.Child(l.v.fieldReturnType)
	if rt.IsZero() {
		return nil
	}
	return []*node.Return{{Pos: rt.Pos(), Type: l.typeRef(rt)}}
}

// params lowers a parameter list: each parameter's name where its
// pattern is an identifier, its type and its attributes, and a variadic
// one of an extern function as positionally variadic. A parameter a cfg
// predicate keeps out is left out, and the carriers of a parameter's
// comments are refused. The self parameter is the receiver, returned
// apart, typed Self behind the reference it states.
func (l *lowering) params(list treesitter.Node) (params []*node.Param, receiver *node.Param) {
	var attrs []treesitter.Node
	for p := range list.NamedChildren() {
		k := p.Kind()
		if k == l.v.attributeItem {
			attrs = append(attrs, p)
			continue
		}
		if k != l.v.selfParameter && k != l.v.parameter && k != l.v.variadicParameter {
			continue
		}
		a := l.attributes(attrs)
		attrs = nil
		if a.excluded != "" {
			l.excluded = append(l.excluded, a.excluded)
			l.skip(p)
			continue
		}
		l.refuse(p, a, "a parameter")
		param := &node.Param{Pos: p.Pos(), Annotations: a.annotations}
		pattern := p.Child(l.v.fieldPattern)
		switch {
		case k == l.v.selfParameter:
			param.Name = keywordSelf
			param.Type = &node.TypeRef{Spelling: l.selfSpelling(p), Pos: p.Pos()}
			receiver = param
			continue
		case pattern.Kind() == l.v.self:
			param.Name = keywordSelf
			param.Type = l.typeRef(p.Child(l.v.fieldType))
			receiver = param
			continue
		case pattern.Kind() == l.v.identifier:
			param.Name = pattern.Text()
		}
		param.Type = l.typeRef(p.Child(l.v.fieldType))
		if k == l.v.variadicParameter {
			param.Variadic = symbol.VariadicPositional
		}
		params = append(params, param)
	}
	return params, receiver
}

// selfSpelling spells a self parameter's type: Self behind the reference,
// the lifetime and the mutability the parameter states. A mut self
// parameter is a mutable binding of a value, whose type is Self.
func (l *lowering) selfSpelling(p treesitter.Node) string {
	var b strings.Builder
	for child := range p.AllChildren() {
		switch {
		case child.Kind() == l.v.lifetime:
			b.WriteString(child.Text() + " ")
		case child.Kind() == l.v.mutableSpecifier && b.Len() > 0:
			b.WriteString(keywordMut + " ")
		case !child.Named():
			b.WriteString(child.Text())
		}
	}
	return b.String() + keywordSelfType
}

// typeParams lowers an item's generic parameters: a type parameter with
// its bounds and default, and the bounds its where clause adds, and a
// const parameter as const with its type and default. It returns the
// lifetime parameters apart, as written.
func (l *lowering) typeParams(n treesitter.Node) ([]*node.TypeParam, []string) {
	var out []*node.TypeParam
	var lifetimes []string
	byName := map[string]*node.TypeParam{}
	for p := range n.Child(l.v.fieldTypeParameters).NamedChildren() {
		switch p.Kind() {
		case l.v.lifetimeParameter:
			lifetimes = append(lifetimes, p.Child(l.v.fieldName).Text())
		case l.v.typeParameter:
			name := p.Child(l.v.fieldName)
			tp := &node.TypeParam{
				Name: name.Text(), Pos: name.Pos(), Bounds: l.bounds(p.Child(l.v.fieldBounds)),
				Default: l.typeRef(p.Child(l.v.fieldDefaultType)),
			}
			byName[tp.Name] = tp
			out = append(out, tp)
		case l.v.constParameter:
			name := p.Child(l.v.fieldName)
			out = append(out, &node.TypeParam{
				Name: name.Text(), Pos: name.Pos(), Const: true,
				Type: l.typeRef(p.Child(l.v.fieldType)), DefaultValue: p.Child(l.v.fieldValue).Text(),
			})
		}
	}
	for pred := range l.firstOf(n, l.v.whereClause).NamedChildren() {
		if tp := byName[pred.Child(l.v.fieldLeft).Text()]; tp != nil {
			tp.Bounds = append(tp.Bounds, l.bounds(pred.Child(l.v.fieldBounds))...)
		}
	}
	return out, lifetimes
}

// bounds lowers a bound list's types, the lifetimes left out.
func (l *lowering) bounds(list treesitter.Node) []*node.TypeRef {
	var out []*node.TypeRef
	for _, b := range l.children(list) {
		if b.Kind() != l.v.lifetime {
			out = append(out, l.typeRef(b))
		}
	}
	return out
}

// lifetimes stamps a declaration's lifetime parameters, which the model
// has no type parameter for.
func (l *lowering) lifetimes(decl symbol.Symbol, lifetimes []string) {
	if len(lifetimes) > 0 {
		l.w.stamp(decl, rust.LifetimeParamsKey, lifetimes, decl.Position())
	}
}

// constant lowers a const item. An anonymous const, named _, declares
// nothing, because no path names it.
func (l *lowering) constant(n treesitter.Node, c container, a attributes) {
	vis, spelled := l.visibility(n)
	if !l.kept(vis) || n.Child(l.v.fieldName).Text() == discardName {
		l.skip(n)
		return
	}
	c.file.Decls = append(c.file.Decls, l.constantOf(n, a, vis, spelled))
}

// constantOf builds a constant: its type and its value verbatim. Its
// carriers attach, and a restricted visibility's spelling is stamped.
func (l *lowering) constantOf(n treesitter.Node, a attributes, vis symbol.Visibility, spelled string) *node.Constant {
	nameNode := n.Child(l.v.fieldName)
	parts, comment := l.declParts(n, a)
	k := &node.Constant{
		Name: nameNode.Text(), Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
		Type: l.typeRef(n.Child(l.v.fieldType)), Value: n.Child(l.v.fieldValue).Text(),
		Annotations: a.annotations,
	}
	l.w.mark(k, parts, spelled, a)
	return k
}

// static lowers a static item as a Variable: mutable for static mut,
// and immutable otherwise.
func (l *lowering) static(n treesitter.Node, c container, a attributes) {
	vis, spelled := l.visibility(n)
	if !l.kept(vis) {
		l.skip(n)
		return
	}
	nameNode := n.Child(l.v.fieldName)
	parts, comment := l.declParts(n, a)
	v := &node.Variable{
		Name: nameNode.Text(), Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
		Mutability: symbol.MutabilityImmutable,
		Type:       l.typeRef(n.Child(l.v.fieldType)), Value: n.Child(l.v.fieldValue).Text(),
		Annotations: a.annotations,
	}
	if !l.firstOf(n, l.v.mutableSpecifier).IsZero() {
		v.Mutability = symbol.MutabilityMutable
	}
	l.declare(c, v, parts, spelled, a)
}

// typeAlias lowers a type item as an Alias of the type it names.
func (l *lowering) typeAlias(n treesitter.Node, c container, a attributes) {
	vis, spelled := l.visibility(n)
	if !l.kept(vis) {
		l.skip(n)
		return
	}
	nameNode := n.Child(l.v.fieldName)
	parts, comment := l.declParts(n, a)
	tps, lifetimes := l.typeParams(n)
	alias := &node.Alias{
		Name: nameNode.Text(), Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
		TypeParams: tps, Target: l.typeRef(n.Child(l.v.fieldType)), Annotations: a.annotations,
	}
	l.declare(c, alias, parts, spelled, a)
	l.lifetimes(alias, lifetimes)
}
