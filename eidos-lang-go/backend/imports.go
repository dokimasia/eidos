// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"path"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/sdk/render"
)

// The import block's syntax.
const (
	importOpen  = "import (\n"
	importClose = ")\n"
	// importQuotes is the two quotes around a path.
	importQuotes = 2
	// quoteScratch is the stack buffer a path is quoted into. A path
	// longer than it quotes into memory of its own.
	quoteScratch = 256
)

// Imports renders the file's collected entries as one
// parenthesised block in two groups, standard library first, a
// blank line between, each group sorted — the shape goimports
// leaves. A named entry spells its name before the path where the
// name differs from the path's last element, which is how a bound
// name, a blank side-effect import and a dot import render. Two
// entries that render one line render it once, because Go refuses a
// package imported twice under one name. A file that qualified
// nothing renders no block at all, because an empty import block is
// not what gofmt leaves either.
//
// # Allocation contract
//
// Imports allocates the sorted entries and the block, which it sizes
// once: two allocations. A path that needs escapes grows the block once
// more, and a path whose quoting is longer than 256 bytes quotes into
// memory of its own.
func Imports(set *render.ImportSet) string {
	entries := set.Entries()
	if len(entries) == 0 {
		return ""
	}
	size := len(importOpen) + len(importClose) + len("\n")
	for _, e := range entries {
		size += len("\t") + len(shownName(e)) + len(" ") + len(e.Path) + importQuotes + len("\n")
	}
	var b strings.Builder
	b.Grow(size)
	b.WriteString(importOpen)
	if writeGroup(&b, entries, false) && hasExternal(entries) {
		b.WriteString("\n")
	}
	writeGroup(&b, entries, true)
	b.WriteString(importClose)
	return b.String()
}

// writeGroup writes the lines of one group, the external packages or
// the standard library, in entry order, each line once, and reports
// whether it wrote one. Entries of one path are adjacent, so a line
// is a repeat where an earlier entry of its path shows the same name.
func writeGroup(b *strings.Builder, entries []render.Entry, external bool) bool {
	var scratch [quoteScratch]byte
	wrote := false
	for i, e := range entries {
		if isExternal(e.Path) != external || repeats(entries, i) {
			continue
		}
		b.WriteString("\t")
		if name := shownName(e); name != "" {
			b.WriteString(name)
			b.WriteString(" ")
		}
		b.Write(strconv.AppendQuote(scratch[:0], e.Path))
		b.WriteString("\n")
		wrote = true
	}
	return wrote
}

// repeats reports whether an earlier entry of the same path shows the
// same name as entries[i], so both render one line.
func repeats(entries []render.Entry, i int) bool {
	for j := i - 1; j >= 0 && entries[j].Path == entries[i].Path; j-- {
		if shownName(entries[j]) == shownName(entries[i]) {
			return true
		}
	}
	return false
}

// shownName returns the name an entry spells before its path: its
// name where that differs from the path's last element, and nothing
// otherwise.
func shownName(e render.Entry) string {
	if e.Name == "" || e.Name == path.Base(e.Path) {
		return ""
	}
	return e.Name
}

// hasExternal reports whether any entry imports a package outside the
// standard library.
func hasExternal(entries []render.Entry) bool {
	for _, e := range entries {
		if isExternal(e.Path) {
			return true
		}
	}
	return false
}

// isExternal reports whether an import path names a package outside
// the standard library: its first element contains a dot, which a
// module path's host does and a standard library path does not.
func isExternal(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return strings.Contains(first, ".")
}
