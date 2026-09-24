// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/render"
)

// Imports renders the file's collected entries as import statements
// through [textfmt.ImportLines]: dots for slashes, sorted, a blank
// line after the block. A named entry imports path.Name, and a bare
// entry renders its path alone, which serves a caller recording a
// class-qualified path.
func Imports(set *render.ImportSet) string {
	return textfmt.ImportLines(set, "import", ".")
}
