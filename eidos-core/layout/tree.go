// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
)

// Residents returns the source files of the workspace tree by
// directory, each with the package that declares it: the index a
// target's package rule reads a routed directory through. The
// package's Name is the name its files declare it under, and each
// directory's residents are sorted by file, then package. A file of a
// dependency store is absent.
func Residents(g *store.Graph) map[string][]plugin.Resident {
	out := map[string][]plugin.Resident{}
	for p := range g.Packages() {
		pkg := p.ID
		pkg.Name = p.Name
		for _, f := range p.Files {
			if f == nil || !workspaceFile(f.Path) {
				continue
			}
			dir := path.Dir(f.Path)
			out[dir] = append(out[dir], plugin.Resident{File: f.Path, Pkg: pkg})
		}
	}
	for _, rs := range out {
		slices.SortFunc(rs, func(a, b plugin.Resident) int {
			return cmp.Or(strings.Compare(a.File, b.File), a.Pkg.Compare(b.Pkg))
		})
	}
	return out
}

// Modules returns the toolchain modules the load resolved, read off the
// module facts the frontends stamp on packages, innermost root first: a
// module whose root is below another's precedes it, and modules at one
// depth sort by root, then language, then path. A package without both
// facts declares no module, and a module rooted in a dependency store
// contains no workspace directory and is absent.
func Modules(g *store.Graph, facts *meta.Facts, k meta.KernelKeys) []plugin.Module {
	seen := map[plugin.Module]struct{}{}
	var out []plugin.Module
	for p := range g.Packages() {
		module, named := meta.Get(facts, p.ID, k.Module)
		root, rooted := meta.Get(facts, p.ID, k.ModuleRoot)
		if !named || !rooted || !workspaceFile(root) {
			continue
		}
		m := plugin.Module{Lang: p.ID.Lang, Path: module, Root: root}
		if _, listed := seen[m]; listed {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b plugin.Module) int {
		return cmp.Or(
			cmp.Compare(depth(b.Root), depth(a.Root)),
			strings.Compare(a.Root, b.Root),
			cmp.Compare(a.Lang, b.Lang),
			strings.Compare(a.Path, b.Path),
		)
	})
	return out
}

// depth counts a workspace-relative directory's elements, zero for the
// tree's root.
func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}
