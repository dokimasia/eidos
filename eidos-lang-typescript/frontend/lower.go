// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"bytes"
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The keywords that the lowering reads from the unnamed children of a
// node.
const (
	keywordDefault = "default"
	keywordDeclare = "declare"
	keywordGlobal  = "global"
	keywordStar    = "*"
	keywordAssign  = "="
)

// namespaceSeparator joins the names of a dotted namespace, as in A.B.
const namespaceSeparator = "."

// wildcardName is the name that a namespace re-export binds, which the
// export entry of the specification uses for the whole module.
const wildcardName = "*"

// lowering is the state of the lowering of one file. It has the unit, the
// vocabulary of the grammar, the path and the module record of the file,
// and the File node that the file contributes to each package. It also
// has the comment walk of the file, the local names that the module
// publishes through an export clause or a default export, and the number
// of inline object bodies around the member under lowering, whose members
// have no identity.
type lowering struct {
	u        *plugin.SourceUnit
	v        *vocabulary
	tree     *treesitter.Tree
	path     string
	module   *module
	files    map[string]*node.File
	comments treesitter.Comments
	exported map[string]bool
	inline   int
}

// container is where the declarations of one statement list go: the File
// node of their package, the packages around it innermost first, the
// dotted name of the namespace that the package is, and whether every
// declaration in it is public, as in the global scope of a script and in
// an ambient context. module reports the top level of the module, whose
// declarations an export clause can publish by name. ambient reports a
// context whose declarations are implemented elsewhere: a declaration
// file, a declare block or an ambient module.
type container struct {
	file    *node.File
	pkg     string
	outer   []string
	dotted  string
	public  bool
	module  bool
	ambient bool
}

// lower lowers one parsed file into the builder of the unit: the module
// record, every declaration into its package, the mark of a generated
// file, the carriers that no declaration took, and the syntax errors. The
// mark is the first comment of the file where the comment contains
// [typescript.GeneratedMark].
func (l *lowering) lower(root treesitter.Node) {
	moduleFile := l.isModule(root)
	if !moduleFile {
		l.module.pkg = ""
	}
	top := container{
		pkg:     l.module.pkg,
		public:  !moduleFile,
		module:  moduleFile,
		ambient: typescript.DeclarationFile(l.path),
	}
	top.file = l.fileIn(top.pkg, nil, root.Pos())
	if first := root.FirstOf(l.v.comment); first.PrevNamedSibling().IsZero() &&
		bytes.Contains(first.Bytes(), []byte(typescript.GeneratedMark)) {

		l.u.Graph().Stamp(top.file, meta.RawStamp{Key: typescript.GeneratedKey, Value: first.Text(), Pos: top.file.Pos})
	}
	l.record(root, top.file)
	l.statements(root, top)
	l.comments.Sweep(root, func(n treesitter.Node) {
		l.u.RefuseCarriers(UnaddressedCarrier, l.u.Comment(n.Text(), n.Pos()).Carriers, unattached)
	})
	if problems := l.tree.Problems(); problems != nil {
		l.u.ReportSyntax(UnparsedFile, problems)
	}
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

// fileIn returns the File node that the file contributes to a package, and
// creates it on first use with the scope of the package: the module
// record, the package, and the packages around it.
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

// record reads the module boundary of the top level of the file into the
// module record and the File node: the imports and what they bind, the
// import aliases, and the export statements that publish a name that the
// file does not declare or a local name.
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

// importStatement lowers one import statement and binds its names in the
// module record. A named import binds its local name to the name that the
// module exports, and a default import to the default export of the
// module. A namespace import and an import-require bind the module itself.
func (l *lowering) importStatement(stmt treesitter.Node) *node.Import {
	imp := &node.Import{Pos: stmt.Pos(), Path: l.unquote(stmt.Child(l.v.fieldSource))}
	for child := range stmt.NamedChildren() {
		switch child.Kind() {
		case l.v.importClause:
			l.importClause(child, imp)
		case l.v.importRequireClause:
			imp.Path = l.unquote(child.Child(l.v.fieldSource))
			if local := child.FirstOf(l.v.identifier); !local.IsZero() {
				imp.Alias = local.Text()
				l.module.imports[imp.Alias] = binding{specifier: imp.Path}
			}
		}
	}
	return imp
}

// importClause reads the default, the namespace and the named bindings of
// one import clause into an import record and the module record.
func (l *lowering) importClause(clause treesitter.Node, imp *node.Import) {
	for part := range clause.NamedChildren() {
		switch part.Kind() {
		case l.v.identifier:
			imp.Default = part.Text()
			l.module.imports[imp.Default] = binding{specifier: imp.Path, name: defaultExport}
		case l.v.namespaceImport:
			if local := part.FirstOf(l.v.identifier); !local.IsZero() {
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
// entity that its local name refers to.
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

// exportOf lowers the publication of one export statement into an export
// record and the module record: an export clause from a module or of the
// names of the file, a star and a namespace re-export, and the default
// export. It returns nil for an export statement that only exports the
// declaration that it wraps.
func (l *lowering) exportOf(stmt treesitter.Node) *node.Export {
	spec := l.unquote(stmt.Child(l.v.fieldSource))
	export := &node.Export{Pos: stmt.Pos(), Path: spec}
	clause := stmt.FirstOf(l.v.exportClause)
	namespace := stmt.FirstOf(l.v.namespaceExport)
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
		if local := namespace.FirstOf(l.v.identifier); !local.IsZero() {
			b := &node.Binding{Pos: local.Pos(), Name: wildcardName, Alias: local.Text()}
			export.Names = append(export.Names, b)
		}
	case spec != "" && stmt.HasToken(keywordStar):
		export.Wildcard = true
		l.module.stars = append(l.module.stars, spec)
	case stmt.HasToken(keywordDefault):
		export.Default = l.defaultName(declaration, value)
		if export.Default == "" {
			return nil
		}
		l.module.defaultName = export.Default
		l.exported[export.Default] = true
	case stmt.HasToken(keywordAssign):
		local := stmt.FirstOf(l.v.identifier)
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

// defaultName returns the local name that a default export publishes: the
// name of the wrapped declaration, default for an anonymous class or
// function, the identifier of an export default, and the empty string for
// any other expression.
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

// anonymousDefault reports whether the value of an export default is an
// anonymous class or function, which declares under the name default.
func (l *lowering) anonymousDefault(value treesitter.Node) bool {
	if value.IsZero() || !value.Child(l.v.fieldName).IsZero() {
		return false
	}
	k := value.Kind()
	return k == l.v.class || k == l.v.functionExpression || k == l.v.generatorFunction
}

// statements lowers the declarations of one statement list into a
// container. The lowering leaves out the implementation of a function
// after its overload signatures, because TypeScript hides it from callers.
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
			if inner := stmt.FirstOf(l.v.internalModule); !inner.IsZero() {
				l.namespace(inner, c, false, stmt, c.public)
			}
		case l.v.internalModule, l.v.module:
			l.namespace(stmt, c, false, stmt, c.public)
		default:
			l.declaration(stmt, c, false, stmt, overloaded)
		}
	}
}

// exportStatement lowers the declaration that an export statement wraps,
// as exported: a named one, an ambient one, a namespace, or the anonymous
// class or function that an export default declares under the name
// default.
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

// ambient lowers a declare statement in an ambient context: a declare
// global block into the global package, an ambient module or namespace,
// whose members are public, or one ambient declaration.
func (l *lowering) ambient(n treesitter.Node, c container, exported bool, outermost treesitter.Node,
	overloaded map[string]bool,
) {
	if n.HasToken(keywordGlobal) {
		block := n.FirstOf(l.v.statementBlock)
		global := container{pkg: "", outer: []string{c.pkg}, public: true, ambient: true}
		global.file = l.fileIn(global.pkg, global.outer, n.Pos())
		l.statements(block, global)
		return
	}
	declared := c
	declared.ambient = true
	for inner := range n.NamedChildren() {
		switch inner.Kind() {
		case l.v.comment:
		case l.v.internalModule, l.v.module:
			l.namespace(inner, declared, exported, outermost, true)
			return
		default:
			l.declaration(inner, declared, exported, outermost, overloaded)
			return
		}
	}
}

// namespace lowers a namespace or a module declaration into the package
// that it is. A name in quotes declares an ambient module, the package of
// that name, whose members are public and ambient. A dotted name declares
// the package of the namespace below the package of the container,
// stamped with the dotted name. Its members are public where public is
// set and exported by name otherwise, and ambient in an ambient context.
// A namespace that the signature depth leaves out takes its comments and
// its members with it.
func (l *lowering) namespace(n treesitter.Node, c container, exported bool, outermost treesitter.Node,
	public bool,
) {
	name := n.Child(l.v.fieldName)
	var inner container
	if name.Kind() == l.v.stringNode {
		inner = container{pkg: l.unquote(name), outer: []string{c.pkg}, public: true, ambient: true}
	} else {
		if l.u.Depth() == plugin.DepthSignatures && !l.visible(c, exported, "") {
			l.skip(outermost)
			return
		}
		segments := strings.Split(name.Compact(), namespaceSeparator)
		inner = container{public: public, ambient: c.ambient, dotted: name.Compact()}
		if c.dotted != "" {
			inner.dotted = c.dotted + namespaceSeparator + inner.dotted
		}
		inner.outer = append([]string{c.pkg}, c.outer...)
		inner.pkg = c.pkg
		for i, segment := range segments {
			inner.pkg = typescript.JoinPackage(inner.pkg, segment)
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

// visible reports whether another module can see a declaration of a
// container: the declaration is public in a public container, wrapped in
// an export statement, or published by an export clause or the default
// export at the top level of the module. A declaration that is not visible
// loads at full depth only.
func (l *lowering) visible(c container, exported bool, name string) bool {
	return c.public || exported || c.module && name != "" && l.exported[name]
}

// visibility returns the visibility of a declaration of a container:
// public where another module can name it, and package otherwise, because
// only its module or its namespace can.
func (l *lowering) visibility(c container, exported bool, name string) symbol.Visibility {
	if l.visible(c, exported, name) {
		return symbol.VisibilityPublic
	}
	return symbol.VisibilityPackage
}

// overloaded returns the names for which a statement list declares
// overload signatures, which are function signatures without a body, so
// the lowering leaves out the implementation after them.
func (l *lowering) overloaded(list treesitter.Node) map[string]bool {
	out := map[string]bool{}
	for decl := range list.NamedChildren() {
		if decl.Kind() == l.v.exportStatement {
			decl = decl.Child(l.v.fieldDeclaration)
		}
		if decl.Kind() == l.v.ambientDeclaration {
			decl = decl.FirstOf(l.v.functionSignature)
		}
		if decl.Kind() == l.v.functionSignature {
			out[l.nameText(decl.Child(l.v.fieldName))] = true
		}
	}
	return out
}

// unquote returns the content of a string literal, and the empty string
// for the zero Node or an empty literal. The content of a literal without
// an escape is its one string fragment.
func (l *lowering) unquote(s treesitter.Node) string {
	if fragment := s.FirstOf(l.v.stringFragment); !fragment.IsZero() {
		return fragment.Text()
	}
	return ""
}

// nameText returns a declared or a published name: the text of an
// identifier, or the content of a string literal.
func (l *lowering) nameText(n treesitter.Node) string {
	if n.Kind() == l.v.stringNode {
		return l.unquote(n)
	}
	return n.Text()
}
