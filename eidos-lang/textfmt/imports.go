// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt

import (
	"strings"

	"go.dokimi.dev/eidos/sdk/render"
)

// renameKeyword introduces the name an imported item binds under.
const renameKeyword = " as "

// pathSeparator separates the segments of an import path as the
// import set records it.
const pathSeparator = "/"

// statementScratch is the size of the stack buffer a statement is
// spelled into before it is written. A longer statement allocates its
// spelling.
const statementScratch = 256

// ImportLines renders a file's collected imports as one statement per
// distinct entry, in the order given. A statement is the keyword, the
// path with its slashes spelled as sep, and the bound name behind
// another sep where the entry names one, closed by a semicolon. An
// entry that renames the item it imports spells the item behind the
// path and the name behind as. A blank line follows the block. An
// empty list of entries renders an empty string. Java renders its
// imports through it with "import" and ".", and Rust its uses with
// "use" and "::".
//
// # Allocation contract
//
// ImportLines writes into one buffer sized to the block, one
// allocation. It spells each statement into a stack buffer and checks
// the spelling against the statements already written, so a statement
// allocates nothing unless it is longer than 256 bytes. The set of
// written statements is on the stack for up to eight entries and takes
// three allocations above that. An empty list of entries allocates
// nothing.
func ImportLines(entries []render.Entry, keyword, sep string) string {
	if len(entries) == 0 {
		return ""
	}
	n := 1
	for _, e := range entries {
		n += len(keyword) + 1 + statementLen(e, sep) + 2
	}
	var b strings.Builder
	b.Grow(n)
	written := make(map[string]struct{}, len(entries))
	var scratch [statementScratch]byte
	for _, e := range entries {
		stmt := appendStatement(scratch[:0], e, sep)
		if _, repeated := written[string(stmt)]; repeated {
			continue
		}
		b.WriteString(keyword)
		b.WriteByte(' ')
		start := b.Len()
		b.Write(stmt)
		written[b.String()[start:]] = struct{}{}
		b.WriteString(";\n")
	}
	b.WriteString("\n")
	return b.String()
}

// statementLen returns the length of the statement [appendStatement]
// spells for e, keyword and semicolon excluded.
func statementLen(e render.Entry, sep string) int {
	n := len(e.Path) + strings.Count(e.Path, pathSeparator)*(len(sep)-len(pathSeparator))
	switch {
	case e.Item != "":
		n += len(sep) + len(e.Item) + len(renameKeyword) + len(e.Name)
	case e.Name != "":
		n += len(sep) + len(e.Name)
	}
	return n
}

// appendStatement appends the statement e spells to dst and returns
// the extended slice: the path with each separator spelled as sep,
// then sep and the item renamed behind as where e renames one, or sep
// and the bound name where e names one. It allocates only where dst
// lacks the capacity.
func appendStatement(dst []byte, e render.Entry, sep string) []byte {
	for path := e.Path; ; {
		segment, rest, more := strings.Cut(path, pathSeparator)
		dst = append(dst, segment...)
		if !more {
			break
		}
		dst = append(dst, sep...)
		path = rest
	}
	switch {
	case e.Item != "":
		dst = append(dst, sep...)
		dst = append(dst, e.Item...)
		dst = append(dst, renameKeyword...)
		dst = append(dst, e.Name...)
	case e.Name != "":
		dst = append(dst, sep...)
		dst = append(dst, e.Name...)
	}
	return dst
}
