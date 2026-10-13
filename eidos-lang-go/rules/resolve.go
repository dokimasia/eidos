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
//   - A member resolves only where Go code of the subject's package can
//     use it, so an unexported member of a type of another package does
//     not resolve.
//   - A pointer counts as the type that it points to.
//   - A predeclared type resolves to a stand-in with the name of the type
//     and no package. A type whose qualifier binds a package that the view
//     does not contain resolves to a stand-in with the import path as its
//     package.
//
// Resolve refuses a spelling that is not a Go name, bare or qualified:
// []Row is the name of no declaration, and a probe of Row would find
// another type. A refusal is a *[rules.ResolveError].
//
// # Allocation contract
//
// A resolution through the probe allocates the scope of the subject's
// file and the list of candidates: four allocations for a file without a
// dot import. A member resolution allocates the list of the types that it
// searches and what the member walk allocates. A callable of a method
// subject resolves among the members first, and through the probe where
// no member has the name. A stand-in and a refusal allocate themselves.
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
// candidate whose package the view does not contain is skipped. It
// refuses a spelling that is not a Go name, and a name that no candidate
// declares.
func (resolver) InScope(scope rules.Scope, name string, kinds []symbol.Kind, v rules.View) (symbol.Symbol, error) {
	if !goName(name) {
		return nil, refuse("%s is not a Go name, so it names no declaration", name)
	}
	for _, c := range bindingsOf(scope.File).Candidates(scope.Subject.Package, name) {
		pkg, held := v.PackageOf(packageIdentity(c.Package))
		if !held {
			continue
		}
		if sym := declared(pkg, c.Name, kinds); sym != nil {
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
// with the import path as its package. It reports false for every other
// name.
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

// outsideView returns the identity of a qualified name whose qualifier the
// file of the scope imports from a package that the view does not contain,
// such as a package of the standard library or of a dependency. The
// identity has the import path as its package and kind as its kind, so a
// backend qualifies and imports it. outsideView reports false for a bare
// name, for a qualifier that no import of the file binds, and for a
// package in the view.
func outsideView(scope rules.Scope, name string, kind symbol.Kind, v rules.View) (symbol.Identity, bool) {
	qualifier, bare, qualified := strings.Cut(name, qualifierSep)
	if !qualified {
		return symbol.Identity{}, false
	}
	path, bound := bindingsOf(scope.File).Import(qualifier)
	if !bound || inView(path, v) {
		return symbol.Identity{}, false
	}
	return symbol.Identity{Lang: golang.Lang, Package: path, Name: bare, Kind: kind}, true
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
