// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/node"
)

// importRefusal describes an import in the refusal of a carrier on it.
const importRefusal = "an import"

// parentModule returns the slash path of the package above a module, and
// the package of the global scope above a module of one name.
func parentModule(module string) string {
	if dir := path.Dir(module); dir != "." {
		return dir
	}
	return ""
}

// futureImport records the features of a future statement in the stamp
// python.future, in statement order. A future statement changes how the
// file compiles and imports no module, so it is no Import of the File. It
// refuses its carriers, because no rule takes an import as its subject.
func (l *lowering) futureImport(stmt treesitter.Node) {
	l.refuse(stmt, importRefusal)
	for name := range stmt.Children(l.v.fieldName) {
		if name.Kind() == l.v.aliasedImport {
			name = name.Child(l.v.fieldName)
		}
		l.future = append(l.future, name.Compact())
	}
}

// importStatement lowers one import statement into the Imports of the File
// and the bindings of the scope. outermost is the statement that the
// comments of the import belong to, the lazy statement around a lazy
// import. Each module of an import statement is an Import of its own, with
// the slash path of the module as its Path and its alias. A from statement
// is one Import of its module, with a Binding for each name and its alias,
// or a wildcard. A relative module resolves against the package of the
// file. A lazy import adds each module that it names, as written, to the
// stamp python.lazy. The trailing comment of the statement is the comment
// of each Import, and the import refuses its carriers.
func (l *lowering) importStatement(stmt, outermost treesitter.Node, lazy bool) {
	carriers, comment := l.declParts(outermost, outermost)
	l.u.RefuseCarriers(UnaddressedCarrier, carriers, importRefusal)
	if stmt.Kind() == l.v.importStatement {
		for name := range stmt.Children(l.v.fieldName) {
			alias := ""
			if name.Kind() == l.v.aliasedImport {
				alias = name.Child(l.v.fieldAlias).Text()
				name = name.Child(l.v.fieldName)
			}
			module := strings.ReplaceAll(name.Compact(), python.ModuleSep, python.PackageSep)
			l.file.Imports = append(l.file.Imports, &node.Import{
				Pos: name.Pos(), Comment: comment, Path: module, Alias: alias,
			})
			l.bindModule(module, alias)
			if lazy {
				l.lazy = append(l.lazy, name.Compact())
			}
		}
		return
	}
	source := stmt.Child(l.v.fieldModuleName)
	module := l.moduleOf(source)
	imp := &node.Import{Pos: stmt.Pos(), Comment: comment, Path: module}
	l.scope.modules[module] = true
	if !stmt.FirstOf(l.v.wildcardImport).IsZero() {
		imp.Wildcard = true
		l.scope.stars = append(l.scope.stars, module)
	}
	for name := range stmt.Children(l.v.fieldName) {
		b := &node.Binding{Pos: name.Pos(), Name: name.Compact()}
		if name.Kind() == l.v.aliasedImport {
			b.Name, b.Alias = name.Child(l.v.fieldName).Compact(), name.Child(l.v.fieldAlias).Text()
		}
		local := b.Name
		if b.Alias != "" {
			local = b.Alias
		}
		imp.Names = append(imp.Names, b)
		l.scope.imports[local] = binding{module: module, name: b.Name}
	}
	l.file.Imports = append(l.file.Imports, imp)
	if lazy {
		l.lazy = append(l.lazy, source.Compact())
	}
}

// bindModule records the binding of an import of a module. An alias binds
// the module itself, and an import without an alias binds the first name of
// the module, as import a.b binds a. The module and each module above it
// are named by the import, so a dotted reference reads through them.
func (l *lowering) bindModule(module, alias string) {
	for m := module; m != "." && m != ""; m = path.Dir(m) {
		l.scope.modules[m] = true
	}
	if alias != "" {
		l.scope.imports[alias] = binding{module: module}
		return
	}
	head, _, _ := strings.Cut(module, python.PackageSep)
	l.scope.imports[head] = binding{module: head}
}

// moduleOf returns the slash path of the module of a from statement. A
// relative module resolves against the package of the file: one dot is the
// package of the file, which is the module itself for the module of a
// package directory, and each further dot is the package above. A module
// above the root of the workspace is the package of the global scope.
func (l *lowering) moduleOf(source treesitter.Node) string {
	if source.Kind() != l.v.relativeImport {
		return strings.ReplaceAll(source.Compact(), python.ModuleSep, python.PackageSep)
	}
	base := l.at.pkg
	if !l.at.init {
		base = parentModule(base)
	}
	for range len(source.FirstOf(l.v.importPrefix).Text()) - 1 {
		base = parentModule(base)
	}
	if rest := source.FirstOf(l.v.dottedName); !rest.IsZero() {
		return joinModule(base, strings.ReplaceAll(rest.Compact(), python.ModuleSep, python.PackageSep))
	}
	return base
}
