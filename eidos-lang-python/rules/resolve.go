// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/spell"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// exportDepth bounds a chain of imports and star imports that a resolution
// follows through the modules that publish a name. A cycle of imports
// publishes nothing new, so the bound only ends a walk over a damaged graph.
const exportDepth = 8

// Resolve returns the declaration that name refers to in the scope of a
// subject. The kernel's walk, [rules.Resolve], selects the declarations that
// each resolution kind searches, and the resolver of Python decides the
// rest. A name in scope resolves in the order of the scopes of an
// annotation of Python:
//
//  1. the classes that enclose the subject, innermost first;
//  2. the module of the subject, where a declaration of the module and a
//     binding of an import of the module come first;
//  3. each module of a star import of the file of the subject, in
//     statement order;
//  4. builtins, for a name without a dot.
//
// A dotted name resolves to a member of the class that its first name names,
// and through a module that an import binds its first name to, to the name
// in the longest module of its names. A name that a module publishes and
// does not declare resolves through the imports and the star imports of the
// module, as a module that imports the name reads it at run time.
//
// A type of builtins without a declaration in the view, such as int where
// the load reads no typeshed, resolves to a stand-in with its name in
// builtins. A name that an import binds from a module outside the view
// resolves to a stand-in with the module as its package. The value of an
// optional is a value of its type, and the value of an alias a value of its
// target.
//
// A member resolves where the subject can use it: a public member on every
// type, a protected member on the own type of the subject, and a private
// member where the own type declares it. Resolve refuses a name that is not
// a dotted name of identifiers, and a name that resolves to no declaration.
// A refusal is a *[rules.ResolveError].
//
// # Allocation contract
//
// A resolution in scope allocates the package path of a module that a
// dotted name names through an import. A member resolution allocates the
// list of the types that it searches and what the member walk allocates. A
// stand-in and a refusal allocate themselves. Every read allocates what the
// read set of the view allocates to record a new edge.
func (r Rules) Resolve(
	scope rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	return rules.Resolve(resolver{r: r}, r, v, scope, name, kind)
}

// resolver is the [rules.Resolver] of Python: the scopes of an annotation,
// the visibility of a member, the stand-ins of the builtins and of the
// modules outside the view, and an optional as its type.
type resolver struct {
	r Rules
}

// InScope resolves a name or a dotted name to the first declaration of one
// of the kinds in the scope of the subject, in the order of
// [Rules.Resolve]. It refuses a name that is not a dotted name of
// identifiers, and a name to which no declaration in scope matches.
func (resolver) InScope(scope rules.Scope, name string, kinds []symbol.Kind, v rules.View) (symbol.Symbol, error) {
	for segment := range strings.SplitSeq(name, python.ModuleSep) {
		if !spell.IsIdentifier(segment) {
			return nil, refuse("%s is not a dotted name of Python, so it cannot refer to a declaration", name)
		}
	}
	if sym := (finder{v: v, kinds: kinds}).first(scope, name); sym != nil {
		return sym, nil
	}
	return nil, refuse("no %s named %s is in scope of %s", kindList(kinds), name, scope.Subject)
}

// Usable reports whether the subject can use a member. A public member and a
// member without a stated visibility are usable on every type. On the own
// type of the subject, a protected member is usable, and so is a private
// member that the own type declares.
func (resolver) Usable(m rules.Member, own symbol.Identity, _ rules.Scope) bool {
	var vis symbol.Visibility
	switch d := m.Symbol.(type) {
	case *node.Field:
		vis = d.Visibility
	case *node.Method:
		vis = d.Visibility
	}
	return vis == symbol.VisibilityPublic || vis == symbol.VisibilityUnknown ||
		!own.IsZero() && (vis == symbol.VisibilityProtected || m.Owner == own)
}

// Placeholder returns the stand-in of a type of builtins that the view does
// not declare, with its name in builtins: a class of the table of
// [Rules.ShapeOf] other than an opaque one, None, and a well-known class. It
// returns the stand-in of a name that an import of the file of the subject
// binds from a module outside the view, with the module as its package and
// the name of the declaration as its name. It reports false for every other
// name.
func (rs resolver) Placeholder(scope rules.Scope, name string, v rules.View) (symbol.Symbol, bool) {
	head, rest, dotted := strings.Cut(name, python.ModuleSep)
	module, bound, imported := bindingOf(scope.File, head)
	switch {
	case !imported && dotted:
		return nil, false
	case !imported:
		stand := &node.TypeRef{Spelling: name}
		s := rs.r.Builtin(stand, v)
		if s.Form == symbol.FormOpaque {
			return nil, false
		}
		id := symbol.Identity{Lang: python.Lang, Package: builtinsModule, Name: name, Kind: symbol.KindAlias}
		return &node.Alias{ID: id, Name: name, Visibility: symbol.VisibilityPublic}, true
	}
	if _, inView := v.PackageOf(symbol.Identity{Lang: python.Lang, Package: module, Kind: symbol.KindPackage}); inView {
		return nil, false
	}
	switch {
	case bound != "" && dotted:
		bound += python.ModuleSep + rest
	case bound == "" && dotted:
		bound = rest
	case bound == "":
		return nil, false
	}
	owner, last, nested := strings.CutLast(bound, python.ModuleSep)
	if !nested {
		owner, last = "", bound
	}
	id := symbol.Identity{Lang: python.Lang, Package: module, Owner: owner, Name: last, Kind: symbol.KindAlias}
	return &node.Alias{ID: id, Name: last, Visibility: symbol.VisibilityPublic}, true
}

// Value returns the type of a value of the type ref: the target of an
// alias, and the type of an optional, also behind an alias.
func (resolver) Value(ref *node.TypeRef, v rules.View) *node.TypeRef {
	t := rules.Unaliased(ref, v)
	for t != nil && t.Form == symbol.FormOptional && len(t.Elems) == 1 {
		t = rules.Unaliased(t.Elems[0], v)
	}
	return t
}

// finder looks up the declaration of one resolution through a view.
type finder struct {
	v     rules.View
	kinds []symbol.Kind // the declaration kinds that the resolution accepts
}

// first returns the first declaration of the kinds of the finder that a name
// or a dotted name refers to in the scope of a subject, in the order of
// [Rules.Resolve], and nil where no declaration matches.
func (f finder) first(scope rules.Scope, name string) symbol.Symbol {
	inner, last, dotted := strings.CutLast(name, python.ModuleSep)
	if !dotted {
		inner, last = "", name
	}
	pkg := scope.Subject.Package
	for chain := classChain(scope.Subject); chain != ""; chain = outerChain(chain) {
		if sym := f.declared(pkg, joinOwner(chain, inner), last); sym != nil {
			return sym
		}
	}
	if sym := f.find(pkg, inner, last, 0); sym != nil {
		return sym
	}
	if head, rest, nested := strings.Cut(name, python.ModuleSep); nested {
		if module, bound, imported := bindingOf(scope.File, head); imported {
			if sym := f.through(module, bound, rest); sym != nil {
				return sym
			}
		}
	}
	if scope.File != nil {
		for _, imp := range scope.File.Imports {
			if imp == nil || !imp.Wildcard {
				continue
			}
			if sym := f.find(imp.Path, inner, last, 1); sym != nil {
				return sym
			}
		}
	}
	if dotted {
		return nil
	}
	return f.find(builtinsModule, "", name, 0)
}

// through returns the declaration that the names rest after the first name
// of a dotted name refer to, where an import binds the first name to the
// module, or to the declaration bound of the module: a member of the class
// that the import names, and then the name in each module that the names
// can name, the longest module first.
func (f finder) through(module, bound, rest string) symbol.Symbol {
	path, last, nested := strings.CutLast(rest, python.ModuleSep)
	if !nested {
		path, last = "", rest
	}
	if bound != "" {
		if sym := f.declared(module, joinOwner(bound, path), last); sym != nil {
			return sym
		}
		module += python.PackageSep + bound
	}
	for prefix := path; ; {
		pkg := module
		if prefix != "" {
			pkg = module + python.PackageSep + strings.ReplaceAll(prefix, python.ModuleSep, python.PackageSep)
		}
		owner := strings.TrimPrefix(strings.TrimPrefix(path, prefix), python.ModuleSep)
		if sym := f.find(pkg, owner, last, 0); sym != nil {
			return sym
		}
		if prefix == "" {
			return nil
		}
		prefix = prefix[:max(strings.LastIndex(prefix, python.ModuleSep), 0)]
	}
}

// find returns the declaration of the package pkg with the owner chain, the
// name and one of the kinds of the finder. Without one, a name at the top
// level resolves through what the package publishes: the declaration that
// an import of a file of the package binds the name to, and then the name
// in each module of a star import, in the order of the files and their
// statements. find returns nil where the view contains no such declaration,
// and below [exportDepth] steps of publication.
func (f finder) find(pkg, owner, name string, depth int) symbol.Symbol {
	if sym := f.declared(pkg, owner, name); sym != nil || owner != "" || depth > exportDepth {
		return sym
	}
	p, held := f.v.PackageOf(symbol.Identity{Lang: python.Lang, Package: pkg, Kind: symbol.KindPackage})
	if !held {
		return nil
	}
	for _, file := range p.Files {
		if module, bound, imported := bindingOf(file, name); imported && bound != "" {
			if sym := f.find(module, "", bound, depth+1); sym != nil {
				return sym
			}
		}
	}
	for _, file := range p.Files {
		for _, imp := range file.Imports {
			if imp == nil || !imp.Wildcard {
				continue
			}
			if sym := f.find(imp.Path, "", name, depth+1); sym != nil {
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
		if k == symbol.KindFunction {
			if sym := f.function(pkg, owner, name); sym != nil {
				return sym
			}
			continue
		}
		id := symbol.Identity{Lang: python.Lang, Package: pkg, Owner: owner, Name: name, Kind: k}
		if sym, held := f.v.Lookup(id); held {
			return sym
		}
	}
	return nil
}

// function returns the first function of the package pkg with the name, in
// the order of the files and their declarations, and nil where the package
// has none. The identity of a function has the discriminator of its
// parameters, so the finder reads the declarations of the package in place
// of one identity. A function of Python is at the top level of its module,
// so a function with an owner chain is nil.
func (f finder) function(pkg, owner, name string) symbol.Symbol {
	if owner != "" {
		return nil
	}
	p, held := f.v.PackageOf(symbol.Identity{Lang: python.Lang, Package: pkg, Kind: symbol.KindPackage})
	if !held {
		return nil
	}
	for _, file := range p.Files {
		for _, sym := range file.Decls {
			if fn, isFunction := sym.(*node.Function); isFunction && fn.Name == name {
				return sym
			}
		}
	}
	return nil
}

// classChain returns the owner chain of the classes that enclose the
// subject id, innermost last: the owner of a member, and the chain of the
// class itself and its owners for a class. A subject at the top level of
// its module has the empty chain.
func classChain(id symbol.Identity) string {
	switch id.Kind {
	case symbol.KindStruct, symbol.KindInterface, symbol.KindEnum:
		return joinOwner(id.Owner, id.Name)
	default:
		return id.Owner
	}
}

// outerChain returns the owner chain of the class that encloses the last
// class of a chain, and the empty chain for a class at the top level.
func outerChain(chain string) string {
	outer, _, nested := strings.CutLast(chain, symbol.OwnerSep)
	if !nested {
		return ""
	}
	return outer
}

// joinOwner joins two parts of an owner chain, either of which can be empty.
func joinOwner(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + symbol.OwnerSep + b
	}
}

// kindList joins the names of a set of kinds for a refusal.
func kindList(kinds []symbol.Kind) string {
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, k.String())
	}
	return strings.Join(parts, " or ")
}

// refuse returns a *[rules.ResolveError] of Python with the reason that
// format and args write.
func refuse(format string, args ...any) error {
	return &rules.ResolveError{Lang: python.Lang, Msg: fmt.Sprintf(format, args...)}
}
