// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"fmt"
	"go/token"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/module"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// qualifierSep separates a package qualifier from the name it qualifies,
// and a full import path from the name under it. pathSep separates the
// elements of an import path.
const (
	qualifierSep = "."
	pathSep      = "/"
)

// Resolve returns the declaration that a directive's spelling names from
// a subject. The kernel's walk, [rules.Resolve], selects the declarations
// that each resolution kind searches, and the resolver of Go decides the
// rest:
//
//   - A name in scope resolves through [golang.Bindings.Candidates] of
//     the file of the subject. The first candidate that the view
//     declares at one of the kinds is the declaration. The variant of an
//     enumeration is a constant, because Go declares it as a constant of
//     the package.
//   - A qualifier that the imports of the subject's file do not bind
//     resolves through the imports of every file of its package, with
//     [golang.PackageBindings.Candidates]. A qualifier that two files bind
//     to different packages names no declaration.
//   - A bare name that neither the package of the subject nor a dot import
//     of its file declares resolves in the external test package of the
//     directory, the package at the path of the subject's package with
//     [golang.TestSuffix]. It then resolves in the external test package
//     of the directory of the route of the scope, where the generated code
//     that refers to the name is. The path of that package is the path of
//     the subject's package joined with the route below the directory of
//     the subject's file, with [golang.TestSuffix], because the directories
//     of one module mirror its import paths.
//   - A name under a full import path, such as
//     example.com/shop/seed.Defaults, resolves in the package of that
//     path, whether the file imports the package or not. The last dot
//     separates the path from the name. The path has a slash, so
//     seed.Defaults is a name under the qualifier of an import.
//   - A member resolves only where Go code of the subject's package can
//     use it, so an unexported member of a type of another package does
//     not resolve.
//   - A pointer counts as the type that it points to.
//   - A predeclared type resolves to a stand-in with the name of the type
//     and no package. A type whose qualifier binds a package that the view
//     does not contain resolves to a stand-in with the import path as its
//     package. A type under the full import path of such a package
//     resolves to the same stand-in.
//
// Resolve refuses a spelling that is not a Go name, bare, qualified or
// under a full import path: []Row is the name of no declaration, and a
// probe of Row would find another type. A refusal is a
// *[rules.ResolveError].
//
// # Allocation contract
//
// A resolution through the probe allocates the scope of the subject's
// file and the list of candidates: four allocations for a file without a
// dot import. A qualified name that the file does not bind also allocates
// the list of its candidates in the imports of the package. A bare name
// that the package does not declare looks in the external test package
// and allocates nothing more. A look in the external test package of a
// route allocates the path of that package: the route below the directory
// of the file, the joined path and the path with its suffix, three
// allocations. A name under a full import path allocates no scope and no
// list of candidates. A member resolution allocates the list of the types
// that it searches and what the member walk allocates. A callable of a
// method subject resolves among the members first, and through the probe
// where no member has the name. A stand-in and a refusal allocate
// themselves.
func (r Rules) Resolve(
	scope rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	return rules.Resolve(resolver{}, r, v, scope, name, kind)
}

// resolver is the [rules.Resolver] of Go: the probe of a file's imports,
// the export of a member by the case of its name, the stand-ins of the
// types outside the view, and a pointer as its element.
type resolver struct{}

// InScope resolves a Go name to the first top-level declaration of one of
// the kinds that the probe of the subject's file finds in the view. A
// candidate whose package the view does not contain is skipped. A
// qualified name that the file does not bind then resolves through the
// imports of the files of the package, and a bare name in the external
// test package of the directory and then in the external test package of
// the directory of the route. A name under a full import path resolves
// among the declarations of the package of the path alone. InScope refuses
// a spelling that is not a Go name, a qualifier that two files of the
// package bind to different packages, and a name that no candidate
// declares.
func (resolver) InScope(scope rules.Scope, name string, kinds []symbol.Kind, v rules.View) (symbol.Symbol, error) {
	if path, bare, full := fullName(name); full {
		if pkg, held := v.PackageOf(packageIdentity(path)); held {
			if sym := declared(pkg, bare, kinds); sym != nil {
				return sym, nil
			}
		}
		return nil, refuse("no %s named %s is in the package %s", kindList(kinds), bare, path)
	}
	if !goName(name) {
		return nil, refuse("%s is not a Go name, so it names no declaration", name)
	}
	file := bindingsOf(scope.File)
	for _, c := range file.Candidates(scope.Subject.Package, name) {
		if sym := candidateIn(c, kinds, v); sym != nil {
			return sym, nil
		}
	}
	if !strings.Contains(name, qualifierSep) {
		external := symbol.Identity{Lang: golang.Lang, Package: scope.Subject.Package + golang.TestSuffix, Name: name}
		if sym := candidateIn(external, kinds, v); sym != nil {
			return sym, nil
		}
		if tests, routed := routeTests(scope); routed {
			inRoute := symbol.Identity{Lang: golang.Lang, Package: tests, Name: name}
			if sym := candidateIn(inRoute, kinds, v); sym != nil {
				return sym, nil
			}
		}
		return nil, refuse("no %s named %s is in scope of %s", kindList(kinds), name, scope.Subject)
	}
	candidates, ambiguous := packageBindings(scope, v).Candidates(file, name)
	if ambiguous {
		paths := make([]string, 0, len(candidates))
		for _, c := range candidates {
			paths = append(paths, c.Package)
		}
		qualifier, _, _ := strings.Cut(name, qualifierSep)
		joined := strings.Join(paths, " and ")
		return nil, refuse("the files of %s bind %s to %s", scope.Subject.Package, qualifier, joined)
	}
	for _, c := range candidates {
		if sym := candidateIn(c, kinds, v); sym != nil {
			return sym, nil
		}
	}
	return nil, refuse("no %s named %s is in scope of %s", kindList(kinds), name, scope.Subject)
}

// Usable reports whether Go code of the subject's package can use a
// member: an exported member in every package, and an unexported member in
// the package that declares it.
func (resolver) Usable(m rules.Member, _ symbol.Identity, scope rules.Scope) bool {
	decl, names := m.Symbol.(node.Declaration)
	if !names {
		return false
	}
	id := decl.Identity()
	return token.IsExported(id.Name) || id.Package == scope.Subject.Package
}

// Placeholder returns the stand-in of a predeclared type, which has the
// name of the type and no package. It returns the stand-in of a qualified
// type whose qualifier binds a package that the view does not contain,
// with the import path as its package. A type under the full import path
// of such a package has the same stand-in. Placeholder reports false for
// every other name.
func (resolver) Placeholder(scope rules.Scope, name string, v rules.View) (symbol.Symbol, bool) {
	if golang.Predeclared(name) {
		return standIn(symbol.Identity{Lang: golang.Lang, Name: name, Kind: symbol.KindAlias}), true
	}
	if !goName(name) {
		return nil, false
	}
	id, outside := outsideView(scope, name, symbol.KindAlias, v)
	if !outside {
		return nil, false
	}
	return standIn(id), true
}

// Value returns the type that a pointer points to, through every level of
// pointers, and every other reference unchanged.
func (resolver) Value(ref *node.TypeRef, _ rules.View) *node.TypeRef {
	for ref != nil && ref.Form == symbol.FormOptional && len(ref.Elems) == 1 {
		ref = ref.Elems[0]
	}
	return ref
}

// bindingsOf returns the bindings of the imports of a file. For a
// subject without a file, it returns the zero Bindings, which binds
// nothing.
func bindingsOf(f *node.File) golang.Bindings {
	if f == nil {
		return golang.Bindings{}
	}
	return golang.NewBindings(f.Imports)
}

// routeTests returns the path of the external test package of the
// directory of the route of scope: the path of the package of the subject's
// directory joined with the route below the directory of the subject's
// file, with [golang.TestSuffix]. It reports false for a scope without a
// route or without a file, for a route that is the directory of the file,
// whose external test package InScope reads already, and for a file outside
// the workspace root. routeTests allocates the path.
func routeTests(scope rules.Scope) (string, bool) {
	if scope.Route == "" || scope.File == nil {
		return "", false
	}
	rel, err := filepath.Rel(filepath.FromSlash(path.Dir(scope.File.Pos.File)), filepath.FromSlash(scope.Route))
	if err != nil || rel == "." {
		return "", false
	}
	dir := strings.TrimSuffix(scope.Subject.Package, golang.TestSuffix)
	return path.Join(dir, filepath.ToSlash(rel)) + golang.TestSuffix, true
}

// packageBindings returns the scope of the imports of the files of the
// package of a subject. For a package that the view does not contain, it
// returns nil, which binds nothing.
func packageBindings(scope rules.Scope, v rules.View) golang.PackageBindings {
	pkg, held := v.PackageOf(packageIdentity(scope.Subject.Package))
	if !held {
		return nil
	}
	return golang.PackageBindings(pkg.Files)
}

// candidateIn returns the top-level declaration of a candidate with one of
// the kinds, and nil where the view does not contain the package of the
// candidate or the package does not declare the name.
func candidateIn(c symbol.Identity, kinds []symbol.Kind, v rules.View) symbol.Symbol {
	pkg, held := v.PackageOf(packageIdentity(c.Package))
	if !held {
		return nil
	}
	return declared(pkg, c.Name, kinds)
}

// goName reports whether a spelling is a Go name: an identifier, two
// identifiers joined by the qualifier's dot, or an identifier under a full
// import path, which [fullName] reads. A decorated, an instantiated and a
// composite spelling are not.
func goName(spelling string) bool {
	if _, _, full := fullName(spelling); full {
		return true
	}
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

// outsideView returns the identity of a qualified name whose qualifier the
// file of the scope imports from a package that the view does not contain,
// such as a package of the standard library or of a dependency. Where the
// file does not bind the qualifier, the import of another file of its
// package does. A name under the full import path of such a package needs
// no import of the file. The identity has the import path as its package
// and kind as its kind, so a backend qualifies and imports it. outsideView
// reports false for a bare name, for a qualifier that no file of the
// package binds to one path, and for a package in the view. It also
// reports false for a full import path that [memberOf] reports.
func outsideView(scope rules.Scope, name string, kind symbol.Kind, v rules.View) (symbol.Identity, bool) {
	path, bare, full := fullName(name)
	if !full {
		qualifier, rest, qualified := strings.Cut(name, qualifierSep)
		if !qualified {
			return symbol.Identity{}, false
		}
		var bound bool
		if path, bound = bindingsOf(scope.File).Import(qualifier); !bound {
			if path, bound = packageBindings(scope, v).Import(qualifier); !bound {
				return symbol.Identity{}, false
			}
		}
		bare = rest
	}
	if inView(path, v) || full && memberOf(path, v) {
		return symbol.Identity{}, false
	}
	return symbol.Identity{Lang: golang.Lang, Package: path, Name: bare, Kind: kind}, true
}

// fullName splits a name under a full import path, such as
// example.com/shop/seed.Defaults, at its last dot into the path and the
// name. It reports false where the name is not an identifier, and where
// the path has no slash or is not a valid import path. A path without a
// slash is the qualifier of an import, as in seed.Defaults. fullName
// allocates nothing for a valid path, and the error of the check of an
// invalid one.
func fullName(spelling string) (path, name string, ok bool) {
	path, name, cut := strings.CutLast(spelling, qualifierSep)
	if !cut || !token.IsIdentifier(name) || !strings.Contains(path, pathSep) ||
		module.CheckImportPath(path) != nil {

		return "", "", false
	}
	return path, name, true
}

// memberOf reports whether a full import path is the path of a package of
// the view with a dot and a name after it, such as
// example.com/shop/seed.Store. A spelling under such a path names a member
// of a declaration of that package, so it has no stand-in.
func memberOf(path string, v rules.View) bool {
	dot := strings.LastIndex(path, qualifierSep)
	return dot > strings.LastIndex(path, pathSep) && inView(path[:dot], v)
}

// packageIdentity returns the identity of the Go package at an import
// path.
func packageIdentity(path string) symbol.Identity {
	return symbol.Identity{Lang: golang.Lang, Package: path, Kind: symbol.KindPackage}
}

// declared returns the top-level declaration of a package with one name
// and one of the kinds, or nil. It reads the top-level declarations of
// each file and nothing nested inside them, except the variants of an
// enumeration where the kinds contain a constant. Go declares such a
// variant as a constant of the package, and the frontend moves it into
// its enumeration.
func declared(pkg *node.Package, name string, kinds []symbol.Kind) symbol.Symbol {
	constants := slices.Contains(kinds, symbol.KindConstant)
	for _, f := range pkg.Files {
		for _, sym := range f.Decls {
			var id symbol.Identity
			if decl, names := sym.(node.Declaration); names {
				id = decl.Identity()
			}
			if id.Owner == "" && id.Name == name && slices.Contains(kinds, id.Kind) {
				return sym
			}
			if e, isEnum := sym.(*node.Enum); isEnum && constants {
				for _, variant := range e.Variants {
					if variant != nil && variant.Name == name {
						return variant
					}
				}
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

// refuse returns a *[rules.ResolveError] of Go with the reason that
// format and args write.
func refuse(format string, args ...any) error {
	return &rules.ResolveError{Lang: golang.Lang, Msg: fmt.Sprintf(format, args...)}
}
