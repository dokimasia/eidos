// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/lang/naming"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// aliasDepth is the longest chain of type aliases that the rules follow.
// TypeScript refuses an alias that refers to itself, so the bound only
// ends a walk over a damaged graph.
const aliasDepth = 8

// contributions lists the clauses that a TypeScript type inherits
// members from. The walk visits the extends clause first and the
// implements clause second. Every policy shares the list, and the
// kernel's walk only reads it, so a policy allocates nothing.
var contributions = []rules.Contribution{rules.ContributesExtends, rules.ContributesImplements}

// Rules implements the projection rules of TypeScript. It also
// implements the optional capabilities for enums, generics, properties,
// constructors and equality.
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
// global type, which no import binds. An alias of it and an optional of
// it are the context too. ParamRole allocates nothing.
func (Rules) ParamRole(p *node.Param, v rules.View) rules.ParamRole {
	if p == nil || p.Type == nil {
		return rules.ParamInput
	}
	t := unaliased(p.Type, v)
	if t.Form == symbol.FormOptional && len(t.Elems) == 1 {
		t = unaliased(t.Elems[0], v)
	}
	if t.Form == symbol.FormNamed && t.Target.IsZero() && t.Package == "" && t.Spelling == spellAbortSignal {
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
		if ret != nil && ret.Type != nil && r.Builtin(unaliased(ret.Type, v), v).Form == symbol.FormStream {
			roles[i] = rules.ReturnStream
		}
	}
	return roles, rules.ErrorsRaised
}

// TypeName returns the base as the author wrote it, followed by the
// generator's word in Pascal case. The word client on the base Store
// gives StoreClient. TypeName allocates the word's case conversion and
// the joined name, two allocations, or one when the word is in Pascal
// case already.
func (Rules) TypeName(word, base string) string { return base + naming.Pascal(word) }

// unaliased follows a chain of aliases in the view, and returns the
// reference at its end. It returns a reference to any other declaration
// as it is, because every TypeScript alias is transparent. unaliased
// reads each alias through the view and allocates nothing.
func unaliased(ref *node.TypeRef, v rules.View) *node.TypeRef {
	for range aliasDepth {
		if ref.Target.Kind != symbol.KindAlias {
			return ref
		}
		sym, _ := v.Lookup(ref.Target)
		alias, isAlias := sym.(*node.Alias)
		if !isAlias || alias.Target == nil {
			return ref
		}
		ref = alias.Target
	}
	return ref
}
