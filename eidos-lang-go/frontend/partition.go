// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"fmt"
	"path"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"

	golang "go.dokimi.dev/eidos/lang/go"
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

// importPath derives the path a directory's package loads under,
// through the derivation the backend routes a written file's package
// by.
func (r moduleRoot) importPath(dir string) string {
	return golang.ImportPath(r.module, r.dir, dir)
}

// unitPlace is where a unit's package directory loads: its import
// path, the module that governs a workspace directory, and whether the
// directory is in the standard library, whose files import the
// packages it vendors under vendor/.
type unitPlace struct {
	importPath string
	root       moduleRoot
	std        bool
}

// placeUnit derives where a unit's package directory loads from its
// first member's path, through the unit's door. A dependency unit
// loads under the import path the go command gives its directory,
// governed by no workspace module:
//
//   - A directory of [GoRootStore] is the standard library package
//     its path inside the store names.
//   - A directory of [ModCacheStore] is the package of the module its
//     path escapes, under the module's path, or under the original
//     path of a module the member's go.mod replaces by it.
//   - A workspace directory under the vendor directory beside the
//     member's go.mod is the package its path under vendor/ names.
//
// Every other directory is the workspace's own, under the governing
// go.mod's module path.
func placeUnit(u *plugin.SourceUnit) (unitPlace, error) {
	first := u.Files()[0]
	if store, inner, qualified := plugin.CutStorePath(first.Path); qualified {
		dir := path.Dir(inner)
		switch store {
		case GoRootStore:
			return unitPlace{importPath: dir, std: true}, nil
		case ModCacheStore:
			importPath, err := cachedImportPath(u, dir, first.Shared)
			return unitPlace{importPath: importPath}, err
		default:
			return unitPlace{}, fmt.Errorf("frontend: %s is in store %s, which the Go frontend does not read",
				first.Path, store)
		}
	}
	dir := path.Dir(first.Path)
	if len(first.Shared) == 1 {
		vendorRoot := path.Join(path.Dir(first.Shared[0]), vendorDir) + "/"
		if rel, vendored := strings.CutPrefix(dir, vendorRoot); vendored {
			return unitPlace{importPath: rel}, nil
		}
	}
	root := (&moduleProbe{reader: unitReader{u}, roots: map[string]moduleRoot{}}).governing(dir)
	return unitPlace{importPath: root.importPath(dir), root: root}, nil
}

// cachedImportPath derives the import path of a module cache
// directory: the module path and version its first elements escape,
// and the directories under the module. A module the member's go.mod
// replaces by that module version keeps the original path, which the
// go.mod states and the parse reads through the unit's door.
func cachedImportPath(u *plugin.SourceUnit, dir string, shared []string) (string, error) {
	escPath, rest, marked := strings.Cut(dir, versionMark)
	if !marked {
		return "", fmt.Errorf("frontend: %s names no module version in the module cache", dir)
	}
	escVersion, rel, _ := strings.Cut(rest, "/")
	modPath, err := module.UnescapePath(escPath)
	if err != nil {
		return "", fmt.Errorf("frontend: %w", err)
	}
	version, err := module.UnescapeVersion(escVersion)
	if err != nil {
		return "", fmt.Errorf("frontend: %w", err)
	}
	importPath := modPath
	if len(shared) == 1 {
		data, err := u.Read(shared[0])
		if err != nil {
			return "", err
		}
		f, err := modfile.Parse(shared[0], data, nil)
		if err != nil {
			return "", fmt.Errorf("frontend: %w", err)
		}
		for _, r := range f.Replace {
			if r.New.Path == modPath && r.New.Version == version {
				importPath = r.Old.Path
				break
			}
		}
	}
	if rel == "" {
		return importPath, nil
	}
	return importPath + "/" + rel, nil
}

// modulePath reads the module directive out of go.mod bytes: the
// first `module` line, its path bare or quoted. The partition needs the
// directive alone. A go.mod the go tool would refuse derives an empty
// path, and the directory keeps its workspace spelling.
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
