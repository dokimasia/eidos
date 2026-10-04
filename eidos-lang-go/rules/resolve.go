// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"fmt"
	"go/token"
	"slices"
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// qualifierSep separates a package qualifier from the name it
// qualifies.
const qualifierSep = "."

// refusalPrefix opens every refusal the rules return: the language's
// identity, as every satellite's refusals open.
const refusalPrefix = string(golang.Lang) + ": "

// typeKinds are the kinds a type spelling names.
var typeKinds = []symbol.Kind{
	symbol.KindStruct, symbol.KindInterface, symbol.KindAlias, symbol.KindEnum, symbol.KindSum,
}

// Resolve returns the declaration a directive's spelling names from
// a subject, the way Go scopes it. A callable, a package variable and
// a type in scope resolve through the probe of the subject's file,
// [golang.Scope.Candidates]: the first candidate the view declares at
// one of the kinds is the declaration. A value field resolves through
// the member walk over the subject's type, a host parameter on the
// subject's own signature, and a member on a handle through the type
// the subject belongs to. A predeclared type resolves to a stand-in
// naming itself with no package, and a type whose qualifier binds a
// package the view does not contain to a stand-in naming the import
// path. A spelling that is not a Go name, bare or qualified, refuses:
// []Row names no declaration, and probing Row would name another type.
//
// # Allocation contract
//
// A resolution through the probe allocates the scope of the subject's
// file and the list of candidates: four allocations for a file without
// a dot import. A member resolution allocates the binding the member
// walk runs on and what the walk allocates. A stand-in and a refusal
// allocate themselves.
func (r Rules) Resolve(
	scope rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, refuse("nothing to resolve")
	}
	switch kind {
	case directive.ResolveCallableInScope:
		return inScope(scope, name, v, symbol.KindFunction)
	case directive.ResolvePackageVar:
		return inScope(scope, name, v, symbol.KindVariable, symbol.KindConstant)
	case directive.ResolveValueField:
		return r.member(scope, name, v, symbol.KindField)
	case directive.ResolveHostParam:
		return hostParam(scope, name, v)
	case directive.ResolveMemberOnHandle:
		return r.member(scope, name, v, symbol.KindField, symbol.KindMethod)
	case directive.ResolveTypeInScope:
		return typeInScope(scope, name, v)
	default:
		return nil, refuse("%s is not a resolution Go performs", kind)
	}
}

// inScope resolves a Go name to the first top-level declaration of
// one of the kinds that the probe of the subject's file finds in the
// view.
func inScope(scope rules.Scope, name string, v rules.View, kinds ...symbol.Kind) (symbol.Symbol, error) {
	if !goName(name) {
		return nil, refuse("%s is not a Go name, so it names no declaration", name)
	}
	return firstDeclared(scope, scopeOf(scope.File), name, v, kinds...)
}

// typeInScope resolves a type spelling: a predeclared type to a
// stand-in naming itself, a qualified name whose import binds a
// package outside the view to a stand-in naming the import path, and
// any other Go name to the first type the probe of the subject's file
// finds in the view.
func typeInScope(scope rules.Scope, name string, v rules.View) (symbol.Symbol, error) {
	if golang.Predeclared(name) {
		return standIn(symbol.Identity{Lang: golang.Lang, Name: name, Kind: symbol.KindAlias}), nil
	}
	if !goName(name) {
		return nil, refuse("%s is not a Go name, so it names no type", name)
	}
	s := scopeOf(scope.File)
	if qualifier, bare, qualified := strings.Cut(name, qualifierSep); qualified {
		if path, bound := s.Import(qualifier); bound && !inView(path, v) {
			// A package outside the workspace, the standard library
			// or a dependency: the type is named by its path, which a
			// backend qualifies and imports.
			return standIn(symbol.Identity{
				Lang: golang.Lang, Package: path, Name: bare, Kind: symbol.KindAlias,
			}), nil
		}
	}
	return firstDeclared(scope, s, name, v, typeKinds...)
}

// firstDeclared returns the first candidate of a name, in the
// scope's probe order, that the view declares at the top level of
// its package at one of the kinds. A candidate whose package the view
// does not contain is skipped.
func firstDeclared(
	scope rules.Scope, s golang.Scope, name string, v rules.View, kinds ...symbol.Kind,
) (symbol.Symbol, error) {
	for _, c := range s.Candidates(scope.Subject.Package, name) {
		pkg, held := v.PackageOf(packageIdentity(c.Package))
		if !held {
			continue
		}
		if sym := declared(pkg, c.Name, kinds...); sym != nil {
			return sym, nil
		}
	}
	return nil, refuse("no %s named %s is in scope of %s", kindList(kinds), name, scope.Subject)
}

// member resolves a name among the effective members of the type
// the subject belongs to: the subject itself for a type, its host
// for a field or a method.
func (r Rules) member(
	scope rules.Scope, name string, v rules.View, kinds ...symbol.Kind,
) (symbol.Symbol, error) {
	host, err := hostType(scope.Subject, v)
	if err != nil {
		return nil, err
	}
	set, is := rules.NewBound(r, v, nil).MembersOf(host)
	if !is {
		return nil, refuse("%s has no members to resolve %s in", scope.Subject, name)
	}
	for _, m := range set.Members {
		decl, names := m.Symbol.(node.Declaration)
		if !names {
			continue
		}
		id := decl.Identity()
		if id.Name == name && slices.Contains(kinds, id.Kind) {
			return m.Symbol, nil
		}
	}
	return nil, refuse("%s has no %s member named %s", host.Identity(), kindList(kinds), name)
}

// hostParam resolves a parameter on the subject's own signature.
func hostParam(scope rules.Scope, name string, v rules.View) (symbol.Symbol, error) {
	sym, held := v.Lookup(scope.Subject)
	if !held {
		return nil, refuse("%s is outside the view", scope.Subject)
	}
	var params []*node.Param
	switch c := sym.(type) {
	case *node.Function:
		params = c.Params
	case *node.Method:
		params = c.Params
	default:
		return nil, refuse("%s is no callable, so it has no parameter named %s", scope.Subject, name)
	}
	for _, p := range params {
		if p != nil && p.Name == name {
			return p, nil
		}
	}
	return nil, refuse("%s declares no parameter named %s", scope.Subject, name)
}

// hostType returns the type a subject belongs to: the subject
// itself where it is a type, and its host where it is a member.
func hostType(subject symbol.Identity, v rules.View) (node.Declaration, error) {
	sym, held := v.Lookup(subject)
	if !held {
		return nil, refuse("%s is outside the view", subject)
	}
	var host symbol.Identity
	switch m := sym.(type) {
	case *node.Struct, *node.Interface, *node.Enum, *node.Sum:
		decl, names := sym.(node.Declaration)
		if !names {
			return nil, refuse("%s has no identity", subject)
		}
		return decl, nil
	case *node.Field:
		host = m.Host
	case *node.Method:
		host = m.Host
	default:
		return nil, refuse("%s belongs to no type", subject)
	}
	owner, held := v.Lookup(host)
	if !held {
		return nil, refuse("%s, the type %s belongs to, is outside the view", host, subject)
	}
	decl, names := owner.(node.Declaration)
	if !names {
		return nil, refuse("%s has no identity", host)
	}
	return decl, nil
}

// scopeOf returns the scope a file's imports bind, and the zero
// scope, which binds nothing, for a subject without a file.
func scopeOf(f *node.File) golang.Scope {
	if f == nil {
		return golang.Scope{}
	}
	return golang.NewScope(f.Imports)
}

// goName reports whether a spelling is a Go name, bare or qualified:
// an identifier, or two joined by the qualifier's dot. A decorated, an
// instantiated and a composite spelling are not.
func goName(spelling string) bool {
	qualifier, name, qualified := strings.Cut(spelling, qualifierSep)
	if !qualified {
		return token.IsIdentifier(spelling)
	}
	return token.IsIdentifier(qualifier) && token.IsIdentifier(name)
}

// inView reports whether the view contains the package at an import
// path.
func inView(path string, v rules.View) bool {
	_, held := v.PackageOf(packageIdentity(path))
	return held
}

// packageIdentity returns the identity of the Go package at an import
// path.
func packageIdentity(path string) symbol.Identity {
	return symbol.Identity{Lang: golang.Lang, Package: path, Kind: symbol.KindPackage}
}

// declared returns a package's top-level declaration of one name
// and one of the kinds, or nil. It reads each file's top-level
// declarations and nothing nested inside them.
func declared(pkg *node.Package, name string, kinds ...symbol.Kind) symbol.Symbol {
	for _, f := range pkg.Files {
		if f == nil {
			continue
		}
		for _, sym := range f.Decls {
			decl, names := sym.(node.Declaration)
			if !names {
				continue
			}
			id := decl.Identity()
			if id.Owner == "" && id.Name == name && slices.Contains(kinds, id.Kind) {
				return decl
			}
		}
	}
	return nil
}

// standIn returns the stand-in declaration for a type the view does
// not contain: a predeclared type, or a type in a package outside
// the workspace, named by the identity alone.
func standIn(id symbol.Identity) *node.Alias {
	return &node.Alias{ID: id, Name: id.Name}
}

// kindList spells a kind set for a refusal.
func kindList(kinds []symbol.Kind) string {
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, k.String())
	}
	return strings.Join(parts, " or ")
}

// refuse builds a refusal under [refusalPrefix].
func refuse(format string, args ...any) error {
	return fmt.Errorf(refusalPrefix+format, args...)
}
