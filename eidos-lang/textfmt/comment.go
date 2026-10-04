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

// argSeparator separates a marker's arguments inside its parentheses.
const argSeparator = ", "

// LineDocs writes documentation as line comments: each line behind the
// given prefix and the marker, which brings its own trailing space. A
// line that contains a line break writes one commented line per part.
// An empty list of lines writes an empty string.
//
// # Allocation contract
//
// LineDocs writes into one buffer sized to the result, one allocation.
// Joining more than one prefix string allocates the joined prefix. An
// empty list of lines allocates nothing.
func LineDocs(lines []string, marker string, prefix ...string) string {
	at := strings.Join(prefix, "")
	var b strings.Builder
	b.Grow(commentedLen(lines, len(at)+len(marker)))
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

// BlockDocs writes documentation as one block comment behind the given
// prefix: the opener on its own line, each line behind the gutter, and
// the closer on its own line. A line that contains a line break writes
// one guttered line per part. A comment closer inside a line is
// escaped, so no line ends the comment early. An empty list of lines
// writes an empty string.
//
// # Allocation contract
//
// BlockDocs writes into one buffer sized to the result, one allocation.
// Joining more than one prefix string allocates the joined prefix, and
// each line part that contains a closer allocates its escaped copy. An
// empty list of lines allocates nothing.
func BlockDocs(lines []string, opener, gutter, closer string, prefix ...string) string {
	if len(lines) == 0 {
		return ""
	}
	at := strings.Join(prefix, "")
	var b strings.Builder
	n := commentedLen(lines, len(at)+len(gutter)) + 2*(len(at)+1) + len(opener) + len(closer)
	for _, text := range lines {
		n += strings.Count(text, blockClose)
	}
	b.Grow(n)
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

// commentedLen returns the length lines take written one part per
// line, each part behind per bytes of prefix and marker and closed by a
// line break: the size [LineDocs] and [BlockDocs] grow their buffer to.
// The size is exact, or larger by the carriage returns the parts end
// in, which the writers trim.
func commentedLen(lines []string, per int) int {
	n := 0
	for _, text := range lines {
		parts := strings.Count(text, lineBreak) + 1
		n += parts*per + len(text) + 1
	}
	return n
}

// Inline writes a trailing comment in block form, " /* text */". The
// text is folded onto one line, and a comment opener or closer inside
// it is escaped, so the comment ends where it is written in every
// target, Rust's nesting block comments included. An empty text writes
// an empty string.
//
// # Allocation contract
//
// Inline allocates the comment, one allocation. A text that needs a
// fold or an escape allocates one copy for each kind it contains: a
// carriage return before a line feed, a line feed, a closer and an
// opener. An empty text allocates nothing.
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

// Marked writes structured markers one per line behind the given
// prefix: the opener, the name, the arguments comma-joined in
// parentheses where any are stated, and the closer. A Java annotation,
// a TypeScript decorator and a Rust attribute each pass their own
// opener and closer.
//
// # Allocation contract
//
// Marked writes into one buffer sized to the result, one allocation.
// Joining more than one prefix string allocates the joined prefix. An
// empty list of markers allocates nothing.
func Marked(as symbol.Annotations, opener, closer string, prefix ...string) string {
	at := strings.Join(prefix, "")
	n := 0
	for _, a := range as {
		n += len(at) + len(opener) + len(a.Name) + len(closer) + 1
		if len(a.Args) > 0 {
			n += 2 + len(argSeparator)*(len(a.Args)-1)
			for _, arg := range a.Args {
				n += len(arg)
			}
		}
	}
	var b strings.Builder
	b.Grow(n)
	for _, a := range as {
		b.WriteString(at)
		b.WriteString(opener)
		b.WriteString(a.Name)
		if len(a.Args) > 0 {
			b.WriteByte('(')
			for i, arg := range a.Args {
				if i > 0 {
					b.WriteString(argSeparator)
				}
				b.WriteString(arg)
			}
			b.WriteByte(')')
		}
		b.WriteString(closer)
		b.WriteByte('\n')
	}
	return b.String()
}
