// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// constructorName, constructName and indexerName are the names that the
// model gives a constructor, the construct signature of an interface and
// an index signature. TypeScript writes these members without a name.
const (
	constructorName = "constructor"
	constructName   = "new"
	indexerName     = "[]"
)

// accessProtected and accessPrivate are the accessibility modifiers that
// restrict a member. A member with public or without a modifier is
// public.
const (
	accessProtected = "protected"
	accessPrivate   = "private"
)

// classBody lowers the members of a class into a struct: its methods and
// overload signatures, its fields, the fields that the parameter
// properties of its constructor declare, and its index signatures. The
// lowering leaves out the implementation of a method after its overload
// signatures, because TypeScript hides it from callers. A decorator
// applies to the member after it. A static block declares nothing.
// classBody refuses a marker on the implementation of an overloaded
// method, because the implementation declares nothing.
func (l *lowering) classBody(body treesitter.Node, st *node.Struct) {
	overloaded := map[string]bool{}
	for m := range body.NamedChildren() {
		if m.Kind() == l.v.methodSignature {
			overloaded[l.nameText(m.Child(l.v.fieldName))] = true
		}
	}
	var pending []treesitter.Node
	for m := range body.Content() {
		switch m.Kind() {
		case l.v.decorator:
			pending = append(pending, m)
			continue
		case l.v.methodDefinition:
			if overloaded[l.nameText(m.Child(l.v.fieldName))] {
				l.refuse(m, "an overloaded method's implementation")
				l.refuseMarkers(pending, "an overloaded method's implementation")
				break
			}
			l.method(m, st, pending)
		case l.v.methodSignature, l.v.abstractMethodSignature:
			l.method(m, st, pending)
		case l.v.publicFieldDefinition:
			if f := l.field(m, pending); f != nil {
				st.Fields = append(st.Fields, f)
			}
		case l.v.indexSignature:
			if m := l.indexer(m); m != nil {
				st.Methods = append(st.Methods, m)
			}
		default:
			l.refuse(m, "a static block")
		}
		pending = nil
	}
}

// method lowers one method, overload signature or abstract signature and
// the decorators before it into a struct. The parameter properties of a
// constructor also declare fields of the struct.
func (l *lowering) method(m treesitter.Node, st *node.Struct, decorators []treesitter.Node) {
	built := l.methodOf(m, decorators, false)
	if built == nil {
		return
	}
	if built.Constructs {
		st.Fields = append(st.Fields, l.parameterProperties(m.Child(l.v.fieldParameters), built.Params)...)
	}
	st.Methods = append(st.Methods, built)
}

// methodOf builds one method: its accessibility, its static level, its
// accessor kind, its async, abstract and optional marks, its override
// modifier, the hard privacy of a # name, its signature and its
// decorators. A name in brackets is computed, and the method keeps the
// brackets in its name. A method named constructor constructs.
// [lowering.result] lowers its result. The method is async when it has
// the async keyword or returns a promise. A constructor and a setter
// without a return type have no result. methodOf stamps
// typescript.generator on a generator method, which has a * before its
// name. inInterface builds the method signature of an interface, which is
// public and abstract. methodOf returns nil for a member that the
// signature depth leaves out: a private one or one with a # name.
func (l *lowering) methodOf(m treesitter.Node, decorators []treesitter.Node, inInterface bool) *node.Method {
	nameNode := m.Child(l.v.fieldName)
	name := l.nameText(nameNode)
	hard := nameNode.Kind() == l.v.privatePropertyIdentifier
	vis := l.access(m, hard)
	if inInterface {
		vis = symbol.VisibilityPublic
	}
	if l.u.Depth() == plugin.DepthSignatures && vis == symbol.VisibilityPrivate {
		l.skip(m)
		return nil
	}
	parts, comment := l.declParts(m)
	params, receiver := l.params(m.Child(l.v.fieldParameters))
	returns, promised := l.result(m.Child(l.v.fieldReturnType),
		name != constructorName && !m.HasToken(keywordSet), nameNode.Pos())
	built := &node.Method{
		Name: name, Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
		Abstract:   inInterface || m.Kind() == l.v.abstractMethodSignature || m.HasToken(keywordAbstract),
		Override:   !m.FirstOf(l.v.overrideModifier).IsZero(),
		Async:      promised || m.HasToken(keywordAsync),
		Hard:       hard,
		Optional:   m.HasToken(keywordOptional),
		Computed:   nameNode.Kind() == l.v.computedPropertyName,
		Receiver:   receiver,
		TypeParams: l.typeParams(m.Child(l.v.fieldTypeParameters)),
		Params:     params,
		Returns:    returns,
	}
	built.Annotations = l.decorate(built, decorators)
	if m.HasToken(keywordStatic) {
		built.Level = symbol.LevelType
	}
	switch {
	case m.HasToken(keywordGet):
		built.Accessor = symbol.AccessorGet
	case m.HasToken(keywordSet):
		built.Accessor = symbol.AccessorSet
	}
	if name == constructorName {
		built.Constructs = true
	}
	if m.HasToken(keywordStar) {
		l.mark(built, typescript.GeneratorKey, m.Pos())
	}
	l.attach(built, parts.Carriers)
	return built
}

// parameterProperties returns the fields that the parameter properties of
// a constructor declare. Each parameter with an accessibility modifier or
// readonly declares a field of its name. A readonly parameter declares an
// immutable field. A parameter with a ? declares an optional field.
// parameterProperties stamps typescript.parameterProperty on the field and
// on the parameter of its name among params. The signature depth leaves
// out a private field and keeps the stamp of its parameter, because the
// constructor still takes the parameter.
func (l *lowering) parameterProperties(list treesitter.Node, params []*node.Param) []*node.Field {
	var out []*node.Field
	for p := range list.NamedChildren() {
		if p.Kind() != l.v.requiredParameter && p.Kind() != l.v.optionalParameter {
			continue
		}
		modifier := p.FirstOf(l.v.accessibilityModifier)
		readonly := p.HasToken(keywordReadonly)
		pattern := p.Child(l.v.fieldPattern)
		if modifier.IsZero() && !readonly || pattern.Kind() != l.v.identifier {
			continue
		}
		for _, param := range params {
			if param.Name == pattern.Text() {
				l.mark(param, typescript.ParameterPropertyKey, pattern.Pos())
			}
		}
		vis := l.access(p, false)
		if l.u.Depth() == plugin.DepthSignatures && vis == symbol.VisibilityPrivate {
			continue
		}
		f := &node.Field{
			Name: pattern.Text(), Pos: pattern.Pos(), Visibility: vis,
			Mutability: symbol.MutabilityMutable,
			Optional:   p.Kind() == l.v.optionalParameter,
			Type:       l.typeRef(p.Child(l.v.fieldType)),
		}
		if readonly {
			f.Mutability = symbol.MutabilityImmutable
		}
		l.mark(f, typescript.ParameterPropertyKey, pattern.Pos())
		out = append(out, f)
	}
	return out
}

// field lowers one class property: its accessibility, its static level,
// readonly as immutable, ? as optional, the hard privacy of a # name, its
// type, its initializer verbatim, and the decorators before it and inside
// it. A name in brackets is computed, and the field keeps the brackets in
// its name. field stamps typescript.definiteAssignment on a
// property declared with !, and typescript.abstract on an abstract
// property. field returns nil for a property that the signature depth
// leaves out.
func (l *lowering) field(m treesitter.Node, decorators []treesitter.Node) *node.Field {
	nameNode := m.Child(l.v.fieldName)
	hard := nameNode.Kind() == l.v.privatePropertyIdentifier
	vis := l.access(m, hard)
	if l.u.Depth() == plugin.DepthSignatures && vis == symbol.VisibilityPrivate {
		l.skip(m)
		return nil
	}
	parts, comment := l.declParts(m)
	f := &node.Field{
		Name: l.nameText(nameNode), Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
		Mutability: symbol.MutabilityMutable,
		Hard:       hard,
		Optional:   m.HasToken(keywordOptional),
		Computed:   nameNode.Kind() == l.v.computedPropertyName,
		Type:       l.typeRef(m.Child(l.v.fieldType)),
		Value:      m.Child(l.v.fieldValue).Text(),
	}
	f.Annotations = l.decorate(f, append(decorators, l.decoratorsOf(m)...))
	if m.HasToken(keywordStatic) {
		f.Level = symbol.LevelType
	}
	if m.HasToken(keywordReadonly) {
		f.Mutability = symbol.MutabilityImmutable
	}
	if m.HasToken(keywordDefinite) {
		l.mark(f, typescript.DefiniteAssignmentKey, m.Pos())
	}
	if m.HasToken(keywordAbstract) {
		l.mark(f, typescript.AbstractKey, m.Pos())
	}
	l.u.AttachCarriers(f, parts.Carriers, BadCarrier)
	return f
}

// indexer lowers an index signature to a method named [] that takes the
// key and returns the value. The method is immutable where the signature
// is readonly, and mutable otherwise. The signature of a mapped type has
// no key type and declares nothing.
func (l *lowering) indexer(m treesitter.Node) *node.Method {
	key := m.Child(l.v.fieldIndexType)
	if key.IsZero() {
		return nil
	}
	parts, comment := l.declParts(m)
	built := &node.Method{
		Name: indexerName, Pos: m.Pos(), Doc: parts.Docs, Comment: comment,
		Visibility: symbol.VisibilityPublic, Indexer: true, Mutability: symbol.MutabilityMutable,
		Params:  []*node.Param{{Name: m.Child(l.v.fieldName).Text(), Pos: key.Pos(), Type: l.typeRef(key)}},
		Returns: []*node.Return{{Pos: m.Child(l.v.fieldType).Pos(), Type: l.typeRef(m.Child(l.v.fieldType))}},
	}
	if m.HasToken(keywordReadonly) {
		built.Mutability = symbol.MutabilityImmutable
	}
	l.attach(built, parts.Carriers)
	return built
}

// objectMembers lowers the members of an interface body or an object type
// into fields and methods. A property signature is a field, a method
// signature an abstract method, a construct signature a method named new
// that constructs, and an index signature a method named []. It returns
// the call signatures as written. The caller of an interface stamps them,
// and the spelling of an inline body keeps them.
//
// objectMembers also returns the types that the members state, in the
// order of the body, as the children of an inline body: the type of each
// property, and for each signature the bounds and the defaults of its type
// parameters, the type of its receiver, its parameter types and its result
// type, as the method records them. A rest parameter has the type of one
// argument, and a void result and the promise of an async result have no
// child of their own. Each child is a copy of the type of its member, which
// [node.CloneRef] returns, because a node of the graph has one place. A
// parameter or a result without a type has no child.
func (l *lowering) objectMembers(
	body treesitter.Node, fields *[]*node.Field, methods *[]*node.Method,
) (calls []string, children []*node.TypeRef) {
	for m := range body.NamedChildren() {
		var built *node.Method
		switch m.Kind() {
		case l.v.propertySignature:
			f := l.property(m)
			*fields = append(*fields, f)
			if f.Type != nil {
				children = append(children, node.CloneRef(f.Type))
			}
		case l.v.methodSignature:
			built = l.methodOf(m, nil, true)
		case l.v.constructSignature:
			built = l.construct(m)
		case l.v.callSignature:
			calls = append(calls, m.Compact())
			l.refuse(m, "a call signature")
		case l.v.indexSignature:
			built = l.indexer(m)
		}
		if built == nil {
			continue
		}
		*methods = append(*methods, built)
		for _, tp := range built.TypeParams {
			for _, b := range tp.Bounds {
				children = append(children, node.CloneRef(b))
			}
			if tp.Default != nil {
				children = append(children, node.CloneRef(tp.Default))
			}
		}
		if built.Receiver != nil && built.Receiver.Type != nil {
			children = append(children, node.CloneRef(built.Receiver.Type))
		}
		for _, p := range built.Params {
			if p.Type != nil {
				children = append(children, node.CloneRef(p.Type))
			}
		}
		for _, r := range built.Returns {
			if r.Type != nil {
				children = append(children, node.CloneRef(r.Type))
			}
		}
	}
	return calls, children
}

// callSignatures returns the call signatures of an object type as written.
func (l *lowering) callSignatures(body treesitter.Node) []string {
	var calls []string
	for m := range body.NamedChildren() {
		if m.Kind() == l.v.callSignature {
			calls = append(calls, m.Compact())
		}
	}
	return calls
}

// property lowers the property signature of an interface to a public field
// with its type. The field is immutable where the signature is readonly,
// optional where the signature has a ?, and computed where its name is in
// brackets.
func (l *lowering) property(m treesitter.Node) *node.Field {
	nameNode := m.Child(l.v.fieldName)
	parts, comment := l.declParts(m)
	f := &node.Field{
		Name: l.nameText(nameNode), Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment,
		Visibility: symbol.VisibilityPublic,
		Mutability: symbol.MutabilityMutable,
		Optional:   m.HasToken(keywordOptional),
		Computed:   nameNode.Kind() == l.v.computedPropertyName,
		Type:       l.typeRef(m.Child(l.v.fieldType)),
	}
	if m.HasToken(keywordReadonly) {
		f.Mutability = symbol.MutabilityImmutable
	}
	l.attach(f, parts.Carriers)
	return f
}

// construct lowers the construct signature of an interface to a public
// method named new that constructs the type of its annotation.
func (l *lowering) construct(m treesitter.Node) *node.Method {
	parts, comment := l.declParts(m)
	params, _ := l.params(m.Child(l.v.fieldParameters))
	built := &node.Method{
		Name: constructName, Pos: m.Pos(), Doc: parts.Docs, Comment: comment,
		Visibility: symbol.VisibilityPublic, Abstract: true, Constructs: true,
		TypeParams: l.typeParams(m.Child(l.v.fieldTypeParameters)),
		Params:     params,
		Returns:    l.returns(m.Child(l.v.fieldType)),
	}
	l.attach(built, parts.Carriers)
	return built
}

// access returns private for a # name, the visibility of the
// accessibility modifier of a member, and public for a member without
// either.
func (l *lowering) access(m treesitter.Node, hard bool) symbol.Visibility {
	if hard {
		return symbol.VisibilityPrivate
	}
	switch m.FirstOf(l.v.accessibilityModifier).Text() {
	case accessProtected:
		return symbol.VisibilityProtected
	case accessPrivate:
		return symbol.VisibilityPrivate
	default:
		return symbol.VisibilityPublic
	}
}
