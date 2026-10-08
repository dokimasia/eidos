// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"cmp"
	"maps"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"

	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// tree is the source tree every plan of one run routes against: the
// files each directory contains, which the layout asks for one directory
// at a time, and the toolchain modules the load resolved.
type tree struct {
	residents func(dir string) []plugin.Resident
	modules   []plugin.Module
}

// residentsOf returns the residents of each directory of a tree that a
// load read: the workspace files that the load's records place in the
// directory, each with the package its unit declared it in, sorted by
// file. A package's name is the name the graph declares it under. A file
// that no unit loaded has no resident. The function reads a directory the
// first time the layout asks for it and keeps it, so a run reads the
// directories of the files it writes and no other. The records are sorted
// by path, so a directory's files are one run of them.
//
// A run over a graph that no load read takes the residents that the graph
// lists, through [layout.Residents].
//
// # Concurrency
//
// The function is safe for concurrent use: a lock guards the directories
// it read, and the plans of one run ask from goroutines of their own.
func residentsOf(g *store.Graph, loaded *load.Report) func(dir string) []plugin.Resident {
	if loaded == nil {
		all := layout.Residents(g)
		return func(dir string) []plugin.Resident { return all[dir] }
	}
	files := loaded.Files
	var mu sync.Mutex
	read := map[string][]plugin.Resident{}
	return func(dir string) []plugin.Resident {
		mu.Lock()
		defer mu.Unlock()
		if rs, done := read[dir]; done {
			return rs
		}
		prefix := dir + "/"
		if dir == "." {
			prefix = ""
		}
		var rs []plugin.Resident
		for i := sort.Search(len(files), func(i int) bool { return files[i].Path >= prefix }); i < len(files) &&
			strings.HasPrefix(files[i].Path, prefix); i++ {
			f := files[i]
			if f.Pkg.IsZero() || path.Dir(f.Path) != dir {
				continue
			}
			if _, _, stored := plugin.CutStorePath(f.Path); stored {
				continue
			}
			pkg := f.Pkg
			if p, held := g.PackageOf(f.Pkg); held {
				pkg.Name = p.Name
			}
			rs = append(rs, plugin.Resident{File: f.Path, Pkg: pkg})
		}
		slices.SortFunc(rs, func(a, b plugin.Resident) int {
			return cmp.Or(strings.Compare(a.File, b.File), a.Pkg.Compare(b.Pkg))
		})
		read[dir] = rs
		return rs
	}
}

// recordedPackage returns the package of the record of a path among
// records sorted by path, and the zero identity where none records the
// path.
func recordedPackage(records []load.FileRecord, p string) symbol.Identity {
	at, found := slices.BinarySearchFunc(records, p, func(r load.FileRecord, p string) int {
		return strings.Compare(r.Path, p)
	})
	if !found {
		return symbol.Identity{}
	}
	return records[at].Pkg
}

// moduleOf returns the module that a package's two module facts name, and
// reports false for a package without both.
func moduleOf(facts *meta.Facts, p symbol.Identity, k meta.KernelKeys) (plugin.Module, bool) {
	module, named := meta.Get(facts, p, k.Module)
	root, rooted := meta.Get(facts, p, k.ModuleRoot)
	return plugin.Module{Lang: p.Lang, Path: module, Root: root}, named && rooted
}

// placements returns the edges of where the layout places the files of a
// plan, which the load changed: the residents edge of each directory
// where a file appeared, vanished or moved to another package, or whose
// package changed its name, and the files edge of each package that a file
// joined or left. The directories come first, sorted, and then the
// packages in identity order.
func (r *warmRun) placements() []state.EdgeHash {
	l := r.loaded
	dirs := map[string]struct{}{}
	pkgs := map[symbol.Identity]struct{}{}
	for _, p := range slices.Concat(l.Moved, l.Vanished) {
		before, after := recordedPackage(l.Was, p), recordedPackage(l.Files, p)
		if before == after {
			continue
		}
		dirs[path.Dir(p)] = struct{}{}
		for _, id := range []symbol.Identity{before, after} {
			if !id.IsZero() {
				pkgs[id] = struct{}{}
			}
		}
	}
	for _, id := range r.changes.Renamed {
		pkg, held := r.g.PackageOf(id)
		if !held {
			continue
		}
		for _, f := range pkg.Files {
			if f == nil {
				continue
			}
			if _, _, stored := plugin.CutStorePath(f.Path); !stored {
				dirs[path.Dir(f.Path)] = struct{}{}
			}
		}
	}
	out := make([]state.EdgeHash, 0, len(dirs)+len(pkgs))
	for _, d := range slices.Sorted(maps.Keys(dirs)) {
		out = append(out, state.DirectoryEdge(d))
	}
	for _, id := range slices.SortedFunc(maps.Keys(pkgs), symbol.Identity.Compare) {
		out = append(out, state.FilesEdge(id))
	}
	return out
}

// modules returns the count of the packages that name each module: the
// generation's count, with each package whose module facts changed, and
// each changed package that the graph no longer contains, counted again.
// It makes the modules edge dirty where the modules that the counts list
// differ. The count is never nil, so a caller tells it from a cold run's.
//
// Error modes: an error wrapping [state.ErrDamaged] for a row of the
// generation that does not read whole.
func (r *warmRun) modules() (map[plugin.Module]int, error) {
	prior, err := r.phases.Modules()
	if err != nil {
		return nil, err
	}
	counts := maps.Clone(prior)
	if counts == nil {
		counts = map[plugin.Module]int{}
	}
	k := r.w.kernel
	again := map[symbol.Identity]struct{}{}
	for f := range r.changed {
		if f.Key == k.Module.Name() || f.Key == k.ModuleRoot.Name() {
			again[f.Subject] = struct{}{}
		}
	}
	for _, p := range r.changes.Packages {
		if !r.g.Holds(p) {
			again[p] = struct{}{}
		}
	}
	for p := range again {
		if m, named := moduleOf(r.recorded, p, k); named {
			if counts[m]--; counts[m] <= 0 {
				delete(counts, m)
			}
		}
		if m, named := moduleOf(r.facts, p, k); named && r.g.Holds(p) {
			counts[m]++
		}
	}
	if !slices.Equal(layout.ModulesOf(prior), layout.ModulesOf(counts)) {
		if err := r.dirty.add(state.ModulesEdge); err != nil {
			return nil, err
		}
	}
	return counts, nil
}
