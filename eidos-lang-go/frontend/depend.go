// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"golang.org/x/mod/module"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The module cache keeps its downloads under cacheDownload. Beside the
// download of a module, a .ziphash record has the hash that the go command
// verified the tree with, and a .partial record marks a download that the
// go command has not finished.
const (
	cacheDownload = "cache/download"
	zipHashSuffix = ".ziphash"
	partialSuffix = ".partial"
)

// versionMark separates the escaped path of a module from its escaped
// version in the name of its directory in the module cache.
const versionMark = "@"

// testSuffix ends the name of a test file. A dependency unit leaves test
// files out.
const testSuffix = "_" + testElement + golang.Extension

// cacheState is what the module cache provides for one module.
type cacheState uint8

// The module cache has the tree of the module, lacks the tree or has an
// unfinished download of it, or is a store that the load does not provide.
const (
	cacheHas   cacheState = 1
	cacheLacks cacheState = 2
	cacheOff   cacheState = 3
)

// dependencies is the dependency round of the Go frontend. It reads the
// build list from the go.mod files of the round and places the package of
// each use of the round once: in the standard library, in the module cache
// at the selected version, or in the vendor tree of a workspace module
// where the cache lacks that version. Each placed package is one unit: the
// .go files of the package directory without its tests. A need without a
// use places nothing, because no declaration and no directive of the load
// names its package.
//
// A package that the build places nowhere yields no unit. Where the
// package is a need of the round, the round reports it with one of these
// reasons:
//
//   - a workspace module declares it, and its packages load from the
//     selection alone;
//   - no selected module provides it, which the go command refuses as
//     provided by no required module;
//   - a module or the standard library lacks the package directory;
//   - the load does not provide the store, which turns its source off.
//
// The round passes over a need whose package directory contains an output
// of the round, because the brand writes the package. The directory is
// below the workspace module that declares the need, or below the
// workspace directory that replaces its module.
//
// The import "C" refers to the preamble of cgo and to no package, so the
// round passes over a use in it. These are errors that name the go command
// that settles them:
//
//   - a module whose hash no go.sum file records;
//   - a module whose cached hash differs from the hash of go.sum;
//   - a directory replacement outside the workspace;
//   - an inconsistent vendor tree;
//   - a module that neither the cache nor a vendor tree provides.
func (*goFrontend) dependencies(
	ctx context.Context, round *plugin.DependencyRound, r plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	b, err := readBuild(r, round.Shared)
	if err != nil {
		return nil, err
	}
	needs := make(map[string]bool, len(round.Needs))
	for _, need := range round.Needs {
		needs[need.Path] = true
	}
	placed := map[string]bool{cgoImport: true}
	var out [][]plugin.SourceRef
	for _, use := range round.Uses {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if placed[use.Package] {
			continue
		}
		placed[use.Package] = true
		unit, reason, err := b.place(r, use.Package)
		switch {
		case err != nil:
			return nil, err
		case len(unit) > 0:
			out = append(out, unit)
		case needs[use.Package] && !b.generated(use.Package, round.Outputs):
			round.Unplace(use.Package, reason)
		}
	}
	return out, nil
}

// place returns the unit that declares one import path. For a need that
// the build places nowhere, it returns no unit and the reason.
func (b *buildList) place(r plugin.StoreReader, importPath string) ([]plugin.SourceRef, string, error) {
	if golang.Standard(importPath) {
		return packageUnit(r, plugin.StorePath(GoRootStore, importPath), nil)
	}
	if _, declared := b.workspaceModule(importPath); declared {
		return nil, "a workspace module declares it, and the selection claims no file of its package", nil
	}
	mod, s, selected := b.moduleOf(importPath)
	if !selected {
		return nil, "no module the go.mod files require provides it", nil
	}
	if s.dir != "" {
		if s.outside {
			return nil, "", fmt.Errorf("frontend: %s replaces %s with the directory %s, "+
				"which is outside the workspace and in no store", s.goMod, mod, s.dir)
		}
		return nil, fmt.Sprintf("%s replaces %s with the workspace directory %s, "+
			"and the selection claims no file of its package", s.goMod, mod, s.dir), nil
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(importPath, mod), "/")
	unit, reason, state, err := b.fromCache(r, s, rel)
	if err != nil || state == cacheHas {
		return unit, reason, err
	}
	unit, reason, vendored, err := b.fromVendor(r, module.Version{Path: mod, Version: s.version}, importPath)
	if err != nil || vendored {
		return unit, reason, err
	}
	if state == cacheOff {
		return nil, fmt.Sprintf("the load provides no %s store, and no vendor tree has %s", ModCacheStore, s.tree), nil
	}
	return nil, "", fmt.Errorf("frontend: %s requires %s@%s, and neither the module cache nor a vendor tree "+
		"provides it: run go mod download %s@%s", s.goMod, mod, s.version, s.tree.Path, s.tree.Version)
}

// fromCache returns the unit of a package directory from the tree of the
// selected module in the module cache, and what the cache provides for the
// module. It returns the reason where the tree has no unit for the
// directory. By the rule of the go command, the cache has the module where
// its tree and its hash record exist and no mark of an unfinished download
// does. The text of the hash record is then the hash that go.sum records
// for the tree, or the round fails.
func (b *buildList) fromCache(
	r plugin.StoreReader, s selection, rel string,
) ([]plugin.SourceRef, string, cacheState, error) {
	escPath, err := module.EscapePath(s.tree.Path)
	if err != nil {
		return nil, "", cacheLacks, fmt.Errorf("frontend: %w", err)
	}
	escVersion, err := module.EscapeVersion(s.tree.Version)
	if err != nil {
		return nil, "", cacheLacks, fmt.Errorf("frontend: %w", err)
	}
	download := plugin.StorePath(ModCacheStore, cacheDownload+"/"+escPath+"/@v/"+escVersion)
	hash, err := r.Read(download + zipHashSuffix)
	if errors.Is(err, plugin.ErrStoreAbsent) {
		return nil, "", cacheOff, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", cacheLacks, nil
	}
	if err != nil {
		return nil, "", cacheLacks, err
	}
	if _, partialErr := r.Read(download + partialSuffix); !plugin.Absent(partialErr) {
		return nil, "", cacheLacks, partialErr
	}
	root := plugin.StorePath(ModCacheStore, escPath+versionMark+escVersion)
	if _, rootErr := r.ReadDir(root); rootErr != nil {
		if plugin.Absent(rootErr) {
			return nil, "", cacheLacks, nil
		}
		return nil, "", cacheLacks, rootErr
	}
	want, recorded := b.sums[s.tree]
	if !recorded {
		return nil, "", cacheLacks, fmt.Errorf("frontend: no go.sum in the workspace records a hash of %s: "+
			"run go mod download %s", s.tree, s.tree.Path)
	}
	if got := strings.TrimSpace(string(hash)); got != want {
		return nil, "", cacheLacks, fmt.Errorf("frontend: the module cache hashes %s as %s, and go.sum records %s",
			s.tree, got, want)
	}
	dir := root
	if rel != "" {
		dir += "/" + rel
	}
	unit, reason, err := packageUnit(r, dir, []string{s.goMod})
	return unit, reason, cacheHas, err
}

// fromVendor returns the unit of a package directory from the vendor tree
// of the first workspace module whose vendor/modules.txt lists the
// selected module version. It returns the reason where that tree has no
// unit for the directory, and reports whether a list names the version.
// The list first passes the consistency checks of the go command against
// its go.mod, and a mismatch is an error that names it.
func (b *buildList) fromVendor(
	r plugin.StoreReader, mod module.Version, importPath string,
) ([]plugin.SourceRef, string, bool, error) {
	for _, goMod := range b.goMods {
		dir := path.Join(path.Dir(goMod), vendorDir)
		data, err := r.Read(path.Join(dir, vendorModules))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, "", false, err
		}
		list := parseVendor(data)
		if !slices.Contains(list.provided, mod) {
			continue
		}
		if mismatch := checkVendor(b.files[goMod], list); mismatch != nil {
			return nil, "", false, fmt.Errorf("frontend: inconsistent vendoring in %s: %w", dir, mismatch)
		}
		unit, reason, err := packageUnit(r, path.Join(dir, importPath), []string{goMod})
		return unit, reason, true, err
	}
	return nil, "", false, nil
}

// packageUnit returns the members of one package directory in name order:
// its .go files without its tests and without the files that the go
// command ignores. Each member declares the shared inputs. For a directory
// without a member, packageUnit returns the reason instead. The directory
// does not exist, has no Go file outside its tests, or is in a store that
// the load does not provide.
func packageUnit(r plugin.StoreReader, dir string, shared []string) ([]plugin.SourceRef, string, error) {
	entries, err := r.ReadDir(dir)
	switch {
	case errors.Is(err, plugin.ErrStoreAbsent):
		store, _, _ := plugin.CutStorePath(dir)
		return nil, "the load provides no " + store + " store", nil
	case errors.Is(err, fs.ErrNotExist):
		return nil, "no package directory is at " + dir, nil
	case err != nil:
		return nil, "", err
	}
	var unit []plugin.SourceRef
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || path.Ext(name) != golang.Extension || strings.HasSuffix(name, testSuffix) ||
			strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {

			continue
		}
		unit = append(unit, plugin.SourceRef{Path: dir + "/" + name, Shared: shared})
	}
	if len(unit) == 0 {
		return nil, dir + " has no Go file outside its tests", nil
	}
	return unit, "", nil
}
