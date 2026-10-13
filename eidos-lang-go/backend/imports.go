// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"path"
	"strconv"
	"strings"

	"golang.org/x/mod/module"

	golang "go.dokimi.dev/eidos/lang/go"
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

// The names of the Go imports that do not bind the package's name in
// the file.
const (
	blankImport = "_"
	dotImport   = "."
)

// The forms of the path element internal, which the go command reads in
// an import path: the element alone, at the start of a path, inside a
// path and at its end.
const (
	internalElem   = "internal"
	internalPrefix = internalElem + "/"
	internalSuffix = "/" + internalElem
	internalInfix  = internalSuffix + "/"
)

// bind imports the package path into set under the name that its package
// clause declares, and returns the name that qualifies a declaration of the
// package. [render.ImportSet.Declared] returns the declared name of a
// package that the run loaded. Any other package is imported under the
// name that [golang.Qualifier] returns. A path bound before returns its
// name again, and the package of the file returns the empty name, because
// a file never imports its own package.
//
// Before bind imports another package, it checks the path by the rules
// of the go command:
//
//   - The path is a valid import path, as [module.CheckImportPath]
//     checks it.
//   - Where the path has the element internal, the package of the file
//     is the parent of the last such element, or a package below it. The
//     suffix _test of an external test package belongs to the package
//     that it tests. The parent of a path that starts with internal is
//     the standard library, which contains no generated file.
//
// Error modes: a path that fails a check returns an error with the
// reason, and binds nothing.
//
// bind allocates what [render.ImportSet.Declared] and
// [render.ImportSet.Bind] allocate for a new binding, and nothing for a
// path that passes the checks. A refusal allocates its error.
func bind(set *render.ImportSet, path string) (string, error) {
	home := set.Home()
	if path == home {
		return "", nil
	}
	if err := module.CheckImportPath(path); err != nil {
		return "", refuse("the go command refuses the import: %v", err)
	}
	internal := true
	var parent string
	switch {
	case strings.HasSuffix(path, internalSuffix):
		parent = strings.TrimSuffix(path, internalSuffix)
	case strings.Contains(path, internalInfix):
		parent = path[:strings.LastIndex(path, internalInfix)]
	case path == internalElem || strings.HasPrefix(path, internalPrefix):
		return "", refuse("%s is internal to the standard library, and the file is outside it", path)
	default:
		internal = false
	}
	importer := strings.TrimSuffix(home, golang.TestSuffix)
	if internal && importer != parent && !strings.HasPrefix(importer, parent+"/") {
		return "", refuse("%s is internal to %s, and the package %s of the file is outside it", path, parent, home)
	}
	local := golang.Qualifier(path)
	if declared, known := set.Declared(path); known {
		local = declared
	}
	return set.Bind(path, local), nil
}

// TemplateImport returns the binding of an import that a template states
// through the use builtin. An import without a name binds the package
// under [golang.Qualifier] of its path, and a named import binds the
// package under its name. A blank import and a dot import do not bind
// the package's name, so TemplateImport reports false for both. It does
// not allocate.
func TemplateImport(importPath, name string) (local, item string, ok bool) {
	switch name {
	case "":
		return golang.Qualifier(importPath), "", true
	case blankImport, dotImport:
		return "", "", false
	default:
		return name, "", true
	}
}

// Imports renders the file's collected entries as one parenthesised
// block in two groups: the standard library first, then a blank line,
// then the other packages. Each group is sorted, as goimports leaves
// it. A named entry spells its name before the path where an import
// without a name binds another name, which is how a bound name, a blank
// side-effect import and a dot import render. An import without a name
// binds the name that the package declares. [render.ImportSet.Declared]
// returns that name for a package that the run loaded, and Imports takes
// the last element of the path as the name of any other package. Two
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
// memory of its own. The lookups of the declared names allocate what
// [render.ImportSet.Declared] allocates.
func Imports(set *render.ImportSet) string {
	entries := set.Entries()
	if len(entries) == 0 {
		return ""
	}
	size := len(importOpen) + len(importClose) + len("\n")
	for _, e := range entries {
		size += len("\t") + len(shownName(set, e)) + len(" ") + len(e.Path) + importQuotes + len("\n")
	}
	var b strings.Builder
	b.Grow(size)
	b.WriteString(importOpen)
	if writeGroup(&b, set, entries, false) && hasExternal(entries) {
		b.WriteString("\n")
	}
	writeGroup(&b, set, entries, true)
	b.WriteString(importClose)
	return b.String()
}

// writeGroup writes the lines of one group, the external packages or
// the standard library, in entry order, each line once, and reports
// whether it wrote one. Entries of one path are adjacent, so a line
// is a repeat where an earlier entry of its path shows the same name.
func writeGroup(b *strings.Builder, set *render.ImportSet, entries []render.Entry, external bool) bool {
	var scratch [quoteScratch]byte
	wrote := false
	for i, e := range entries {
		if golang.Standard(e.Path) == external || repeats(set, entries, i) {
			continue
		}
		b.WriteString("\t")
		if name := shownName(set, e); name != "" {
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
func repeats(set *render.ImportSet, entries []render.Entry, i int) bool {
	for j := i - 1; j >= 0 && entries[j].Path == entries[i].Path; j-- {
		if shownName(set, entries[j]) == shownName(set, entries[i]) {
			return true
		}
	}
	return false
}

// shownName returns the name an entry spells before its path: its name
// where an import of the path without a name binds another name, and
// nothing otherwise. Such an import binds the name that set returns for
// the path, and the path's last element where set has no name for it.
func shownName(set *render.ImportSet, e render.Entry) string {
	if e.Name == "" || e.Name != path.Base(e.Path) {
		return e.Name
	}
	if declared, known := set.Declared(e.Path); known && declared != e.Name {
		return e.Name
	}
	return ""
}

// hasExternal reports whether any entry imports a package outside the
// standard library.
func hasExternal(entries []render.Entry) bool {
	for _, e := range entries {
		if !golang.Standard(e.Path) {
			return true
		}
	}
	return false
}
