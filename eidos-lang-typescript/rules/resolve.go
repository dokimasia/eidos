// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names and marks of the module system of TypeScript.
const (
	// wildcardName is the name that a namespace re-export binds.
	wildcardName = "*"
	// packageSep separates the segments of a package path.
	packageSep = "/"
	// globalPackage is the package of the global scope of TypeScript.
	globalPackage = ""
)

// exportDepth bounds a chain of re-exports that resolution follows. A
// cycle of re-exports publishes nothing, so the bound only ends a walk
// over a damaged graph.
const exportDepth = 8

// chainSize is the number of packages in the scope chain of a subject
// that fit on the stack. The chain lists the package of the subject, the
// namespaces that enclose it, and its module.
const chainSize = 8

// Resolve returns the declaration that name refers to in the scope of a
// subject. The kernel's walk, [rules.Resolve], selects the declarations
// that each resolution kind searches, and the resolver of TypeScript
// decides the rest. A name in scope resolves in the scope order of
// TypeScript:
//
//  1. the package of the subject and the namespaces that enclose it,
//     innermost first
//  2. the module of the file of the subject
//  3. the module from which an import of the file binds the name
//  4. the global package
//
// A qualified name resolves through a namespace import of its first
// name, an import of a namespace, or a namespace of one of those
// packages. A specifier that the governing tsconfig maps through its
// paths or its baseUrl refers to the packages that the import or the
// re-export records as its Modules, in order. A relative specifier refers
// to its module file before the index of the directory, and every other
// specifier refers to the ambient module of its text. A name that a
// module publishes and does not declare resolves through the export
// clauses of the module, its default export and its export-star
// statements.
//
// A global type without a shadowing declaration in scope, such as string
// or Date, resolves to a stand-in with its name and no package. A name
// that an import binds from a module outside the view resolves to a
// stand-in with the module as its package. An optional counts as its
// type, and an alias as its target.
//
// A member resolves only where the subject can use it. A public member
// resolves on every type. A protected member resolves on the own type of
// the subject, and a private member only when the own type of the subject
// declares it. Resolve refuses a name that is not a TypeScript name or
// path. A refusal is a *[rules.ResolveError].
//
// # Allocation contract
//
// A resolution in scope allocates the module path of each relative
// specifier that it follows, which path.Join builds in three
// allocations. It also allocates the package path of a namespace in a
// path of names. The chain of the packages of the subject is on the stack
// up to eight packages. A member resolution allocates the list of the
// types that it searches and what the member walk allocates. A stand-in
// and a refusal allocate themselves. Every read allocates what the read
// set of the view allocates to record a new edge.
func (r Rules) Resolve(
	scope rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	return rules.Resolve(resolver{}, r, v, scope, name, kind)
}

// resolver is the [rules.Resolver] of TypeScript: the scope order of a
// module and its imports, the visibility of a member, the global types and
// the modules outside the view, and an optional as its type.
type resolver struct{}

// InScope resolves a name or a path to the first declaration of one of
// the kinds in the scope of the subject, in the order of [Rules.Resolve].
// It refuses a name that is not a TypeScript name or path, and a name to
// which no declaration in scope matches.
func (resolver) InScope(scope rules.Scope, name string, kinds []symbol.Kind, v rules.View) (symbol.Symbol, error) {
	if lit, scanned := scanLiteral(name); !scanned || lit.kind != literalPath || lit.text != name {
		return nil, refuse("%s is not a TypeScript name, so it cannot refer to a declaration", name)
	}
	if sym := (finder{v: v, kinds: kinds}).first(scope, name); sym != nil {
		return sym, nil
	}
	return nil, refuse("no %s named %s is in scope of %s", kindList(kinds), name, scope.Subject)
}

// Usable reports whether the subject can use a member. A public member
// and a member without a stated visibility are usable on every type. On
// the own type of the subject, a protected member is usable, and so is a
// private member that the own type declares.
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

// Placeholder returns the stand-in of a global type of TypeScript, which
// has the name of the type and no package. It returns the stand-in of a
// type that an import binds from a module outside the view, which has the
// module and the name that the module exports. It reports false for every
// other name.
func (resolver) Placeholder(scope rules.Scope, name string, v rules.View) (symbol.Symbol, bool) {
	if typescript.GlobalType(name) || typescript.KeywordType(name) {
		return &node.Alias{
			ID:   symbol.Identity{Lang: typescript.Lang, Name: name, Kind: symbol.KindAlias},
			Name: name,
		}, true
	}
	file := moduleFile(scope, v)
	head, inner, last := segments(name)
	src, exported, bound := binding(file, head)
	if !bound || exported == "" && head == name {
		return nil, false
	}
	base, relative := moduleOf(src.specifier, file.Path)
	var buf [2]string
	tried := src.modules
	if len(tried) == 0 {
		tried = append(buf[:0], base)
		if relative {
			tried = append(tried, typescript.JoinPackage(base, typescript.IndexModule))
		}
	}
	for _, pkg := range tried {
		_, inView := v.PackageOf(symbol.Identity{Lang: typescript.Lang, Package: pkg, Kind: symbol.KindPackage})
		if inView {
			return nil, false
		}
	}
	id := symbol.Identity{Lang: typescript.Lang, Package: base, Name: exported, Kind: symbol.KindAlias}
	if head != name {
		id.Package, id.Name = typescript.JoinPackage(typescript.JoinPackage(base, exported), inner), last
	}
	return &node.Alias{ID: id, Name: id.Name}, true
}

// Value returns the type of a value of the type ref: the target of an
// alias, and the type of an optional, also behind an alias.
func (resolver) Value(ref *node.TypeRef, v rules.View) *node.TypeRef {
	t := rules.Unaliased(ref, v)
	if t != nil && t.Form == symbol.FormOptional && len(t.Elems) == 1 {
		t = rules.Unaliased(t.Elems[0], v)
	}
	return t
}

// segments splits a path of names into its first name, the namespace
// path between its first and its last name as a package path, and its
// last name. A bare name is its own first and last name.
func segments(name string) (head, inner, last string) {
	head, _, _ = strings.Cut(name, pathSep)
	at := strings.LastIndex(name, pathSep)
	if at < 0 {
		return name, "", name
	}
	middle := strings.TrimPrefix(name[len(head):at], pathSep)
	return head, strings.ReplaceAll(middle, pathSep, packageSep), name[at+len(pathSep):]
}

// namespaces appends a package to dst, followed by the package of each
// namespace that encloses it, innermost first. It returns the extended
// list. The typescript.namespace stamp on the package of a namespace
// contains the dotted name of the namespace. Each name in it is one
// segment of the path of the package, so the stamp gives the number of
// segments that the namespaces add to the package that contains them. A
// view without the key reads no stamp.
func namespaces(dst []string, pkg string, v rules.View) []string {
	dst = append(dst, pkg)
	id := symbol.Identity{Lang: typescript.Lang, Package: pkg, Kind: symbol.KindPackage}
	dotted, stamped := rules.Fact(v, id, meta.Named[string](typescript.NamespaceKey))
	if !stamped {
		return dst
	}
	for range strings.Count(dotted, pathSep) + 1 {
		pkg = pkg[:max(strings.LastIndex(pkg, packageSep), 0)]
		dst = append(dst, pkg)
	}
	return dst
}

// moduleFile returns the File node of the file of a scope, in the package
// of the module path of the file. That File node records the imports and the
// exports of the module. moduleFile returns nil for a scope without a
// file, and for a script, which declares into the global package and
// imports nothing.
func moduleFile(scope rules.Scope, v rules.View) *node.File {
	if scope.File == nil {
		return nil
	}
	pkg, held := v.PackageOf(symbol.Identity{
		Lang: typescript.Lang, Package: typescript.ModulePath(scope.File.Path), Kind: symbol.KindPackage,
	})
	if !held {
		return nil
	}
	for _, f := range pkg.Files {
		if f != nil && f.Path == scope.File.Path {
			return f
		}
	}
	return nil
}

// source is the module that an import or a re-export names: the
// specifier as written, and the packages that a configuration of the
// workspace maps it onto, which are nil where none maps it.
type source struct {
	specifier string
	modules   []string
}

// binding returns the module source and the exported name that an import
// of a file binds a local name to. A named import binds the exported
// name, and a default import binds default. A namespace import binds the
// module itself, without a name. binding reports false when no import
// binds the name, and for a nil file.
func binding(file *node.File, local string) (source, string, bool) {
	if file == nil {
		return source{}, "", false
	}
	for _, imp := range file.Imports {
		if imp == nil {
			continue
		}
		src := source{specifier: imp.Path, modules: imp.Modules}
		switch {
		case imp.Alias == local:
			return src, "", true
		case imp.Default == local:
			return src, typescript.DefaultExport, true
		default:
			for _, b := range imp.Names {
				if b != nil && (b.Alias == local || b.Alias == "" && b.Name == local) {
					return src, b.Name, true
				}
			}
		}
	}
	return source{}, "", false
}

// moduleOf returns the package of the module that a specifier refers to
// from a file, and reports whether the specifier is relative. A relative
// specifier refers to the module file of its path, and TypeScript tries
// the index of the directory of the path next. Any other specifier refers to
// the ambient module of its text.
func moduleOf(specifier, from string) (string, bool) {
	if !typescript.Relative(specifier) {
		return specifier, false
	}
	return typescript.ModulePath(path.Join(path.Dir(from), specifier)), true
}

// finder looks up the declaration of one resolution through a view.
type finder struct {
	v     rules.View
	kinds []symbol.Kind // the declaration kinds that the resolution accepts
}

// first returns the first declaration of the kinds of the finder to which
// a name or a path refers in the scope of a subject, in the order of
// [Rules.Resolve]. It returns nil when no declaration matches. A bare
// name probes the chain of the packages of the subject, then the module
// from which an import binds the name, then the global package. A path
// probes the module or the namespace to which an import binds its first
// name. Without such an import, a path probes the namespace path in each
// package of the chain and in the global package.
func (f finder) first(scope rules.Scope, name string) symbol.Symbol {
	file := moduleFile(scope, f.v)
	var buf [chainSize]string
	chain := namespaces(buf[:0], scope.Subject.Package, f.v)
	if file != nil {
		if own := typescript.ModulePath(file.Path); !slices.Contains(chain, own) {
			chain = append(chain, own)
		}
	}
	head, inner, last := segments(name)
	src, exported, bound := binding(file, head)
	switch {
	case head == name:
		for _, pkg := range chain {
			if sym := f.find(pkg, name, 0); sym != nil {
				return sym
			}
		}
		if bound && exported != "" {
			if sym := f.inModule(src, file.Path, "", exported, 0); sym != nil {
				return sym
			}
		}
	case bound:
		return f.inModule(src, file.Path, typescript.JoinPackage(exported, inner), last, 0)
	default:
		for _, pkg := range chain {
			if sym := f.find(typescript.JoinPackage(pkg, typescript.JoinPackage(head, inner)), last, 0); sym != nil {
				return sym
			}
		}
	}
	if slices.Contains(chain, globalPackage) {
		return nil
	}
	if head == name {
		return f.find(globalPackage, name, 0)
	}
	return f.find(typescript.JoinPackage(head, inner), last, 0)
}

// inModule returns the declaration that the module of a source, or a
// namespace path inside it, publishes under a name. Where a configuration
// maps the specifier, it searches the packages of the mapping in order.
// For a relative specifier, it searches the module file of the path and
// then the index of the directory. For every other specifier, it searches
// the ambient module of its text.
func (f finder) inModule(src source, from, inner, name string, depth int) symbol.Symbol {
	for _, pkg := range src.modules {
		if sym := f.find(typescript.JoinPackage(pkg, inner), name, depth); sym != nil {
			return sym
		}
	}
	if len(src.modules) > 0 {
		return nil
	}
	base, relative := moduleOf(src.specifier, from)
	if sym := f.find(typescript.JoinPackage(base, inner), name, depth); sym != nil || !relative {
		return sym
	}
	return f.find(typescript.JoinPackage(typescript.JoinPackage(base, typescript.IndexModule), inner), name, depth)
}

// find returns the top-level declaration of a package that has a name
// and one of the kinds of the finder. Without one, it returns the declaration
// that the package publishes under the name through a re-export. A
// re-export is an export clause, the default export or an export-star
// statement, in source order. find returns nil when the view does not
// contain the package, when neither search finds the name, and below
// [exportDepth] re-exports.
func (f finder) find(pkg, name string, depth int) symbol.Symbol {
	if depth > exportDepth {
		return nil
	}
	p, held := f.v.PackageOf(symbol.Identity{Lang: typescript.Lang, Package: pkg, Kind: symbol.KindPackage})
	if !held {
		return nil
	}
	for _, file := range p.Files {
		for _, sym := range file.Decls {
			if decl, names := sym.(node.Declaration); names {
				id := decl.Identity()
				if id.Owner == "" && id.Name == name && slices.Contains(f.kinds, id.Kind) {
					return sym
				}
			}
		}
	}
	for _, file := range p.Files {
		for _, e := range file.Exports {
			if sym := f.published(pkg, file, e, name, depth); sym != nil {
				return sym
			}
		}
	}
	return nil
}

// published returns the declaration that one export statement of the
// file of a module publishes under a name. A default export publishes a
// local name. An export clause publishes a name from its source module,
// or from the scope of the file. An export-star statement publishes every
// name of its module except default. A frontend records no nil export
// statement.
func (f finder) published(pkg string, file *node.File, e *node.Export, name string, depth int) symbol.Symbol {
	if e.Default != "" && name == typescript.DefaultExport {
		if sym := f.local(pkg, file, e.Default, depth); sym != nil {
			return sym
		}
	}
	src := source{specifier: e.Path, modules: e.Modules}
	for _, b := range e.Names {
		if b == nil || b.Name == wildcardName || b.Alias != name && (b.Alias != "" || b.Name != name) {
			continue
		}
		if e.Path == "" {
			return f.local(pkg, file, b.Name, depth)
		}
		if sym := f.inModule(src, file.Path, "", b.Name, depth+1); sym != nil {
			return sym
		}
	}
	if e.Wildcard && name != typescript.DefaultExport {
		return f.inModule(src, file.Path, "", name, depth+1)
	}
	return nil
}

// local returns the declaration to which a name of the scope of a module
// refers. That is the declaration to which an import of the file binds
// the name, or otherwise the declaration of the module.
func (f finder) local(pkg string, file *node.File, name string, depth int) symbol.Symbol {
	src, exported, bound := binding(file, name)
	if !bound || exported == "" {
		return f.find(pkg, name, depth+1)
	}
	return f.inModule(src, file.Path, "", exported, depth+1)
}

// kindList joins the names of a set of kinds for a refusal.
func kindList(kinds []symbol.Kind) string {
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, k.String())
	}
	return strings.Join(parts, " or ")
}

// refuse returns a *[rules.ResolveError] of TypeScript with the reason
// that format and args write.
func refuse(format string, args ...any) error {
	return &rules.ResolveError{Lang: typescript.Lang, Msg: fmt.Sprintf(format, args...)}
}
