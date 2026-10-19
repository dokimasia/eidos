// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"fmt"
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// packageSep separates the segments of a package path, which is
// slash-separated.
const packageSep = "/"

// Resolve returns the declaration that name refers to from a subject. The
// kernel's walk, [rules.Resolve], selects the declarations that each
// resolution kind searches, and the resolver of GraphQL decides the rest:
//
//   - A name in scope resolves in the package of the subject, then in the
//     package of its documents, and then, for a subject in the package of
//     the documents of a schema, in the package of the schema. A name of a
//     nested type of a selection is the chain of the types that declare it
//     and its own name, joined by dots.
//   - A built-in scalar resolves to a stand-in with its name and no
//     package, because no declaration declares it.
//   - Every member of GraphQL is public, so a subject can use every member.
//   - The value of a nullable type is a value of the type.
//
// Resolve refuses a name that is not a GraphQL name or a chain of them, and
// a name that resolves to no declaration. A refusal is a
// *[rules.ResolveError].
//
// # Allocation contract
//
// A resolution in scope that does not resolve in the package of the subject
// allocates the path of the package of its documents. A member resolution
// also allocates the list of the types that it searches and what the member
// walk allocates. A stand-in and a refusal allocate themselves. Every read
// allocates what the read set of the view allocates to record a new edge.
func (r Rules) Resolve(
	scope rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	return rules.Resolve(resolver{}, r, v, scope, name, kind)
}

// resolver is the [rules.Resolver] of GraphQL: the packages of a schema and
// its documents, the built-in scalars as stand-ins, the public members, and
// a nullable type as its type.
type resolver struct{}

// InScope resolves a name or a chain of names to the first declaration of
// one of the kinds, in the order of the packages of [Rules.Resolve]. It
// refuses a name that is not a GraphQL name or a chain of them, and a name
// that no declaration of the kinds has.
func (resolver) InScope(scope rules.Scope, name string, kinds []symbol.Kind, v rules.View) (symbol.Symbol, error) {
	for segment := range strings.SplitSeq(name, symbol.OwnerSep) {
		if !naming.IsIdentifier(segment) {
			return nil, &rules.ResolveError{
				Lang: graphql.Lang, Msg: name + " is not a GraphQL name, so it cannot refer to a declaration",
			}
		}
	}
	owner, last, nested := strings.CutLast(name, symbol.OwnerSep)
	if !nested {
		owner, last = "", name
	}
	pkg := scope.Subject.Package
	if sym, held := lookup(v, pkg, owner, last, kinds); held {
		return sym, nil
	}
	documents := path.Join(pkg, graphql.DocumentsSegment)
	if sym, held := lookup(v, documents, owner, last, kinds); held {
		return sym, nil
	}
	if path.Base(pkg) == graphql.DocumentsSegment {
		schema := strings.TrimSuffix(strings.TrimSuffix(pkg, graphql.DocumentsSegment), packageSep)
		if sym, held := lookup(v, schema, owner, last, kinds); held {
			return sym, nil
		}
	}
	names := make([]string, 0, len(kinds))
	for _, k := range kinds {
		names = append(names, k.String())
	}
	return nil, &rules.ResolveError{Lang: graphql.Lang, Msg: fmt.Sprintf(
		"no %s named %s is in the packages of the schema of %s", strings.Join(names, " or "), name, scope.Subject,
	)}
}

// Usable reports true, because every field, argument and value of GraphQL
// is public.
func (resolver) Usable(rules.Member, symbol.Identity, rules.Scope) bool { return true }

// Placeholder returns the stand-in of a built-in scalar of GraphQL: a
// defined alias without a target, with the name of the scalar and no
// package, as the frontend loads a scalar. It reports false for every other
// name.
func (resolver) Placeholder(_ rules.Scope, name string, _ rules.View) (symbol.Symbol, bool) {
	if !graphql.IsBuiltin(name) {
		return nil, false
	}
	return &node.Alias{
		ID:   symbol.Identity{Lang: graphql.Lang, Name: name, Kind: symbol.KindAlias},
		Name: name, Visibility: symbol.VisibilityPublic, Defined: true,
	}, true
}

// Value returns the type of a value of the type ref: the type itself, and
// the type of a nullable type, whose value is a value of its type or null.
func (resolver) Value(ref *node.TypeRef, _ rules.View) *node.TypeRef {
	for ref.Form == symbol.FormOptional && len(ref.Elems) == 1 {
		ref = ref.Elems[0]
	}
	return ref
}

// lookup returns the declaration of the package pkg with the owner chain,
// the name and the first of the kinds that the view contains. It reports
// false where the view contains none.
func lookup(v rules.View, pkg, owner, name string, kinds []symbol.Kind) (symbol.Symbol, bool) {
	for _, k := range kinds {
		id := symbol.Identity{Lang: graphql.Lang, Package: pkg, Owner: owner, Name: name, Kind: k}
		if sym, held := v.Lookup(id); held {
			return sym, true
		}
	}
	return nil, false
}
