// Copyright ThesmOS B.V. 2026
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

// The records the module cache keeps beside a module's download: the
// hash the go command verified the tree with, and the mark of a
// download it has not finished.
const (
	cacheDownload = "cache/download"
	zipHashSuffix = ".ziphash"
	partialSuffix = ".partial"
)

// versionMark separates a module's escaped path from its escaped
// version in the name of its module cache directory.
const versionMark = "@"

// testSuffix ends the name of a test file, which a dependency unit
// leaves out.
const testSuffix = "_" + testElement + golang.Extension

// cacheState is what the module cache provides for one module.
type cacheState uint8

// The module cache has the module's tree, lacks it or an unfinished
// download of it, or is a store the load does not provide.
const (
	cacheHas   cacheState = 1
	cacheLacks cacheState = 2
	cacheOff   cacheState = 3
)

// dependencies is the Go frontend's dependency round. It reads the
// build list from the round's go.mod files and places each need: in the
// standard library, in the module cache at the selected version, or in
// the vendor tree of a workspace module when the cache lacks that
// version. Each placed need is one unit, the package directory's .go
// files without its tests.
//
// A need the build places nowhere yields no unit, and the round reports
// it with the reason: an import of a workspace module, whose packages
// load from the selection alone, an import no selected module provides,
// which the go command refuses as provided by no required module, a
// package directory a module or the standard library lacks, and a
// store the load does not provide, which turns its source off. The
// import "C" names cgo's preamble and no package, so the round passes
// over it. A module the go.sum files record no hash for, a module whose
// cached hash differs from go.sum's, a directory replacement outside
// the workspace, an inconsistent vendor tree, and a module neither the
// cache nor a vendor tree provides are errors that name the go command
// that settles them.
func (*goFrontend) dependencies(
	ctx context.Context, round *plugin.DependencyRound, r plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	b, err := readBuild(r, round.Shared)
	if err != nil {
		return nil, err
	}
	var out [][]plugin.SourceRef
	for _, need := range round.Needs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if need.Path == cgoImport {
			continue
		}
		unit, reason, err := b.place(r, need.Path)
		if err != nil {
			return nil, err
		}
		if len(unit) == 0 {
			round.Unplace(need.Path, reason)
			continue
		}
		out = append(out, unit)
	}
	return out, nil
}

// place returns the unit that declares one import path, and for a need
// the build places nowhere no unit and the reason.
func (b *buildList) place(r plugin.StoreReader, importPath string) ([]plugin.SourceRef, string, error) {
	if standard(importPath) {
		return packageUnit(r, plugin.StorePath(GoRootStore, importPath), nil)
	}
	if b.workspaceModule(importPath) {
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

// fromCache returns the unit of a package directory from the selected
// module's tree in the module cache, the reason where the module's tree
// has no unit for the directory, and what the cache provides for the
// module. The cache has the module when its tree and its hash record
// exist and no mark of an unfinished download does, which is the go
// command's rule. The hash record's text is then the hash go.sum
// records for the tree, or the round fails.
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
	if _, partialErr := r.Read(download + partialSuffix); !absent(partialErr) {
		return nil, "", cacheLacks, partialErr
	}
	root := plugin.StorePath(ModCacheStore, escPath+versionMark+escVersion)
	if _, rootErr := r.ReadDir(root); rootErr != nil {
		if absent(rootErr) {
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

// fromVendor returns the unit of a package directory from the vendor
// tree of the first workspace module whose vendor/modules.txt lists the
// selected module version, the reason where that tree has no unit for
// the directory, and reports whether one lists it. The list passes the
// go command's consistency checks against its go.mod first, and a
// mismatch is an error that names it.
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

// packageUnit returns one package directory's members, its .go files
// in name order without its tests and without the files the go
// command ignores, each declaring the shared inputs. A directory
// without a member returns the reason instead: the directory does not
// exist, it has no Go file outside its tests, or it is in a store the
// load does not provide.
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

// absent reports whether a read or a listing found nothing to read:
// the file does not exist, or the load provides no store for it.
func absent(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, plugin.ErrStoreAbsent)
}

// standard reports whether an import path is in the standard library:
// its first element has no dot, which is the go command's rule.
func standard(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}
