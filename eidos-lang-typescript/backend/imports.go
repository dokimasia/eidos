// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/sdk/render"
)

// Imports renders the file's collected entries, sorted by path:
// the names bound under a path render as one named import, and a
// path binding no names renders as a side-effect import, which a
// named import already implies, so a path carrying both forms
// renders the named one alone.
func Imports(set *render.ImportSet) string {
	entries := set.Entries()
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(entries); {
		path := entries[i].Path
		var names []string
		for ; i < len(entries) && entries[i].Path == path; i++ {
			if n := entries[i].Name; n != "" {
				names = append(names, n)
			}
		}
		if len(names) == 0 {
			b.WriteString("import ")
			b.WriteString(strconv.Quote(path))
			b.WriteString(";\n")
			continue
		}
		b.WriteString("import { ")
		b.WriteString(strings.Join(names, ", "))
		b.WriteString(" } from ")
		b.WriteString(strconv.Quote(path))
		b.WriteString(";\n")
	}
	b.WriteString("\n")
	return b.String()
}
