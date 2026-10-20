// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/internal/project"
	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// partition makes each module a unit of its own, because Python scopes a
// name to its module. A module is a source file, a stub file, or both files
// of one stem in one directory, and a unit lists its members in path order.
// The stub of a pair states the types of the module, and its source loads
// without declarations.
//
// Each member declares its shared inputs: the nearest pyproject.toml above
// its directory, and the modules of the package directories that place a
// file outside every source root of its project, which [project.Place]
// returns. A file in a directory that contains __init__.py or __init__.pyi
// among the claimed files is in a package directory. A project file that
// does not read or decode places no root, and the parse reports it. Every
// read folds into the keys of the units, so a project file that appears,
// changes or vanishes changes the keys of the files that it governs.
func partition(_ context.Context, files []plugin.SourceRef, r plugin.FileReader) ([][]plugin.SourceRef, error) {
	packages := map[string]bool{}
	for _, ref := range files {
		if stem := strings.TrimSuffix(path.Base(ref.Path), path.Ext(ref.Path)); stem == python.InitModule {
			packages[path.Dir(ref.Path)] = true
		}
	}
	isPackage := func(dir string) bool { return packages[dir] }
	seen := map[string]string{}
	projects := map[string]*project.Project{}
	modules := map[string]int{}
	var out [][]plugin.SourceRef
	for _, ref := range slices.SortedFunc(slices.Values(files), func(a, b plugin.SourceRef) int {
		return strings.Compare(a.Path, b.Path)
	}) {
		governing := frontend.Nearest(r, path.Dir(ref.Path), project.File, seen)
		proj, read := projects[governing]
		if !read && governing != "" {
			if data, err := r.Read(governing); err == nil {
				if p, err := project.Read(governing, data); err == nil {
					proj = &p
				}
			}
			projects[governing] = proj
		}
		_, probes := project.Place(ref.Path, proj, isPackage)
		var shared []string
		if governing != "" {
			shared = append(shared, governing)
		}
		member := plugin.SourceRef{Path: ref.Path, Shared: append(shared, probes...)}
		stem := strings.TrimSuffix(ref.Path, path.Ext(ref.Path))
		if i, paired := modules[stem]; paired {
			out[i] = append(out[i], member)
			continue
		}
		modules[stem] = len(out)
		out = append(out, []plugin.SourceRef{member})
	}
	return out, nil
}
