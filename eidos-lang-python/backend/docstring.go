// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strings"
	"unicode"
)

// The parts of a docstring: its triple quotes, the prefix of a raw string,
// the escaped quotes that a docstring writes for a triple quote of its text,
// and the backslash and its escape.
const (
	docQuotes   = `"""`
	rawPrefix   = "r"
	docEscaped  = `\"\"\"`
	backslash   = `\`
	doubleSlash = `\\`
	newline     = "\n"
)

// tabWidth is the number of columns to which a tab in the indentation of a
// line of a docstring advances, as Python's str.expandtabs counts them.
const tabWidth = 8

// Docstring writes the documentation lines of a declaration as a docstring
// at the indentation indent, in the form that ruff's formatter writes:
//
//   - a text of one line is one line between triple quotes. A text of more
//     lines has its first line after the opening quotes and the closing
//     quotes on a line of their own;
//   - the first line loses its leading and trailing whitespace, and every
//     other line loses its trailing whitespace and the indentation that the
//     lines that are not blank share, so the text keeps the indentation of a
//     line relative to the others. Each such line starts at the indentation,
//     a blank line has no indentation, and the blank lines at the end of the
//     text go;
//   - a text with a backslash is a raw string, so the backslash stays one
//     character, unless the text contains a triple quote or ends with a
//     backslash, which a raw string cannot write. Such a text escapes the
//     backslash and the quotes instead;
//   - a space separates the opening quotes from a quote at the start of the
//     text, and the closing quotes of a text of one line from a quote at its
//     end.
//
// Docstring writes nothing for no lines, and for lines that are all blank.
//
// # Allocation contract
//
// Docstring allocates the docstring, and the escaped text of each line that
// needs escapes.
func Docstring(lines []string, indent string) string {
	var b strings.Builder
	b.Grow(docstringSize(lines, indent))
	writeDocstring(&b, lines, indent)
	return b.String()
}

// docstringSize returns the length of the docstring of lines at the
// indentation indent without escapes, which bounds the docstring of lines
// that need none.
func docstringSize(lines []string, indent string) int {
	if len(lines) == 0 {
		return 0
	}
	size := len(indent) + len(rawPrefix) + 2*len(docQuotes) + 2*len(space) + len(newline)
	for _, line := range lines {
		size += len(line) + len(indent) + len(newline)
	}
	return size
}

// writeDocstring writes the docstring of lines at the indentation indent
// into b, as [Docstring] writes it.
func writeDocstring(b *strings.Builder, lines []string, indent string) {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return
	}
	shared, raw, escaped := -1, false, false
	for i, line := range lines {
		raw = raw || strings.Contains(line, backslash)
		escaped = escaped || strings.Contains(line, docQuotes)
		if i > 0 && strings.TrimSpace(line) != "" {
			if c := columns(line); shared < 0 || c < shared {
				shared = c
			}
		}
	}
	last := strings.TrimRightFunc(lines[len(lines)-1], unicode.IsSpace)
	escaped = escaped || raw && strings.HasSuffix(last, backslash)
	prefix := ""
	if raw && !escaped {
		prefix = rawPrefix
	}
	b.WriteString(indent)
	b.WriteString(prefix)
	b.WriteString(docQuotes)
	first := escapedText(strings.TrimSpace(lines[0]), escaped)
	if strings.HasPrefix(first, doubleQuote) {
		b.WriteString(space)
	}
	b.WriteString(first)
	if len(lines) == 1 {
		if strings.HasSuffix(first, doubleQuote) {
			b.WriteString(space)
		}
		b.WriteString(docQuotes)
		b.WriteString(newline)
		return
	}
	b.WriteString(newline)
	for _, line := range lines[1:] {
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if line != "" {
			b.WriteString(indent)
			for range columns(line) - shared {
				b.WriteString(space)
			}
			b.WriteString(escapedText(strings.TrimLeft(line, " \t"), escaped))
		}
		b.WriteString(newline)
	}
	b.WriteString(indent)
	b.WriteString(docQuotes)
	b.WriteString(newline)
}

// escapedText returns a line of a docstring with its backslashes and its
// triple quotes escaped where escaped is true, and the line as it is
// otherwise. A line without either is not copied.
func escapedText(line string, escaped bool) string {
	if !escaped {
		return line
	}
	return strings.ReplaceAll(strings.ReplaceAll(line, backslash, doubleSlash), docQuotes, docEscaped)
}

// columns returns the width of the indentation of a line in columns, with a
// space as one column and a tab as the advance to the next multiple of
// [tabWidth].
func columns(line string) int {
	n := 0
	for i := 0; i < len(line) && (line[i] == ' ' || line[i] == '\t'); i++ {
		if line[i] == '\t' {
			n += tabWidth - n%tabWidth
		} else {
			n++
		}
	}
	return n
}
