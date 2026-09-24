// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/render"
)

// Imports renders the file's collected entries as use statements
// through [textfmt.ImportLines]: double colons for slashes, in the
// set's path order, a blank line after the block. A named entry uses
// path::Name, and a bare entry uses the module itself.
func Imports(set *render.ImportSet) string {
	return textfmt.ImportLines(set, "use", "::")
}
