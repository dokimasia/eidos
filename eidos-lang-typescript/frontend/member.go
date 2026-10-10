// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go.dokimi.dev/eidos/lang/treesitter"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names the model gives the members TypeScript writes without one:
// a constructor, an interface's construct signature, and an index
// signature.
const (
	constructorName = "constructor"
	constructName   = "new"
	indexerName     = "[]"
)

// The accessibility modifiers' spellings that restrict a member. A
// member without one, or with public, is public.
const (
	accessProtected = "protected"
	accessPrivate   = "private"
)

// classBody lowers a class's members into a struct: its methods and
// overload signatures, its fields, the fields its constructor's
// parameter properties declare, and its index signatures. A method's
// implementation after its overload signatures is left out, because
// TypeScript hides it from callers. A decorator applies to the member
// after it. A static block declares nothing. A marker on an overloaded
// method's implementation is refused, because the implementation
// declares nothing.
func (l *lowering) classBody(body treesitter.Node, st *node.Struct) {
	overloaded := map[string]bool{}
	for m := range body.NamedChildren() {
		if m.Kind() == l.v.methodSignature {
			overloaded[l.nameText(m.Child(l.v.fieldName))] = true
		}
	}
	var pending []treesitter.Node
	for m := range body.NamedChildren() {
		switch m.Kind() {
		case l.v.comment:
			continue
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

// method lowers one method, overload signature or abstract signature
// into a struct, with the decorators before it. A constructor's
// parameter properties declare the struct's fields too.
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

// methodOf builds one method: its accessibility, static level, accessor
// kind, async and abstract marks, override modifier, a # name's hard
// privacy, its signature and its decorators. A method named
// constructor constructs. [lowering.result] lowers its result. The
// method is async when it has the async keyword or returns a promise. A
// constructor and a setter without a return type have no result.
// A generator method, which its * declares, is stamped
// typescript.generator, and a method declared with ?
// typescript.optional. inInterface builds an interface's method
// signature, which is public and abstract. It returns nil for a member
// signature depth leaves out: a private or #-named one.
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
		name != constructorName && !l.token(m, keywordSet), nameNode.Pos())
	built := &node.Method{
		Name: name, Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment, Visibility: vis,
		Abstract:   inInterface || m.Kind() == l.v.abstractMethodSignature || l.token(m, keywordAbstract),
		Override:   !l.firstOf(m, l.v.overrideModifier).IsZero(),
		Async:      promised || l.token(m, keywordAsync),
		Hard:       hard,
		Receiver:   receiver,
		TypeParams: l.typeParams(m.Child(l.v.fieldTypeParameters)),
		Params:     params,
		Returns:    returns,
	}
	built.Annotations = l.decorate(built, decorators)
	if l.token(m, keywordStatic) {
		built.Level = symbol.LevelType
	}
	switch {
	case l.token(m, keywordGet):
		built.Accessor = symbol.AccessorGet
	case l.token(m, keywordSet):
		built.Accessor = symbol.AccessorSet
	}
	if name == constructorName {
		built.Constructs = true
	}
	if l.token(m, keywordStar) {
		l.mark(built, typescript.GeneratorKey, m.Pos())
	}
	if l.token(m, keywordOptional) {
		l.mark(built, typescript.OptionalKey, m.Pos())
	}
	l.attach(built, parts.Carriers)
	return built
}

// parameterProperties returns the fields a constructor's parameter
// properties declare: each parameter with an accessibility modifier or
// readonly, under its name, readonly as immutable, a ? parameter as
// optional. The field and the constructor's parameter of its name,
// among params, are stamped typescript.parameterProperty. Signature
// depth leaves out a private field and keeps its parameter's stamp,
// because the constructor still takes the parameter.
func (l *lowering) parameterProperties(list treesitter.Node, params []*node.Param) []*node.Field {
	var out []*node.Field
	for p := range list.NamedChildren() {
		if p.Kind() != l.v.requiredParameter && p.Kind() != l.v.optionalParameter {
			continue
		}
		modifier := l.firstOf(p, l.v.accessibilityModifier)
		readonly := l.token(p, keywordReadonly)
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

// field lowers one class property: its accessibility, static level,
// readonly as immutable, ? as optional, a # name's hard privacy, its
// type, its initializer verbatim, and the decorators before it and
// inside it. A property declared with ! is stamped
// typescript.definiteAssignment. It returns nil for a property
// signature depth leaves out.
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
		Optional:   l.token(m, keywordOptional),
		Type:       l.typeRef(m.Child(l.v.fieldType)),
		Value:      m.Child(l.v.fieldValue).Text(),
	}
	f.Annotations = l.decorate(f, append(decorators, l.decoratorsOf(m)...))
	if l.token(m, keywordStatic) {
		f.Level = symbol.LevelType
	}
	if l.token(m, keywordReadonly) {
		f.Mutability = symbol.MutabilityImmutable
	}
	if l.token(m, keywordDefinite) {
		l.mark(f, typescript.DefiniteAssignmentKey, m.Pos())
	}
	l.u.AttachCarriers(f, parts.Carriers, BadCarrier)
	return f
}

// indexer lowers an index signature: a method named [] that takes the
// key and returns the value, stamped typescript.readonly where the
// signature is readonly. A mapped type's signature names no key type and
// declares nothing.
func (l *lowering) indexer(m treesitter.Node) *node.Method {
	key := m.Child(l.v.fieldIndexType)
	if key.IsZero() {
		return nil
	}
	parts, comment := l.declParts(m)
	built := &node.Method{
		Name: indexerName, Pos: m.Pos(), Doc: parts.Docs, Comment: comment,
		Visibility: symbol.VisibilityPublic, Indexer: true,
		Params:  []*node.Param{{Name: m.Child(l.v.fieldName).Text(), Pos: key.Pos(), Type: l.typeRef(key)}},
		Returns: []*node.Return{{Pos: m.Child(l.v.fieldType).Pos(), Type: l.typeRef(m.Child(l.v.fieldType))}},
	}
	if l.token(m, keywordReadonly) {
		l.mark(built, typescript.ReadonlyKey, m.Pos())
	}
	l.attach(built, parts.Carriers)
	return built
}

// objectMembers lowers an interface body's or an object type's members
// into fields and methods: a property signature a field, a method
// signature an abstract method, a construct signature a method named
// new that constructs, and an index signature a method named []. It
// returns the call signatures as written, which an interface's caller
// stamps and an inline body's spelling keeps.
func (l *lowering) objectMembers(body treesitter.Node, fields *[]*node.Field, methods *[]*node.Method) []string {
	var calls []string
	for m := range body.NamedChildren() {
		switch m.Kind() {
		case l.v.propertySignature:
			*fields = append(*fields, l.property(m))
		case l.v.methodSignature:
			if built := l.methodOf(m, nil, true); built != nil {
				*methods = append(*methods, built)
			}
		case l.v.constructSignature:
			*methods = append(*methods, l.construct(m))
		case l.v.callSignature:
			calls = append(calls, m.Compact())
			l.refuse(m, "a call signature")
		case l.v.indexSignature:
			if built := l.indexer(m); built != nil {
				*methods = append(*methods, built)
			}
		}
	}
	return calls
}

// callSignatures returns an object type's call signatures as written.
func (l *lowering) callSignatures(body treesitter.Node) []string {
	var calls []string
	for m := range body.NamedChildren() {
		if m.Kind() == l.v.callSignature {
			calls = append(calls, m.Compact())
		}
	}
	return calls
}

// property lowers an interface's property signature: public, readonly
// as immutable, ? as optional, with its type.
func (l *lowering) property(m treesitter.Node) *node.Field {
	nameNode := m.Child(l.v.fieldName)
	parts, comment := l.declParts(m)
	f := &node.Field{
		Name: l.nameText(nameNode), Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment,
		Visibility: symbol.VisibilityPublic,
		Mutability: symbol.MutabilityMutable,
		Optional:   l.token(m, keywordOptional),
		Type:       l.typeRef(m.Child(l.v.fieldType)),
	}
	if l.token(m, keywordReadonly) {
		f.Mutability = symbol.MutabilityImmutable
	}
	l.attach(f, parts.Carriers)
	return f
}

// construct lowers an interface's construct signature: a public method
// named new that constructs what its type annotation names.
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

// access returns a member's visibility: its accessibility modifier's,
// private for a # name, and public without either.
func (l *lowering) access(m treesitter.Node, hard bool) symbol.Visibility {
	if hard {
		return symbol.VisibilityPrivate
	}
	switch l.firstOf(m, l.v.accessibilityModifier).Text() {
	case accessProtected:
		return symbol.VisibilityProtected
	case accessPrivate:
		return symbol.VisibilityPrivate
	default:
		return symbol.VisibilityPublic
	}
}
