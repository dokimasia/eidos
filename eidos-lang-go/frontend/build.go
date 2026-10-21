// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"

	"go.dokimi.dev/eidos/sdk/plugin"
)

// sumFile is the file beside a go.mod that records the hashes of the
// modules its build needs.
const sumFile = "go.sum"

// parentDir is the path element that leaves a directory for its
// parent, which a directory replacement outside the workspace opens
// with once it is joined and cleaned.
const parentDir = ".."

// buildList is the build list that the go.mod files of a dependency round
// select. It has the parsed go.mod files, the workspace module that each
// declares, the selection of every module that they require, and the tree
// hashes that their go.sum files record.
type buildList struct {
	goMods    []string
	files     map[string]*modfile.File
	workspace []moduleRoot
	selected  map[string]selection
	sums      map[module.Version]string
}

// selection is one required module at the highest version any
// workspace go.mod requires, the go.mod that requires that version,
// and the tree that provides the module after that go.mod's
// replacements.
type selection struct {
	version string
	goMod   string
	// tree is the module the tree is: a replacement's, or the
	// required module's own.
	tree module.Version
	// dir is a directory replacement as its go.mod spells it, empty
	// for none, and outside reports whether it leaves the workspace.
	dir     string
	outside bool
}

// readBuild reads the build list through a round's door, from the
// go.mod files a round lists as its shared inputs, in their order. A
// workspace go.mod is a main module's, whose replace directives apply,
// so it parses strictly: modfile.ParseLax drops them, because it reads
// a dependency's go.mod. A go.mod that does not parse is an error, and
// a go.sum that does not exist records nothing. Where two go.mod files
// require one module at one version, the first selects it.
func readBuild(r plugin.FileReader, goMods []string) (*buildList, error) {
	b := &buildList{
		goMods:   goMods,
		files:    map[string]*modfile.File{},
		selected: map[string]selection{},
		sums:     map[module.Version]string{},
	}
	for _, goMod := range goMods {
		data, err := r.Read(goMod)
		if err != nil {
			return nil, err
		}
		f, err := modfile.Parse(goMod, data, nil)
		if err != nil {
			return nil, fmt.Errorf("frontend: %w", err)
		}
		b.files[goMod] = f
		if f.Module != nil {
			b.workspace = append(b.workspace, moduleRoot{dir: path.Dir(goMod), module: f.Module.Mod.Path})
		}
		for _, req := range f.Require {
			prior, met := b.selected[req.Mod.Path]
			if met && semver.Compare(req.Mod.Version, prior.version) <= 0 {
				continue
			}
			b.selected[req.Mod.Path] = selectionOf(goMod, f, req.Mod)
		}
		sums, err := r.Read(path.Join(path.Dir(goMod), sumFile))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		parseSums(sums, b.sums)
	}
	return b, nil
}

// selectionOf returns a requirement's selection under the replacements
// of the go.mod that requires it. A replacement of the required
// version takes precedence over a replacement of every version, as in
// the go command.
func selectionOf(goMod string, f *modfile.File, mod module.Version) selection {
	s := selection{version: mod.Version, goMod: goMod, tree: mod}
	var replacement *modfile.Replace
	for _, r := range f.Replace {
		if r.Old.Path != mod.Path {
			continue
		}
		if r.Old.Version == mod.Version {
			replacement = r
			break
		}
		if r.Old.Version == "" {
			replacement = r
		}
	}
	if replacement == nil {
		return s
	}
	if replacement.New.Version != "" {
		s.tree = replacement.New
		return s
	}
	s.dir = replacement.New.Path
	joined := path.Join(path.Dir(goMod), filepath.ToSlash(replacement.New.Path))
	s.outside = filepath.IsAbs(replacement.New.Path) || path.IsAbs(joined) ||
		joined == parentDir || strings.HasPrefix(joined, parentDir+"/")
	return s
}

// workspaceModule returns the workspace module whose path is the longest
// prefix of an import path, and false where no workspace go.mod declares
// a module that the import path is in.
func (b *buildList) workspaceModule(importPath string) (moduleRoot, bool) {
	var best moduleRoot
	for _, root := range b.workspace {
		if within(importPath, root.module) && len(root.module) > len(best.module) {
			best = root
		}
	}
	return best, best.module != ""
}

// generated reports whether an output of a round is in the workspace
// directory of the package of an import path. The directory is below the
// workspace module whose path is the longest prefix of the import path,
// or below the directory that replaces the selected module of the import
// path.
func (b *buildList) generated(importPath string, outputs []string) bool {
	root, declared := b.workspaceModule(importPath)
	if !declared {
		mod, s, selected := b.moduleOf(importPath)
		if !selected || s.dir == "" {
			return false
		}
		root = moduleRoot{dir: path.Join(path.Dir(s.goMod), filepath.ToSlash(s.dir)), module: mod}
	}
	dir := path.Join(root.dir, strings.TrimPrefix(strings.TrimPrefix(importPath, root.module), "/"))
	return slices.ContainsFunc(outputs, func(o string) bool { return path.Dir(o) == dir })
}

// moduleOf returns the selected module whose path is the longest
// prefix of an import path, and false where no selected module
// prefixes it.
func (b *buildList) moduleOf(importPath string) (string, selection, bool) {
	best := ""
	for mod := range b.selected {
		if within(importPath, mod) && len(mod) > len(best) {
			best = mod
		}
	}
	s, met := b.selected[best]
	return best, s, met
}

// within reports whether an import path is a module's path or a path
// under it.
func within(importPath, mod string) bool {
	return importPath == mod || strings.HasPrefix(importPath, mod+"/")
}
