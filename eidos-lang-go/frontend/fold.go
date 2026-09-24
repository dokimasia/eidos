// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import "go.dokimi.dev/eidos/sdk/node"

// foldMethods moves a package's methods onto the struct or the
// enumeration that declares their receiver, whichever of the
// package's files spelled them, in file then declaration order, so
// the type states its members the way the model does and the member
// walk sees them. Promotion folds an enumeration's methods in its
// own file, and this pass folds the ones in its package's other
// files, such as a generated String method. A type name declared
// twice folds onto its first declaration, the one the load keeps. A
// method whose receiver names neither, a defined type over a builtin
// for instance, remains a file-level declaration whose owner is the
// receiver's name, because the alias kind has no members.
func foldMethods(files []*node.File) {
	structs := map[string]*node.Struct{}
	enums := map[string]*node.Enum{}
	for _, f := range files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *node.Struct:
				if _, held := structs[d.Name]; !held {
					structs[d.Name] = d
				}
			case *node.Enum:
				if _, held := enums[d.Name]; !held {
					enums[d.Name] = d
				}
			}
		}
	}
	if len(structs) == 0 && len(enums) == 0 {
		return
	}
	for _, f := range files {
		kept := f.Decls[:0]
		for _, decl := range f.Decls {
			m, is := decl.(*node.Method)
			if !is || m.Receives == nil {
				kept = append(kept, decl)
				continue
			}
			if host, declared := structs[m.Receives.Spelling]; declared {
				host.Methods = append(host.Methods, m)
				continue
			}
			if host, declared := enums[m.Receives.Spelling]; declared {
				host.Methods = append(host.Methods, m)
				continue
			}
			kept = append(kept, decl)
		}
		f.Decls = kept
	}
}
