// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"path"
	"strings"

	"go.dokimi.dev/eidos/sdk/plugin"
)

// modFile is the shared input governing a unit's import paths.
const modFile = "go.mod"

// partition groups the claimed files into package directories and
// derives each unit's import path from the governing go.mod: the
// module directive plus the directory's path under the module
// root. A directory outside any module keeps its workspace path,
// which is what a fixture tree without modules loads under. Every
// go.mod probe that reads folds into the resulting unit keys, so a
// module file appearing or vanishing re-keys what it governs.
func (*goFrontend) partition(
	_ context.Context, files []plugin.SourceRef, r plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	byDir := map[string][]plugin.SourceRef{}
	var dirs []string
	for _, ref := range files {
		dir := path.Dir(ref.Path)
		if _, met := byDir[dir]; !met {
			dirs = append(dirs, dir)
		}
		byDir[dir] = append(byDir[dir], ref)
	}

	modules := &moduleProbe{reader: r, roots: map[string]moduleRoot{}}
	out := make([][]plugin.SourceRef, 0, len(dirs))
	for _, dir := range dirs {
		root := modules.governing(dir)
		unit := make([]plugin.SourceRef, 0, len(byDir[dir]))
		for _, ref := range byDir[dir] {
			unit = append(unit, plugin.SourceRef{Path: ref.Path, Shared: root.shared})
		}
		out = append(out, unit)
	}
	return out, nil
}

// moduleRoot is one resolved go.mod: its directory, the module it
// names, and the shared-input list of every governed ref.
type moduleRoot struct {
	dir    string
	module string
	shared []string
}

// moduleProbe finds and caches governing modules: one read per
// go.mod, and one probe per directory, however many package
// directories share an ancestry.
type moduleProbe struct {
	reader plugin.FileReader
	roots  map[string]moduleRoot
}

// governing walks a directory's ancestry for the nearest go.mod and
// caches the answer for every directory the walk visited, because
// each of them has the same nearest go.mod, so a sibling's walk ends
// at the first ancestor a previous walk visited. A probe that misses
// is not recorded, because it read nothing, and the cache lasts one
// partition: the load that finds a new go.mod reads it, and the read
// re-keys the unit.
func (p *moduleProbe) governing(dir string) moduleRoot {
	var visited []string
	at := dir
	for {
		if root, met := p.roots[at]; met {
			p.remember(visited, root)
			return root
		}
		visited = append(visited, at)
		candidate := path.Join(at, modFile)
		if b, err := p.reader.Read(candidate); err == nil {
			root := moduleRoot{
				dir:    at,
				module: modulePath(string(b)),
				shared: []string{candidate},
			}
			p.remember(visited, root)
			return root
		}
		if at == "." {
			break
		}
		at = path.Dir(at)
	}
	root := moduleRoot{}
	p.remember(visited, root)
	return root
}

// remember caches one walk's answer for every directory it visited.
func (p *moduleProbe) remember(dirs []string, root moduleRoot) {
	for _, dir := range dirs {
		p.roots[dir] = root
	}
}

// importPath derives the path a directory's package loads under.
func (r moduleRoot) importPath(dir string) string {
	if r.module == "" {
		return dir
	}
	if dir == r.dir {
		return r.module
	}
	return r.module + "/" + strings.TrimPrefix(dir, r.dir+"/")
}

// modulePath reads the module directive out of go.mod bytes: the
// first `module` line, its path bare or quoted. The subset is
// deliberate — the directive is all the partition needs, and a
// go.mod the go tool would refuse derives an empty path and the
// directory keeps its workspace spelling.
func modulePath(src string) string {
	for line := range strings.SplitSeq(src, "\n") {
		line = strings.TrimSpace(line)
		if comment := strings.Index(line, "//"); comment >= 0 {
			line = strings.TrimSpace(line[:comment])
		}
		rest, found := strings.CutPrefix(line, "module")
		if !found || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		spelled := strings.TrimSpace(rest)
		return strings.Trim(spelled, `"`)
	}
	return ""
}
