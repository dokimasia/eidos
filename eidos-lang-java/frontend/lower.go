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
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings that the lowering reads: the separator of a dotted name,
// the file that documents a package, and the name of the explicit
// receiver.
const (
	nameSeparator   = "."
	packageInfoFile = "package-info.java"
	receiverName    = "this"
)

// modifierWords maps each keyword that the lowering reads to its bit.
var modifierWords = map[string]modifier{
	"public": modPublic, "protected": modProtected, "private": modPrivate, "static": modStatic,
	"final": modFinal, "abstract": modAbstract, "sealed": modSealed, "default": modDefault,
}

// modifier is one keyword that the modifiers of a declaration state, as a
// bit of a set.
type modifier uint16

// The modifier keywords that the lowering reads.
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

// modifiers is what the modifiers of a declaration state: the keywords,
// the annotations, each with its name as written and its arguments
// verbatim, and the markers of the brand among the annotations.
type modifiers struct {
	words       modifier
	annotations symbol.Annotations
	sugars      []plugin.Sugar
}

// has reports whether the modifiers state a keyword.
func (m modifiers) has(w modifier) bool { return m.words&w != 0 }

// visibility returns the visibility that the modifiers state, and def
// where they state none.
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

// lowering is the state of the lowering of one file: the unit, the
// vocabulary of the grammar, the parsed tree and the path of the file,
// the File node and the scope that the references of the file resolve
// through, and the comment walk of the file. The walk records the
// comments that a declaration took and the subtrees that the load left
// out, which the sweep passes over.
type lowering struct {
	u        *plugin.SourceUnit
	v        *vocabulary
	tree     *treesitter.Tree
	path     string
	file     *node.File
	scope    *scope
	comments treesitter.Comments
}

// lower lowers one parsed file into the builder of the unit. The file
// declares into the package that its package clause names, and into the
// unnamed package, with the empty path, where it has no package clause. A
// package-info.java file documents and annotates its package. A
// module-info.java file declares no type, and java.module stamps its File
// node with the name of the module. A compact source file, which has a
// method outside a type, declares the class that it implicitly declares,
// and the methods, the fields and the types of the file are the members of
// that class. java.generated stamps the File node of a file whose
// top-level type has the annotation of a foreign generator, also where the
// signature depth leaves the type out.
func (l *lowering) lower(root treesitter.Node) {
	clause := root.FirstOf(l.v.packageDeclaration)
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
	for child := range root.NamedChildren() {
		if l.typeDeclaration(child.Kind()) && l.stampGenerated(child) {
			break
		}
	}
	compact := !root.FirstOf(l.v.methodDeclaration).IsZero()
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
	l.comments.Sweep(root, func(n treesitter.Node) {
		l.u.RefuseCarriers(UnaddressedCarrier, l.u.Comment(n.Text(), n.Pos()).Carriers, unattached)
	})
	if problems := l.tree.Problems(); problems != nil {
		l.u.ReportSyntax(UnparsedFile, problems)
	}
}

// importDecl records one import declaration in the scope of the file and
// as an Import of the File node. A single-type import binds its simple
// name and imports the name from its package. An on-demand import imports
// every type of a package or every member type of a type. A static import
// imports from the type that declares the members, whose path is the path
// of the Import, and resolves a type name as a type import does.
func (l *lowering) importDecl(n treesitter.Node) {
	segments := strings.Split(l.nameIn(n), nameSeparator)
	imp := &node.Import{Pos: n.Pos()}
	switch {
	case !n.FirstOf(l.v.asterisk).IsZero():
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

// modifiersOf reads the modifiers of a declaration: the keywords and the
// annotations of its modifiers node, with the markers among the
// annotations. The text of an annotation or a comment is not a keyword,
// so only the keyword tokens set bits.
func (l *lowering) modifiersOf(n treesitter.Node) modifiers {
	var m modifiers
	mods := n.FirstOf(l.v.modifiers)
	for child := range mods.AllChildren() {
		m.words |= modifierWords[string(child.Bytes())]
	}
	m.annotations, m.sugars = l.annotationsIn(mods)
	return m
}

// annotationsIn returns the annotations among the children of a node,
// each with its name as written and its arguments verbatim, one for each
// argument in source order. It also returns the markers of the annotations
// whose name starts with the brand of the unit, which [lowering.marker]
// lifts.
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
		for arg := range child.Child(l.v.fieldArguments).Content() {
			a.Args = append(a.Args, arg.Text())
		}
		out = append(out, a)
		if head, _, _ := strings.Cut(name, nameSeparator); head == l.u.Brand() {
			sugars = append(sugars, l.marker(child, name))
		}
	}
	return out, sugars
}

// stampGenerated stamps java.generated on the File node where a top-level
// type n has an annotation that [lowering.generated] reports, and reports
// whether it stamped. The value of the stamp is the first such annotation
// as written, with its arguments.
func (l *lowering) stampGenerated(n treesitter.Node) bool {
	for child := range n.FirstOf(l.v.modifiers).NamedChildren() {
		if k := child.Kind(); k != l.v.annotation && k != l.v.markerAnnotation {
			continue
		}
		if l.generated(child.Child(l.v.fieldName).Compact()) {
			l.u.Graph().Stamp(l.file, meta.RawStamp{Key: java.GeneratedKey, Value: child.Text(), Pos: l.file.Pos})
			return true
		}
	}
	return false
}

// generated reports whether the name of an annotation refers to
// [java.GeneratedAnnotation] or [java.LegacyGeneratedAnnotation]. The name
// is the qualified name, a simple name that a single-type import binds to
// the annotation, or a simple name that an on-demand import of the package
// of the annotation imports. A single-type import of the simple name
// shadows every on-demand import. generated allocates only for the simple
// name of the two annotations.
func (l *lowering) generated(name string) bool {
	for _, qualified := range [...]string{java.GeneratedAnnotation, java.LegacyGeneratedAnnotation} {
		pkg, simple, _ := strings.CutLast(qualified, nameSeparator)
		if name == qualified {
			return true
		}
		if name != simple {
			continue
		}
		if segments, met := l.scope.single[name]; met {
			if strings.Join(segments, nameSeparator) == qualified {
				return true
			}
			continue
		}
		for _, imported := range l.scope.onDemand {
			if strings.Join(imported, nameSeparator) == pkg {
				return true
			}
		}
	}
	return false
}

// attach attaches the carriers of a declaration and the directives of its
// markers.
func (l *lowering) attach(subject symbol.Symbol, carriers []plugin.Carrier, sugars []plugin.Sugar) {
	l.u.AttachCarriers(subject, carriers, BadCarrier)
	for _, s := range sugars {
		l.u.AttachSugar(subject, s, BadMarker)
	}
}

// kept reports whether a load at the depth of the unit keeps a declaration
// of a visibility: every declaration at full depth, and a public or a
// protected one at signature depth.
func (l *lowering) kept(vis symbol.Visibility) bool {
	return l.u.Depth() == plugin.DepthFull || vis == symbol.VisibilityPublic || vis == symbol.VisibilityProtected
}

// nameIn returns the dotted name that a declaration states, as in a
// package clause or an import, without the whitespace between its tokens,
// and the empty string for a node that states none.
func (l *lowering) nameIn(n treesitter.Node) string {
	for child := range n.NamedChildren() {
		if k := child.Kind(); k == l.v.identifier || k == l.v.scopedIdentifier {
			return child.Compact()
		}
	}
	return ""
}

// slashed returns the segments of a dotted name as a slash path, which is
// the form of the identity of a Java package.
func slashed(segments []string) string {
	return strings.Join(segments, "/")
}
