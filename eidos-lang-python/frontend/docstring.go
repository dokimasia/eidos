// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
)

// tabWidth is the width of the columns to which a tab of a docstring
// expands, the default of str.expandtabs.
const tabWidth = 8

// The characters of a docstring that its trim reads: the line feed that
// ends a line, the tab and the space.
const (
	lineFeed = '\n'
	tab      = '\t'
	space    = ' '
)

// trimDoc returns the lines of a docstring as PEP 257 trims them. A line
// ends at a line feed, and the tabs expand to columns of eight. The first
// line loses its indentation, the other lines lose the indentation that
// they share, and every line loses its trailing blanks. trimDoc drops the
// blank lines at both ends.
func trimDoc(value string) []string {
	lines := strings.Split(expandTabs(value), string(lineFeed))
	indent := -1
	for _, line := range lines[1:] {
		if stripped := strings.TrimLeft(line, string(space)); stripped != "" {
			if margin := len(line) - len(stripped); indent < 0 || margin < indent {
				indent = margin
			}
		}
	}
	out := make([]string, 0, len(lines))
	out = append(out, strings.TrimSpace(lines[0]))
	for _, line := range lines[1:] {
		if indent > 0 {
			line = line[min(indent, len(line)):]
		}
		out = append(out, strings.TrimRight(line, blanks))
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	for len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	return out
}

// expandTabs returns s with each tab replaced by the spaces to the next
// column that is a multiple of eight, counted from the start of its line.
// It returns s itself where s has no tab.
func expandTabs(s string) string {
	if !strings.ContainsRune(s, tab) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + tabWidth)
	column := 0
	for _, r := range s {
		switch r {
		case tab:
			spaces := tabWidth - column%tabWidth
			for range spaces {
				b.WriteByte(space)
			}
			column += spaces
		case lineFeed:
			b.WriteRune(r)
			column = 0
		default:
			b.WriteRune(r)
			column++
		}
	}
	return b.String()
}

// docstring returns the documentation of a body, the module or a block: the
// docstring of its first statement, and the node of that statement. The
// first statement is a docstring where it is a string literal.
func (l *lowering) docstring(body treesitter.Node) ([]string, treesitter.Node) {
	for first := range body.Content() {
		if lines, is := l.docLines(first); is {
			return lines, first
		}
		break
	}
	return nil, treesitter.Node{}
}

// attributeDoc returns the docstring of an attribute: the string literal
// statement directly after its assignment, as Sphinx's autodoc and griffe
// read it. The comments between the two statements do not count.
func (l *lowering) attributeDoc(stmt treesitter.Node) []string {
	next := stmt.NextNamedSibling()
	for next.Kind() == l.v.comment {
		next = next.NextNamedSibling()
	}
	lines, _ := l.docLines(next)
	return lines
}

// docLines returns the lines of a string literal statement, trimmed as PEP
// 257 trims a docstring, and reports false for a statement that is no
// string literal. A concatenation of literals is one string, and an f-string
// or bytes is no docstring.
func (l *lowering) docLines(stmt treesitter.Node) ([]string, bool) {
	if k := stmt.Kind(); k != l.v.stringNode && k != l.v.concatenatedString {
		return nil, false
	}
	value, is := l.stringValue(stmt)
	if !is {
		return nil, false
	}
	return l.u.DocLines(trimDoc(value)), true
}
