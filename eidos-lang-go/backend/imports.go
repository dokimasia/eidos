// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/sdk/render"
)

// Imports renders the file's collected entries as one
// parenthesised block in two groups, standard library first, a
// blank line between, each group sorted — the shape goimports
// leaves. A named entry spells its name before the path, which is
// how aliases, blank side-effect imports and dot imports render.
// A file that qualified nothing renders no block at all, because
// an empty import block is not what gofmt leaves either.
func Imports(set *render.ImportSet) string {
	entries := set.Entries()
	if len(entries) == 0 {
		return ""
	}

	var std, external []string
	for _, e := range entries {
		line := strconv.Quote(e.Path)
		if e.Name != "" {
			line = e.Name + " " + line
		}
		if first, _, _ := strings.Cut(e.Path, "/"); strings.Contains(first, ".") {
			external = append(external, line)
			continue
		}
		std = append(std, line)
	}

	var b strings.Builder
	b.WriteString("import (\n")
	for _, line := range std {
		b.WriteString("\t" + line + "\n")
	}
	if len(std) > 0 && len(external) > 0 {
		b.WriteString("\n")
	}
	for _, line := range external {
		b.WriteString("\t" + line + "\n")
	}
	b.WriteString(")\n")
	return b.String()
}
