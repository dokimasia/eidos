// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package rust

import (
	"strings"

	"go.dokimi.dev/eidos/core/render"
)

// Imports renders the file's collected paths as use statements,
// double colons for slashes, sorted, a blank line after the
// block.
func Imports(set *render.ImportSet) string {
	paths := set.Paths()
	if len(paths) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range paths {
		b.WriteString("use ")
		b.WriteString(strings.ReplaceAll(p, "/", "::"))
		b.WriteString(";\n")
	}
	b.WriteString("\n")
	return b.String()
}
