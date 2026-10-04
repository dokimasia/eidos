// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
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
