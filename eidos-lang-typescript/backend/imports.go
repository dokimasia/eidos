// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/core/render"
)

// Imports renders the file's collected paths, one side-effect
// import per path, sorted. The set carries bare paths and no
// imported names, so a named form cannot be written from it yet;
// the spelling that records names arrives with the type lowering,
// and this renderer grows with it.
func Imports(set *render.ImportSet) string {
	paths := set.Paths()
	if len(paths) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range paths {
		b.WriteString("import ")
		b.WriteString(strconv.Quote(p))
		b.WriteString(";\n")
	}
	b.WriteString("\n")
	return b.String()
}
