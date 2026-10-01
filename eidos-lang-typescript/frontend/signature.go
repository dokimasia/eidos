// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// thisName is the name of the explicit receiver a this parameter types.
const thisName = "this"

// The generic types whose one argument is the element of the array they
// name.
const (
	arrayName         = "Array"
	readonlyArrayName = "ReadonlyArray"
)

// params lowers a parameter list: each parameter's name, type, default
// verbatim and decorators, a ? parameter as optional and a rest
// parameter as positionally variadic, typed as one argument it takes. A
// this parameter types the receiver and is returned apart. A
// destructuring parameter has no name, and its identity is its
// position.
func (l *lowering) params(list treesitter.Node) (params []*node.Param, receiver *node.Param) {
	for p := range list.NamedChildren() {
		if p.Kind() != l.v.requiredParameter && p.Kind() != l.v.optionalParameter {
			continue
		}
		param := &node.Param{
			Pos:         p.Pos(),
			Type:        l.typeRef(p.Child(l.v.fieldType)),
			Default:     p.Child(l.v.fieldValue).Text(),
			Optional:    p.Kind() == l.v.optionalParameter,
			Annotations: l.decorators(p),
		}
		pattern := p.Child(l.v.fieldPattern)
		switch pattern.Kind() {
		case l.v.this:
			param.Name = thisName
			receiver = param
			continue
		case l.v.restPattern:
			param.Variadic = symbol.VariadicPositional
			param.Name = l.firstOf(pattern, l.v.identifier).Text()
			param.Type = element(param.Type)
		case l.v.identifier:
			param.Name = pattern.Text()
		}
		l.refuse(p, "a parameter")
		params = append(params, param)
	}
	return params, receiver
}

// element returns the type of one argument a rest parameter of a type
// takes: the element of an array type, readonly or not, and the argument
// of Array<T> and ReadonlyArray<T>, because the model types a variadic
// parameter as one argument it takes. Any other type, a tuple's
// included, states no element, and the parameter keeps it as written.
func element(t *node.TypeRef) *node.TypeRef {
	switch {
	case t == nil:
		return nil
	case t.Form == symbol.FormList:
		return t.Elems[0]
	case t.Form == symbol.FormNamed && (t.Spelling == arrayName || t.Spelling == readonlyArrayName) &&
		len(t.Args) == 1:
		return t.Args[0]
	default:
		return t
	}
}

// typeParams lowers a type parameter list: each parameter's name, its
// constraint as its bound, and its default.
func (l *lowering) typeParams(list treesitter.Node) []*node.TypeParam {
	var out []*node.TypeParam
	for tp := range list.NamedChildren() {
		if tp.Kind() != l.v.typeParameter {
			continue
		}
		name := tp.Child(l.v.fieldName)
		p := &node.TypeParam{Name: name.Text(), Pos: name.Pos()}
		if c := tp.Child(l.v.fieldConstraint); !c.IsZero() {
			p.Bounds = []*node.TypeRef{l.typeRef(l.firstType(c))}
		}
		if d := tp.Child(l.v.fieldValue); !d.IsZero() {
			p.Default = l.typeRef(l.firstType(d))
		}
		out = append(out, p)
	}
	return out
}

// returns lowers a return type annotation into the one result it
// states, a type predicate's and an assertion's included, and nothing
// for a callable that states none.
func (l *lowering) returns(annotation treesitter.Node) []*node.Return {
	if annotation.IsZero() {
		return nil
	}
	t := l.typeRef(annotation)
	return []*node.Return{{Pos: t.Pos, Type: t}}
}
