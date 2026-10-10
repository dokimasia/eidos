// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"cmp"
	"fmt"
	"path"
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// testFile ends the name of a Go test file, whose package can be the
// directory's external test package.
const testFile = golang.TestSuffix + golang.Extension

// treeRoot is the workspace-relative spelling of the tree's root
// directory, the base directory of an import base the plan states
// without an output directory.
const treeRoot = "."

// Package names the package a Go file at a routed path declares, the
// way the Go frontend names the package of a directory it loads:
//
//   - for a test file, the package of the file's origin where a file of
//     the directory declares it, such as the external test package
//     store_test;
//   - the package the directory's non-test files declare, with the
//     name their clause writes, main included;
//   - in a directory without one, the innermost module that contains
//     the directory, joined with the directory's path below the
//     module's root;
//   - in a directory outside every module, the plan's import base
//     joined with the directory's path below the plan's output
//     directory.
//
// Any other test file declares the directory's package, so it compiles
// into the package's test binary and reads its unexported names. A
// directory outside every module and outside the import base's
// directory derives no package, and Package returns an error naming
// the directory.
//
// # Allocation contract
//
// Package returns a resident's package without allocating, and
// allocates the import path it derives for a directory without one,
// one allocation. A refusal allocates its error.
func Package(p plugin.Placement) (symbol.Identity, error) {
	if strings.HasSuffix(p.Path, testFile) {
		for _, r := range p.Residents {
			if r.Pkg.Lang == golang.Lang && r.Pkg.Package == p.Origin.Package {
				return r.Pkg, nil
			}
		}
	}
	for _, r := range p.Residents {
		if r.Pkg.Lang == golang.Lang && !strings.HasSuffix(r.File, testFile) {
			return r.Pkg, nil
		}
	}
	dir := path.Dir(p.Path)
	if m, contained := p.ModuleOf(golang.Lang, dir); contained {
		return derived(golang.ImportPath(m.Path, m.Root, dir)), nil
	}
	base := plugin.Module{Lang: golang.Lang, Path: p.ImportBase, Root: cmp.Or(p.BaseDir, treeRoot)}
	if p.ImportBase != "" && base.Contains(dir) {
		return derived(golang.ImportPath(base.Path, base.Root, dir)), nil
	}
	return symbol.Identity{}, fmt.Errorf(
		"spell: no Go module contains %s, and the plan states no import base for it", dir)
}

// derived returns the package of an import path no loaded package
// declares, named the way an import of the path binds it.
func derived(importPath string) symbol.Identity {
	return symbol.Identity{
		Lang: golang.Lang, Package: importPath, Name: golang.AssumedName(importPath), Kind: symbol.KindPackage,
	}
}
