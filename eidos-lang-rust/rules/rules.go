// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// contributions lists the clauses that a Rust type inherits methods from: the
// supertraits of a trait, which the frontend loads into the extends clause,
// and the traits that the trait impls of a struct name, which it loads into
// the implements clause. Every policy shares the list, and the walk of the
// kernel only reads it, so a policy allocates nothing.
var contributions = []rules.Contribution{rules.ContributesExtends, rules.ContributesImplements}

// Rules implements the projection rules of Rust. It also implements the
// optional capabilities for enums, error types, results, ownership, equality,
// generics, constructors and texts.
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
// parameter, the member policy and the shape of a type without an iterator
// allocate nothing.
type Rules struct{}

// The capabilities of the rules beside the source rules.
var (
	_ rules.EnumRules      = Rules{}
	_ rules.ErrorTypeRules = Rules{}
	_ rules.ResultRules    = Rules{}
	_ rules.OwnershipRules = Rules{}
	_ rules.EqualityRules  = Rules{}
	_ rules.GenericsRules  = Rules{}
	_ rules.ConstructRules = Rules{}
	_ rules.TextRules      = Rules{}
)

// New returns the Rust rules as the [rules.SourceRules] that a composition
// registers. The value is [Rules], so a caller that needs a capability
// asserts its interface on it. It allocates nothing.
func New() rules.SourceRules { return Rules{} }

// Lang returns [rust.Lang], the language that a composition keys the rules
// under. It allocates nothing.
func (Rules) Lang() symbol.Lang { return rust.Lang }

// Members returns the member policy of Rust. A trait has the methods of its
// supertraits, and a struct has the methods of the traits that its trait
// impls name, to the default depth of the kernel. A member that the type
// declares hides the inherited members of its name, as an inherent method
// hides a trait method in a method call of Rust. Two traits that declare one
// name at one depth cancel each other, because Rust refuses such a call with
// error E0034 unless the caller names the trait. Members allocates nothing.
func (Rules) Members() rules.MemberPolicy {
	return rules.MemberPolicy{Contributes: contributions, Shadowing: rules.ShadowPromote}
}

// ParamRole reports [rules.ParamInput] for every parameter. A caller cancels
// a future of Rust when it drops the future, so no parameter has the role of
// a context, and a generated call samples every parameter. The view is
// unread, and ParamRole allocates nothing.
func (Rules) ParamRole(*node.Param, rules.View) rules.ParamRole { return rules.ParamInput }

// ReturnRoles reports one role per return and the error model. A return of a
// result type, as [Rules.Result] reads one, gives the callable the error
// model [rules.ErrorsResultType], and its role is the role of its value. A
// callable without one has the model [rules.ErrorsNone], because a panic is
// no failure that a signature states. A return whose type, or the value of
// whose result, yields a stream is [rules.ReturnStream]: impl and dyn of
// Iterator, IntoIterator, DoubleEndedIterator, ExactSizeIterator and Stream,
// also behind a Box or a borrow. Every other return is [rules.ReturnValue].
//
// # Allocation contract
//
// ReturnRoles allocates the list of roles, one entry per return, which the
// caller keeps. A return of a result type also allocates what [Rules.Result]
// allocates, and each read allocates what the view allocates to record it.
func (r Rules) ReturnRoles(rs []*node.Return, v rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	roles := make([]rules.ReturnRole, len(rs))
	model := rules.ErrorsNone
	for i, ret := range rs {
		if ret == nil {
			continue
		}
		ref := ret.Type
		if value, _, ok := r.Result(ref, v); ok {
			model, ref = rules.ErrorsResultType, value
		}
		if streams(ref, v) {
			roles[i] = rules.ReturnStream
		}
	}
	return roles, model
}

// streams reports whether a value of the type ref yields a stream: a
// reference that [Rules.Builtin] folds to a stream, an intersection of impl
// or dyn with such a bound among its bounds, and a borrow or a Box of such a
// type, through the aliases of the crate. It reports false past
// [deriveDepth] steps. It allocates what the view allocates to record the
// read of each alias.
func streams(ref *node.TypeRef, v rules.View) bool {
	for range deriveDepth {
		ref = rules.Unaliased(ref, v)
		if ref == nil {
			return false
		}
		switch ref.Form {
		case symbol.FormIntersection:
			for _, bound := range ref.Elems {
				if bound == nil || !bound.Target.IsZero() {
					continue
				}
				if t, _, known := libTypeOf(bound); known && t.form == symbol.FormStream {
					return true
				}
			}
			return false
		case symbol.FormBorrow:
			ref = firstElem(ref)
			continue
		case symbol.FormNamed:
		default:
			return false
		}
		t, _, known := libTypeOf(ref)
		switch {
		case !known || !ref.Target.IsZero():
			return false
		case t.form == symbol.FormStream:
			return true
		case t.form != symbol.FormBorrow:
			return false
		}
		ref = typeArg(ref, 0)
	}
	return false
}
