// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/treesitter"
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

// The Rust keywords the lowering reads: an async function's modifier, a
// mutable binding's, the receiver's name and its type, the path
// segments that name the crate root and a module's parent, and the name
// that binds nothing.
const (
	keywordAsync    = "async"
	keywordMut      = "mut"
	keywordSelf     = "self"
	keywordSelfType = "Self"
	keywordCrate    = "crate"
	keywordSuper    = "super"
	discardName     = "_"
)

// The visibilities Rust spells: public everywhere, and public within the
// crate. Any other restriction is internal too, and its spelling is
// stamped.
const (
	visPub      = "pub"
	visPubCrate = "pub(crate)"
)

// container is where one item list's declarations go: the package of
// the module, the File node the file contributes to it, the scope its
// references resolve through, the directory its file modules are in,
// and whether it is an inline module block, whose path attributes are
// relative to that directory.
type container struct {
	pkg    string
	file   *node.File
	scope  *scope
	dir    string
	inline bool
}

// lowering is one file's lowering state: the crate it is in, the
// grammar's vocabulary, the parsed tree and the file's path, the scope
// of the module whose items are lowering, the comments a declaration
// took and the items the load left out, which the sweep passes over,
// and the cfg predicates that kept an item of the file out.
type lowering struct {
	w        *crate
	v        *vocabulary
	tree     *treesitter.Tree
	path     string
	scope    *scope
	taken    map[position.Pos]bool
	excluded []string
}

// container creates the container of one module of the file: its File
// node and the scope the module's declarations resolve through.
func (l *lowering) container(pkg, dir string, inline bool, at position.Pos) container {
	sc := newScope(l.w.target.name, pkg)
	return container{pkg: pkg, file: l.w.fileNode(pkg, l.path, at, sc), scope: sc, dir: dir, inline: inline}
}

// header reads the inner attributes and inner doc comments that open a
// module, and returns the inner cfg predicate outside the load's set
// that keeps every item of the module out, empty where the module
// loads. A module that loads takes the documentation and the carriers
// of its inner doc comments and #![doc] attributes onto its package,
// with the directives of its inner markers, and its inner attributes as
// its File node's annotations, and #![cfg(test)] stamps the package
// rust.test.
func (l *lowering) header(list treesitter.Node, c container) string {
	var attrs, docs []treesitter.Node
	for child := range list.NamedChildren() {
		switch {
		case child.Kind() == l.v.innerAttributeItem:
			attrs = append(attrs, child)
		case l.comment(child) && l.innerDoc(child):
			docs = append(docs, child)
		}
	}
	a := l.attributes(attrs)
	if a.excluded != "" {
		l.excluded = append(l.excluded, a.excluded)
		return a.excluded
	}
	p := l.w.packageAt(c.pkg)
	parts := l.parts(docs)
	p.Doc = append(append(p.Doc, parts.Docs...), a.docs...)
	l.w.u.AttachCarriers(p, parts.Carriers, BadCarrier)
	for _, s := range a.sugars {
		l.w.u.AttachSugar(p, s, BadMarker)
	}
	c.file.Annotations = append(c.file.Annotations, a.annotations...)
	if a.test {
		l.w.stamp(p, rust.TestKey, true, c.file.Pos)
	}
	return ""
}

// items lowers an item list into a container: the module boundary its
// use, extern crate and mod items state first, and then each item with
// the outer attributes before it. The container's scope is the one an
// item's types resolve through while the list lowers.
func (l *lowering) items(list treesitter.Node, c container) {
	outer := l.scope
	l.scope = c.scope
	defer func() { l.scope = outer }()
	l.record(list, c)
	var attrs []treesitter.Node
	for item := range list.NamedChildren() {
		switch k := item.Kind(); {
		case k == l.v.attributeItem:
			attrs = append(attrs, item)
			continue
		case l.comment(item) || k == l.v.innerAttributeItem || k == l.v.shebang || k == l.v.emptyStatement:
			continue
		}
		l.item(item, c, attrs)
		attrs = nil
	}
}

// item lowers one item. An item a cfg predicate outside the load's set
// keeps out declares nothing and takes its comments with it, and the
// predicate stamps the file. A use or an extern crate declaration lowers
// nothing here, because the module boundary is read before the items,
// and a marker on one, or on an extern block, is refused.
func (l *lowering) item(n treesitter.Node, c container, attrs []treesitter.Node) {
	a := l.attributes(attrs)
	if a.excluded != "" {
		l.excluded = append(l.excluded, a.excluded)
		l.skip(n)
		if n.Kind() == l.v.modItem {
			l.exclude(n, c, a)
		}
		return
	}
	switch n.Kind() {
	case l.v.structItem, l.v.unionItem:
		l.structItem(n, c, a)
	case l.v.enumItem:
		l.enumItem(n, c, a)
	case l.v.traitItem:
		l.traitItem(n, c, a)
	case l.v.implItem:
		l.implItem(n, c, a)
	case l.v.functionItem, l.v.functionSignatureItem:
		l.function(n, c, a)
	case l.v.constItem:
		l.constant(n, c, a)
	case l.v.staticItem:
		l.static(n, c, a)
	case l.v.typeItem:
		l.typeAlias(n, c, a)
	case l.v.modItem:
		l.module(n, c, a)
	case l.v.foreignModItem:
		refuseMarkers(l.w.u, a.sugars, "an extern block")
		l.items(n.Child(l.v.fieldBody), c)
	case l.v.useDeclaration, l.v.externCrateDeclaration:
		refuseMarkers(l.w.u, a.sugars, "a use or an extern crate declaration")
	default:
		l.refuse(n, a, "an item the model does not contain, such as a macro")
	}
}

// exclude records a module a cfg predicate keeps out: its file and every
// member below the directory its file modules are in load nothing, so
// none of them loads as an unlinked file.
func (l *lowering) exclude(n treesitter.Node, c container, a attributes) {
	name := n.Child(l.v.fieldName).Text()
	e := exclusion{dir: path.Join(c.dir, name), pred: a.excluded}
	if n.Child(l.v.fieldBody).IsZero() {
		if e.file = l.moduleFile(c, name, a.path); e.file != "" {
			e.dir = moduleDir(e.file, l.w.root)
		}
	}
	l.w.exclusions = append(l.w.exclusions, e)
}

// module lowers a mod item: an inline block into the module below the
// container's, and a file module by walking the file it names. A file
// module's file is the path attribute's, relative to the declaring
// file's directory, or relative to the inline module's directory inside
// an inline block, and otherwise <name>.rs, or <name>/mod.rs, in the
// container's directory. The documentation above the item is the
// module's, and so are its carriers and the directives of its markers. A
// module loads at every depth, because a pub use can publish the pub
// items of a private module.
func (l *lowering) module(n treesitter.Node, c container, a attributes) {
	name := n.Child(l.v.fieldName).Text()
	pkg := join(c.pkg, name)
	parts, _ := l.declParts(n, a)
	p := l.w.packageAt(pkg)
	p.Doc = append(p.Doc, parts.Docs...)
	l.w.u.AttachCarriers(p, parts.Carriers, BadCarrier)
	for _, s := range a.sugars {
		l.w.u.AttachSugar(p, s, BadMarker)
	}
	if a.test {
		l.w.stamp(p, rust.TestKey, true, n.Pos())
	}
	body := n.Child(l.v.fieldBody)
	if body.IsZero() {
		if file := l.moduleFile(c, name, a.path); file != "" {
			l.w.walk(file, pkg)
		}
		return
	}
	inner := l.container(pkg, path.Join(c.dir, name), true, n.Pos())
	if pred := l.header(body, inner); pred != "" {
		l.w.exclusions = append(l.w.exclusions, exclusion{dir: inner.dir, pred: pred})
		l.taken[body.Pos()] = true
		return
	}
	l.items(body, inner)
}

// moduleFile returns the member a file module names, and empty where the
// unit has no member at its path.
func (l *lowering) moduleFile(c container, name, attr string) string {
	if attr != "" {
		base := path.Dir(l.path)
		if c.inline {
			base = c.dir
		}
		if p := path.Join(base, attr); l.w.members[p] {
			return p
		}
		return ""
	}
	for _, candidate := range []string{path.Join(c.dir, name+rustExtension), path.Join(c.dir, name, modFile)} {
		if l.w.members[candidate] {
			return candidate
		}
	}
	return ""
}

// visibility returns the visibility an item's modifier spells, as
// [lowering.visibilityOf] reads it.
func (l *lowering) visibility(n treesitter.Node) (symbol.Visibility, string) {
	return l.visibilityOf(l.firstOf(n, l.v.visibilityModifier))
}

// visibilityOf returns the visibility a modifier spells: private for no
// modifier, public for pub, internal for pub(crate), and internal for
// every other restriction, whose spelling it returns for the
// rust.visibility stamp.
func (*lowering) visibilityOf(vm treesitter.Node) (symbol.Visibility, string) {
	if vm.IsZero() {
		return symbol.VisibilityPrivate, ""
	}
	switch spelled := vm.Compact(); spelled {
	case visPub:
		return symbol.VisibilityPublic, ""
	case visPubCrate:
		return symbol.VisibilityInternal, ""
	default:
		return symbol.VisibilityInternal, spelled
	}
}

// kept reports whether a load at the unit's depth keeps an item of a
// visibility: every item at full depth, and only a public one at
// signature depth.
func (l *lowering) kept(vis symbol.Visibility) bool {
	return l.w.u.Depth() == plugin.DepthFull || vis == symbol.VisibilityPublic
}

// declare appends a declaration to its container, attaches its carriers,
// and stamps its restricted visibility's spelling and its test mark.
func (l *lowering) declare(c container, decl symbol.Symbol, parts plugin.CommentParts, spelled string, a attributes) {
	c.file.Decls = append(c.file.Decls, decl)
	l.w.mark(decl, parts, spelled, a)
}

// reportSyntax reports each ERROR and MISSING node of the tree at its
// own position, capped, and the remainder counted once.
func (l *lowering) reportSyntax() {
	n := 0
	for bad := range l.tree.Errors() {
		if n == maxSyntaxFindings {
			l.w.u.Errorf(UnparsedFile, bad.Pos(), "and more syntax errors")
			return
		}
		n++
		if bad.IsMissing() {
			l.w.u.Errorf(UnparsedFile, bad.Pos(), "%s is missing here", l.v.grammar.KindName(bad.Kind()))
			continue
		}
		l.w.u.Errorf(UnparsedFile, bad.Pos(), "%q does not parse", quoted(bad.Text()))
	}
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
