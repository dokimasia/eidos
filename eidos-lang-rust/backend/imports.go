// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"path"

	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/render"
)

// Imports renders the file's collected entries as use statements
// through [textfmt.ImportLines]: double colons for slashes, in the
// set's path order, a blank line after the block. A named entry uses
// path::Name, a renamed item path::Item as Name, and a bare entry
// uses the module itself. Imports allocates the sorted entries and what
// [textfmt.ImportLines] allocates, two allocations, and nothing for an
// empty set.
func Imports(set *render.ImportSet) string {
	return textfmt.ImportLines(set.Entries(), "use", "::")
}

// TemplateImport returns the binding of an import that a template states
// through the use builtin. An import without a name binds the module
// under the last segment of its path, as a use statement of the module
// does. A named import binds the item of that name from the module. It
// does not allocate.
func TemplateImport(module, name string) (local, item string, ok bool) {
	if name == "" {
		return path.Base(module), "", true
	}
	return name, name, true
}
