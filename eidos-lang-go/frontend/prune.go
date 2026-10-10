// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// pruneImports keeps the imports a signature-only file's retained
// declarations name, so the next dependency round follows the packages
// exported signatures reference and not every package a function body
// calls. A file keeps an import a qualified reference names, every
// unaliased import where a qualified reference names none, because a
// package clause can declare a name its path does not assume, and
// every dot import. A file that retains no declaration keeps no import.
// The scope the parse recorded is unchanged, so every retained
// spelling still resolves.
func pruneImports(file *node.File) {
	if len(file.Decls) == 0 {
		file.Imports = nil
		return
	}
	named := map[string]bool{}
	unbound := false
	for _, decl := range file.Decls {
		node.Walk(decl, func(s symbol.Symbol) bool {
			ref, is := s.(*node.TypeRef)
			if !is || ref.Form != symbol.FormNamed {
				return true
			}
			if ref.Package != "" {
				named[ref.Package] = true
			} else if strings.Contains(ref.Spelling, ".") {
				unbound = true
			}
			return true
		})
	}
	kept := file.Imports[:0]
	for _, imp := range file.Imports {
		if named[imp.Path] || imp.Wildcard || (unbound && imp.Alias == "") {
			kept = append(kept, imp)
		}
	}
	file.Imports = kept
}
