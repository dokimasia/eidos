// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"path"
	"slices"

	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/render"
)

// langPackage is the package Java imports into every file.
const langPackage = "java/lang"

// Imports renders the file's collected entries as import statements
// through [textfmt.ImportLines]: dots for slashes, sorted, a blank
// line after the block. A named entry imports path.Name, and a bare
// entry renders its path alone, which serves a caller recording a
// class-qualified path. A class of java.lang renders no statement,
// because Java imports the package into every file, and its simple
// name is still the file's, which the speller's claim records.
//
// Imports allocates the sorted entries and what [textfmt.ImportLines]
// allocates, two allocations, and nothing for an empty set.
func Imports(set *render.ImportSet) string {
	entries := slices.DeleteFunc(set.Entries(), func(e render.Entry) bool {
		return e.Path == langPackage
	})
	return textfmt.ImportLines(entries, "import", ".")
}

// TemplateImport returns the binding of an import that a template states
// through the use builtin. An import without a name binds the class at
// the last segment of its path under its simple name. A named import
// binds the class of that name from the package, as the speller's claim
// binds it. It does not allocate.
func TemplateImport(class, name string) (local, item string, ok bool) {
	if name == "" {
		return path.Base(class), "", true
	}
	return name, name, true
}
