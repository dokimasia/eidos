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
	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// modFile is the name of the module file, the shared input that governs
// the import paths of a unit.
const modFile = "go.mod"

// partition groups the claimed files into package directories. Each member
// declares the nearest go.mod above its directory as its shared input,
// which states the module of the package. A directory outside every module
// declares none and keeps its workspace path, under which a fixture tree
// without modules loads. Every probe for a go.mod that reads folds into
// the resulting unit keys, so a module file that appears or vanishes
// re-keys what it governs. The members of one module share one list of
// shared inputs.
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
	seen := map[string]string{}
	shared := map[string][]string{}
	out := make([][]plugin.SourceRef, 0, len(dirs))
	for _, dir := range dirs {
		gomod := frontend.Nearest(r, dir, modFile, seen)
		if _, met := shared[gomod]; !met && gomod != "" {
			shared[gomod] = []string{gomod}
		}
		unit := make([]plugin.SourceRef, 0, len(byDir[dir]))
		for _, ref := range byDir[dir] {
			unit = append(unit, plugin.SourceRef{Path: ref.Path, Shared: shared[gomod]})
		}
		out = append(out, unit)
	}
	return out, nil
}

// moduleRoot is one resolved go.mod: its directory and the path of its
// module.
type moduleRoot struct {
	dir    string
	module string
}

// importPath derives the import path of the package of a directory,
// through the derivation by which the backend routes the package of a
// written file.
func (r moduleRoot) importPath(dir string) string {
	return golang.ImportPath(r.module, r.dir, dir)
}

// unitPlace is the place where the package directory of a unit loads: its
// import path, the module that governs a workspace directory, and whether
// the directory is in the standard library, whose files import the
// packages that it vendors under vendor/.
type unitPlace struct {
	importPath string
	root       moduleRoot
	std        bool
}

// placeUnit derives the place where the package directory of a unit loads
// from the path of its first member, through the door of the unit. A
// dependency unit loads under the import path that the go command gives
// its directory, and no workspace module governs it:
//
//   - A directory of [GoRootStore] is the standard library package of its
//     path inside the store.
//   - A directory of [ModCacheStore] is the package of the module that
//     its path escapes, under the path of the module, or under the
//     original path of a module that the go.mod of the member replaces by
//     it.
//   - A workspace directory under the vendor directory beside the go.mod
//     of the member is the package of its path under vendor/.
//
// Every other directory belongs to the workspace, under the module path of
// its shared go.mod. A go.mod that does not read leaves the directory
// under its workspace path.
//
// Error modes: placeUnit returns an error for a directory in a store that
// the Go frontend does not read, and the error of cachedImportPath.
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
	if len(first.Shared) == 0 {
		return unitPlace{importPath: dir}, nil
	}
	gomod := first.Shared[0]
	vendorRoot := path.Join(path.Dir(gomod), vendorDir) + "/"
	if rel, vendored := strings.CutPrefix(dir, vendorRoot); vendored {
		return unitPlace{importPath: rel}, nil
	}
	root := moduleRoot{dir: path.Dir(gomod)}
	if data, err := u.Read(gomod); err == nil {
		root.module = modulePath(string(data))
	}
	return unitPlace{importPath: root.importPath(dir), root: root}, nil
}

// cachedImportPath derives the import path of a directory of the module
// cache: the module path and the version that its first elements escape,
// and the directories under the module. A module that the go.mod of the
// member replaces by that module version keeps the original path, which
// the go.mod states and the parse reads through the door of the unit.
//
// Error modes: cachedImportPath returns an error for a directory without a
// module version, for a path or a version that does not unescape, for a
// go.mod that does not read, and for a go.mod that does not parse.
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

// modulePath reads the module directive out of the bytes of a go.mod: the
// first module line, with its path bare or quoted. The partition needs the
// directive alone. A go.mod that the go tool would refuse derives an empty
// path, and the directory keeps its workspace path.
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
