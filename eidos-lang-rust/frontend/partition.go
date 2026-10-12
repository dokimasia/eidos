// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"maps"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// rootDir is the directory that path.Dir returns for a path at the root of
// the workspace.
const rootDir = "."

// partition groups the claimed files into crate targets, because an
// inherent impl block anywhere in a crate adds methods to a type of the
// crate, and the module tree starts at the crate root. Each file belongs
// to the package of the nearest Cargo.toml above it, which is the shared
// input of every member, and to the target in which the layout of Cargo
// and the manifest place it. The first member of a unit is its crate
// root. A shared module directory under tests, examples or benches that no
// target roots is a unit of its own, with its mod.rs first. A file without
// a Cargo.toml above it is a crate of its own. A manifest that does not
// parse, and a manifest that names no package, as a virtual workspace
// manifest does, states no target. Each of its files is then a unit of its
// own, and the parse reports a manifest that does not parse.
func partition(_ context.Context, files []plugin.SourceRef, r plugin.FileReader) ([][]plugin.SourceRef, error) {
	seen := map[string]string{}
	byManifest := map[string][]string{}
	var loose []string
	for _, ref := range files {
		m := frontend.Nearest(r, path.Dir(ref.Path), manifestName, seen)
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

// units groups the files of one package into the units of its targets.
// Each member declares the manifest as its shared input, and the crate
// root of each unit comes first.
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

// relative returns the path of a file relative to the directory of a
// package. A workspace path never starts with ./, so the path of a file
// below the root is its own path.
func relative(dir, f string) string {
	return strings.TrimPrefix(f, dir+"/")
}
