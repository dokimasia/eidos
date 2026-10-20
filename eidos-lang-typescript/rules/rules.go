// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// contributions lists the clauses that a TypeScript type inherits
// members from. The walk visits the extends clause first and the
// implements clause second. Every policy shares the list, and the
// kernel's walk only reads it, so a policy allocates nothing.
var contributions = []rules.Contribution{rules.ContributesExtends, rules.ContributesImplements}

// Rules implements the projection rules of TypeScript. It also
// implements the optional capabilities for enums, generics, properties,
// constructors, equality and the shapes of the declarations of a library.
//
// # Concurrency
//
// Rules has no state, so goroutines can share one value.
//
// # Allocation contract
//
// The docblock of each method gives its allocations. A parameter's role,
// the member policy and a builtin's shape allocate nothing.
type Rules struct{}

// New returns the TypeScript rules. It allocates nothing.
func New() rules.SourceRules { return Rules{} }

// Lang returns the language that the rules apply to. It allocates
// nothing.
func (Rules) Lang() symbol.Lang { return typescript.Lang }

// Members returns the member policy of TypeScript. The member walk
// visits a type's extends clause and then its implements clause, to the
// kernel's default depth. A name that arrives more than once is kept as
// an overload, the way TypeScript merges the members of an interface. A
// member of the type's own declaration hides the inherited members of
// the same name. Every policy shares one contribution list, so Members
// allocates nothing.
func (Rules) Members() rules.MemberPolicy {
	return rules.MemberPolicy{Contributes: contributions, Shadowing: rules.ShadowMerge}
}

// ParamRole classifies a parameter of the type AbortSignal as the
// context, and every other parameter as an input. The AbortSignal is the
// global type, which no import binds: a name without a target, or the
// declaration of a library. An alias of it and an optional of it are the
// context too. Every TypeScript alias is transparent, so [rules.Unaliased]
// follows each alias to its target. ParamRole allocates nothing.
func (Rules) ParamRole(p *node.Param, v rules.View) rules.ParamRole {
	if p == nil || p.Type == nil {
		return rules.ParamInput
	}
	t := rules.Unaliased(p.Type, v)
	if t.Form == symbol.FormOptional && len(t.Elems) == 1 {
		t = rules.Unaliased(t.Elems[0], v)
	}
	if t.Form == symbol.FormNamed && typescript.Global(t.Target, t.Package) &&
		t.Spelling == typescript.TypeAbortSignal {

		return rules.ParamContext
	}
	return rules.ParamInput
}

// ReturnRoles classifies a return of an asynchronous stream as a stream,
// and every other return as a value. The asynchronous streams are
// AsyncIterable, AsyncIterableIterator and AsyncGenerator, and an alias
// of one of them. A TypeScript callable raises its errors without
// declaring them, so the error model of every callable is
// [rules.ErrorsRaised]. ReturnRoles allocates the list of roles, one
// allocation.
func (r Rules) ReturnRoles(rs []*node.Return, v rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	roles := make([]rules.ReturnRole, len(rs))
	for i, ret := range rs {
		if ret != nil && ret.Type != nil && r.Builtin(rules.Unaliased(ret.Type, v), v).Form == symbol.FormStream {
			roles[i] = rules.ReturnStream
		}
	}
	return roles, rules.ErrorsRaised
}
