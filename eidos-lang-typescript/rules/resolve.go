// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"fmt"
	"path"
	"slices"
	"strings"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// refusalPrefix starts every refusal that the rules return. It is the
// language's identity, as in the refusals of every satellite.
const refusalPrefix = string(typescript.Lang) + ": "

// The names and marks of TypeScript's module system.
const (
	// indexModule is the module that resolution tries in a directory
	// when the specifier's path has no module file.
	indexModule = "index"
	// defaultExport is the name of a module's default export, and the
	// name that a default import binds.
	defaultExport = "default"
	// wildcardName is the name that a namespace re-export binds.
	wildcardName = "*"
	// packageSep separates the segments of a package path.
	packageSep = "/"
	// globalPackage is the package of TypeScript's global scope.
	globalPackage = ""
	// currentDir and parentDir open a relative specifier. currentDirName
	// and parentDirName are relative specifiers on their own.
	currentDir     = "./"
	parentDir      = "../"
	currentDirName = "."
	parentDirName  = ".."
)

// exportDepth bounds a chain of re-exports that resolution follows. A
// cycle of re-exports publishes nothing, so the bound only ends a walk
// over a damaged graph.
const exportDepth = 8

// chainSize is the number of packages in the chain of a subject's scope
// that fit on the stack. The chain lists the subject's package, the
// namespaces that enclose it, and its module.
const chainSize = 8

// The declaration kinds of each resolution.
var (
	typeKinds     = []symbol.Kind{symbol.KindStruct, symbol.KindInterface, symbol.KindEnum, symbol.KindAlias}
	valueKinds    = []symbol.Kind{symbol.KindConstant, symbol.KindVariable}
	functionKinds = []symbol.Kind{symbol.KindFunction}
	methodKinds   = []symbol.Kind{symbol.KindMethod}
	fieldKinds    = []symbol.Kind{symbol.KindField}
	memberKinds   = []symbol.Kind{symbol.KindField, symbol.KindMethod}
)

// through selects the types whose members a member reference of a
// callable subject resolves against.
type through uint8

const (
	// throughValue selects the type of the callable's first value
	// return, then the type of each input parameter.
	throughValue through = 1
	// throughHandle selects the type of the callable's first value
	// return, which is the handle that the callable returns.
	throughHandle through = 2
)

// Resolve returns the declaration that name refers to in the scope of a
// subject. What name can refer to depends on the kind of the resolution:
//
//   - A callable resolves among the methods of the type that a method
//     subject belongs to, and then to a function in scope.
//   - A package variable resolves to a constant or a variable in scope.
//   - A type resolves to a class, an interface, an enum or a type alias
//     in scope. A global type without a shadowing declaration in scope,
//     such as string or Date, resolves to a stand-in with its name and
//     no package. A name that an import binds from a module outside the
//     view resolves to a stand-in with the module as its package.
//   - A value field and a member on a handle resolve among the members
//     of the subject's value or handle. For a type, that is the subject
//     itself, and for a member, the type that the member belongs to. For
//     a callable, it is the type of its first value return and, through
//     the value, the type of each input parameter.
//   - A host parameter resolves on the subject's own signature.
//
// A name in scope resolves in TypeScript's scope order:
//
//  1. the subject's package and the namespaces that enclose it,
//     innermost first;
//  2. the module of the subject's file;
//  3. the module that an import of the file binds the name from;
//  4. the global package.
//
// A qualified name resolves through a namespace import of its first
// name, an import of a namespace, or a namespace of one of those
// packages. A relative specifier refers to its module file before the
// directory's index, and every other specifier refers to the ambient
// module of its text. The graph does not record a tsconfig, so
// resolution does not read a paths pattern or a baseUrl. A name that a
// module publishes and does not declare resolves through the module's
// export clauses, its default export and its export-star statements.
//
// A member resolves only where the subject can use it. A public member
// resolves on every type. A protected member resolves on the subject's
// own type, and a private member only when the subject's own type
// declares it. Resolve refuses a name that is not a TypeScript name or
// path.
//
// # Allocation contract
//
// A resolution in scope allocates the module path of each relative
// specifier that it follows, which path.Join builds in three
// allocations. It also allocates the package path of a namespace in a
// path of names. The chain of the subject's packages is on the stack up
// to eight packages. A member resolution allocates the list of the types
// that it searches, the binding of the member walk, and what the walk
// allocates. A stand-in and a refusal allocate themselves. Every read
// allocates what the view's read set allocates to record a new edge.
func (r Rules) Resolve(
	scope rules.Scope, name string, kind directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf(refusalPrefix + "nothing to resolve")
	}
	switch kind {
	case directive.ResolveCallableInScope:
		return r.callable(scope, name, v)
	case directive.ResolvePackageVar:
		return inScope(scope, name, v, valueKinds)
	case directive.ResolveValueField:
		return r.member(scope, name, v, throughValue, fieldKinds)
	case directive.ResolveHostParam:
		return hostParam(scope, name, v)
	case directive.ResolveMemberOnHandle:
		return r.member(scope, name, v, throughHandle, memberKinds)
	case directive.ResolveTypeInScope:
		return typeInScope(scope, name, v)
	default:
		return nil, fmt.Errorf(refusalPrefix+"%s is not a resolution that TypeScript performs", kind)
	}
}

// callable resolves the name of a callable from a subject. For a method
// subject, a method of the method's type comes before a function in
// scope. For every other subject, only a function in scope resolves.
func (r Rules) callable(scope rules.Scope, name string, v rules.View) (symbol.Symbol, error) {
	sym, _ := v.Lookup(scope.Subject)
	method, isMethod := sym.(*node.Method)
	if !isMethod {
		return inScope(scope, name, v, functionKinds)
	}
	owner, _ := v.Lookup(method.Host)
	if host, isType := owner.(node.Declaration); isType {
		if sibling, found := memberNamed(rules.NewBound(r, v, nil), host, name, method.Host, methodKinds); found {
			return sibling, nil
		}
	}
	if fn, err := inScope(scope, name, v, functionKinds); err == nil {
		return fn, nil
	}
	return nil, fmt.Errorf(refusalPrefix+"%s has no method named %s, and no function named %s is in scope of %s",
		method.Host, name, name, scope.Subject)
}

// inScope resolves a name or a path to the first declaration of one of
// the kinds in the subject's scope, in the order of [Rules.Resolve].
func inScope(scope rules.Scope, name string, v rules.View, kinds []symbol.Kind) (symbol.Symbol, error) {
	if lit, scanned := scanLiteral(name); !scanned || lit.kind != literalPath || lit.text != name {
		return nil, fmt.Errorf(refusalPrefix+"%s is not a TypeScript name, so it cannot refer to a declaration", name)
	}
	if sym := (finder{v: v, kinds: kinds}).first(scope, name); sym != nil {
		return sym, nil
	}
	return nil, fmt.Errorf(refusalPrefix+"no %s named %s is in scope of %s", kindList(kinds), name, scope.Subject)
}

// typeInScope resolves a type name in scope. When no declaration in
// scope has the name, it returns a stand-in. The stand-in of a global
// type of TypeScript has the type's name. The stand-in of a type that an
// import binds from a module outside the view has the module, and the
// name that the module exports.
func typeInScope(scope rules.Scope, name string, v rules.View) (symbol.Symbol, error) {
	sym, err := inScope(scope, name, v, typeKinds)
	if err == nil {
		return sym, nil
	}
	if slices.Contains(globalTypes, name) || slices.Contains(keywordTypes, name) {
		return &node.Alias{
			ID:   symbol.Identity{Lang: typescript.Lang, Name: name, Kind: symbol.KindAlias},
			Name: name,
		}, nil
	}
	file := moduleFile(scope, v)
	head, inner, last := segments(name)
	spec, exported, bound := binding(file, head)
	if !bound || exported == "" && head == name {
		return nil, err
	}
	base, relative := moduleOf(spec, file.Path)
	for _, pkg := range [2]string{base, join(base, indexModule)} {
		_, inView := v.PackageOf(symbol.Identity{Lang: typescript.Lang, Package: pkg, Kind: symbol.KindPackage})
		if inView {
			return nil, err
		}
		if !relative {
			break
		}
	}
	id := symbol.Identity{Lang: typescript.Lang, Package: base, Name: exported, Kind: symbol.KindAlias}
	if head != name {
		id.Package, id.Name = join(join(base, exported), inner), last
	}
	return &node.Alias{ID: id, Name: id.Name}, nil
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
// contains the namespace's dotted name. Each name in it is one segment
// of the package's path, so the stamp gives the number of segments that
// the namespaces add to the package that contains them.
func namespaces(dst []string, pkg string, v rules.View) []string {
	dst = append(dst, pkg)
	if v.Facts == nil {
		return dst
	}
	key, registered := meta.Lookup[string](v.Facts.Registry(), typescript.NamespaceKey)
	if !registered {
		return dst
	}
	id := symbol.Identity{Lang: typescript.Lang, Package: pkg, Kind: symbol.KindPackage}
	dotted, stamped := rules.Fact(v, id, key)
	if !stamped {
		return dst
	}
	for range strings.Count(dotted, pathSep) + 1 {
		pkg = pkg[:max(strings.LastIndex(pkg, packageSep), 0)]
		dst = append(dst, pkg)
	}
	return dst
}

// moduleFile returns the File node of a scope's file in the package of
// the file's module path. That File node records the imports and the
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

// binding returns the module specifier and the exported name that an
// import of a file binds a local name to. A named import binds the
// exported name, and a default import binds default. A namespace import
// binds the module itself, without a name. binding reports false when no
// import binds the name, and for a nil file.
func binding(file *node.File, local string) (string, string, bool) {
	if file == nil {
		return "", "", false
	}
	for _, imp := range file.Imports {
		switch {
		case imp == nil:
		case imp.Alias == local:
			return imp.Path, "", true
		case imp.Default == local:
			return imp.Path, defaultExport, true
		default:
			for _, b := range imp.Names {
				if b != nil && (b.Alias == local || b.Alias == "" && b.Name == local) {
					return imp.Path, b.Name, true
				}
			}
		}
	}
	return "", "", false
}

// moduleOf returns the package of the module that a specifier refers to
// from a file, and reports whether the specifier is relative. A relative
// specifier refers to the module file of its path, and TypeScript tries
// the index of the path's directory next. Any other specifier refers to
// the ambient module of its text.
func moduleOf(specifier, from string) (string, bool) {
	if !strings.HasPrefix(specifier, currentDir) && !strings.HasPrefix(specifier, parentDir) &&
		specifier != currentDirName && specifier != parentDirName {
		return specifier, false
	}
	return typescript.ModulePath(path.Join(path.Dir(from), specifier)), true
}

// join appends a path to a package path. Below the global package, the
// result is the path alone, and for an empty path it is the package
// alone.
func join(pkg, segment string) string {
	switch {
	case pkg == "":
		return segment
	case segment == "":
		return pkg
	default:
		return pkg + packageSep + segment
	}
}

// finder looks up the declaration of one resolution through a view.
type finder struct {
	v     rules.View
	kinds []symbol.Kind // the declaration kinds that the resolution accepts
}

// first returns the first declaration of the finder's kinds that a name
// or a path refers to in a subject's scope, in the order of
// [Rules.Resolve]. It returns nil when no declaration matches. A bare
// name probes the chain of the subject's packages, then the module that
// an import binds the name from, then the global package. A path probes
// the module or the namespace that an import binds its first name to.
// Without such an import, a path probes the namespace path in each
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
	spec, exported, bound := binding(file, head)
	switch {
	case head == name:
		for _, pkg := range chain {
			if sym := f.find(pkg, name, 0); sym != nil {
				return sym
			}
		}
		if bound && exported != "" {
			if sym := f.inModule(spec, file.Path, "", exported, 0); sym != nil {
				return sym
			}
		}
	case bound:
		return f.inModule(spec, file.Path, join(exported, inner), last, 0)
	default:
		for _, pkg := range chain {
			if sym := f.find(join(pkg, join(head, inner)), last, 0); sym != nil {
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
	return f.find(join(head, inner), last, 0)
}

// inModule returns the declaration that the module of a specifier, or a
// namespace path inside it, publishes under a name. For a relative
// specifier, it searches the module file of the path and then the index
// of the directory. For every other specifier, it searches the ambient
// module of its text.
func (f finder) inModule(specifier, from, inner, name string, depth int) symbol.Symbol {
	base, relative := moduleOf(specifier, from)
	if sym := f.find(join(base, inner), name, depth); sym != nil || !relative {
		return sym
	}
	return f.find(join(join(base, indexModule), inner), name, depth)
}

// find returns the top-level declaration of a package that has a name
// and one of the finder's kinds. Without one, it returns the declaration
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

// published returns the declaration that one export statement of a
// module's file publishes under a name. A default export publishes a
// local name. An export clause publishes a name from its source module,
// or from the file's own scope. An export-star statement publishes every
// name of its module except default. A frontend records no nil export
// statement.
func (f finder) published(pkg string, file *node.File, e *node.Export, name string, depth int) symbol.Symbol {
	if e.Default != "" && name == defaultExport {
		if sym := f.local(pkg, file, e.Default, depth); sym != nil {
			return sym
		}
	}
	for _, b := range e.Names {
		if b == nil || b.Name == wildcardName || b.Alias != name && (b.Alias != "" || b.Name != name) {
			continue
		}
		if e.Path == "" {
			return f.local(pkg, file, b.Name, depth)
		}
		if sym := f.inModule(e.Path, file.Path, "", b.Name, depth+1); sym != nil {
			return sym
		}
	}
	if e.Wildcard && name != defaultExport {
		return f.inModule(e.Path, file.Path, "", name, depth+1)
	}
	return nil
}

// local returns the declaration that a name of a module's own scope
// refers to. That is the declaration that an import of the file binds
// the name to, or otherwise the module's own declaration.
func (f finder) local(pkg string, file *node.File, name string, depth int) symbol.Symbol {
	spec, exported, bound := binding(file, name)
	if !bound || exported == "" {
		return f.find(pkg, name, depth+1)
	}
	return f.inModule(spec, file.Path, "", exported, depth+1)
}

// member resolves a name among the effective members of the types that
// [Rules.holders] returns for a subject. The first type with a usable
// member of the name and one of the kinds decides.
func (r Rules) member(
	scope rules.Scope, name string, v rules.View, via through, kinds []symbol.Kind,
) (symbol.Symbol, error) {
	own, types, err := r.holders(scope.Subject, v, via)
	if err != nil {
		return nil, err
	}
	b := rules.NewBound(r, v, nil)
	for _, t := range types {
		if m, found := memberNamed(b, t, name, own, kinds); found {
			return m, nil
		}
	}
	searched := make([]string, 0, len(types))
	for _, t := range types {
		searched = append(searched, t.Identity().String())
	}
	return nil, fmt.Errorf(refusalPrefix+"%s has no %s member named %s that %s can use",
		strings.Join(searched, " or "), kindList(kinds), name, scope.Subject)
}

// holders returns the subject's own type, and the types whose members a
// member reference of the subject resolves against. For a type, that is
// the subject itself, and for a member, the type that the member belongs
// to. For a callable, it is the types of its value under via, in the
// order of [Rules.valueTypes]. A function has no own type. holders
// refuses a subject, or the type of a member, that the view does not
// declare.
func (r Rules) holders(
	subject symbol.Identity, v rules.View, via through,
) (symbol.Identity, []node.Declaration, error) {
	sym, _ := v.Lookup(subject)
	switch s := sym.(type) {
	case *node.Field:
		owner, _ := v.Lookup(s.Host)
		if decl, isType := owner.(node.Declaration); isType {
			return s.Host, []node.Declaration{decl}, nil
		}
	case *node.Struct, *node.Interface, *node.Enum:
		decl, _ := sym.(node.Declaration)
		return subject, []node.Declaration{decl}, nil
	case *node.Function:
		types, err := r.valueTypes(subject, s.Params, s.Returns, v, via)
		return symbol.Identity{}, types, err
	case *node.Method:
		types, err := r.valueTypes(subject, s.Params, s.Returns, v, via)
		return s.Host, types, err
	}
	return symbol.Identity{}, nil, fmt.Errorf(refusalPrefix+"%s does not belong to a type that the view declares",
		subject)
}

// valueTypes returns the declarations of a callable's value. The first is
// the type of its first value return. Through the value, the type of
// each input parameter follows in order. An optional counts as its type,
// and an alias as its target. valueTypes leaves out a type without a
// declaration in the view, such as a global type, because it has no
// members. It refuses a callable whose value has no declaration.
func (r Rules) valueTypes(
	subject symbol.Identity, params []*node.Param, returns []*node.Return, v rules.View, via through,
) ([]node.Declaration, error) {
	var refs []*node.TypeRef
	roles, _ := r.ReturnRoles(returns, v)
	for i, ret := range returns {
		if ret != nil && roles[i] == rules.ReturnValue {
			refs = append(refs, ret.Type)
			break
		}
	}
	if via == throughValue {
		for _, p := range params {
			if p != nil && r.ParamRole(p, v) == rules.ParamInput {
				refs = append(refs, p.Type)
			}
		}
	}
	var out []node.Declaration
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		t := unaliased(ref, v)
		if t.Form == symbol.FormOptional && len(t.Elems) == 1 {
			t = unaliased(t.Elems[0], v)
		}
		sym, _ := v.Lookup(t.Target)
		switch sym.(type) {
		case *node.Struct, *node.Interface, *node.Enum:
			decl, _ := sym.(node.Declaration)
			out = append(out, decl)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf(refusalPrefix+"%s has no value of a type that the view declares", subject)
	}
	return out, nil
}

// memberNamed returns the effective member of a type with a name and one
// of the kinds, when the subject can use it. own is the subject's own
// type. memberNamed reports false when the type has no such member. A
// public member is usable on every type. A protected member is usable on
// the subject's own type, and a private member only where the subject's
// own type declares it.
func memberNamed(
	b rules.Bound, host node.Declaration, name string, own symbol.Identity, kinds []symbol.Kind,
) (symbol.Symbol, bool) {
	set, _ := b.MembersOf(host)
	ownType := !own.IsZero() && host.Identity() == own
	for _, m := range set.Members {
		decl, _ := m.Symbol.(node.Declaration)
		if id := decl.Identity(); id.Name != name || !slices.Contains(kinds, id.Kind) {
			continue
		}
		// The kinds of a member resolution are a field and a method.
		var vis symbol.Visibility
		switch d := m.Symbol.(type) {
		case *node.Field:
			vis = d.Visibility
		case *node.Method:
			vis = d.Visibility
		}
		if vis == symbol.VisibilityPublic || vis == symbol.VisibilityUnknown ||
			ownType && (vis == symbol.VisibilityProtected || m.Owner == own) {
			return m.Symbol, true
		}
	}
	return nil, false
}

// hostParam resolves a parameter on the subject's own signature.
func hostParam(scope rules.Scope, name string, v rules.View) (symbol.Symbol, error) {
	sym, held := v.Lookup(scope.Subject)
	if !held {
		return nil, fmt.Errorf(refusalPrefix+"%s is outside the view", scope.Subject)
	}
	var params []*node.Param
	switch c := sym.(type) {
	case *node.Function:
		params = c.Params
	case *node.Method:
		params = c.Params
	default:
		return nil, fmt.Errorf(refusalPrefix+"%s is not a callable, so it has no parameter named %s",
			scope.Subject, name)
	}
	for _, p := range params {
		if p != nil && p.Name == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf(refusalPrefix+"%s has no parameter named %s", scope.Subject, name)
}

// kindList joins the names of a set of kinds for a refusal.
func kindList(kinds []symbol.Kind) string {
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, k.String())
	}
	return strings.Join(parts, " or ")
}
