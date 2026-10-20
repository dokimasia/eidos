// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// exportDepth bounds a chain of use declarations that a resolution follows
// through the modules that bind a name. A cycle binds nothing new, so the
// bound only ends a walk over a damaged graph.
const exportDepth = 8

// The keywords that open a path: the crate root, the module itself and its
// parent.
const (
	crateKeyword = "crate"
	selfKeyword  = "self"
	superKeyword = "super"
)

// typeKinds are the declaration kinds of a type, which name the owner of an
// associated item in a path.
var typeKinds = []symbol.Kind{
	symbol.KindStruct, symbol.KindInterface, symbol.KindAlias, symbol.KindEnum, symbol.KindSum,
}

// Resolve returns the declaration that name refers to in the scope of a
// subject. The kernel's walk, [rules.Resolve], selects the declarations that
// each resolution kind searches, and the resolver of Rust decides the rest. A
// name resolves in the scope order of Rust:
//
//  1. A bare name is an item of the module of the subject or the target of
//     its use binding in the file of the subject, and then a name that a glob
//     import of the file imports, in statement order.
//  2. crate, self and super open a path at the crate root, at the module of
//     the subject and at its parent.
//  3. Any other path opens at a child module of the module, at the target of
//     the use binding of its first segment, or at an external crate.
//  4. A path to an associated item, as Store::Item, is an item of the type
//     that its other segments name. A function of a path is also an
//     associated function of the type, as Store::open.
//  5. Self is the type of the subject: the type that declares a member, and
//     a type itself. A path from Self, as Self::new, is a path to an
//     associated item of that type.
//
// A module that does not declare a name resolves it through its own use
// declarations and glob imports, as a path to a name that a module imports
// reads it. The items of a module are not in scope in its child modules.
//
// A type of the standard library resolves to a stand-in without a
// declaration in the view, and so does a type of a crate outside the view.
// The value of an optional, a pointer and a borrow is a value of its type,
// the value of a result is a value of the type of its value, and the value of
// an alias is a value of its target.
//
// A member resolves where the subject can use it: a public member everywhere,
// a member of crate visibility in the crate of its type, and a private member
// in the module of its type and the modules below it. Resolve refuses a name
// that is not a path of identifiers, and a name that resolves to no
// declaration. A refusal is a *[rules.ResolveError].
//
// # Allocation contract
//
// A resolution in scope allocates the segments of a path and the package
// paths of its modules. A member resolution allocates the list of the types
// that it searches and what the member walk allocates. A stand-in and a
// refusal allocate themselves. Every read allocates what the read set of the
// view allocates to record a new edge.
func (r Rules) Resolve(
	scope rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	return rules.Resolve(resolver{r: r}, r, v, scope, name, kind)
}

// resolver is the [rules.Resolver] of Rust: the scope order of Rust, the
// visibility of a member, the stand-ins of the standard library and of the
// crates outside the view, and the type of the value of a wrapper.
type resolver struct {
	r Rules
}

// InScope resolves a name or a path to the first declaration of one of the
// kinds in the scope of the subject, in the order of [Rules.Resolve]. It
// refuses a name that is not a path of identifiers, and a name to which no
// declaration in scope matches.
func (resolver) InScope(scope rules.Scope, name string, kinds []symbol.Kind, v rules.View) (symbol.Symbol, error) {
	for segment := range strings.SplitSeq(name, pathSep) {
		if !naming.IsIdentifier(segment) {
			return nil, &rules.ResolveError{
				Lang: rust.Lang, Msg: name + " is not a path of Rust, so it cannot refer to a declaration",
			}
		}
	}
	if sym := (finder{v: v, kinds: kinds}).first(scope, name); sym != nil {
		return sym, nil
	}
	return nil, &rules.ResolveError{
		Lang: rust.Lang, Msg: fmt.Sprintf("no %s named %s is in scope of %s", kindList(kinds), name, scope.Subject),
	}
}

// Usable reports whether the subject can use a member. A public member and a
// member without a stated visibility are usable everywhere. A member of crate
// visibility, which the model states for pub(crate), pub(super) and
// pub(in path) alike, is usable in the crate of its type, the first segment
// of the package path. A private member is usable in the module of its type
// and in the modules below it.
func (resolver) Usable(m rules.Member, _ symbol.Identity, scope rules.Scope) bool {
	var vis symbol.Visibility
	switch d := m.Symbol.(type) {
	case *node.Field:
		vis = d.Visibility
	case *node.Method:
		vis = d.Visibility
	}
	pkg, at := m.Owner.Package, scope.Subject.Package
	switch vis {
	case symbol.VisibilityPublic, symbol.VisibilityUnknown:
		return true
	case symbol.VisibilityInternal:
		crate, _, _ := strings.Cut(pkg, packageSep)
		subject, _, _ := strings.Cut(at, packageSep)
		return crate == subject
	default:
		return at == pkg || len(at) > len(pkg) && strings.HasPrefix(at, pkg) && at[len(pkg)] == packageSep[0]
	}
}

// Placeholder returns the stand-in of a type that the view does not declare:
// a type of the table of [Rules.Builtin], the unit type, and a type of a
// crate outside the view. The stand-in has the name of the type and the
// package of its module: the module of its path, the module of its use
// binding, and no package for a name of the prelude. It reports false for
// every other name.
func (resolver) Placeholder(scope rules.Scope, name string, v rules.View) (symbol.Symbol, bool) {
	module, last := "", name
	if prefix, tail, qualified := strings.CutLast(name, pathSep); qualified {
		modules := (finder{v: v}).modules(scope, prefix)
		if len(modules) == 0 {
			return nil, false
		}
		module, last = modules[0], tail
	} else if bound, item, imported := bindingOf(scope.File, name); imported && item != "" {
		module, last = bound, item
	}
	_, _, known := libTypeOf(&node.TypeRef{Spelling: last, Package: module})
	_, inView := v.PackageOf(symbol.Identity{Lang: rust.Lang, Package: module, Kind: symbol.KindPackage})
	if !known && name != unitType && (module == "" || inView) {
		return nil, false
	}
	id := symbol.Identity{Lang: rust.Lang, Package: module, Name: last, Kind: symbol.KindAlias}
	return &node.Alias{ID: id, Name: last, Visibility: symbol.VisibilityPublic}, true
}

// Value returns the type of a value of the type ref: the type of the value
// of a result, of an optional, of a borrow and of a pointer of the standard
// library, such as Box and Arc, and the target of an alias, to [deriveDepth]
// steps. It returns ref for a named type of another kind.
func (rs resolver) Value(ref *node.TypeRef, v rules.View) *node.TypeRef {
	for range deriveDepth {
		if value, _, ok := rs.r.Result(ref, v); ok {
			ref = value
			continue
		}
		ref = rules.Unaliased(ref, v)
		switch {
		case ref == nil:
			return nil
		case ref.Form == symbol.FormBorrow || ref.Form == symbol.FormOptional:
			ref = firstElem(ref)
			continue
		case ref.Form != symbol.FormNamed || !ref.Target.IsZero():
			return ref
		}
		t, _, known := libTypeOf(ref)
		if !known || t.form != symbol.FormOptional && t.form != symbol.FormBorrow {
			return ref
		}
		ref = typeArg(ref, 0)
	}
	return nil
}

// finder looks up the declaration of one resolution through a view.
type finder struct {
	v     rules.View
	kinds []symbol.Kind // the declaration kinds that the resolution accepts
}

// first returns the first declaration of the kinds of the finder that a name
// or a path refers to in the scope of a subject, in the order of
// [Rules.Resolve], and nil where no declaration matches.
func (f finder) first(scope rules.Scope, name string) symbol.Symbol {
	prefix, last, qualified := strings.CutLast(name, pathSep)
	if !qualified {
		return f.bare(scope, name)
	}
	for _, pkg := range f.modules(scope, prefix) {
		if sym := f.find(pkg, last, 0); sym != nil {
			return sym
		}
	}
	return f.associated(scope, prefix, last)
}

// bare returns the declaration that a bare name refers to in the scope of a
// subject: the type of the subject for Self, an item of the module of the
// subject, then the target of the use binding of the name in the file of the
// subject, and then a name that a glob import of the file imports.
func (f finder) bare(scope rules.Scope, name string) symbol.Symbol {
	if name == selfType {
		return f.selfOf(scope.Subject)
	}
	if sym := f.declared(scope.Subject.Package, "", name); sym != nil {
		return sym
	}
	if module, item, imported := bindingOf(scope.File, name); imported && item != "" {
		if sym := f.find(module, item, 1); sym != nil {
			return sym
		}
	}
	if scope.File == nil {
		return nil
	}
	for _, imp := range scope.File.Imports {
		if imp == nil || !imp.Wildcard {
			continue
		}
		if sym := f.find(imp.Path, name, 1); sym != nil {
			return sym
		}
	}
	return nil
}

// selfOf returns the declaration of one of the kinds of the finder that Self
// refers to for the subject id: the type at the head of the owner chain of a
// member, as the data enum of a field of its variant, and the subject itself
// where it is a type. It returns nil for a subject that no type encloses.
func (f finder) selfOf(id symbol.Identity) symbol.Symbol {
	switch {
	case id.Owner != "":
		owner, _, _ := strings.Cut(id.Owner, symbol.OwnerSep)
		return f.declared(id.Package, "", owner)
	case slices.Contains(typeKinds, id.Kind):
		return f.declared(id.Package, "", id.Name)
	default:
		return nil
	}
}

// modules returns the package paths that the module path prefix of a path
// refers to in the scope of a subject, in the order of [Rules.Resolve]. A
// path from crate, self and super refers to one package, and nil above the
// crate root. A path from Self refers to an associated item, so modules
// returns nil for it. A path from a child module or from a use binding refers
// to one package. Any other path refers to an external crate, and then to a
// child module that no mod item of the load declares, such as one that a
// macro declares.
func (f finder) modules(scope rules.Scope, prefix string) []string {
	segments := strings.Split(prefix, pathSep)
	pkg := scope.Subject.Package
	switch segments[0] {
	case crateKeyword:
		return []string{joinPackage(f.crate(pkg), segments[1:])}
	case selfKeyword:
		return []string{joinPackage(pkg, segments[1:])}
	case superKeyword:
		root := f.crate(pkg)
		for len(segments) > 0 && segments[0] == superKeyword {
			if pkg == root {
				return nil
			}
			pkg, segments = path.Dir(pkg), segments[1:]
		}
		return []string{joinPackage(pkg, segments)}
	case selfType:
		return nil
	}
	if _, child := f.v.PackageOf(symbol.Identity{
		Lang: rust.Lang, Package: pkg + packageSep + segments[0], Kind: symbol.KindPackage,
	}); child {
		return []string{joinPackage(pkg, segments)}
	}
	if module, item, imported := bindingOf(scope.File, segments[0]); imported {
		if item != "" {
			module += packageSep + item
		}
		return []string{joinPackage(module, segments[1:])}
	}
	external := strings.Join(segments, packageSep)
	return []string{external, pkg + packageSep + external}
}

// associated returns the item of the name that the type of the path prefix
// declares, where prefix names a type in the scope: an item of one of the
// kinds of the finder whose owner is the type, and, for a resolution of a
// function, an associated function of the type, a method at the level of the
// type. It returns nil where prefix names no type or the type has no such
// item.
func (f finder) associated(scope rules.Scope, prefix, name string) symbol.Symbol {
	owner, typed := (finder{v: f.v, kinds: typeKinds}).first(scope, prefix).(node.Declaration)
	if !typed {
		return nil
	}
	id := owner.Identity()
	chain := id.Name
	if id.Owner != "" {
		chain = id.Owner + symbol.OwnerSep + id.Name
	}
	if sym := f.declared(id.Package, chain, name); sym != nil {
		return sym
	}
	if !slices.Contains(f.kinds, symbol.KindFunction) {
		return nil
	}
	sym, _ := f.v.Lookup(symbol.Identity{
		Lang: rust.Lang, Package: id.Package, Owner: chain, Name: name, Kind: symbol.KindMethod,
	})
	if m, isMethod := sym.(*node.Method); isMethod && m.Level == symbol.LevelType {
		return m
	}
	return nil
}

// find returns the declaration of the package pkg with the name and one of
// the kinds of the finder. Without one, the name resolves through the use
// declarations of the files of the package: the target of a use binding of
// the name, and then the name in each module of a glob import, in the order
// of the files and their statements. find returns nil where the view
// contains no such declaration, and past [exportDepth] steps of the walk.
func (f finder) find(pkg, name string, depth int) symbol.Symbol {
	if sym := f.declared(pkg, "", name); sym != nil || depth > exportDepth {
		return sym
	}
	p, held := f.v.PackageOf(symbol.Identity{Lang: rust.Lang, Package: pkg, Kind: symbol.KindPackage})
	if !held {
		return nil
	}
	for _, file := range p.Files {
		if module, item, imported := bindingOf(file, name); imported && item != "" {
			if sym := f.find(module, item, depth+1); sym != nil {
				return sym
			}
		}
	}
	for _, file := range p.Files {
		for _, imp := range file.Imports {
			if imp == nil || !imp.Wildcard {
				continue
			}
			if sym := f.find(imp.Path, name, depth+1); sym != nil {
				return sym
			}
		}
	}
	return nil
}

// declared returns the declaration of the package pkg with the owner chain,
// the name and the first of the kinds of the finder that the view contains,
// and nil where it contains none.
func (f finder) declared(pkg, owner, name string) symbol.Symbol {
	for _, k := range f.kinds {
		id := symbol.Identity{Lang: rust.Lang, Package: pkg, Owner: owner, Name: name, Kind: k}
		if sym, held := f.v.Lookup(id); held {
			return sym
		}
	}
	return nil
}

// crate returns the package path of the crate root of the package pkg: the
// module that the frontend stamps on the package, and the first segment of
// pkg where the view reads no stamp.
func (f finder) crate(pkg string) string {
	id := symbol.Identity{Lang: rust.Lang, Package: pkg, Kind: symbol.KindPackage}
	if module, stamped := rules.Fact(f.v, id, f.v.Kernel().Module); stamped && module != "" {
		return module
	}
	root, _, _ := strings.Cut(pkg, packageSep)
	return root
}

// bindingOf returns what the use declarations of the file f bind the local
// name to: the package path of a module and the name of an item of the module
// for a use binding, as use std::collections::HashMap binds HashMap, and the
// path of a crate without an item for an extern crate declaration under
// another name. Rust refuses a name that two use declarations of one module
// bind, so the first binding decides. bindingOf reports false for a name that
// no use declaration binds, and for a nil file. It allocates nothing.
func bindingOf(f *node.File, local string) (module, item string, ok bool) {
	if f == nil {
		return "", "", false
	}
	for _, imp := range f.Imports {
		if imp == nil {
			continue
		}
		if imp.Alias == local && len(imp.Names) == 0 && !imp.Wildcard {
			return imp.Path, "", true
		}
		for _, b := range imp.Names {
			if b != nil && (b.Alias == local || b.Alias == "" && b.Name == local) {
				return imp.Path, b.Name, true
			}
		}
	}
	return "", "", false
}

// joinPackage appends the segments of a module path to the package path pkg.
// It allocates the joined path, and nothing for no segments.
func joinPackage(pkg string, segments []string) string {
	if len(segments) == 0 {
		return pkg
	}
	return pkg + packageSep + strings.Join(segments, packageSep)
}

// kindList joins the names of a set of kinds for a refusal.
func kindList(kinds []symbol.Kind) string {
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, k.String())
	}
	return strings.Join(parts, " or ")
}
