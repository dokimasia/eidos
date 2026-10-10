// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/treesitter"
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

// The spellings the lowering reads: the separator of a dotted name, the
// file that documents a package, and the explicit receiver's name.
const (
	nameSeparator   = "."
	packageInfoFile = "package-info.java"
	receiverName    = "this"
)

// modifierWords maps each keyword the lowering reads to its bit.
var modifierWords = map[string]modifier{
	"public": modPublic, "protected": modProtected, "private": modPrivate, "static": modStatic,
	"final": modFinal, "abstract": modAbstract, "sealed": modSealed, "default": modDefault,
}

// modifier is one keyword a declaration's modifiers state, as a bit of a
// set.
type modifier uint16

// The modifier keywords the lowering reads.
const (
	modPublic    modifier = 1
	modProtected modifier = 2
	modPrivate   modifier = 4
	modStatic    modifier = 8
	modFinal     modifier = 16
	modAbstract  modifier = 32
	modSealed    modifier = 64
	modDefault   modifier = 128
)

// modifiers is what a declaration's modifiers state: the keywords, the
// annotations, each its name as written and its arguments verbatim, and
// the markers of the brand among the annotations.
type modifiers struct {
	words       modifier
	annotations symbol.Annotations
	sugars      []plugin.Sugar
}

// has reports whether the modifiers state a keyword.
func (m modifiers) has(w modifier) bool { return m.words&w != 0 }

// visibility returns the visibility the modifiers state, and def where
// they state none.
func (m modifiers) visibility(def symbol.Visibility) symbol.Visibility {
	switch {
	case m.has(modPublic):
		return symbol.VisibilityPublic
	case m.has(modProtected):
		return symbol.VisibilityProtected
	case m.has(modPrivate):
		return symbol.VisibilityPrivate
	default:
		return def
	}
}

// lowering is one file's lowering state: the unit, the grammar's
// vocabulary, the parsed tree and the file's path, the File node and the
// scope the file's references resolve through, and the comments a
// declaration took and the subtrees the load left out, which the sweep
// passes over.
type lowering struct {
	u     *plugin.SourceUnit
	v     *vocabulary
	tree  *treesitter.Tree
	path  string
	file  *node.File
	scope *scope
	taken map[position.Pos]bool
}

// lower lowers one parsed file into the unit's builder and returns the
// package it declares into: the one its package clause names, and the
// unnamed package, with the empty path, for a file without one. A
// package-info.java file documents and annotates its package. A
// module-info.java file declares no type, and java.module stamps its
// File node with the module's name. A compact source file, one with a
// method outside a type, declares the class it implicitly declares, and
// the file's methods, fields and types are that class's members.
func (l *lowering) lower(root treesitter.Node) *node.Package {
	clause := l.firstOf(root, l.v.packageDeclaration)
	pkgPath := ""
	if name := l.nameIn(clause); name != "" {
		pkgPath = strings.ReplaceAll(name, nameSeparator, "/")
	}
	gb := l.u.Graph()
	p := gb.Package(pkgPath)
	if pkgPath != "" {
		p.Name = path.Base(pkgPath)
	}
	l.file = &node.File{Path: l.path, Pos: root.Pos()}
	p.Files = append(p.Files, l.file)
	l.scope = newScope(pkgPath)
	gb.Scope(l.file, l.scope)
	if path.Base(l.path) == packageInfoFile && !clause.IsZero() {
		parts, _ := l.declParts(clause)
		p.Doc = append(p.Doc, parts.Docs...)
		annotations, sugars := l.annotationsIn(clause)
		l.attach(p, parts.Carriers, sugars)
		l.file.Annotations = append(l.file.Annotations, annotations...)
	}
	for child := range root.NamedChildren() {
		if child.Kind() == l.v.importDeclaration {
			l.importDecl(child)
		}
	}
	compact := !l.firstOf(root, l.v.methodDeclaration).IsZero()
	var implicit *host
	if compact && l.kept(symbol.VisibilityPackage) {
		implicit = l.implicitClass()
	}
	for child := range root.NamedChildren() {
		switch k := child.Kind(); {
		case k == l.v.packageDeclaration, k == l.v.importDeclaration, l.comment(child):
		case k == l.v.moduleDeclaration:
			gb.Stamp(l.file, meta.RawStamp{Key: java.ModuleKey, Value: l.nameIn(child), Pos: child.Pos()})
		case compact && implicit == nil:
			l.skip(child)
		case compact && l.member(child, implicit):
		case !compact && l.typeDeclaration(k):
			if decl := l.typeDecl(child, hostFile); decl != nil {
				l.file.Decls = append(l.file.Decls, decl)
			}
		default:
			l.refuse(child, "a statement outside a method")
		}
	}
	l.sweep(root)
	l.reportSyntax()
	return p
}

// importDecl records one import declaration in the file's scope and as
// an Import of the File node. A single-type import binds its simple name
// and imports the name from its package. An on-demand import imports
// every type of a package or every member type of a type. A static
// import imports from the type that declares the members, whose path is
// the Import's, and resolves a type name the way a type import does.
func (l *lowering) importDecl(n treesitter.Node) {
	segments := strings.Split(l.nameIn(n), nameSeparator)
	imp := &node.Import{Pos: n.Pos()}
	switch {
	case !l.firstOf(n, l.v.asterisk).IsZero():
		imp.Path, imp.Wildcard = slashed(segments), true
		l.scope.onDemand = append(l.scope.onDemand, segments)
	default:
		simple := segments[len(segments)-1]
		imp.Path = slashed(segments[:len(segments)-1])
		imp.Names = []*node.Binding{{Pos: n.Pos(), Name: simple}}
		if _, met := l.scope.single[simple]; !met {
			l.scope.single[simple] = segments
		}
	}
	l.file.Imports = append(l.file.Imports, imp)
}

// modifiersOf reads a declaration's modifiers: the keywords and the
// annotations of its modifiers node, with the markers among the
// annotations. An annotation's or a comment's text is not a keyword, so
// only the keyword tokens set bits.
func (l *lowering) modifiersOf(n treesitter.Node) modifiers {
	var m modifiers
	mods := l.firstOf(n, l.v.modifiers)
	for child := range mods.AllChildren() {
		m.words |= modifierWords[child.Text()]
	}
	m.annotations, m.sugars = l.annotationsIn(mods)
	return m
}

// annotationsIn returns the annotations among a node's children: each
// its name as written and its arguments verbatim, one per argument in
// source order. It also returns the markers of the annotations whose
// name starts with the unit's brand, which [lowering.marker] lifts.
func (l *lowering) annotationsIn(n treesitter.Node) (symbol.Annotations, []plugin.Sugar) {
	var (
		out    symbol.Annotations
		sugars []plugin.Sugar
	)
	for child := range n.NamedChildren() {
		if k := child.Kind(); k != l.v.annotation && k != l.v.markerAnnotation {
			continue
		}
		name := child.Child(l.v.fieldName).Compact()
		a := symbol.Annotation{Name: name}
		for _, arg := range l.children(child.Child(l.v.fieldArguments)) {
			a.Args = append(a.Args, arg.Text())
		}
		out = append(out, a)
		if head, _, _ := strings.Cut(name, nameSeparator); head == l.u.Brand() {
			sugars = append(sugars, l.marker(child, name))
		}
	}
	return out, sugars
}

// attach attaches a declaration's carriers and the directives of its
// markers.
func (l *lowering) attach(subject symbol.Symbol, carriers []plugin.Carrier, sugars []plugin.Sugar) {
	l.u.AttachCarriers(subject, carriers, BadCarrier)
	for _, s := range sugars {
		l.u.AttachSugar(subject, s, BadMarker)
	}
}

// kept reports whether a load at the unit's depth keeps a declaration of
// a visibility: every declaration at full depth, and a public or a
// protected one at signature depth.
func (l *lowering) kept(vis symbol.Visibility) bool {
	return l.u.Depth() == plugin.DepthFull || vis == symbol.VisibilityPublic || vis == symbol.VisibilityProtected
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

// nameIn returns the dotted name a declaration states, as in a package
// clause or an import, without the whitespace between its tokens, and
// empty for a node that states none.
func (l *lowering) nameIn(n treesitter.Node) string {
	for child := range n.NamedChildren() {
		if k := child.Kind(); k == l.v.identifier || k == l.v.scopedIdentifier {
			return child.Compact()
		}
	}
	return ""
}

// firstOf returns a node's first named child of a kind, and the zero
// Node where it has none.
func (*lowering) firstOf(n treesitter.Node, kind treesitter.Kind) treesitter.Node {
	return firstWhere(n, func(child treesitter.Node) bool { return child.Kind() == kind })
}

// children returns a node's named children that are not comments.
func (l *lowering) children(n treesitter.Node) []treesitter.Node {
	var out []treesitter.Node
	for child := range n.NamedChildren() {
		if !l.comment(child) {
			out = append(out, child)
		}
	}
	return out
}

// firstWhere returns a node's first named child that satisfies a
// predicate, and the zero Node where none does.
func firstWhere(n treesitter.Node, accept func(treesitter.Node) bool) treesitter.Node {
	for child := range n.NamedChildren() {
		if accept(child) {
			return child
		}
	}
	return treesitter.Node{}
}

// quoted returns the first line of a node's source, capped at maxQuoted
// bytes, for a finding to quote.
func quoted(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	if len(line) > maxQuoted {
		return line[:maxQuoted] + "..."
	}
	return line
}

// slashed returns a dotted name's segments as a slash path, the form a
// Java package identity takes.
func slashed(segments []string) string {
	return strings.Join(segments, "/")
}
