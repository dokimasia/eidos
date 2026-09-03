// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package node

import "iter"

// Imports returns a package's imports: the union of its files'
// import statements, in file order and then statement order. The
// model carries no package-level import list, because the files
// record the scope as written and a second list would drift from
// them; this is the one derivation.
func Imports(p *Package) iter.Seq[*Import] {
	return func(yield func(*Import) bool) {
		if p == nil {
			return
		}
		for _, f := range p.Files {
			for _, imp := range f.Imports {
				if !yield(imp) {
					return
				}
			}
		}
	}
}
