// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest

import (
	"cmp"
	"path"
	"slices"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// fileKey addresses one grouped file: the spelled name under its
// package, because two packages can spell one filename and remain
// two files.
type fileKey struct {
	pkg  symbol.Identity
	name string
}

// Files routes a hand-built store into the files a render renders,
// the way a plan routes a store whose packages are at their package
// paths: each unit splits through the target's speller, each part
// takes its spelled name under its package's path, and parts sharing
// a package and a name assemble one file, in the store's order. A plan
// unit, which has no package, takes its name alone. The files sort by
// path, then by package, which is the order a plan's layout returns.
func Files(e *plugin.Emit, s plugin.FileSpeller) []plugin.File {
	byKey := map[fileKey]int{}
	var files []plugin.File
	for u := range e.Units() {
		for _, part := range s.SplitUnit(u) {
			k := fileKey{pkg: part.Pkg, name: s.FileName(part)}
			at, grouped := byKey[k]
			if !grouped {
				at = len(files)
				byKey[k] = at
				files = append(files, plugin.File{Path: path.Join(k.pkg.Package, k.name), Pkg: k.pkg})
			}
			files[at].Units = append(files[at].Units, part)
		}
	}
	slices.SortStableFunc(files, func(a, b plugin.File) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), a.Pkg.Compare(b.Pkg))
	})
	return files
}
