// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// fileKey addresses one grouped file: the routed path under the package of
// its units, because a hand-built store can route two packages to one path,
// and they remain two files.
type fileKey struct {
	pkg  symbol.Identity
	path string
}

// Files routes a hand-built store into the files a render renders, the way
// a plan's layout routes a store whose packages are at their package paths
// under the default configuration:
//
//   - a per-source unit is written in the directory of its source file, its
//     key, and a unit whose key names no file of the workspace in the
//     directory of its package path;
//   - a per-package unit is written in the directory of its package path;
//   - a plan unit is written at the root.
//
// Each unit splits through the target's speller, each part takes its
// spelled name in that directory, and the parts of one package at one path
// assemble one file, in the store's order. Each file has the header that
// [plugin.HeaderOf] returns for its units, as a plan's layout gives it.
//
// Where s is also the [plugin.Packager] of its backend, each file declares
// the package that the packager derives for its path, as a plan's layout
// asks it. The residents of a directory are the source files of the
// per-source units of the store in it, each with the package of its unit.
// A packager can need the modules of a loaded graph, which a hand-built
// store does not have. Where the packager derives no package, the file
// keeps the package of its first unit, as every file of a speller without
// a package half does. The files sort by path, then by package, which is
// the order a plan's layout returns.
func Files(e *plugin.Emit, s plugin.FileSpeller) []plugin.File {
	byKey := map[fileKey]int{}
	var files []plugin.File
	for u := range e.Units() {
		dir := unitDir(u)
		for _, part := range s.SplitUnit(u) {
			k := fileKey{pkg: part.Pkg, path: path.Join(dir, s.FileName(part))}
			at, grouped := byKey[k]
			if !grouped {
				at = len(files)
				byKey[k] = at
				files = append(files, plugin.File{Path: k.path, Pkg: k.pkg})
			}
			files[at].Units = append(files[at].Units, part)
		}
	}
	packager, packages := s.(plugin.Packager)
	var residents map[string][]plugin.Resident
	if packages {
		residents = residentsOf(e)
	}
	for i := range files {
		f := &files[i]
		f.Header = plugin.HeaderOf(f.Units)
		if !packages {
			continue
		}
		pkg, err := packager.PackageAt(plugin.Placement{
			Path: f.Path, Origin: f.Pkg, Residents: residents[path.Dir(f.Path)], Header: f.Header.Annotations,
		})
		if err != nil {
			continue
		}
		f.Pkg = pkg
	}
	slices.SortStableFunc(files, func(a, b plugin.File) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), a.Pkg.Compare(b.Pkg))
	})
	return files
}

// unitDir returns the directory in which [Files] writes the file of the
// unit u: the directory of the source file of a per-source unit, and the
// package path of every other unit, which is empty for a plan unit.
func unitDir(u plugin.Unit) string {
	if u.Per == plugin.PerSource && sourceFile(u.Key) {
		return path.Dir(u.Key)
	}
	return u.Pkg.Package
}

// residentsOf returns the source files of the per-source units of the store
// by directory, each with the package of its unit, once and sorted by file,
// as a load places the source files of a directory.
func residentsOf(e *plugin.Emit) map[string][]plugin.Resident {
	out := map[string][]plugin.Resident{}
	for u := range e.Units() {
		if u.Per != plugin.PerSource || !sourceFile(u.Key) {
			continue
		}
		dir := path.Dir(u.Key)
		if r := (plugin.Resident{File: u.Key, Pkg: u.Pkg}); !slices.Contains(out[dir], r) {
			out[dir] = append(out[dir], r)
		}
	}
	for _, rs := range out {
		slices.SortFunc(rs, func(a, b plugin.Resident) int {
			return cmp.Or(strings.Compare(a.File, b.File), a.Pkg.Compare(b.Pkg))
		})
	}
	return out
}

// sourceFile reports whether the key of a unit names a file of the
// workspace tree, and not a file of a dependency store.
func sourceFile(key string) bool {
	_, _, stored := plugin.CutStorePath(key)
	return key != "" && !stored
}
