// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"cmp"
	"fmt"
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// treeRoot is the workspace-relative spelling of the tree's root
// directory: the output directory of a plan that states an import base
// without one, and the import base that makes the output directory the
// root of the imports.
const treeRoot = "."

// pathSep separates the directories of an import path.
const pathSep = "/"

// Package names the package of a protobuf file at a routed path. Every
// file is a package of its own, because a protobuf file imports a file and
// not a package. The package path is the import path of the file:
//
//   - with an import base in the plan, a file below the output directory
//     of the plan has the import base joined with its path below that
//     directory, and an import base of "." makes the output directory the
//     root of the imports;
//   - any other file has its workspace-relative path.
//
// The name of the package is the package that the file declares, which
// is the first package of this list:
//
//  1. the argument of the header marker [protobuf.PackageMarker];
//  2. the package of the origin, where the units of the file derive from
//     a protobuf package;
//  3. the directories of the import path, joined with dots, each in snake
//     case, as buf's lint rule PACKAGE_DIRECTORY_MATCH requires.
//
// A file at the root of the imports has the empty name, and the backend
// writes no package statement for it.
//
// Error modes, each with the path of the file:
//   - a header that states two packages;
//   - a package marker without exactly one argument;
//   - a package marker whose argument is not a dotted name of
//     identifiers;
//   - a directory of the import path whose snake case is not an
//     identifier.
//
// # Allocation contract
//
// Package allocates the import path that it joins under an import base of
// a directory, and the name that it derives from the directories of the
// import path, once more for each directory that is not snake case
// already. A header package and the package of a protobuf origin allocate
// nothing. A refusal allocates its error.
func Package(p plugin.Placement) (symbol.Identity, error) {
	imported := p.Path
	base := plugin.Module{Lang: protobuf.Lang, Path: p.ImportBase, Root: cmp.Or(p.BaseDir, treeRoot)}
	if p.ImportBase != "" && base.Contains(path.Dir(p.Path)) {
		imported = base.Rel(p.Path)
		if p.ImportBase != treeRoot {
			imported = path.Join(p.ImportBase, imported)
		}
	}
	clause, stated, err := headerPackage(p)
	switch {
	case err != nil:
		return symbol.Identity{}, err
	case !stated && p.Origin.Lang == protobuf.Lang:
		clause = p.Origin.Package
	case !stated:
		clause, err = derivedPackage(imported)
		if err != nil {
			return symbol.Identity{}, err
		}
	}
	return symbol.Identity{Lang: protobuf.Lang, Package: imported, Name: clause, Kind: symbol.KindPackage}, nil
}

// headerPackage returns the package that the package markers of the header
// of p state, and reports false for a header without one. It returns an
// error for two markers of different packages, for a marker without
// exactly one argument and for an argument that is not a dotted name of
// identifiers.
func headerPackage(p plugin.Placement) (string, bool, error) {
	var stated string
	found := false
	for _, a := range p.Header {
		if a.Name != string(protobuf.PackageMarker) {
			continue
		}
		switch {
		case len(a.Args) != 1:
			return "", false, fmt.Errorf("spell: the package marker of %s takes one argument, and it states %d",
				p.Path, len(a.Args))
		case found && a.Args[0] != stated:
			return "", false, fmt.Errorf("spell: the header of %s states the packages %s and %s, "+
				"and a file declares one package", p.Path, stated, a.Args[0])
		}
		stated, found = a.Args[0], true
	}
	if !found {
		return "", false, nil
	}
	for rest, more := stated, true; more; {
		var part string
		part, rest, more = strings.Cut(rest, protobuf.NameSep)
		if !naming.IsIdentifier(part) {
			return "", false, fmt.Errorf("spell: the header of %s states the package %q, "+
				"which is not a dotted name of identifiers", p.Path, stated)
		}
	}
	return stated, true, nil
}

// derivedPackage returns the directories of an import path joined with
// dots, each in snake case, and the empty name for a path at the root of
// the imports. It returns an error for a directory whose snake case is not
// an identifier.
func derivedPackage(imported string) (string, error) {
	dir := path.Dir(imported)
	if dir == treeRoot {
		return "", nil
	}
	var b strings.Builder
	b.Grow(len(dir))
	for rest, more := dir, true; more; {
		var part string
		part, rest, more = strings.Cut(rest, pathSep)
		snake := naming.Snake(part)
		if !naming.IsIdentifier(snake) {
			return "", fmt.Errorf("spell: the directory %q of the import path %s is not an identifier "+
				"in snake case, so the file derives no protobuf package", part, imported)
		}
		if b.Len() > 0 {
			b.WriteString(protobuf.NameSep)
		}
		b.WriteString(snake)
	}
	return b.String(), nil
}
