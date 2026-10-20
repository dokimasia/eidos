// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// contributions lists the clauses that a Python class inherits members
// from: its bases, which the frontend loads into the extends clause. Every
// policy shares the list, and the walk of the kernel only reads it, so a
// policy allocates nothing.
var contributions = []rules.Contribution{rules.ContributesExtends}

// Rules implements the projection rules of Python. It also implements the
// optional capabilities for enums, well-known types, shapes, error types,
// properties, constructors, equality and generics.
//
// The zero value is ready and is the only value. Every method reads through
// the [rules.View] that it receives and caches nothing.
//
// # Concurrency
//
// Rules has no state, so goroutines can share one value.
//
// # Allocation contract
//
// The docblock of each method gives its allocations. The role of a
// parameter, the member policy, the shape of a builtin and a well-known
// identity allocate nothing.
type Rules struct{}

// The capabilities of the rules beside the source rules.
var (
	_ rules.EnumRules      = Rules{}
	_ rules.WellKnownRules = Rules{}
	_ rules.ShapeRules     = Rules{}
	_ rules.ErrorTypeRules = Rules{}
	_ rules.PropertyRules  = Rules{}
	_ rules.ConstructRules = Rules{}
	_ rules.EqualityRules  = Rules{}
	_ rules.GenericsRules  = Rules{}
)

// New returns the Python rules as the [rules.SourceRules] that a composition
// registers. The value is [Rules], so a caller that needs a capability
// asserts its interface on it. It allocates nothing.
func New() rules.SourceRules { return Rules{} }

// Lang returns [python.Lang], the language that a composition keys the rules
// under. It allocates nothing.
func (Rules) Lang() symbol.Lang { return python.Lang }

// Members returns the member policy of Python. The walk visits the bases of
// a class in the order of the class statement, to the default depth of the
// kernel. A name that arrives more than once takes the members of the class
// that comes first in the C3 linearization of the class, as the method
// resolution order of Python finds an attribute. A member of the class
// itself hides the inherited members of the same name. Members allocates
// nothing.
func (Rules) Members() rules.MemberPolicy {
	return rules.MemberPolicy{Contributes: contributions, Shadowing: rules.ShadowLinearise}
}

// ParamRole reports [rules.ParamInput] for every parameter. Python passes no
// context through a parameter of a role of its own, so a generated call
// samples every parameter. The view is unread, and ParamRole allocates
// nothing.
func (Rules) ParamRole(*node.Param, rules.View) rules.ParamRole { return rules.ParamInput }

// ReturnRoles reports one role per return and the error model. A result
// whose type folds to a stream, such as Iterator[T], AsyncIterator[T] and
// Generator[T, None, None], is [rules.ReturnStream], and every other result
// is [rules.ReturnValue]. The model is [rules.ErrorsRaised], because Python
// raises an exception without declaring it in a signature.
//
// # Allocation contract
//
// ReturnRoles allocates the list of roles, one entry per return, which the
// caller keeps, and what the view allocates to record the read of the
// declaration of each result type.
func (r Rules) ReturnRoles(rs []*node.Return, v rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	roles := make([]rules.ReturnRole, len(rs))
	for i, ret := range rs {
		if ret != nil && r.formOf(ret.Type, v) == symbol.FormStream {
			roles[i] = rules.ReturnStream
		}
	}
	return roles, rules.ErrorsRaised
}

// formOf returns the form that the reference ref folds to, without the
// children of a structural form: the form of the reference itself for a
// structural form, the form that [Rules.Builtin] gives a name without a
// target, and the form that [Rules.ShapeOf] gives a declared class of the
// table. An alias counts as its target. formOf returns [symbol.FormOpaque]
// for a nil reference, for a target outside the view and for every other
// declaration. It allocates what the view allocates to record its reads.
func (r Rules) formOf(ref *node.TypeRef, v rules.View) symbol.TypeForm {
	ref = rules.Unaliased(ref, v)
	switch {
	case ref == nil:
		return symbol.FormOpaque
	case ref.Form != symbol.FormNamed:
		return ref.Form
	case ref.Target.IsZero():
		return r.Builtin(ref, v).Form
	}
	decl, held := v.Lookup(ref.Target)
	if !held {
		return symbol.FormOpaque
	}
	if s, shaped := r.ShapeOf(ref, decl, v); shaped {
		return s.Form
	}
	return symbol.FormOpaque
}
