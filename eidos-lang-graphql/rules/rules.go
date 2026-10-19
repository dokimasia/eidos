// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Rules implements the projection rules of GraphQL. It also implements the
// optional capabilities for enums and for the shapes of custom scalars.
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
// parameter, the member policy and the shape of a built-in scalar allocate
// nothing.
type Rules struct{}

// The capabilities of the rules beside the source rules.
var (
	_ rules.EnumRules  = Rules{}
	_ rules.ShapeRules = Rules{}
)

// New returns the GraphQL rules as the [rules.SourceRules] that a
// composition registers. The value is [Rules], so a caller that needs the
// enum or the shape capability asserts [rules.EnumRules] or
// [rules.ShapeRules] on it. It allocates nothing.
func New() rules.SourceRules { return Rules{} }

// Lang returns [graphql.Lang], the language that a composition keys the
// rules under and the language of every subject that they project. It
// allocates nothing.
func (Rules) Lang() symbol.Lang { return graphql.Lang }

// Members returns the member policy of GraphQL, under which nothing
// contributes to the members of a type. An object type and an interface
// restate every field of the interfaces that they implement, so the members
// of a type are the fields and the methods that it declares. The frontend
// merges an extension into its type, so its members are declared members
// too. The policy states override as its shadowing rule, so the policy is
// total and not zero. Members allocates nothing.
func (Rules) Members() rules.MemberPolicy {
	return rules.MemberPolicy{Shadowing: rules.ShadowOverride}
}

// ParamRole reports [rules.ParamInput] for every parameter. An argument of a
// field and a variable of an operation are inputs, and GraphQL has no
// parameter of another role. The view is unread, and ParamRole allocates
// nothing.
func (Rules) ParamRole(*node.Param, rules.View) rules.ParamRole { return rules.ParamInput }

// ReturnRoles reports one role per return and the error model. A field and
// an operation have one result. The result of a subscription, a return of
// [symbol.FormStream], is [rules.ReturnStream], and every other return is
// [rules.ReturnValue]. The model is [rules.ErrorsNone], because a schema
// states no failure in a signature: a response reports the errors of its
// fields in a list beside its data.
//
// # Allocation contract
//
// ReturnRoles allocates the list of roles, one entry per return, which the
// caller keeps. A callable without returns allocates nothing.
func (Rules) ReturnRoles(rs []*node.Return, _ rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	roles := make([]rules.ReturnRole, len(rs))
	for i, r := range rs {
		if r != nil && r.Type != nil && r.Type.Form == symbol.FormStream {
			roles[i] = rules.ReturnStream
		}
	}
	return roles, rules.ErrorsNone
}
