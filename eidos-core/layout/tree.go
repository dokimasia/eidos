// Copyright Dokimasia B.V. 2026
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
//
// Residents counts each directory's files before it places any, so
// every directory's residents share one backing array. Each
// directory's slice is capped at its own residents, so an append to
// it copies the slice and leaves the next directory's intact.
//
// # Allocation contract
//
// Residents allocates the map it returns and the backing array. Past
// eight directories it also allocates the map that counts each
// directory's files: 3 allocations for four directories, and 27 for
// 1,000 directories.
func Residents(g *store.Graph) map[string][]plugin.Resident {
	counts := map[string]int{}
	total := 0
	for p := range g.Packages() {
		for _, f := range p.Files {
			if f != nil && workspaceFile(f.Path) {
				counts[path.Dir(f.Path)]++
				total++
			}
		}
	}
	all := make([]plugin.Resident, total)
	out := make(map[string][]plugin.Resident, len(counts))
	start := 0
	for dir, n := range counts {
		out[dir] = all[start : start : start+n]
		start += n
	}
	for p := range g.Packages() {
		pkg := p.ID
		pkg.Name = p.Name
		for _, f := range p.Files {
			if f != nil && workspaceFile(f.Path) {
				dir := path.Dir(f.Path)
				out[dir] = append(out[dir], plugin.Resident{File: f.Path, Pkg: pkg})
			}
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
// contains no workspace directory and is absent. A tree without a
// module returns nil.
//
// # Allocation contract
//
// Modules allocates the list it returns. Past eight modules it also
// allocates the set that removes duplicates: 1 allocation for up to
// eight modules, and none for a tree without a module.
func Modules(g *store.Graph, facts *meta.Facts, k meta.KernelKeys) []plugin.Module {
	seen := map[plugin.Module]struct{}{}
	for p := range g.Packages() {
		module, named := meta.Get(facts, p.ID, k.Module)
		root, rooted := meta.Get(facts, p.ID, k.ModuleRoot)
		if named && rooted && workspaceFile(root) {
			seen[plugin.Module{Lang: p.ID.Lang, Path: module, Root: root}] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]plugin.Module, 0, len(seen))
	for m := range seen {
		out = append(out, m)
	}
	slices.SortFunc(out, compareModules)
	return out
}

// ModulesOf returns the modules of a count of packages by module, in the
// order [Modules] returns them: each module that at least one package
// names, rooted in the workspace. A warm run keeps such a count, and reads
// the modules off it without a walk of the graph. A count without such a
// module returns nil.
//
// # Allocation contract
//
// ModulesOf counts the modules first, and allocates the list it returns at
// that length, and nothing for a count without a module.
func ModulesOf(counts map[plugin.Module]int) []plugin.Module {
	n := 0
	for m, c := range counts {
		if c > 0 && workspaceFile(m.Root) {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	out := make([]plugin.Module, 0, n)
	for m, c := range counts {
		if c > 0 && workspaceFile(m.Root) {
			out = append(out, m)
		}
	}
	slices.SortFunc(out, compareModules)
	return out
}

// compareModules orders modules innermost root first: a module whose root
// is below another's precedes it, and modules at one depth sort by root,
// then language, then path.
func compareModules(a, b plugin.Module) int {
	return cmp.Or(
		cmp.Compare(depth(b.Root), depth(a.Root)),
		strings.Compare(a.Root, b.Root),
		cmp.Compare(a.Lang, b.Lang),
		strings.Compare(a.Path, b.Path),
	)
}

// depth counts a workspace-relative directory's elements, zero for the
// tree's root.
func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}
