// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
)

// The base names of the module of a package directory: its source and its
// stub.
const (
	InitSource = python.InitModule + python.Extension
	InitStub   = python.InitModule + python.StubExtension
)

// Place returns the source root of the file at the slash path file, and
// the paths that the choice of the root depends on beside the project
// file. proj is the project that governs the file, and nil for a file
// outside every project or in a project whose file does not read.
//
// A file below a root of its project takes that root, and depends on no
// further path. Every other file takes the directory above the run of
// package directories that contain it, as pytest and mypy name such a
// module: the parent of the highest directory of the run, where pkg
// reports whether a directory is a package directory. The root of the
// workspace ends every run. The paths are the module of a package
// directory and its stub, two for each directory of the run and for the
// directory that ends it, from the directory of the file upward. A
// partition declares them as shared inputs, so a module of a package
// directory that appears or disappears changes the key of the unit.
//
// # Allocation contract
//
// Place allocates nothing for a file below a root of its project. It
// allocates the joined paths and their list for any other file.
func Place(file string, proj *Project, pkg func(dir string) bool) (root string, probes []string) {
	if proj != nil {
		if root, below := proj.Root(file); below {
			return root, nil
		}
	}
	for dir := path.Dir(file); ; dir = path.Dir(dir) {
		probes = append(probes, path.Join(dir, InitSource), path.Join(dir, InitStub))
		if dir == "." || !pkg(dir) {
			return dir, probes
		}
	}
}

// Package returns the package of the module at the slash path file below
// the source root root: the path of the file below the root without its
// extension, as shop/models for src/shop/models.py below src. The module
// of a package directory is the package of the directory, as shop for
// src/shop/__init__.py, and the module of a root that is a package
// directory itself is the package of the global scope, the empty path.
// Package reads a file outside the root as a file at the root of the
// workspace. It allocates nothing, because the package shares the bytes
// of file.
func Package(file, root string) string {
	rel := file
	if root != "." && len(file) > len(root) && file[len(root)] == '/' && strings.HasPrefix(file, root) {
		rel = file[len(root)+1:]
	}
	rel = strings.TrimSuffix(rel, path.Ext(rel))
	if path.Base(rel) != python.InitModule {
		return rel
	}
	if dir := path.Dir(rel); dir != "." {
		return dir
	}
	return ""
}
