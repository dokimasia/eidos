// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt

import (
	"strings"

	"go.dokimi.dev/eidos/sdk/symbol"
)

// The block-comment delimiters, the escapes a comment's text takes
// for them, and the line break a comment's text is split at.
const (
	blockOpen    = "/*"
	blockClose   = "*/"
	escapedOpen  = `/\*`
	escapedClose = `*\/`
	lineBreak    = "\n"
	carriage     = "\r"
)

// LineDocs writes documentation in a line-comment form: one marker
// per line, everything behind the given prefix, nothing for no
// lines. The marker carries its own trailing space. A line
// containing a line break writes as one commented line per part.
func LineDocs(lines []string, marker string, prefix ...string) string {
	at := strings.Join(prefix, "")
	var b strings.Builder
	for _, text := range lines {
		for line := range strings.SplitSeq(text, lineBreak) {
			b.WriteString(at)
			b.WriteString(marker)
			b.WriteString(strings.TrimSuffix(line, carriage))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// BlockDocs writes documentation in a block-comment form: the
// opener, one guttered line each, the closer, everything behind
// the given prefix, nothing for no lines. A line containing a line
// break writes as one guttered line per part, and a comment closer
// inside a line is escaped, so no line ends the comment early.
func BlockDocs(lines []string, opener, gutter, closer string, prefix ...string) string {
	if len(lines) == 0 {
		return ""
	}
	at := strings.Join(prefix, "")
	var b strings.Builder
	b.WriteString(at)
	b.WriteString(opener)
	b.WriteByte('\n')
	for _, text := range lines {
		for line := range strings.SplitSeq(text, lineBreak) {
			b.WriteString(at)
			b.WriteString(gutter)
			b.WriteString(strings.ReplaceAll(strings.TrimSuffix(line, carriage), blockClose, escapedClose))
			b.WriteByte('\n')
		}
	}
	b.WriteString(at)
	b.WriteString(closer)
	b.WriteByte('\n')
	return b.String()
}

// Inline writes a trailing comment in block form, " /* text */",
// and nothing for no text. The text is folded onto one line, and
// a comment opener or closer inside it is escaped, so the comment
// ends where it is written in every target, Rust's nesting block
// comments included.
func Inline(text string) string {
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, carriage+lineBreak, " ")
	text = strings.ReplaceAll(text, lineBreak, " ")
	text = strings.ReplaceAll(text, blockClose, escapedClose)
	text = strings.ReplaceAll(text, blockOpen, escapedOpen)
	return " " + blockOpen + " " + text + " " + blockClose
}

// Marked writes structured markers one per line: the opener, the
// name, the arguments comma-joined in parentheses where any are
// stated, the closer, everything behind the given prefix. The
// shape serves a Java annotation, a TypeScript decorator and a
// Rust attribute alike, each naming its own opener and closer.
func Marked(as symbol.Annotations, opener, closer string, prefix ...string) string {
	at := strings.Join(prefix, "")
	var b strings.Builder
	for _, a := range as {
		b.WriteString(at)
		b.WriteString(opener)
		b.WriteString(a.Name)
		if len(a.Args) > 0 {
			b.WriteString("(")
			b.WriteString(strings.Join(a.Args, ", "))
			b.WriteString(")")
		}
		b.WriteString(closer)
		b.WriteByte('\n')
	}
	return b.String()
}
