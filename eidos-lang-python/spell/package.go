// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"cmp"
	"fmt"
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// treeRoot is the workspace-relative spelling of the root directory of the
// tree. It is the output directory of a plan that states an import base
// without one, and the import base that makes the output directory the root
// of the imports.
const treeRoot = "."

// Package names the module of a Python file at a routed path, as the Python
// frontend names the module of a file that it loads. The package path of a
// module is its dotted name with slashes, as shop/models for shop.models,
// and the __init__.py of a directory is the module of the package of the
// directory. The module is the first of these:
//
//   - In a directory where the load placed a Python file, the package of
//     the directory joined with the stem of the file. The package of the
//     directory is the module of its __init__.py, or the parent of the
//     module of another Python file in it.
//   - In a directory below the output directory of a plan that states an
//     import base, the import base joined with the path of the file below
//     that directory. The import base . makes the output directory the root
//     of the imports.
//   - The workspace-relative path of the file without its extension.
//
// A stub file is the module of its stem, as a file of source is. The name of
// the package is the last segment of its path, such as models.
//
// Error modes, each with the path of the file:
//   - a segment of the module that is not an identifier, or that is a
//     keyword of Python, because no import can name the module;
//   - the __init__.py of the root of the imports, which is the module of no
//     package.
//
// # Allocation contract
//
// Package allocates the module path that it joins, one allocation. A module
// that is a part of the path of the file or of a resident, such as the
// __init__.py of a directory whose package a resident names, allocates
// nothing. A refusal allocates its error.
func Package(p plugin.Placement) (symbol.Identity, error) {
	base := path.Base(p.Path)
	stem := strings.TrimSuffix(base, path.Ext(base))
	initFile := stem == python.InitModule
	var module string
	if dir, placed := directoryPackage(p); placed {
		switch {
		case initFile:
			module = dir
		case dir == "":
			module = stem
		default:
			module = dir + python.PackageSep + stem
		}
	} else {
		file := p.Path
		imports := plugin.Module{Lang: python.Lang, Path: p.ImportBase, Root: cmp.Or(p.BaseDir, treeRoot)}
		if p.ImportBase != "" && imports.Contains(path.Dir(p.Path)) {
			file = imports.Rel(p.Path)
			if p.ImportBase != treeRoot {
				file = path.Join(p.ImportBase, file)
			}
		}
		module = strings.TrimSuffix(file, path.Ext(file))
		if initFile {
			module = path.Dir(module)
		}
	}
	if module == "" || module == treeRoot {
		return symbol.Identity{}, fmt.Errorf("spell: %s is the __init__.py of the root of the imports, "+
			"which is the module of no package", p.Path)
	}
	for rest, more := module, true; more; {
		var segment string
		segment, rest, more = strings.Cut(rest, python.PackageSep)
		if !IsIdentifier(segment) || keywords[segment] {
			return symbol.Identity{}, fmt.Errorf("spell: an import names a module by identifiers that are no "+
				"keywords, and the segment %q of the module %s of %s is not one", segment, module, p.Path)
		}
	}
	return symbol.Identity{Lang: python.Lang, Package: module, Name: path.Base(module), Kind: symbol.KindPackage}, nil
}

// directoryPackage returns the package of the directory of the routed file
// of p, and false where the load placed no Python file in the directory. The
// package of a directory is the module of its __init__.py, or the parent of
// the module of another Python file of it. The parent of a module at the
// root of the imports is the empty package.
func directoryPackage(p plugin.Placement) (string, bool) {
	var parent string
	found := false
	for _, r := range p.Residents {
		if r.Pkg.Lang != python.Lang {
			continue
		}
		base := path.Base(r.File)
		if strings.TrimSuffix(base, path.Ext(base)) == python.InitModule {
			return r.Pkg.Package, true
		}
		if !found {
			if dir, _, nested := strings.CutLast(r.Pkg.Package, python.PackageSep); nested {
				parent = dir
			}
			found = true
		}
	}
	return parent, found
}
