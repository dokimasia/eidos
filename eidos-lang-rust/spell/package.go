// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"fmt"
	"path"
	"slices"
	"strings"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// modFile is the file of a directory module.
const modFile = "mod" + rust.Extension

// Package names the module a Rust file at a routed path declares, the
// way the Rust frontend names a file module: the module the
// directory's files belong to, joined with the file's stem. The
// directory's module is the module of a mod.rs or a crate root in it,
// and the parent of any other file's module. A crate root is the file
// whose module is a crate the load resolved, one of the placement's
// Rust modules, so a crate of its own, named for its path, is told
// apart from a file module. A directory without a Rust file belongs to
// no module the load knows, and Package returns an error naming the
// directory.
func Package(p plugin.Placement) (symbol.Identity, error) {
	dir := path.Dir(p.Path)
	parent, found := directoryModule(p)
	if !found {
		return symbol.Identity{}, fmt.Errorf("spell: no Rust module is declared in %s", dir)
	}
	stem := strings.TrimSuffix(path.Base(p.Path), rust.Extension)
	return symbol.Identity{
		Lang: rust.Lang, Package: parent + "/" + stem, Name: stem, Kind: symbol.KindPackage,
	}, nil
}

// directoryModule returns the module whose file modules a routed
// directory's Rust files are, and false where the directory has none.
// A mod.rs names the directory's module itself, and so does a crate
// root.
func directoryModule(p plugin.Placement) (string, bool) {
	var parent string
	found := false
	for _, r := range p.Residents {
		if r.Pkg.Lang != rust.Lang {
			continue
		}
		module := r.Pkg.Package
		if path.Base(r.File) == modFile || crate(p.Modules, module) {
			return module, true
		}
		if !found {
			parent, found = path.Dir(module), true
		}
	}
	return parent, found
}

// crate reports whether a module path is the root module of a crate the
// load resolved.
func crate(modules []plugin.Module, module string) bool {
	return slices.ContainsFunc(modules, func(m plugin.Module) bool {
		return m.Lang == rust.Lang && m.Path == module
	})
}
