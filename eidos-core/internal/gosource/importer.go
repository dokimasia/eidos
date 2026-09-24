// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package gosource

import (
	"fmt"
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
// through this importer in [Complete] mode. Everything else, the
// standard library included, goes to the compiler's source importer.
//
// Resolving from source is what keeps generation independent of a
// prior build: a checkout with a cold cache generates the same
// bytes as one with a warm cache.
//
// Every nested module-local load resolves its own imports through
// the same importer, so one import path yields one package however
// deep the load goes, and a module-local path imported while it is
// still loading refuses as an import cycle.
//
// An Importer caches each package it resolves and is not safe for
// concurrent use.
type Importer struct {
	fset    *token.FileSet
	modRoot string
	modPath string
	std     types.Importer
	cache   map[string]*types.Package
	// loading marks the package paths whose type-check is under
	// way, and cycle is the first import cycle a nested load
	// refused, which the outermost call returns.
	loading map[string]bool
	cycle   error
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
		loading: map[string]bool{},
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
//
// A module-local path that a nested load imports while it is still
// loading refuses as an import cycle. A load in [Complete] mode
// tolerates the refusal the way it tolerates any type-checking
// error, so the outermost call returns the cycle once the load
// unwinds.
func (im *Importer) Import(path string) (*types.Package, error) {
	if pkg, ok := im.cache[path]; ok {
		return pkg, nil
	}
	if !im.owns(path) {
		return im.std.Import(path)
	}
	if im.loading[path] {
		err := fmt.Errorf("gosource: import cycle through %s", path)
		if im.cycle == nil {
			im.cycle = err
		}
		return nil, err
	}
	outermost := len(im.loading) == 0
	rel := strings.TrimPrefix(strings.TrimPrefix(path, im.modPath), "/")
	pkg, _, err := im.load(filepath.Join(im.modRoot, rel), path, Complete)
	if err != nil {
		return nil, err
	}
	if outermost && im.cycle != nil {
		cycle := im.cycle
		im.cycle = nil
		return nil, cycle
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
