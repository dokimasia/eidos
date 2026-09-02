// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt

import (
	"strings"

	"go.dokimi.dev/eidos/sdk/symbol"
)

// LineDocs writes documentation in a line-comment form: one marker
// per line, everything behind the given prefix, nothing for no
// lines. The marker carries its own trailing space.
func LineDocs(lines []string, marker string, prefix ...string) string {
	at := strings.Join(prefix, "")
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(at)
		b.WriteString(marker)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// BlockDocs writes documentation in a block-comment form: the
// opener, one guttered line each, the closer, everything behind
// the given prefix, nothing for no lines.
func BlockDocs(lines []string, opener, gutter, closer string, prefix ...string) string {
	if len(lines) == 0 {
		return ""
	}
	at := strings.Join(prefix, "")
	var b strings.Builder
	b.WriteString(at)
	b.WriteString(opener)
	b.WriteByte('\n')
	for _, line := range lines {
		b.WriteString(at)
		b.WriteString(gutter)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString(at)
	b.WriteString(closer)
	b.WriteByte('\n')
	return b.String()
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
