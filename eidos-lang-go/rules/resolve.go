// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"fmt"
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

// typeKinds are the kinds a type spelling names.
var typeKinds = []symbol.Kind{
	symbol.KindStruct, symbol.KindInterface, symbol.KindAlias, symbol.KindEnum, symbol.KindSum,
}

// Resolve says what a directive's spelling names from a subject,
// the way Go scopes it: a bare name in the subject's package, a
// qualified name through the file's imports, a value field through
// the member walk over the subject's type, a host parameter on the
// subject's own signature, a member on a handle through the type
// the subject belongs to, and a type in scope the way a bare or
// qualified type spelling resolves, a builtin standing in for
// itself with no package and a type in a package the graph does
// not hold standing in with its import path.
func (r Rules) Resolve(
	scope rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("go: nothing to resolve")
	}
	switch kind {
	case directive.ResolveCallableInScope:
		return r.inScope(scope, name, v, symbol.KindFunction)
	case directive.ResolvePackageVar:
		return r.inScope(scope, name, v, symbol.KindVariable, symbol.KindConstant)
	case directive.ResolveValueField:
		return r.member(scope, name, v, symbol.KindField)
	case directive.ResolveHostParam:
		return hostParam(scope, name, v)
	case directive.ResolveMemberOnHandle:
		return r.member(scope, name, v, symbol.KindField, symbol.KindMethod)
	case directive.ResolveTypeInScope:
		return r.typeInScope(scope, name, v)
	default:
		return nil, fmt.Errorf("go: %s is not a resolution Go performs", kind)
	}
}

// inScope resolves a bare or qualified name to a top-level
// declaration of one of the kinds, in the subject's package or the
// package an import qualifies.
func (Rules) inScope(
	scope rules.Scope, name string, v rules.View, kinds ...symbol.Kind,
) (symbol.Symbol, error) {
	pkg, bare, err := packageFor(scope, name, v)
	if err != nil {
		return nil, err
	}
	if sym := declared(pkg, bare, kinds...); sym != nil {
		return sym, nil
	}
	return nil, fmt.Errorf("go: %s declares no %s named %s", pkg.ID.Package, kindList(kinds), bare)
}

// typeInScope resolves a type spelling: a builtin stands in for
// itself, a bare name resolves in the subject's package, and a
// qualified name resolves through the file's imports, standing in
// with the import path where the graph does not hold the package.
func (Rules) typeInScope(scope rules.Scope, name string, v rules.View) (symbol.Symbol, error) {
	if builtinType(name) {
		return standIn(symbol.Identity{Lang: golang.Lang, Name: name, Kind: symbol.KindAlias}), nil
	}
	qualifier, bare, qualified := strings.Cut(name, qualifierSep)
	if !qualified {
		pkg, held := v.PackageOf(scope.Subject)
		if !held {
			return nil, fmt.Errorf(
				"go: the view does not hold the package declaring %s",
				scope.Subject,
			)
		}
		if sym := declared(pkg, name, typeKinds...); sym != nil {
			return sym, nil
		}
		return nil, fmt.Errorf("go: %s declares no type named %s", pkg.ID.Package, name)
	}
	path, imported := importPath(scope.File, qualifier)
	if !imported {
		return nil, fmt.Errorf("go: no import of the subject's file binds %s", qualifier)
	}
	id := symbol.Identity{Lang: golang.Lang, Package: path, Name: bare, Kind: symbol.KindAlias}
	pkg, held := v.PackageOf(
		symbol.Identity{Lang: golang.Lang, Package: path, Kind: symbol.KindPackage},
	)
	if !held {
		// A package outside the workspace, the standard library
		// or a dependency: the type is named by its path, which a
		// backend qualifies and imports.
		return standIn(id), nil
	}
	if sym := declared(pkg, bare, typeKinds...); sym != nil {
		return sym, nil
	}
	return nil, fmt.Errorf("go: %s declares no type named %s", path, bare)
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
		return nil, fmt.Errorf("go: %s has no members to resolve %s in", scope.Subject, name)
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
	return nil, fmt.Errorf(
		"go: the members of %s hold no %s named %s",
		host.Identity(),
		kindList(kinds),
		name,
	)
}

// hostParam resolves a parameter on the subject's own signature.
func hostParam(scope rules.Scope, name string, v rules.View) (symbol.Symbol, error) {
	sym, held := v.Lookup(scope.Subject)
	if !held {
		return nil, fmt.Errorf("go: the view does not hold %s", scope.Subject)
	}
	var params []*node.Param
	switch c := sym.(type) {
	case *node.Function:
		params = c.Params
	case *node.Method:
		params = c.Params
	default:
		return nil, fmt.Errorf("go: %s is no callable, so it has no parameter named %s", scope.Subject, name)
	}
	for _, p := range params {
		if p != nil && p.Name == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("go: %s declares no parameter named %s", scope.Subject, name)
}

// hostType returns the type a subject belongs to: the subject
// itself where it is a type, and its host where it is a member.
func hostType(subject symbol.Identity, v rules.View) (node.Declaration, error) {
	sym, held := v.Lookup(subject)
	if !held {
		return nil, fmt.Errorf("go: the view does not hold %s", subject)
	}
	var host symbol.Identity
	switch m := sym.(type) {
	case *node.Struct, *node.Interface, *node.Enum, *node.Sum:
		decl, names := sym.(node.Declaration)
		if !names {
			return nil, fmt.Errorf("go: %s carries no identity", subject)
		}
		return decl, nil
	case *node.Field:
		host = m.Host
	case *node.Method:
		host = m.Host
	default:
		return nil, fmt.Errorf("go: %s belongs to no type", subject)
	}
	owner, held := v.Lookup(host)
	if !held {
		return nil, fmt.Errorf(
			"go: the view does not hold %s, the type %s belongs to",
			host,
			subject,
		)
	}
	decl, names := owner.(node.Declaration)
	if !names {
		return nil, fmt.Errorf("go: %s carries no identity", host)
	}
	return decl, nil
}

// packageFor returns the package a bare or qualified name resolves
// in, and the bare name: the subject's own package, or the one the
// qualifier's import names.
func packageFor(scope rules.Scope, name string, v rules.View) (*node.Package, string, error) {
	qualifier, bare, qualified := strings.Cut(name, qualifierSep)
	if !qualified {
		pkg, held := v.PackageOf(scope.Subject)
		if !held {
			return nil, "", fmt.Errorf(
				"go: the view does not hold the package declaring %s",
				scope.Subject,
			)
		}
		return pkg, name, nil
	}
	path, imported := importPath(scope.File, qualifier)
	if !imported {
		return nil, "", fmt.Errorf("go: no import of the subject's file binds %s", qualifier)
	}
	pkg, held := v.PackageOf(
		symbol.Identity{Lang: golang.Lang, Package: path, Kind: symbol.KindPackage},
	)
	if !held {
		return nil, "", fmt.Errorf(
			"go: the view does not hold %s, which %s imports",
			path,
			qualifier,
		)
	}
	return pkg, bare, nil
}

// importPath returns the path an import of the file binds under a
// qualifier: its alias, or the last segment of its path.
func importPath(f *node.File, qualifier string) (string, bool) {
	if f == nil {
		return "", false
	}
	for _, imp := range f.Imports {
		if imp == nil {
			continue
		}
		local := imp.Alias
		if local == "" {
			local = imp.Path[strings.LastIndex(imp.Path, "/")+1:]
		}
		if local == qualifier {
			return imp.Path, true
		}
	}
	return "", false
}

// declared returns a package's top-level declaration of one name
// and one of the kinds, or nil.
func declared(pkg *node.Package, name string, kinds ...symbol.Kind) symbol.Symbol {
	for decl := range node.Declarations(pkg) {
		id := decl.Identity()
		if id.Owner == "" && id.Name == name && slices.Contains(kinds, id.Kind) {
			return decl
		}
	}
	return nil
}

// standIn returns a declaration standing in for a type the graph
// does not hold: a builtin, or a type in a package outside the
// workspace, named by the identity alone.
func standIn(id symbol.Identity) *node.Alias {
	return &node.Alias{ID: id, Name: id.Name}
}

// builtinType says whether a spelling is one of Go's predeclared
// types, which no package owns.
func builtinType(spelling string) bool {
	if numeric(spelling) {
		return true
	}
	switch spelling {
	case spellString,
		boolSpelling,
		spellAny,
		errorSpelling,
		"comparable",
		"complex64",
		"complex128":
		return true
	default:
		return false
	}
}

// kindList spells a kind set for a refusal.
func kindList(kinds []symbol.Kind) string {
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, k.String())
	}
	return strings.Join(parts, " or ")
}
