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
	nearby := projectFiles{r: r}
	modules := map[string]int{}
	var out [][]plugin.SourceRef
	for _, ref := range slices.SortedFunc(slices.Values(files), func(a, b plugin.SourceRef) int {
		return strings.Compare(a.Path, b.Path)
	}) {
		governing, proj := nearby.nearest(path.Dir(ref.Path))
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

// projectFiles reads the project file nearest above each directory through
// the door r, and decodes each path once. A project file that does not
// read or decode has a nil project. A projectFiles with r set and no maps
// is ready for use.
type projectFiles struct {
	r        plugin.FileReader
	seen     map[string]string
	projects map[string]*project.Project
}

// nearest returns the path of the project file nearest above a directory,
// and its project. The path is empty for a directory below no project file.
// The project is nil for an empty path and for a project file that does
// not read or decode.
func (f *projectFiles) nearest(dir string) (string, *project.Project) {
	if f.seen == nil {
		f.seen, f.projects = map[string]string{}, map[string]*project.Project{}
	}
	governing := frontend.Nearest(f.r, dir, project.File, f.seen)
	proj, read := f.projects[governing]
	if !read && governing != "" {
		if data, err := f.r.Read(governing); err == nil {
			if p, err := project.Read(governing, data); err == nil {
				proj = &p
			}
		}
		f.projects[governing] = proj
	}
	return governing, proj
}
