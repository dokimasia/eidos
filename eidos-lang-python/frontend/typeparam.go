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

// The keyword arguments of a call of TypeVar that the lowering reads.
const (
	boundKeyword         = "bound"
	covariantKeyword     = "covariant"
	contravariantKeyword = "contravariant"
	defaultKeyword       = "default"
)

// The marks of a variadic type parameter: the star of a type variable tuple
// and the two stars of a parameter specification. A type parameter keeps
// its mark in its name, as *Ts and **P, because the model has no form for a
// variadic type parameter.
const (
	tupleMark = "*"
	specMark  = "**"
)

// typeVarMarks maps each callee that declares a type variable to the mark
// of its type parameter.
var typeVarMarks = map[string]string{python.TypeVar: "", python.TypeVarTuple: tupleMark, python.ParamSpec: specMark}

// The separators of the spelling of the constraints of a type variable.
const (
	constraintsOpen  = "("
	constraintsSep   = ", "
	constraintsClose = ")"
)

// typeVar is one type variable that the module declares through a call of
// TypeVar, ParamSpec or TypeVarTuple: the name of its type parameter with
// its mark, its bounds, its default and its variance, at the position of
// the call.
type typeVar struct {
	name     string
	bounds   []*node.TypeRef
	def      *node.TypeRef
	variance symbol.Variance
	pos      position.Pos
}

// param returns a new type parameter of the variable. Its references are
// copies, because a node of the graph has one place.
func (v *typeVar) param() *node.TypeParam {
	p := &node.TypeParam{Name: v.name, Pos: v.pos, Variance: v.variance, Default: node.CloneRef(v.def)}
	for _, b := range v.bounds {
		p.Bounds = append(p.Bounds, node.CloneRef(b))
	}
	return p
}

// typeParams lowers a type parameter list of PEP 695: the name of each
// parameter, *Ts and **P with their marks, its bound or the union of its
// constraints, and its default. The constraints of T: (int, str) are one
// bound of the form Union, as [lowering.bound] spells it, because a type
// parameter of another language has no constraints.
func (l *lowering) typeParams(list treesitter.Node) []*node.TypeParam {
	var out []*node.TypeParam
	for child := range list.Content() {
		switch child.Kind() {
		case l.v.typeNode:
			inner := l.inner(child)
			p := &node.TypeParam{Name: inner.Compact(), Pos: l.pos(inner)}
			if inner.Kind() == l.v.constrainedType {
				var parts []treesitter.Node
				for part := range inner.Content() {
					parts = append(parts, l.inner(part))
				}
				p.Name, p.Pos = parts[0].Compact(), l.pos(parts[0])
				if len(parts) > 1 {
					p.Bounds = []*node.TypeRef{l.bound(parts[1])}
				}
			}
			out = append(out, p)
		case l.v.defaultType:
			if len(out) > 0 {
				out[len(out)-1].Default = l.typeRef(firstNamed(child))
			}
		}
	}
	return out
}

// bound lowers the bound of a type parameter: the union of the members of a
// tuple of constraints, and the type of any other bound. The union of the
// constraints has the spelling of the constraints of a call of TypeVar, so
// both forms of one type variable have one spelling.
func (l *lowering) bound(n treesitter.Node) *node.TypeRef {
	if n.Kind() != l.v.tuple {
		return l.typeRef(n)
	}
	var members []*node.TypeRef
	var spelled []string
	for m := range n.Content() {
		members = append(members, l.typeRef(m))
		spelled = append(spelled, m.Compact())
	}
	spelling := constraintsOpen + strings.Join(spelled, constraintsSep) + constraintsClose
	return plugin.Composite(spelling, l.pos(n), symbol.FormUnion, members...)
}

// typeVarCall reads a call of TypeVar, ParamSpec or TypeVarTuple of typing,
// the callee, and returns the type variable of the name. The keyword bound
// and the constraints after the name are the bounds, as a union of the
// constraints, and the keyword default is the default. The keywords
// covariant and contravariant state the variance.
func (l *lowering) typeVarCall(name string, call treesitter.Node, callee string) *typeVar {
	tv := &typeVar{name: typeVarMarks[callee] + name, pos: l.pos(call)}
	var constraints []*node.TypeRef
	var spelled []string
	positional := 0
	for arg := range call.Child(l.v.fieldArguments).Content() {
		if arg.Kind() != l.v.keywordArgument {
			if positional++; positional > 1 {
				constraints = append(constraints, l.typeRef(arg))
				spelled = append(spelled, arg.Compact())
			}
			continue
		}
		value := arg.Child(l.v.fieldValue)
		switch arg.Child(l.v.fieldName).Text() {
		case boundKeyword:
			tv.bounds = []*node.TypeRef{l.typeRef(value)}
		case defaultKeyword:
			tv.def = l.typeRef(value)
		case covariantKeyword:
			if value.Kind() == l.v.trueNode {
				tv.variance = symbol.VarianceOut
			}
		case contravariantKeyword:
			if value.Kind() == l.v.trueNode {
				tv.variance = symbol.VarianceIn
			}
		}
	}
	if len(constraints) > 0 {
		spelling := constraintsOpen + strings.Join(spelled, constraintsSep) + constraintsClose
		tv.bounds = []*node.TypeRef{plugin.Composite(spelling, l.pos(call), symbol.FormUnion, constraints...)}
	}
	return tv
}

// usedVars returns a type parameter for each type variable of the module
// that the references name, in the order of their first use, each once. A
// variable whose type parameter taken lists is a parameter of an enclosing
// declaration or of the syntax of PEP 695, and usedVars leaves it out. A
// reference names a variable by its first name, as P.args names P, with the
// stars of an unpacking left out, as *Ts names Ts.
func (l *lowering) usedVars(refs []*node.TypeRef, taken map[string]bool) []*node.TypeParam {
	var out []*node.TypeParam
	seen := map[string]bool{}
	var walk func(r *node.TypeRef)
	walk = func(r *node.TypeRef) {
		if r == nil {
			return
		}
		head, _, _ := strings.Cut(strings.TrimLeft(r.Spelling, tupleMark), python.ModuleSep)
		if tv, declared := l.typeVars[head]; declared && r.Form == symbol.FormNamed && !seen[head] && !taken[tv.name] {
			seen[head] = true
			out = append(out, tv.param())
		}
		for _, e := range r.Elems {
			walk(e)
		}
		for _, a := range r.Args {
			walk(a)
		}
	}
	for _, r := range refs {
		walk(r)
	}
	return out
}
