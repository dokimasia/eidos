// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/sdk/render"
)

// Imports renders the file's collected paths as one parenthesised
// block, sorted, which is the shape gofmt leaves a single import
// group in. A file that qualified nothing renders no block at all,
// because an empty import block is not what gofmt leaves either.
func Imports(set *render.ImportSet) string {
	paths := set.Paths()
	if len(paths) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("import (\n")
	for _, p := range paths {
		b.WriteByte('\t')
		b.WriteString(strconv.Quote(p))
		b.WriteByte('\n')
	}
	b.WriteString(")\n")
	return b.String()
}
