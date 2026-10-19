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
	statementEnd  = ";\n"
	// optionModifier is the modifier of an import that imports the custom
	// options of a file, which edition 2024 writes after every other
	// import.
	optionModifier = "option"
	// importQuotes is the two quotes around a path.
	importQuotes = 2
	// quoteScratch is the stack buffer a path is quoted into. A path
	// longer than it quotes into memory of its own.
	quoteScratch = 256
)

// Imports renders the import block of a file from its collected entries:
// one statement for each file, in the order of the paths, with the
// modifier that an entry of the file names, such as public. A file that the
// header imports with the modifier public or weak takes the modifier, also
// where a reference imports it without one. A regular import also imports
// the options of a file, so a file that the header imports with the
// modifier option and a reference imports without one takes a regular
// import. The option imports come after every other import, because
// edition 2024 requires them last. A file without an import has no block.
//
// # Allocation contract
//
// Imports allocates the sorted entries and the block, which it sizes once:
// two allocations. A path that needs escapes grows the block once more,
// and a path whose quoting is longer than 256 bytes quotes into memory of
// its own. An empty set allocates nothing.
func Imports(set *render.ImportSet) string {
	entries := set.Entries()
	if len(entries) == 0 {
		return ""
	}
	size := 0
	for _, e := range entries {
		size += len(importKeyword) + len(e.Name) + len(space) + len(e.Path) + importQuotes + len(statementEnd)
	}
	var b strings.Builder
	b.Grow(size)
	writeImports(&b, entries, false)
	writeImports(&b, entries, true)
	return b.String()
}

// TemplateImport returns the binding of an import that a template states
// through the use builtin. An import of protobuf names a file and binds no
// name, so TemplateImport reports false for every import, and the use
// builtin records the name that the template states as the modifier of the
// import. It does not allocate.
func TemplateImport(_, _ string) (local, item string, ok bool) {
	return "", "", false
}

// writeImports writes the statement of each path of entries whose modifier
// is option where options is set, and of every other path otherwise. The
// set sorts the entries of a path by name, so the entries of a path are
// adjacent, and the empty name sorts before option, which sorts before
// public and weak. The modifier of a path is therefore the name of its last
// entry, except that a path whose names are the empty name and option takes
// the regular import.
func writeImports(b *strings.Builder, entries []render.Entry, options bool) {
	var scratch [quoteScratch]byte
	first := 0
	for i, e := range entries {
		if i+1 < len(entries) && entries[i+1].Path == e.Path {
			continue
		}
		modifier := e.Name
		if modifier == optionModifier && entries[first].Name == "" {
			modifier = ""
		}
		first = i + 1
		if (modifier == optionModifier) != options {
			continue
		}
		b.WriteString(importKeyword)
		if modifier != "" {
			b.WriteString(modifier)
			b.WriteString(space)
		}
		b.Write(strconv.AppendQuote(scratch[:0], e.Path))
		b.WriteString(statementEnd)
	}
}
