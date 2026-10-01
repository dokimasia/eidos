// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// maxSyntaxFindings caps the findings one file's syntax errors report,
// and the count past the cap reports once.
const maxSyntaxFindings = 10

// maxQuoted caps how much of an ERROR node's source a finding quotes.
const maxQuoted = 32

// The keywords the lowering reads off a node's unnamed children.
const (
	keywordDefault = "default"
	keywordDeclare = "declare"
	keywordGlobal  = "global"
	keywordStar    = "*"
	keywordAssign  = "="
)

// namespaceSeparator joins the names of a dotted namespace, as in A.B.
const namespaceSeparator = "."

// wildcardName is the name a namespace re-export binds, as the
// specification's export entry spells the whole module.
const wildcardName = "*"

// lowering is one file's lowering state: the unit and the grammar's
// vocabulary, the file's path and module record, the File node the file
// contributes to each package, the comments a declaration took, and the
// local names the module publishes through an export clause or a
// default export.
type lowering struct {
	u        *plugin.SourceUnit
	v        *vocabulary
	tree     *treesitter.Tree
	path     string
	module   *module
	files    map[string]*node.File
	consumed map[position.Pos]bool
	exported map[string]bool
}

// container is where one statement list's declarations go: the File
// node of their package, the packages that enclose it innermost first,
// the dotted name of the namespace the package is, and whether every
// declaration in it is public, as in a script's global scope and an
// ambient context. module reports the module's own top level, whose
// declarations an export clause can publish by name.
type container struct {
	file   *node.File
	pkg    string
	outer  []string
	dotted string
	public bool
	module bool
}

// lower lowers one parsed file into the unit's builder: the module
// record, every declaration into its package, the carriers no
// declaration took, and the syntax errors.
func (l *lowering) lower(root treesitter.Node) {
	moduleFile := l.isModule(root)
	if !moduleFile {
		l.module.pkg = ""
	}
	top := container{pkg: l.module.pkg, public: !moduleFile, module: moduleFile}
	top.file = l.fileIn(top.pkg, nil, root.Pos())
	l.record(root, top.file)
	l.statements(root, top)
	l.sweep(root)
	l.reportSyntax()
}

// isModule reports whether a file is a module: it has a top-level
// import or export statement. Any other file is a script, which
// declares into the global scope.
func (l *lowering) isModule(root treesitter.Node) bool {
	for stmt := range root.NamedChildren() {
		if k := stmt.Kind(); k == l.v.importStatement || k == l.v.exportStatement {
			return true
		}
	}
	return false
}

// fileIn returns the File node the file contributes to a package,
// created on first use with the package's scope: the module record, the
// package, and the packages enclosing it.
func (l *lowering) fileIn(pkg string, outer []string, at position.Pos) *node.File {
	if f, met := l.files[pkg]; met {
		return f
	}
	gb := l.u.Graph()
	p := gb.Package(pkg)
	if pkg != "" {
		p.Name = path.Base(pkg)
	}
	f := &node.File{Path: l.path, Pos: at}
	p.Files = append(p.Files, f)
	gb.Scope(f, &scope{module: l.module, own: pkg, outer: outer})
	l.files[pkg] = f
	return f
}

// record reads the module boundary of the file's top level into the
// module record and the File node: the imports and what they bind, the
// import aliases, and the export statements that publish what the file
// does not declare or publish a local name.
func (l *lowering) record(root treesitter.Node, file *node.File) {
	for stmt := range root.NamedChildren() {
		switch stmt.Kind() {
		case l.v.importStatement:
			file.Imports = append(file.Imports, l.importStatement(stmt))
		case l.v.importAlias:
			l.importAlias(stmt)
		case l.v.exportStatement:
			if export := l.exportOf(stmt); export != nil {
				file.Exports = append(file.Exports, export)
			}
		}
	}
}

// importStatement lowers one import statement and binds its names in
// the module record: a named import binds its local name to the name
// the module exports, a default import to the module's default export,
// and a namespace import and an import-require to the module itself.
func (l *lowering) importStatement(stmt treesitter.Node) *node.Import {
	imp := &node.Import{Pos: stmt.Pos(), Path: l.unquote(stmt.Child(l.v.fieldSource))}
	for child := range stmt.NamedChildren() {
		switch child.Kind() {
		case l.v.importClause:
			l.importClause(child, imp)
		case l.v.importRequireClause:
			imp.Path = l.unquote(child.Child(l.v.fieldSource))
			if local := l.firstOf(child, l.v.identifier); !local.IsZero() {
				imp.Alias = local.Text()
				l.module.imports[imp.Alias] = binding{specifier: imp.Path}
			}
		}
	}
	return imp
}

// importClause reads one import clause's default, namespace and named
// bindings into an import record and the module record.
func (l *lowering) importClause(clause treesitter.Node, imp *node.Import) {
	for part := range clause.NamedChildren() {
		switch part.Kind() {
		case l.v.identifier:
			imp.Default = part.Text()
			l.module.imports[imp.Default] = binding{specifier: imp.Path, name: defaultExport}
		case l.v.namespaceImport:
			if local := l.firstOf(part, l.v.identifier); !local.IsZero() {
				imp.Alias = local.Text()
				l.module.imports[imp.Alias] = binding{specifier: imp.Path}
			}
		case l.v.namedImports:
			for spec := range part.NamedChildren() {
				if spec.Kind() != l.v.importSpecifier {
					continue
				}
				b := &node.Binding{Pos: spec.Pos(), Name: l.nameText(spec.Child(l.v.fieldName))}
				local := b.Name
				if alias := spec.Child(l.v.fieldAlias); !alias.IsZero() {
					b.Alias = alias.Text()
					local = b.Alias
				}
				imp.Names = append(imp.Names, b)
				l.module.imports[local] = binding{specifier: imp.Path, name: b.Name}
			}
		}
	}
}

// importAlias records an import alias, import x = A.B, as the dotted
// entity its local name refers to.
func (l *lowering) importAlias(stmt treesitter.Node) {
	var names []treesitter.Node
	for child := range stmt.NamedChildren() {
		if k := child.Kind(); k == l.v.identifier || k == l.v.nestedIdentifier {
			names = append(names, child)
		}
	}
	if len(names) == 2 {
		l.module.aliases[names[0].Text()] = names[1].Compact()
	}
}

// exportOf lowers one export statement's publication into an export
// record and the module record: an export clause, from a module or of
// the file's own names, a star and a namespace re-export, and the
// default export. It returns nil for an export statement that only
// exports the declaration it wraps.
func (l *lowering) exportOf(stmt treesitter.Node) *node.Export {
	spec := l.unquote(stmt.Child(l.v.fieldSource))
	export := &node.Export{Pos: stmt.Pos(), Path: spec}
	clause := l.firstOf(stmt, l.v.exportClause)
	namespace := l.firstOf(stmt, l.v.namespaceExport)
	declaration := stmt.Child(l.v.fieldDeclaration)
	value := stmt.Child(l.v.fieldValue)
	switch {
	case !clause.IsZero():
		for item := range clause.NamedChildren() {
			if item.Kind() != l.v.exportSpecifier {
				continue
			}
			b := &node.Binding{Pos: item.Pos(), Name: l.nameText(item.Child(l.v.fieldName))}
			published := b.Name
			if alias := item.Child(l.v.fieldAlias); !alias.IsZero() {
				b.Alias = l.nameText(alias)
				published = b.Alias
			}
			export.Names = append(export.Names, b)
			r := reexport{name: b.Name, published: published, specifier: spec}
			l.module.reexports = append(l.module.reexports, r)
			if spec == "" {
				l.exported[b.Name] = true
			}
		}
	case !namespace.IsZero():
		if local := l.firstOf(namespace, l.v.identifier); !local.IsZero() {
			b := &node.Binding{Pos: local.Pos(), Name: wildcardName, Alias: local.Text()}
			export.Names = append(export.Names, b)
		}
	case spec != "" && l.token(stmt, keywordStar):
		export.Wildcard = true
		l.module.stars = append(l.module.stars, spec)
	case l.token(stmt, keywordDefault):
		export.Default = l.defaultName(declaration, value)
		if export.Default == "" {
			return nil
		}
		l.module.defaultName = export.Default
		l.exported[export.Default] = true
	case l.token(stmt, keywordAssign):
		local := l.firstOf(stmt, l.v.identifier)
		if local.IsZero() {
			return nil
		}
		export.Default = local.Text()
		l.module.defaultName = export.Default
		l.exported[export.Default] = true
	default:
		return nil
	}
	return export
}

// defaultName returns the local name a default export publishes: the
// wrapped declaration's, default for an anonymous class or function,
// the identifier an export default names, and nothing for any other
// expression.
func (l *lowering) defaultName(declaration, value treesitter.Node) string {
	if !declaration.IsZero() {
		return l.nameText(declaration.Child(l.v.fieldName))
	}
	switch {
	case value.Kind() == l.v.identifier:
		return value.Text()
	case l.anonymousDefault(value):
		return defaultExport
	default:
		return ""
	}
}

// anonymousDefault reports whether an export default's value is an
// anonymous class or function, which declares under the name default.
func (l *lowering) anonymousDefault(value treesitter.Node) bool {
	if value.IsZero() || !value.Child(l.v.fieldName).IsZero() {
		return false
	}
	k := value.Kind()
	return k == l.v.class || k == l.v.functionExpression || k == l.v.generatorFunction
}

// statements lowers one statement list's declarations into a container.
// A function's implementation after its overload signatures is left
// out, because TypeScript hides it from callers.
func (l *lowering) statements(list treesitter.Node, c container) {
	overloaded := l.overloaded(list)
	for stmt := range list.NamedChildren() {
		switch stmt.Kind() {
		case l.v.comment, l.v.importStatement, l.v.importAlias:
		case l.v.exportStatement:
			l.exportStatement(stmt, c, overloaded)
		case l.v.ambientDeclaration:
			l.ambient(stmt, c, false, stmt, overloaded)
		case l.v.expressionStatement:
			if inner := l.firstOf(stmt, l.v.internalModule); !inner.IsZero() {
				l.namespace(inner, c, false, stmt, c.public)
			}
		case l.v.internalModule, l.v.module:
			l.namespace(stmt, c, false, stmt, c.public)
		default:
			l.declaration(stmt, c, false, stmt, overloaded)
		}
	}
}

// exportStatement lowers the declaration an export statement wraps, as
// exported: a named one, an ambient one, a namespace, or the anonymous
// class or function an export default declares under the name default.
func (l *lowering) exportStatement(stmt treesitter.Node, c container, overloaded map[string]bool) {
	declaration := stmt.Child(l.v.fieldDeclaration)
	switch {
	case declaration.Kind() == l.v.ambientDeclaration:
		l.ambient(declaration, c, true, stmt, overloaded)
	case declaration.Kind() == l.v.internalModule || declaration.Kind() == l.v.module:
		l.namespace(declaration, c, true, stmt, c.public)
	case !declaration.IsZero():
		l.declaration(declaration, c, true, stmt, overloaded)
	default:
		if value := stmt.Child(l.v.fieldValue); l.anonymousDefault(value) {
			l.declaration(value, c, true, stmt, overloaded)
		}
	}
}

// ambient lowers a declare statement: a declare global block into the
// global package, an ambient module or namespace, whose members are
// public, or one ambient declaration.
func (l *lowering) ambient(n treesitter.Node, c container, exported bool, outermost treesitter.Node,
	overloaded map[string]bool,
) {
	if l.token(n, keywordGlobal) {
		block := l.firstOf(n, l.v.statementBlock)
		global := container{pkg: "", outer: []string{c.pkg}, public: true}
		global.file = l.fileIn(global.pkg, global.outer, n.Pos())
		l.statements(block, global)
		return
	}
	for inner := range n.NamedChildren() {
		switch inner.Kind() {
		case l.v.comment:
		case l.v.internalModule, l.v.module:
			l.namespace(inner, c, exported, outermost, true)
			return
		default:
			l.declaration(inner, c, exported, outermost, overloaded)
			return
		}
	}
}

// namespace lowers a namespace or a module declaration into the package
// it is. A name in quotes declares an ambient module, the package of
// that name, whose members are public. A dotted name declares the
// namespace's package below the container's, stamped with the dotted
// name, whose members are public in an ambient context and exported by
// name otherwise. A namespace signature depth leaves out takes its
// comments and its members with it.
func (l *lowering) namespace(n treesitter.Node, c container, exported bool, outermost treesitter.Node,
	ambient bool,
) {
	name := n.Child(l.v.fieldName)
	var inner container
	if name.Kind() == l.v.stringNode {
		inner = container{pkg: l.unquote(name), outer: []string{c.pkg}, public: true}
	} else {
		if l.u.Depth() == plugin.DepthSignatures && !l.visible(c, exported, "") {
			l.skip(outermost)
			return
		}
		segments := strings.Split(name.Compact(), namespaceSeparator)
		inner = container{public: ambient, dotted: name.Compact()}
		if c.dotted != "" {
			inner.dotted = c.dotted + namespaceSeparator + inner.dotted
		}
		inner.outer = append([]string{c.pkg}, c.outer...)
		inner.pkg = c.pkg
		for i, segment := range segments {
			inner.pkg = join(inner.pkg, segment)
			if i < len(segments)-1 {
				inner.outer = append([]string{inner.pkg}, inner.outer...)
			}
		}
	}
	inner.file = l.fileIn(inner.pkg, inner.outer, n.Pos())
	gb := l.u.Graph()
	pkg := gb.Package(inner.pkg)
	if inner.dotted != "" {
		gb.Stamp(pkg, meta.RawStamp{Key: typescript.NamespaceKey, Value: inner.dotted, Pos: n.Pos()})
	}
	parts, _ := l.declParts(outermost)
	if len(pkg.Doc) == 0 {
		pkg.Doc = parts.Docs
	}
	l.u.AttachCarriers(pkg, parts.Carriers, BadCarrier)
	if body := n.Child(l.v.fieldBody); !body.IsZero() {
		l.statements(body, inner)
	}
}

// visible reports whether a declaration in a container is visible to
// another module: public in a public container, wrapped in an export
// statement, or published by an export clause or the default export at
// the module's top level. A declaration that is not visible loads at
// full depth only.
func (l *lowering) visible(c container, exported bool, name string) bool {
	return c.public || exported || c.module && name != "" && l.exported[name]
}

// visibility returns the visibility of a declaration in a container:
// public where another module can name it, and package otherwise,
// because its module or namespace alone can.
func (l *lowering) visibility(c container, exported bool, name string) symbol.Visibility {
	if l.visible(c, exported, name) {
		return symbol.VisibilityPublic
	}
	return symbol.VisibilityPackage
}

// overloaded returns the names a statement list declares overload
// signatures for, a function signature without a body, so the
// implementation that follows them is left out.
func (l *lowering) overloaded(list treesitter.Node) map[string]bool {
	out := map[string]bool{}
	for stmt := range list.NamedChildren() {
		decl := stmt
		if stmt.Kind() == l.v.exportStatement {
			decl = stmt.Child(l.v.fieldDeclaration)
		}
		if decl.Kind() == l.v.ambientDeclaration {
			decl = l.firstOf(decl, l.v.functionSignature)
		}
		if decl.Kind() == l.v.functionSignature {
			out[l.nameText(decl.Child(l.v.fieldName))] = true
		}
	}
	return out
}

// reportSyntax reports each ERROR and MISSING node of the tree at its
// own position, capped, and the remainder counted once.
func (l *lowering) reportSyntax() {
	n := 0
	for bad := range l.tree.Errors() {
		if n == maxSyntaxFindings {
			l.u.Errorf(UnparsedFile, bad.Pos(), "and more syntax errors")
			return
		}
		n++
		if bad.IsMissing() {
			l.u.Errorf(UnparsedFile, bad.Pos(), "%s is missing here", l.v.grammar.KindName(bad.Kind()))
			continue
		}
		l.u.Errorf(UnparsedFile, bad.Pos(), "%q does not parse", quoted(bad.Text()))
	}
}

// quoted returns the first line of a node's source, capped at
// maxQuoted bytes, for a finding to quote.
func quoted(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	if len(line) > maxQuoted {
		return line[:maxQuoted] + "..."
	}
	return line
}

// join appends a path to a package path: the path alone below the
// global package, and the package alone for an empty path.
func join(pkg, segment string) string {
	switch {
	case pkg == "":
		return segment
	case segment == "":
		return pkg
	default:
		return pkg + "/" + segment
	}
}

// token reports whether a node has an unnamed child spelled text: a
// keyword or a mark of the grammar.
func (*lowering) token(n treesitter.Node, text string) bool {
	for child := range n.AllChildren() {
		if !child.Named() && child.Text() == text {
			return true
		}
	}
	return false
}

// firstOf returns a node's first named child of a kind, and the zero
// Node where it has none.
func (*lowering) firstOf(n treesitter.Node, kind treesitter.Kind) treesitter.Node {
	for child := range n.NamedChildren() {
		if child.Kind() == kind {
			return child
		}
	}
	return treesitter.Node{}
}

// unquote returns a string literal's content, and nothing for the zero
// Node or an empty literal.
func (l *lowering) unquote(s treesitter.Node) string {
	if fragment := l.firstOf(s, l.v.stringFragment); !fragment.IsZero() {
		return fragment.Text()
	}
	return ""
}

// nameText returns a declared or published name: an identifier's text,
// and a string literal's content.
func (l *lowering) nameText(n treesitter.Node) string {
	if n.Kind() == l.v.stringNode {
		return l.unquote(n)
	}
	return n.Text()
}
