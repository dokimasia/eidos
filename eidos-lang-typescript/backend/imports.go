// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/render"
)

// The syntax of an import statement.
const (
	importKeyword = "import "
	typeKeyword   = "type "
	namedOpen     = "{ "
	namedClose    = " } from "
	specifierSep  = ", "
	statementEnd  = ";\n"
	// renameKeyword introduces the name an imported declaration binds
	// under.
	renameKeyword = " as "
	// relativeMark opens a relative specifier.
	relativeMark = "."
	// specifierQuotes is the two quotes around a specifier.
	specifierQuotes = 2
	// specifierScratch is the size of the buffer on the stack that a
	// specifier is quoted into. A longer literal allocates.
	specifierScratch = 256
)

// specifierKind is the group of the block a path's statements belong
// to: package specifiers render before relative ones.
type specifierKind uint8

// The two groups, in the order the block writes them.
const (
	packageSpecifier specifierKind = iota
	relativeSpecifier
)

// Imports renders the file's collected entries: package specifiers
// first, relative ones after, each group sorted, which is the
// order the ecosystem's formatters leave. The names bound under a
// path render as one named import, a renamed declaration as its
// exported name behind the name it binds under, and each specifier
// once. The import is import type when every binding is type-only,
// because one value binding makes the whole import a value import. A
// value import writes type before each specifier whose bindings are all
// type-only, as in import { type Arms, answer }, because tsc refuses a
// type among the plain specifiers of a value import under
// verbatimModuleSyntax. A path binding no names renders as a side-effect
// import. A value
// import runs the module too, so it replaces the side-effect import
// of its path. TypeScript erases import type, so the side-effect
// import renders before it. Each specifier is a string in the quote that
// the policy of the set selects, as [typescript.AppendQuote] writes it.
//
// # Allocation contract
//
// Imports allocates the sorted entries and the block, which it sizes
// once: two allocations. A specifier that needs escapes grows the block
// once more, and a specifier of more than 254 bytes allocates its literal.
// An empty set allocates nothing.
func Imports(set *render.ImportSet) string {
	entries := set.Entries()
	if len(entries) == 0 {
		return ""
	}
	size := len("\n")
	for rest := entries; len(rest) > 0; {
		group := rest[:pathRun(rest)]
		size += importOf(group).size()
		rest = rest[len(group):]
	}
	style, _ := set.Policy().Lookup(typescript.QuoteStyle)
	var b strings.Builder
	b.Grow(size)
	writeImports(&b, entries, packageSpecifier, style)
	writeImports(&b, entries, relativeSpecifier, style)
	b.WriteString("\n")
	return b.String()
}

// TemplateImport returns the binding of an import that a template states
// through the use builtin. A named import binds the declaration of that
// name from the module. An import without a name runs the module for its
// effects and does not bind a name, so TemplateImport reports false for
// it. It does not allocate.
func TemplateImport(_, name string) (local, item string, ok bool) {
	if name == "" {
		return "", "", false
	}
	return name, name, true
}

// writeImports writes the statements of every path of one group, in
// entry order, with each specifier in the quote that style selects.
func writeImports(b *strings.Builder, entries []render.Entry, kind specifierKind, style plugin.Choice) {
	for rest := entries; len(rest) > 0; {
		group := rest[:pathRun(rest)]
		if kindOf(group[0].Path) == kind {
			importOf(group).write(b, style)
		}
		rest = rest[len(group):]
	}
}

// kindOf returns the group a specifier belongs to.
func kindOf(path string) specifierKind {
	if strings.HasPrefix(path, relativeMark) {
		return relativeSpecifier
	}
	return packageSpecifier
}

// pathRun returns how many entries from the first share its path. The
// import set sorts its entries by path, so the entries of a path are
// adjacent. The entries are not empty.
func pathRun(entries []render.Entry) int {
	n := 1
	for n < len(entries) && entries[n].Path == entries[0].Path {
		n++
	}
	return n
}

// pathImport is what the entries of one path render: a side-effect
// import where a bare entry asks for the module's run and no value
// binding runs it, and one named import of the distinct specifiers. The
// named import is import type where every binding is type-only, and
// marks each type-only specifier of a value import with type otherwise.
type pathImport struct {
	// entries are the path's entries, all of one path.
	entries []render.Entry
	// bare is set where an entry binds no name.
	bare bool
	// named is set where an entry binds a name.
	named bool
	// typeOnly is set where every binding is type-only, which a path
	// without bindings is.
	typeOnly bool
}

// importOf returns what the entries of one path render. It allocates
// nothing.
func importOf(group []render.Entry) pathImport {
	p := pathImport{entries: group, typeOnly: true}
	for _, e := range group {
		if e.Name == "" {
			p.bare = true
			continue
		}
		p.named = true
		p.typeOnly = p.typeOnly && e.TypeOnly
	}
	return p
}

// size returns the length of the statements p writes, its specifier
// counted without escapes.
func (p pathImport) size() int {
	quoted := len(p.entries[0].Path) + specifierQuotes
	n := 0
	if p.bare && p.typeOnly {
		n += len(importKeyword) + quoted + len(statementEnd)
	}
	if !p.named {
		return n
	}
	n += len(importKeyword) + len(namedOpen) + len(namedClose) + quoted + len(statementEnd)
	if p.typeOnly {
		n += len(typeKeyword)
	}
	written := 0
	for i, e := range p.entries {
		if !p.specifies(i) {
			continue
		}
		if written > 0 {
			n += len(specifierSep)
		}
		if !p.typeOnly && p.typeOnlyAt(i) {
			n += len(typeKeyword)
		}
		if e.Item != "" {
			n += len(e.Item) + len(renameKeyword)
		}
		n += len(e.Name)
		written++
	}
	return n
}

// write writes the statements of p into b, with the specifier in the
// quote that style selects.
func (p pathImport) write(b *strings.Builder, style plugin.Choice) {
	var scratch [specifierScratch]byte
	quoted := typescript.AppendQuote(scratch[:0], p.entries[0].Path, style)
	if p.bare && p.typeOnly {
		b.WriteString(importKeyword)
		b.Write(quoted)
		b.WriteString(statementEnd)
	}
	if !p.named {
		return
	}
	b.WriteString(importKeyword)
	if p.typeOnly {
		b.WriteString(typeKeyword)
	}
	b.WriteString(namedOpen)
	written := 0
	for i, e := range p.entries {
		if !p.specifies(i) {
			continue
		}
		if written > 0 {
			b.WriteString(specifierSep)
		}
		if !p.typeOnly && p.typeOnlyAt(i) {
			b.WriteString(typeKeyword)
		}
		if e.Item != "" {
			b.WriteString(e.Item)
			b.WriteString(renameKeyword)
		}
		b.WriteString(e.Name)
		written++
	}
	b.WriteString(namedClose)
	b.Write(quoted)
	b.WriteString(statementEnd)
}

// typeOnlyAt reports whether every entry of the specifier at i binds its
// name for the type checker alone. The entries of one specifier are
// adjacent from i on, as [pathImport.specifies] states. It allocates
// nothing.
func (p pathImport) typeOnlyAt(i int) bool {
	e := p.entries[i]
	for _, other := range p.entries[i:] {
		if other.Name != e.Name || other.Item != e.Item {
			break
		}
		if !other.TypeOnly {
			return false
		}
	}
	return true
}

// specifies reports whether the i-th entry renders a specifier: it
// binds a name, and the entry before it binds another name or item. A
// value and a type-only binding of one declaration render one
// specifier, and the set sorts a path's entries by name and item, so
// the two are adjacent.
func (p pathImport) specifies(i int) bool {
	e := p.entries[i]
	if e.Name == "" {
		return false
	}
	return i == 0 || p.entries[i-1].Name != e.Name || p.entries[i-1].Item != e.Item
}
