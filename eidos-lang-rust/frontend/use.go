// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// useEntry is one name a use tree binds: the path it names as segments,
// the local name it binds, _ for an import that binds none, whether it
// is a glob, which binds every name of its path, and where it is
// written.
type useEntry struct {
	path  []string
	local string
	glob  bool
	at    position.Pos
}

// importsOf groups a use declaration's names by the module each comes
// from, one Import per module in source order: a name's module is its
// path without its last segment, and a glob's is its whole path, each
// spelled as the first package path the module's scope gives it.
func importsOf(at position.Pos, entries []useEntry, sc *scope) []*node.Import {
	var out []*node.Import
	byPath := map[string]*node.Import{}
	for _, e := range entries {
		module := e.path
		if !e.glob {
			module = e.path[:len(e.path)-1]
		}
		p := sc.first(module)
		imp, met := byPath[p]
		if !met {
			imp = &node.Import{Pos: at, Path: p}
			byPath[p] = imp
			out = append(out, imp)
		}
		if e.glob {
			imp.Wildcard = true
			continue
		}
		b := &node.Binding{Pos: e.at, Name: e.path[len(e.path)-1]}
		if e.local != b.Name {
			b.Alias = e.local
		}
		imp.Names = append(imp.Names, b)
	}
	return out
}

// exportsOf returns the exports of a pub use declaration: one Export for
// each of its imports, with bindings of its own.
func exportsOf(imports []*node.Import) []*node.Export {
	out := make([]*node.Export, 0, len(imports))
	for _, imp := range imports {
		ex := &node.Export{Pos: imp.Pos, Path: imp.Path, Wildcard: imp.Wildcard}
		for _, b := range imp.Names {
			ex.Names = append(ex.Names, &node.Binding{Pos: b.Pos, Name: b.Name, Alias: b.Alias})
		}
		out = append(out, ex)
	}
	return out
}

// record reads a module's boundary before its items lower, because an
// item's types resolve through every binding of its module wherever the
// module states it: the child modules its mod items declare, the crates
// its extern crate declarations bind under another name, and the names
// its use declarations bind, into the module's scope. The use and
// extern crate declarations then become the File node's imports, in
// source order, and a pub use declaration its exports too. A
// declaration a cfg predicate keeps out binds nothing.
func (l *lowering) record(list treesitter.Node, c container) {
	var pending []treesitter.Node
	var entries [][]useEntry
	var attrs []treesitter.Node
	for item := range list.NamedChildren() {
		k := item.Kind()
		if k == l.v.attributeItem {
			attrs = append(attrs, item)
			continue
		}
		if l.comment(item) {
			continue
		}
		outer := attrs
		attrs = nil
		if k != l.v.modItem && k != l.v.useDeclaration && k != l.v.externCrateDeclaration ||
			l.attributes(outer).excluded != "" {
			continue
		}
		switch k {
		case l.v.modItem:
			c.scope.children[item.Child(l.v.fieldName).Text()] = true
			continue
		case l.v.externCrateDeclaration:
			if alias := item.Child(l.v.fieldAlias); !alias.IsZero() {
				c.scope.crates[alias.Text()] = item.Child(l.v.fieldName).Text()
			}
			entries = append(entries, nil)
		default:
			tree := l.useTree(item.Child(l.v.fieldArgument), nil)
			c.scope.bind(tree)
			entries = append(entries, tree)
		}
		pending = append(pending, item)
	}
	for i, item := range pending {
		if item.Kind() == l.v.externCrateDeclaration {
			c.file.Imports = append(c.file.Imports, &node.Import{
				Pos: item.Pos(), Path: item.Child(l.v.fieldName).Text(), Alias: item.Child(l.v.fieldAlias).Text(),
			})
			continue
		}
		imports := importsOf(item.Pos(), entries[i], c.scope)
		c.file.Imports = append(c.file.Imports, imports...)
		if vis, _ := l.visibility(item); vis == symbol.VisibilityPublic {
			c.file.Exports = append(c.file.Exports, exportsOf(imports)...)
		}
	}
}

// useTree flattens a use tree below a path prefix into the names it
// binds, in source order. A path binds its last segment, and self in a
// list binds the list's own path, as in use a::{self}. A glob binds
// every name of its path, and a glob of no path binds nothing.
func (l *lowering) useTree(n treesitter.Node, prefix []string) []useEntry {
	switch n.Kind() {
	case l.v.useList:
		var out []useEntry
		for _, child := range l.children(n) {
			out = append(out, l.useTree(child, prefix)...)
		}
		return out
	case l.v.scopedUseList:
		return l.useTree(n.Child(l.v.fieldList), l.extend(prefix, n.Child(l.v.fieldPath)))
	case l.v.useWildcard:
		target := prefix
		if inner := l.children(n); len(inner) > 0 {
			target = l.extend(prefix, inner[0])
		}
		if len(target) == 0 {
			return nil
		}
		return []useEntry{{path: target, glob: true, at: n.Pos()}}
	case l.v.useAsClause:
		return []useEntry{{
			path: l.extend(prefix, n.Child(l.v.fieldPath)), local: n.Child(l.v.fieldAlias).Text(), at: n.Pos(),
		}}
	default:
		target := l.extend(prefix, n)
		return []useEntry{{path: target, local: target[len(target)-1], at: n.Pos()}}
	}
}

// extend returns a prefix followed by a path node's segments, in a new
// slice. A lone self below a prefix names the prefix itself.
func (*lowering) extend(prefix []string, p treesitter.Node) []string {
	segments := strings.Split(p.Compact(), pathSeparator)
	if len(prefix) > 0 && len(segments) == 1 && segments[0] == keywordSelf {
		return slices.Clone(prefix)
	}
	return append(slices.Clone(prefix), segments...)
}
