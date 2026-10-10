// Copyright Dokimasia B.V. 2026
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

// through names the types whose members a member reference of a
// callable subject resolves among.
type through uint8

const (
	// throughValue is the callable's value: the type of its first value
	// return, then the type of each input parameter.
	throughValue through = 1
	// throughHandle is the handle the callable returns: the type of its
	// first value return.
	throughHandle through = 2
)

// Resolve returns the declaration a directive's spelling names from
// a subject, the way Go scopes it:
//
//   - A callable resolves among the methods of the type a method
//     subject belongs to, which the method calls through its receiver,
//     and then through the probe of the subject's file,
//     [golang.Scope.Candidates], to a function.
//   - A package variable and a type in scope resolve through the probe:
//     the first candidate the view declares at one of the kinds is the
//     declaration.
//   - A value field resolves among the fields of the subject's value:
//     the subject itself for a type, the type a field belongs to, and
//     for a callable the type of its first value return, then the type
//     of each input parameter.
//   - A member on a handle resolves among the fields and the methods of
//     the subject's handle: the subject itself for a type, the type a
//     field belongs to, and for a callable the type of its first value
//     return.
//   - A host parameter resolves on the subject's own signature.
//
// A member resolves only where Go code of the subject's package can use
// it, so an unexported member of a type of another package does not
// resolve. A pointer counts as the type it points to. A predeclared type
// resolves to a stand-in with the name of the type and no package. A
// type whose qualifier binds a package that the view does not contain
// resolves to a stand-in with the import path as its package. Resolve
// refuses a spelling that is not a Go name, bare or qualified: []Row is
// the name of no declaration, and a probe of Row would find another
// type.
//
// # Allocation contract
//
// A resolution through the probe allocates the scope of the subject's
// file and the list of candidates: four allocations for a file without
// a dot import. A member resolution allocates the list of the types it
// searches, the binding the member walk runs on, and what the walk
// allocates. A callable of a method subject resolves among the members
// first, and through the probe where no member has the name. A
// stand-in and a refusal allocate themselves.
func (r Rules) Resolve(
	scope rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, refuse("nothing to resolve")
	}
	switch kind {
	case directive.ResolveCallableInScope:
		return r.callable(scope, name, v)
	case directive.ResolvePackageVar:
		return inScope(scope, name, v, symbol.KindVariable, symbol.KindConstant)
	case directive.ResolveValueField:
		return r.member(scope, name, v, throughValue, symbol.KindField)
	case directive.ResolveHostParam:
		return hostParam(scope, name, v)
	case directive.ResolveMemberOnHandle:
		return r.member(scope, name, v, throughHandle, symbol.KindField, symbol.KindMethod)
	case directive.ResolveTypeInScope:
		return typeInScope(scope, name, v)
	default:
		return nil, refuse("%s is not a resolution Go performs", kind)
	}
}

// callable resolves a callable from a subject: a method of the type a
// method subject belongs to, which the method calls through its
// receiver, before a function the probe of the subject's file finds.
// Every other subject resolves a function through the probe alone.
func (r Rules) callable(scope rules.Scope, name string, v rules.View) (symbol.Symbol, error) {
	sym, _ := v.Lookup(scope.Subject)
	method, isMethod := sym.(*node.Method)
	if !isMethod {
		return inScope(scope, name, v, symbol.KindFunction)
	}
	owner, _ := v.Lookup(method.Host)
	if host, isType := owner.(node.Declaration); isType {
		b := rules.NewBound(r, v, nil)
		if sibling, found := memberNamed(b, host, name, scope.Subject.Package, symbol.KindMethod); found {
			return sibling, nil
		}
	}
	if fn, err := inScope(scope, name, v, symbol.KindFunction); err == nil {
		return fn, nil
	}
	return nil, refuse("%s has no method named %s, and no function named %s is in scope of %s",
		method.Host, name, name, scope.Subject)
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

// member resolves a name among the effective members of the types that
// [Rules.holders] returns for the subject, the first type that has a
// member of the name and one of the kinds deciding.
func (r Rules) member(
	scope rules.Scope, name string, v rules.View, via through, kinds ...symbol.Kind,
) (symbol.Symbol, error) {
	types, err := r.holders(scope.Subject, v, via)
	if err != nil {
		return nil, err
	}
	b := rules.NewBound(r, v, nil)
	for _, t := range types {
		if m, found := memberNamed(b, t, name, scope.Subject.Package, kinds...); found {
			return m, nil
		}
	}
	searched := make([]string, 0, len(types))
	for _, t := range types {
		searched = append(searched, t.Identity().String())
	}
	return nil, refuse("%s has no %s member named %s that the package %s can use",
		strings.Join(searched, " or "), kindList(kinds), name, scope.Subject.Package)
}

// holders returns the types whose members a member reference of a
// subject resolves among: the subject itself for a type, the type a
// field belongs to, and for a callable the types of its value under
// via, in the order [Rules.valueTypes] gives. It refuses a subject, or
// the type of a field, that the view does not declare.
func (r Rules) holders(subject symbol.Identity, v rules.View, via through) ([]node.Declaration, error) {
	sym, _ := v.Lookup(subject)
	if field, isField := sym.(*node.Field); isField {
		sym, _ = v.Lookup(field.Host)
	}
	switch s := sym.(type) {
	case *node.Struct, *node.Interface, *node.Enum, *node.Sum:
		decl, _ := sym.(node.Declaration)
		return []node.Declaration{decl}, nil
	case *node.Function:
		return r.valueTypes(subject, s.Params, s.Returns, v, via)
	case *node.Method:
		return r.valueTypes(subject, s.Params, s.Returns, v, via)
	default:
		return nil, refuse("%s belongs to no type the view declares", subject)
	}
}

// valueTypes returns the declarations of a callable's value: the type
// of its first value return, then, through the value, the type of each
// input parameter, in order. A pointer counts as the type it points to.
// A reference without a declaration in the view, such as a predeclared
// type, has no members and is left out. A callable whose value has no
// declaration refuses.
func (r Rules) valueTypes(
	subject symbol.Identity, params []*node.Param, returns []*node.Return, v rules.View, via through,
) ([]node.Declaration, error) {
	var out []node.Declaration
	roles, _ := r.ReturnRoles(returns, v)
	for i, ret := range returns {
		if ret != nil && roles[i] == rules.ReturnValue {
			out = appendDeclared(out, ret.Type, v)
			break
		}
	}
	if via == throughValue {
		for _, p := range params {
			if p != nil && r.ParamRole(p, v) == rules.ParamInput {
				out = appendDeclared(out, p.Type, v)
			}
		}
	}
	if len(out) == 0 {
		return nil, refuse("%s has no value of a type the view declares", subject)
	}
	return out, nil
}

// appendDeclared appends the declaration a reference names to out, a
// pointer counting as the type it points to, and returns out unchanged
// for a reference without a declaration in the view.
func appendDeclared(out []node.Declaration, ref *node.TypeRef, v rules.View) []node.Declaration {
	for ref != nil && ref.Form == symbol.FormOptional && len(ref.Elems) == 1 {
		ref = ref.Elems[0]
	}
	// A structural reference and an unresolved name have the zero
	// target, which the view looks up as nothing.
	var target symbol.Identity
	if ref != nil {
		target = ref.Target
	}
	sym, held := v.Lookup(target)
	decl, names := sym.(node.Declaration)
	if !held || !names {
		return out
	}
	return append(out, decl)
}

// memberNamed returns the effective member of a type with a name and one
// of the kinds that Go code of the package from can use, and false where
// the type has none. An unexported member is usable only in the package
// that declares it. A symbol that is no type has no members.
func memberNamed(
	b rules.Bound, host node.Declaration, name, from string, kinds ...symbol.Kind,
) (symbol.Symbol, bool) {
	set, _ := b.MembersOf(host)
	for _, m := range set.Members {
		var id symbol.Identity
		if decl, names := m.Symbol.(node.Declaration); names {
			id = decl.Identity()
		}
		usable := token.IsExported(id.Name) || id.Package == from
		if id.Name == name && slices.Contains(kinds, id.Kind) && usable {
			return m.Symbol, true
		}
	}
	return nil, false
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
		for _, sym := range f.Decls {
			var id symbol.Identity
			if decl, names := sym.(node.Declaration); names {
				id = decl.Identity()
			}
			if id.Owner == "" && id.Name == name && slices.Contains(kinds, id.Kind) {
				return sym
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
