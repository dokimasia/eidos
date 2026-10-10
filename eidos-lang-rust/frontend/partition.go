// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"maps"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/plugin"
)

// rootDir is the workspace's root directory, where a manifest probe ends.
const rootDir = "."

// partition groups the claimed files into crate targets, because an
// inherent impl block anywhere in a crate adds methods to a type the
// crate declares, and the module tree starts at the crate root. Each
// file belongs to the package of the nearest Cargo.toml above it, which
// is every member's shared input, and to the target Cargo's layout and
// the manifest place it in. A unit's first member is its crate root. A
// shared module directory under tests, examples or benches that no
// target roots is a unit of its own, its mod.rs first. A file without a
// Cargo.toml above it is a crate of its own. A manifest that does not
// parse, and one that names no package, as a virtual workspace manifest
// does, states no target, so each of its files is a unit of its own,
// and the parse reports a manifest that does not parse.
func partition(_ context.Context, files []plugin.SourceRef, r plugin.FileReader) ([][]plugin.SourceRef, error) {
	nearest := map[string]string{}
	byManifest := map[string][]string{}
	var loose []string
	for _, ref := range files {
		m := governing(r, path.Dir(ref.Path), nearest)
		if m == "" {
			loose = append(loose, ref.Path)
			continue
		}
		byManifest[m] = append(byManifest[m], ref.Path)
	}
	var out [][]plugin.SourceRef
	for _, mpath := range slices.Sorted(maps.Keys(byManifest)) {
		data, _ := r.Read(mpath)
		if m, err := parseManifest(data); err == nil && m.crateName() != "" {
			out = append(out, units(mpath, m, byManifest[mpath])...)
			continue
		}
		for _, f := range byManifest[mpath] {
			out = append(out, []plugin.SourceRef{{Path: f, Shared: []string{mpath}}})
		}
	}
	for _, f := range loose {
		out = append(out, []plugin.SourceRef{{Path: f}})
	}
	return out, nil
}

// governing returns the nearest Cargo.toml above a directory, and empty
// where no directory up to the root has one. It caches the result for
// every directory the probe visited.
func governing(r plugin.FileReader, dir string, nearest map[string]string) string {
	var visited []string
	found := ""
	for at := dir; ; at = path.Dir(at) {
		if m, met := nearest[at]; met {
			found = m
			break
		}
		visited = append(visited, at)
		candidate := path.Join(at, manifestName)
		if _, err := r.Read(candidate); err == nil {
			found = candidate
			break
		}
		if at == rootDir {
			break
		}
	}
	for _, at := range visited {
		nearest[at] = found
	}
	return found
}

// units groups one package's files into its targets' units, each
// member declaring the manifest as its shared input and each unit's
// crate root first.
func units(mpath string, m *manifest, files []string) [][]plugin.SourceRef {
	dir := path.Dir(mpath)
	rel := make(map[string]bool, len(files))
	for _, f := range files {
		rel[relative(dir, f)] = true
	}
	targets := m.targets(rel)
	groups := map[string][]string{}
	for _, f := range files {
		t := owner(targets, m.crateName(), relative(dir, f))
		groups[t.root] = append(groups[t.root], f)
	}
	out := make([][]plugin.SourceRef, 0, len(groups))
	for _, root := range slices.Sorted(maps.Keys(groups)) {
		members := groups[root]
		first := path.Join(dir, root)
		if !rel[root] {
			first = path.Join(dir, root, modFile)
		}
		slices.SortFunc(members, func(a, b string) int {
			switch {
			case a == first:
				return -1
			case b == first:
				return 1
			default:
				return strings.Compare(a, b)
			}
		})
		unit := make([]plugin.SourceRef, 0, len(members))
		for _, f := range members {
			unit = append(unit, plugin.SourceRef{Path: f, Shared: []string{mpath}})
		}
		out = append(out, unit)
	}
	return out
}

// relative returns a file's path relative to a package's directory. A
// workspace path never opens with ./, so a file below the root is its
// own path.
func relative(dir, f string) string {
	return strings.TrimPrefix(f, dir+"/")
}
