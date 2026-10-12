// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"

	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The Rust keywords that the lowering reads: the modifier of an async
// function, the modifier of a mutable binding, the name and the type of
// the receiver, the path segments of the crate root and of the parent of a
// module, and the name that binds nothing.
const (
	keywordAsync    = "async"
	keywordMut      = "mut"
	keywordSelf     = "self"
	keywordSelfType = "Self"
	keywordCrate    = "crate"
	keywordSuper    = "super"
	discardName     = "_"
)

// The visibilities that Rust writes as such: public everywhere, and public
// within the crate. Any other restriction is internal too, and the
// lowering stamps its spelling.
const (
	visPub      = "pub"
	visPubCrate = "pub(crate)"
)

// container is the target of the declarations of one item list: the
// package of the module, the File node of the file in that package, the
// scope through which its references resolve, the directory of its file
// modules, and whether it is an inline module block, whose path
// attributes are relative to that directory.
type container struct {
	pkg    string
	file   *node.File
	scope  *scope
	dir    string
	inline bool
}

// lowering is the state of the lowering of one file: the crate of the
// file, the vocabulary of the grammar, the parsed tree and the path of the
// file, the scope of the module whose items lower, the comment walk of the
// file, and the cfg predicates that kept an item of the file out. The
// walk records the comments that a declaration took and the items that
// the load left out, which the sweep passes over.
type lowering struct {
	w        *crate
	v        *vocabulary
	tree     *treesitter.Tree
	path     string
	scope    *scope
	comments treesitter.Comments
	excluded []string
}

// container creates the container of one module of the file: its File
// node and the scope through which the declarations of the module resolve.
func (l *lowering) container(pkg, dir string, inline bool, at position.Pos) container {
	sc := newScope(l.w.target.name, pkg)
	return container{pkg: pkg, file: l.w.fileNode(pkg, l.path, at, sc), scope: sc, dir: dir, inline: inline}
}

// header reads the inner attributes and the inner doc comments that open a
// module. It returns the inner cfg predicate outside the set of the load
// that keeps every item of the module out, and the empty string where the
// module loads. A module that loads takes the documentation and the
// carriers of its inner doc comments and #![doc] attributes onto its
// package, with the directives of its inner markers. Its inner attributes
// annotate its File node, and #![cfg(test)] stamps rust.test on the
// package.
func (l *lowering) header(list treesitter.Node, c container) string {
	var attrs, docs []treesitter.Node
	for child := range list.NamedChildren() {
		switch {
		case child.Kind() == l.v.innerAttributeItem:
			attrs = append(attrs, child)
		case l.comment(child) && !child.Child(l.v.fieldInner).IsZero():
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

// items lowers an item list into a container. It reads the module boundary
// that the use, extern crate and mod items state first, and then lowers
// each item with the outer attributes before it. The items of the list
// resolve their types through the scope of the container.
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

// item lowers one item. An item that a cfg predicate outside the set of
// the load keeps out declares nothing and takes its comments with it, and
// the predicate stamps the file. A use or an extern crate declaration
// lowers nothing here, because the lowering reads the module boundary
// before the items. A marker on such a declaration, or on an extern block,
// reports under [UnaddressedCarrier].
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
		l.w.u.RefuseSugar(UnaddressedCarrier, a.sugars, pathSeparator, "an extern block")
		l.items(n.Child(l.v.fieldBody), c)
	case l.v.useDeclaration, l.v.externCrateDeclaration:
		l.w.u.RefuseSugar(UnaddressedCarrier, a.sugars, pathSeparator, "a use or an extern crate declaration")
	default:
		l.refuse(n, a, "an item the model does not contain, such as a macro")
	}
}

// exclude records a module that a cfg predicate keeps out. Its file and
// every member below the directory of its file modules load nothing, so
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
// module of the container, and a file module by a walk of its file. The
// file of a file module is the path attribute relative to the directory of
// the declaring file, or relative to the directory of the inline module
// inside an inline block. Without a path attribute, the file is <name>.rs
// or <name>/mod.rs in the directory of the container. The documentation
// above the item documents the module, and its carriers and the directives
// of its markers attach to the module. A module loads at every depth,
// because a pub use can publish the pub items of a private module.
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
		l.comments.Take(body)
		return
	}
	l.items(body, inner)
}

// moduleFile returns the member that a file module names, and the empty
// string where the unit has no member at its path.
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
	for _, candidate := range []string{path.Join(c.dir, name+rust.Extension), path.Join(c.dir, name, modFile)} {
		if l.w.members[candidate] {
			return candidate
		}
	}
	return ""
}

// visibility returns the visibility of the modifier of an item, as
// [lowering.visibilityOf] reads it.
func (l *lowering) visibility(n treesitter.Node) (symbol.Visibility, string) {
	return l.visibilityOf(n.FirstOf(l.v.visibilityModifier))
}

// visibilityOf returns the visibility of a modifier: private for no
// modifier, public for pub, internal for pub(crate), and internal for
// every other restriction. For such a restriction it also returns the
// spelling for the rust.visibility stamp.
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

// kept reports whether a load at the depth of the unit keeps an item of a
// visibility: every item at full depth, and only a public one at signature
// depth.
func (l *lowering) kept(vis symbol.Visibility) bool {
	return l.w.u.Depth() == plugin.DepthFull || vis == symbol.VisibilityPublic
}

// declare appends a declaration to its container, attaches its carriers,
// and stamps the spelling of its restricted visibility and its test mark.
func (l *lowering) declare(c container, decl symbol.Symbol, parts plugin.CommentParts, spelled string, a attributes) {
	c.file.Decls = append(c.file.Decls, decl)
	l.w.mark(decl, parts, spelled, a)
}
