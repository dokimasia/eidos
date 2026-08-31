// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"strings"

	"go.dokimi.dev/eidos/core/render"
)

// Imports renders the file's collected paths as import
// statements, dots for slashes, sorted, a blank line after the
// block. The set carries the paths the spellings qualified with;
// which simple names they bind is the type lowering's knowledge,
// so this renderer grows with it.
func Imports(set *render.ImportSet) string {
	paths := set.Paths()
	if len(paths) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range paths {
		b.WriteString("import ")
		b.WriteString(strings.ReplaceAll(p, "/", "."))
		b.WriteString(";\n")
	}
	b.WriteString("\n")
	return b.String()
}
