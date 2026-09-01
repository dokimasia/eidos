// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"strings"

	"go.dokimi.dev/eidos/sdk/render"
)

// Imports renders the file's collected entries as use statements,
// double colons for slashes, sorted, a blank line after the
// block. A named entry binds path::Name; a bare entry renders its
// path alone, which imports the module. Two entries spelling one
// statement render it once.
func Imports(set *render.ImportSet) string {
	entries := set.Entries()
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	written := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		stmt := strings.ReplaceAll(e.Path, "/", "::")
		if e.Name != "" {
			stmt += "::" + e.Name
		}
		if _, held := written[stmt]; held {
			continue
		}
		written[stmt] = struct{}{}
		b.WriteString("use ")
		b.WriteString(stmt)
		b.WriteString(";\n")
	}
	b.WriteString("\n")
	return b.String()
}
