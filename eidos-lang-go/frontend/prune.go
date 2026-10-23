// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import "go.dokimi.dev/eidos/sdk/node"

// pruneImports keeps the imports of a signature-only file whose packages
// the file uses, as [recordUses] and [recordPackageUses] collect them, so a
// dependency package lists the imports that its exported signatures and its
// directives name and not the imports that a function body calls. A file
// that uses no package keeps no import. The scope that the parse recorded
// is unchanged, so every retained spelling still resolves.
func pruneImports(file *node.File, used map[string]bool) {
	kept := file.Imports[:0]
	for _, imp := range file.Imports {
		if used[imp.Path] {
			kept = append(kept, imp)
		}
	}
	file.Imports = kept
}
