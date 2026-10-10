// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strconv"
	"strings"

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
// path binding no names renders as a side-effect import. A value
// import runs the module too, so it replaces the side-effect import
// of its path. TypeScript erases import type, so the side-effect
// import renders before it. Specifiers quote single, TypeScript's own
// canon.
//
// # Allocation contract
//
// Imports allocates the sorted entries and the block, which it sizes
// once: two allocations. A specifier that needs escapes grows the block
// once more. An empty set allocates nothing.
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
	var b strings.Builder
	b.Grow(size)
	writeImports(&b, entries, packageSpecifier)
	writeImports(&b, entries, relativeSpecifier)
	b.WriteString("\n")
	return b.String()
}

// writeImports writes the statements of every path of one group, in
// entry order.
func writeImports(b *strings.Builder, entries []render.Entry, kind specifierKind) {
	for rest := entries; len(rest) > 0; {
		group := rest[:pathRun(rest)]
		if kindOf(group[0].Path) == kind {
			importOf(group).write(b)
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
// binding runs it, and one named import of the distinct specifiers,
// import type where every binding is type-only.
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
		if e.Item != "" {
			n += len(e.Item) + len(renameKeyword)
		}
		n += len(e.Name)
		written++
	}
	return n
}

// write writes the statements of p into b.
func (p pathImport) write(b *strings.Builder) {
	path := p.entries[0].Path
	if p.bare && p.typeOnly {
		b.WriteString(importKeyword)
		writeQuoted(b, path)
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
		if e.Item != "" {
			b.WriteString(e.Item)
			b.WriteString(renameKeyword)
		}
		b.WriteString(e.Name)
		written++
	}
	b.WriteString(namedClose)
	writeQuoted(b, path)
	b.WriteString(statementEnd)
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

// The two Unicode line separators a string literal escapes, because
// an engine before ES2019 ends the literal at either.
const (
	lineSeparator      = 0x2028
	paragraphSeparator = 0x2029
)

// The hexadecimal escape a character without a named escape takes:
// its opener, and the zeros that pad its digits to four.
const (
	hexEscape = `\u`
	hexPad    = "0000"
)

// quote spells a string in single quotes, TypeScript's own canon,
// through [writeQuoted]. A specifier, a quoted member key and a string
// value all spell through it. It sizes its buffer for the string and
// its quotes, one allocation, and an escape grows it once more.
func quote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + specifierQuotes)
	writeQuoted(&b, s)
	return b.String()
}

// writeQuoted writes s into b in single quotes, in TypeScript's escape
// grammar: the backslash, the quote and the named escapes \b \f \n \r
// \t \v, every other control character and the two line separators as
// a four-digit hex escape, and everything else as itself. It allocates
// only where b grows.
func writeQuoted(b *strings.Builder, s string) {
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\v':
			b.WriteString(`\v`)
		default:
			if r < ' ' || r == 0x7f || r == lineSeparator || r == paragraphSeparator {
				var scratch [len(hexPad)]byte
				digits := strconv.AppendUint(scratch[:0], uint64(r), 16)
				b.WriteString(hexEscape)
				b.WriteString(hexPad[len(digits):])
				b.Write(digits)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
}
