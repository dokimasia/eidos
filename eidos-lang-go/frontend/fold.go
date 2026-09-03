// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import "go.dokimi.dev/eidos/sdk/node"

// foldMethods moves a package's methods onto the struct that
// declares their receiver, whichever of the package's files spelled
// them, in file then declaration order, so the type carries its
// members the way the model states them and the member walk sees
// them. A method on an enumeration folded already when the
// enumeration was promoted. A method whose receiver names no
// struct in the package, a defined type over a builtin for
// instance, stays a file-level declaration owned by the receiver's
// name, because the alias kind carries no members.
func foldMethods(files []*node.File) {
	structs := map[string]*node.Struct{}
	for _, f := range files {
		for _, decl := range f.Decls {
			if s, is := decl.(*node.Struct); is {
				structs[s.Name] = s
			}
		}
	}
	if len(structs) == 0 {
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
			host, declared := structs[m.Receives.Spelling]
			if !declared {
				kept = append(kept, decl)
				continue
			}
			host.Methods = append(host.Methods, m)
		}
		f.Decls = kept
	}
}
