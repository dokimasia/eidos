// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/emit"
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
	return block(set.Entries())
}

// block writes the statements of entries in their order, the option
// imports last, and the empty string for no entry. It sizes the block once.
func block(entries []render.Entry) string {
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

// ImportBlock renders the import block of the file whose header is h, as
// [Imports] does, with the files that the header imports first in the
// order of the header and every other file after them in the order of its
// path. The order of the imports of a file is the order of the dependencies
// of its descriptor, so a file keeps the order of its source. The file
// skeleton calls it after the declarations rendered, so the import set is
// complete.
//
// # Allocation contract
//
// ImportBlock allocates what [Imports] allocates, and the reordered
// entries and the order of the header where the header imports a file.
func (v *Vocabulary) ImportBlock(h *emit.File) string {
	entries := v.set.Entries()
	var order []string
	for _, a := range h.Annotations {
		if a.Name == string(protobuf.ImportMarker) && len(a.Args) == 1 {
			_, path := importOf(a.Args[0])
			if !slices.Contains(order, path) {
				order = append(order, path)
			}
		}
	}
	if len(order) > 0 {
		rank := func(path string) int {
			if i := slices.Index(order, path); i >= 0 {
				return i
			}
			return len(order)
		}
		entries = slices.Clone(entries)
		slices.SortStableFunc(entries, func(a, b render.Entry) int { return cmp.Compare(rank(a.Path), rank(b.Path)) })
	}
	return block(entries)
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
