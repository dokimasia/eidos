// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package gosource

import (
	"go/importer"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"
)

// Importer resolves imports from source rather than from a build
// cache.
//
// Packages inside the module rooted at modRoot load from disk
// through [Load] in [Complete] mode. Everything else, the standard
// library included, goes to the compiler's source importer.
//
// Resolving from source is what keeps generation independent of a
// prior build: a checkout with a cold cache generates the same
// bytes as one with a warm cache.
//
// An Importer caches each package it resolves and is not safe for
// concurrent use.
type Importer struct {
	fset    *token.FileSet
	modRoot string
	modPath string
	std     types.Importer
	cache   map[string]*types.Package
}

// NewImporter builds an importer for the module rooted at modRoot.
//
// An empty modRoot resolves nothing locally and delegates every
// path to the standard-library source importer, which is what a
// self-contained package wants.
func NewImporter(fset *token.FileSet, modRoot string) (*Importer, error) {
	imp := &Importer{
		fset:    fset,
		modRoot: modRoot,
		std:     importer.ForCompiler(fset, "source", nil),
		cache:   map[string]*types.Package{},
	}
	if modRoot == "" {
		return imp, nil
	}
	path, err := ModulePath(modRoot)
	if err != nil {
		return nil, err
	}
	imp.modPath = path
	return imp, nil
}

// Import resolves one import path to a type-checked package.
func (im *Importer) Import(path string) (*types.Package, error) {
	if pkg, ok := im.cache[path]; ok {
		return pkg, nil
	}
	if !im.owns(path) {
		return im.std.Import(path)
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(path, im.modPath), "/")
	pkg, _, err := Load(im.fset, filepath.Join(im.modRoot, rel), path, im.modRoot, Complete)
	if err != nil {
		return nil, err
	}
	im.cache[path] = pkg
	return pkg, nil
}

// owns reports whether an import path names a package inside this
// importer's module.
func (im *Importer) owns(path string) bool {
	return im.modPath != "" &&
		(path == im.modPath || strings.HasPrefix(path, im.modPath+"/"))
}
