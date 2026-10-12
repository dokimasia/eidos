// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// thisName is the name of the explicit receiver that a this parameter
// types.
const thisName = "this"

// params lowers a parameter list: the name, the type, the default
// verbatim and the decorators of each parameter. A ? parameter is
// optional. A rest parameter is positionally variadic and has the type of
// one argument that it takes. params reports [ReadonlyRest] for a rest
// parameter of a readonly array, because that type has no readonly. A this
// parameter types the receiver, and params returns it apart. A
// destructuring parameter has no name, and its identity is its position.
// A parameter is a subject that the model cannot address, so params
// refuses its carriers and its markers.
func (l *lowering) params(list treesitter.Node) (params []*node.Param, receiver *node.Param) {
	for p := range list.NamedChildren() {
		if p.Kind() != l.v.requiredParameter && p.Kind() != l.v.optionalParameter {
			continue
		}
		param := &node.Param{
			Pos:      p.Pos(),
			Type:     l.typeRef(p.Child(l.v.fieldType)),
			Default:  p.Child(l.v.fieldValue).Text(),
			Optional: p.Kind() == l.v.optionalParameter,
		}
		decorators := l.decoratorsOf(p)
		for _, d := range decorators {
			param.Annotations = append(param.Annotations, l.decorator(d))
		}
		l.refuseMarkers(decorators, "a parameter")
		pattern := p.Child(l.v.fieldPattern)
		switch pattern.Kind() {
		case l.v.this:
			param.Name = thisName
			receiver = param
			continue
		case l.v.restPattern:
			param.Variadic = symbol.VariadicPositional
			param.Name = pattern.FirstOf(l.v.identifier).Text()
			var readonly bool
			param.Type, readonly = element(param.Type)
			if readonly {
				l.u.Warnf(ReadonlyRest, p.Pos(),
					"the model types the rest parameter %s as one argument without the readonly of its array",
					param.Name)
			}
		case l.v.identifier:
			param.Name = pattern.Text()
		}
		l.refuse(p, "a parameter")
		params = append(params, param)
	}
	return params, receiver
}

// element returns the type of one argument that a rest parameter of type
// t takes, and reports whether t is a readonly array. The model types a
// variadic parameter as one argument that it takes. The element of an
// array type is its child, and the element of Array<T> and
// ReadonlyArray<T> is T. Any other type, a tuple included, has no element,
// and element returns it as written.
func element(t *node.TypeRef) (*node.TypeRef, bool) {
	switch {
	case t == nil:
		return nil, false
	case t.Form == symbol.FormList:
		return t.Elems[0], t.Mutability == symbol.MutabilityImmutable
	case t.Form == symbol.FormNamed &&
		(t.Spelling == typescript.TypeArray || t.Spelling == typescript.TypeReadonlyArray) && len(t.Args) == 1:
		return t.Args[0], t.Spelling == typescript.TypeReadonlyArray
	default:
		return t, false
	}
}

// typeParams lowers a type parameter list: the name of each parameter, its
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

// result lowers the return type annotation of a callable to the results
// of the canonical callable, and reports whether the result is
// asynchronous:
//
//   - Promise<T> lowers to the asynchronous result T, and Promise<void>
//     lowers to no result, asynchronously;
//   - void lowers to no result;
//   - without an annotation, the callable has one result without a type
//     at the position at when typed is set, and no result otherwise;
//   - any other type is the one result, as [lowering.returns] lowers it.
//
// Promise and void are global types of TypeScript, so a name that an
// import binds is neither of them.
func (l *lowering) result(annotation treesitter.Node, typed bool, at position.Pos) ([]*node.Return, bool) {
	if annotation.IsZero() {
		if !typed {
			return nil, false
		}
		return []*node.Return{{Pos: at}}, false
	}
	t := l.typeRef(annotation)
	async := false
	if t.Form == symbol.FormNamed && t.Package == "" && t.Spelling == typescript.TypePromise && len(t.Args) == 1 {
		t, async = t.Args[0], true
	}
	if t.Form == symbol.FormNamed && t.Package == "" && t.Spelling == typescript.TypeVoid && len(t.Args) == 0 {
		return nil, async
	}
	return []*node.Return{{Pos: t.Pos, Type: t}}, async
}
