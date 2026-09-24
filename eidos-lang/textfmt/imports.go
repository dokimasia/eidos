// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt

import (
	"strings"

	"go.dokimi.dev/eidos/sdk/render"
)

// ImportLines renders a file's collected imports as one statement per
// distinct entry, in the set's order: the keyword, the path with its
// slashes spelled as sep, and the bound name behind another sep where
// the entry names one, each closed by a semicolon. A blank line
// follows the block, and an empty set renders nothing. Java renders
// its imports through it with "import" and ".", and Rust its uses
// with "use" and "::".
func ImportLines(set *render.ImportSet, keyword, sep string) string {
	entries := set.Entries()
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	written := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		stmt := strings.ReplaceAll(e.Path, "/", sep)
		if e.Name != "" {
			stmt += sep + e.Name
		}
		if _, repeated := written[stmt]; repeated {
			continue
		}
		written[stmt] = struct{}{}
		b.WriteString(keyword)
		b.WriteByte(' ')
		b.WriteString(stmt)
		b.WriteString(";\n")
	}
	b.WriteString("\n")
	return b.String()
}
