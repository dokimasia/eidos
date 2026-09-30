// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt

import (
	"strings"

	"go.dokimi.dev/eidos/sdk/render"
)

// renameKeyword introduces the name an imported item binds under.
const renameKeyword = " as "

// ImportLines renders a file's collected imports as one statement per
// distinct entry, in the order given: the keyword, the path with its
// slashes spelled as sep, and the bound name behind another sep where
// the entry names one, each closed by a semicolon. An entry that
// renames the item it imports spells the item behind the path and the
// name behind as. A blank line follows the block, and no entries
// render nothing. Java renders its imports through it with "import"
// and ".", and Rust its uses with "use" and "::".
func ImportLines(entries []render.Entry, keyword, sep string) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	written := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		stmt := strings.ReplaceAll(e.Path, "/", sep)
		switch {
		case e.Item != "":
			stmt += sep + e.Item + renameKeyword + e.Name
		case e.Name != "":
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
