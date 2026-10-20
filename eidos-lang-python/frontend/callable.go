// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the decorators and of the methods that the lowering of a
// callable reads.
const (
	staticMethod   = "staticmethod"
	classMethod    = "classmethod"
	propertyName   = "property"
	cachedProperty = "cached_property"
	functoolsMod   = "functools"
	abcMod         = "abc"
	abstractMethod = "abstractmethod"
	overloadName   = "overload"
	finalName      = "final"
	overrideName   = "override"
	initMethod     = "__init__"
	newMethod      = "__new__"
	setterMark     = ".setter"
	getterMark     = ".getter"
	deleterMark    = ".deleter"
	asyncKeyword   = "async"
	returnsMark    = "->"
)

// signature is the lowered signature of a callable: its receiver, its
// parameters, its results, its type parameters, whether it is asynchronous
// and whether its body yields, and its docstring.
type signature struct {
	receiver   *node.Param
	params     []*node.Param
	returns    []*node.Return
	typeParams []*node.TypeParam
	async      bool
	generator  bool
	doc        []string
}

// accessorOf reports whether a decorator of a method is the decorator of a
// property of the name of the method with the mark, as name.setter.
func accessorOf(decorators []decorator, name, mark string) bool {
	for _, d := range decorators {
		if d.dotted && d.annotation.Name == name+mark {
			return true
		}
	}
	return false
}

// definition lowers the class or the callable of a statement. outermost is
// the statement, a decorated definition or the definition itself, whose
// comments and decorators belong to the declaration. host is the body of
// the class whose member the definition is, and nil at the top level of a
// module. definition returns the declaration at the top level of a module,
// and nil for a member, which it adds to the body of its class.
func (l *lowering) definition(outermost treesitter.Node, host *classBody) symbol.Symbol {
	def := outermost
	var decorators []decorator
	if outermost.Kind() == l.v.decoratedDefinition {
		def = outermost.Child(l.v.fieldDefinition)
		decorators = l.decoratorsOf(outermost)
	}
	switch {
	case def.Kind() == l.v.classDefinition:
		return l.class(def, outermost, decorators, host)
	case host != nil:
		if m := l.method(def, outermost, decorators, host); m != nil {
			host.methods = append(host.methods, m)
		}
		return nil
	default:
		return l.function(def, outermost, decorators)
	}
}

// function lowers a function at the top level of a module. The overloads
// of a name and the implementation after them are functions of one name,
// each with its discriminator. A function under the decorator overload of
// typing has the mark Overload. A function whose own body yields is
// stamped python.generator. A function that the signature depth leaves
// out, and a second declaration of a name, take their comments with them.
func (l *lowering) function(def, outermost treesitter.Node, decorators []decorator) symbol.Symbol {
	nameNode := def.Child(l.v.fieldName)
	name := nameNode.Text()
	overload := decoratedBy(decorators, typingModule, overloadName)
	if !l.declare(name, l.pos(nameNode), overload) {
		l.skip(outermost)
		return nil
	}
	vis := l.moduleVisibility(name)
	if l.signatures && vis != symbol.VisibilityPublic {
		l.skip(outermost)
		return nil
	}
	carriers, comment := l.declParts(outermost, def)
	sig := l.signature(def, false, nil)
	fn := &node.Function{
		Name: name, Pos: l.pos(nameNode), Doc: sig.doc, Comment: comment, Visibility: vis, Async: sig.async,
		Overload: overload, TypeParams: sig.typeParams, Params: sig.params, Returns: sig.returns,
	}
	fn.Annotations = l.decorate(fn, decorators)
	if sig.generator {
		l.u.Graph().Stamp(fn, meta.RawStamp{Key: python.GeneratorKey, Value: true, Pos: l.pos(def)})
	}
	l.u.AttachCarriers(fn, carriers, BadCarrier)
	return fn
}

// method lowers a method of a class: its receiver, which is its first
// parameter unless it is static, its level, its accessor, and the marks
// that its decorators and its body state. A static method and a class
// method are at type level, and so is __new__. __init__ and __new__
// construct. A getter of a property, and its setter and its deleter under
// the decorators of the name of the property, are three methods of one
// name. A method under the decorator overload of typing has the mark
// Overload. A method of a Protocol is abstract where its body has no
// statement beside a docstring, ... and pass, and has a default body
// otherwise.
func (l *lowering) method(def, outermost treesitter.Node, decorators []decorator, c *classBody) *node.Method {
	nameNode := def.Child(l.v.fieldName)
	name := nameNode.Text()
	accessor := symbol.AccessorNone
	switch {
	case decoratedBy(decorators, builtinsModule, propertyName), decoratedBy(decorators, functoolsMod, cachedProperty):
		accessor = symbol.AccessorGet
	case accessorOf(decorators, name, setterMark):
		accessor = symbol.AccessorSet
	case accessorOf(decorators, name, deleterMark):
		accessor = symbol.AccessorDelete
	case accessorOf(decorators, name, getterMark):
		accessor = symbol.AccessorGet
	}
	overload := decoratedBy(decorators, typingModule, overloadName)
	if !c.declare(l, name, l.pos(nameNode), overload, accessor != symbol.AccessorNone) {
		l.skip(outermost)
		return nil
	}
	vis := memberVisibility(name)
	if l.signatures && vis != symbol.VisibilityPublic {
		l.skip(outermost)
		return nil
	}
	static := decoratedBy(decorators, builtinsModule, staticMethod)
	carriers, comment := l.declParts(outermost, def)
	sig := l.signature(def, !static, c.params)
	abstract := decoratedBy(decorators, abcMod, abstractMethod)
	m := &node.Method{
		Name: name, Pos: l.pos(nameNode), Doc: sig.doc, Comment: comment, Visibility: vis,
		Hard:       vis == symbol.VisibilityPrivate,
		Abstract:   abstract || c.kind == protocolClass && l.bodiless(def),
		Final:      decoratedBy(decorators, typingModule, finalName),
		Override:   decoratedBy(decorators, typingModule, overrideName),
		Async:      sig.async,
		Overload:   overload,
		Accessor:   accessor,
		Constructs: name == initMethod || name == newMethod,
		Receiver:   sig.receiver,
		TypeParams: sig.typeParams,
		Params:     sig.params,
		Returns:    sig.returns,
	}
	m.HasDefault = c.kind == protocolClass && !m.Abstract
	if static || name == newMethod || decoratedBy(decorators, builtinsModule, classMethod) {
		m.Level = symbol.LevelType
	}
	m.Annotations = l.decorate(m, decorators)
	if sig.generator {
		l.u.Graph().Stamp(m, meta.RawStamp{Key: python.GeneratorKey, Value: true, Pos: l.pos(def)})
	}
	l.u.AttachCarriers(m, carriers, BadCarrier)
	if !l.signatures && !static && sig.receiver != nil {
		body := methodBody{block: def.Child(l.v.fieldBody), self: sig.receiver.Name, init: name == initMethod}
		c.bodies = append(c.bodies, body)
	}
	return m
}

// signature lowers the signature of a callable: its type parameters, its
// parameters, its receiver where method is set, its results, and its
// asynchrony. The type parameters are the parameters of the syntax of PEP
// 695, and then the type variables of the module that the signature names
// and the enclosing class does not bind. A type comment states the types of
// the parameters and the result that the signature does not annotate. The
// docstring is the documentation of the callable. The body of the callable
// decides whether it yields at every depth, because a call of a generator
// returns its iterator, and an asynchronous generator is not asynchronous.
func (l *lowering) signature(def treesitter.Node, method bool, bound map[string]bool) signature {
	var sig signature
	sig.doc, _ = l.docstring(def.Child(l.v.fieldBody))
	sig.typeParams = l.typeParams(def.Child(l.v.fieldTypeParameters))
	params := l.params(def.Child(l.v.fieldParameters))
	if method && len(params) > 0 {
		sig.receiver, params = params[0], params[1:]
	}
	positional := slices.ContainsFunc(params, func(p *node.Param) bool {
		return p.Passing == symbol.PassingPositional
	})
	for _, p := range params {
		if positional || p.Variadic != symbol.VariadicNone || !strings.HasPrefix(p.Name, dunderMark) || dunder(p.Name) {
			break
		}
		p.Passing = symbol.PassingPositional
	}
	sig.params = params
	result := def.Child(l.v.fieldReturnType)
	commentTypes, commentResult := l.commentSignature(def)
	if len(commentTypes) == len(params) {
		for i, p := range params {
			if p.Type == nil {
				p.Type = commentTypes[i]
			}
		}
	}
	name := def.Child(l.v.fieldName).Text()
	switch {
	case !result.IsZero():
		sig.returns = l.returns(result)
	case commentResult != nil:
		sig.returns = []*node.Return{{Pos: commentResult.Pos, Type: commentResult}}
		if commentResult.Spelling == noneName {
			sig.returns = nil
		}
	case name != initMethod:
		sig.returns = []*node.Return{{Pos: l.pos(def.Child(l.v.fieldName))}}
	}
	sig.async = def.HasToken(asyncKeyword)
	sig.generator = l.yields(def.Child(l.v.fieldBody))
	if sig.generator {
		sig.async = false
	}
	taken := map[string]bool{}
	maps.Copy(taken, bound)
	for _, tp := range sig.typeParams {
		taken[tp.Name] = true
	}
	refs := make([]*node.TypeRef, 0, len(params)+1)
	for _, p := range params {
		refs = append(refs, p.Type)
	}
	for _, r := range sig.returns {
		refs = append(refs, r.Type)
	}
	sig.typeParams = append(sig.typeParams, l.usedVars(refs, taken)...)
	return sig
}

// params lowers a parameter list: the name, the annotation and the default
// of each parameter as written. A parameter before / passes by position,
// and a parameter after * or after *args by keyword. *args is variadic by
// position and **kwargs by keyword, each with the type of one argument that
// it takes. A tuple parameter of Python 2 has no form in the model, and
// reports under [UnmodeledConstruct]. A parameter is a subject that the
// model cannot address in a comment, so params refuses its carriers.
func (l *lowering) params(list treesitter.Node) []*node.Param {
	var out []*node.Param
	passing := symbol.PassingDefault
	for p := range list.Content() {
		param := &node.Param{Pos: l.pos(p), Passing: passing}
		var annotated treesitter.Node
		switch p.Kind() {
		case l.v.positionalSeparator:
			for _, before := range out {
				before.Passing = symbol.PassingPositional
			}
			continue
		case l.v.keywordSeparator:
			passing = symbol.PassingKeyword
			continue
		case l.v.identifier:
			param.Name = p.Text()
		case l.v.typedParameter:
			param.Name, param.Variadic = l.paramName(firstNamed(p))
			annotated = p.Child(l.v.fieldType)
		case l.v.defaultParameter, l.v.typedDefaultParameter:
			if name := p.Child(l.v.fieldName); name.Kind() != l.v.identifier {
				l.unmodeled(p)
				continue
			}
			param.Name = p.Child(l.v.fieldName).Text()
			param.Default = p.Child(l.v.fieldValue).Text()
			annotated = p.Child(l.v.fieldType)
		case l.v.listSplatPattern, l.v.dictionarySplatPattern:
			param.Name, param.Variadic = l.paramName(p)
		default:
			l.unmodeled(p)
			continue
		}
		if param.Variadic == symbol.VariadicPositional {
			passing = symbol.PassingKeyword
		}
		if !annotated.IsZero() {
			a := l.annotationOf(annotated)
			param.Type = a.ref
			l.annotatedStamp(param, a)
		}
		l.refuse(p, "a parameter")
		out = append(out, param)
	}
	return out
}

// paramName returns the name of a parameter target and its variadic form:
// a name, *name by position and **name by keyword.
func (l *lowering) paramName(target treesitter.Node) (string, symbol.Variadic) {
	switch target.Kind() {
	case l.v.listSplatPattern:
		return firstNamed(target).Text(), symbol.VariadicPositional
	case l.v.dictionarySplatPattern:
		return firstNamed(target).Text(), symbol.VariadicKeyword
	default:
		return target.Text(), symbol.VariadicNone
	}
}

// unmodeled reports a construct of Python 2 in a signature under
// [UnmodeledConstruct], and takes its comments.
func (l *lowering) unmodeled(n treesitter.Node) {
	l.u.Warnf(
		UnmodeledConstruct,
		l.pos(n),
		"the parameter %s is a construct of Python 2 without a form in the model, and the declaration loads without it",
		n.Compact(),
	)
	l.skip(n)
}

// returns lowers the return annotation of a callable: one result, and no
// result for -> None.
func (l *lowering) returns(annotated treesitter.Node) []*node.Return {
	a := l.annotationOf(annotated)
	if a.ref != nil && a.ref.Form == symbol.FormNamed && a.ref.Spelling == noneName && len(a.ref.Args) == 0 {
		return nil
	}
	r := &node.Return{Pos: l.pos(annotated), Type: a.ref}
	l.annotatedStamp(r, a)
	return []*node.Return{r}
}

// annotatedStamp stamps the metadata of an Annotated on a declaration
// under python.annotated: the elements after the type where Annotated wraps
// the whole annotation, and the whole annotation where an Annotated is
// inside another type.
func (l *lowering) annotatedStamp(subject symbol.Symbol, a annotation) {
	value := a.metadata
	if a.nested {
		value = append(slices.Clone(value), a.whole)
	}
	if len(value) > 0 {
		l.u.Graph().Stamp(subject, meta.RawStamp{Key: python.AnnotatedKey, Value: value, Pos: a.pos})
	}
}

// commentSignature returns the types of the parameters and the result that
// the type comment of a callable states, as (A, B) -> R, and nil for a
// callable without one. The types of the parameters leave out the
// receiver, as PEP 484 writes them, and ... states no parameter type.
func (l *lowering) commentSignature(def treesitter.Node) ([]*node.TypeRef, *node.TypeRef) {
	text, at := l.typeComment(def)
	params, result, signed := strings.Cut(text, returnsMark)
	if !signed {
		return nil, nil
	}
	var types []*node.TypeRef
	var ret *node.TypeRef
	l.parseAt(strings.TrimSpace(params), at, func(typed treesitter.Node) {
		inner := l.inner(typed)
		switch {
		case inner.Kind() == l.v.tuple:
			for e := range inner.Content() {
				if k := e.Kind(); k == l.v.listSplat || k == l.v.dictionarySplat {
					e = firstNamed(e)
				}
				types = append(types, l.typeRef(e))
			}
		case inner.Kind() != l.v.ellipsis:
			types = append(types, l.typeRef(inner))
		}
	})
	resultAt := at
	resultAt.Col += len(text) - len(strings.TrimLeft(result, blanks))
	l.parseAt(strings.TrimSpace(result), resultAt, func(typed treesitter.Node) { ret = l.typeRef(typed) })
	return types, ret
}

// bodiless reports whether the body of a callable has no statement beside a
// docstring, ... and pass, as the body of a method of a Protocol that a
// class must implement has.
func (l *lowering) bodiless(def treesitter.Node) bool {
	for stmt := range def.Child(l.v.fieldBody).Content() {
		if k := stmt.Kind(); k != l.v.stringNode && k != l.v.ellipsis && k != l.v.passStatement {
			return false
		}
	}
	return true
}

// yields reports whether a body yields: a yield in a statement of the body,
// outside a callable, a class and a lambda nested in it.
func (l *lowering) yields(n treesitter.Node) bool {
	for child := range n.NamedChildren() {
		switch child.Kind() {
		case l.v.yield:
			return true
		case l.v.functionDefinition, l.v.classDefinition, l.v.lambda, l.v.decoratedDefinition:
		default:
			if l.yields(child) {
				return true
			}
		}
	}
	return false
}
