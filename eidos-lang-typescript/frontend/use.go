// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// packageSep separates the segments of a package path, as
// [go.dokimi.dev/eidos/lang/typescript.JoinPackage] joins them.
const packageSep = "/"

// fileScope is one File node that a lowering contributes to a package, and
// the scope that the File node records for the resolution step.
type fileScope struct {
	file  *node.File
	scope *scope
}

// recordUses records a use of the unit for each global name of the library
// of the file that a reference of the file can resolve to. The use is in the
// package of the library, the last package of the global scope of the
// file. A named reference resolves through the tiers of its spelling, as
// [scope.candidates] returns them, and the first tier with a declaration
// decides. recordUses reads the tiers in order up to the first tier with a
// declaration of the unit. Each candidate in the library before that tier
// is a use: its name for a candidate in the package of the library, and the
// first name of the namespace for a candidate inside a namespace of the
// library. A name that no file of the library of the release declares is no
// use. A file without a library, whose lowering has no set of declarations,
// records none.
func (l *lowering) recordUses() {
	if l.declared == nil {
		return
	}
	lib := l.module.globals[len(l.module.globals)-1]
	decls := declarations()
	for _, f := range l.files {
		node.Walk(f.file, func(s symbol.Symbol) bool {
			ref, is := s.(*node.TypeRef)
			if !is {
				return true
			}
			//dokimi:mutate-skip sbr-delete,ror-false,lcr-left,lcr-right: no global of the release has a structural or an empty spelling, so the check spares the tiers alone
			if ref.Form != symbol.FormNamed || ref.Spelling == "" {
				return true
			}
			for _, tier := range f.scope.candidates(ref.Spelling, nil) {
				if slices.ContainsFunc(tier, func(c symbol.Identity) bool { return l.declared[c] }) {
					break
				}
				for _, c := range tier {
					rest, inLibrary := strings.CutPrefix(c.Package, lib)
					if !inLibrary {
						continue
					}
					name := c.Name
					if rest != "" {
						name, _, _ = strings.Cut(strings.TrimPrefix(rest, packageSep), packageSep)
					}
					if _, declares := decls[name]; declares {
						l.u.Use(symbol.Identity{Lang: Lang, Package: lib, Name: name})
					}
				}
			}
			return true
		})
	}
}
