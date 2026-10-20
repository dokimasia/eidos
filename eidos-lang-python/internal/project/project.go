// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"cmp"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// File is the base name of the project file of a Python project.
const File = "pyproject.toml"

// srcDir is the source root of a project that states none, for the files
// below it.
const srcDir = "src"

// parentDir is the path element that leaves a directory. No source root of
// the workspace starts with it.
const parentDir = ".."

// backend is a build backend that the table build-system of a project
// file names.
type backend string

// The build backends whose tables state source roots.
const (
	setuptools       backend = "setuptools.build_meta"
	setuptoolsLegacy backend = "setuptools.build_meta:__legacy__"
	hatchling        backend = "hatchling.build"
	poetry           backend = "poetry.core.masonry.api"
	uvBuild          backend = "uv_build"
	pdmBackend       backend = "pdm.backend"
	maturin          backend = "maturin"
)

// The keys of a project file that state the name, the backend and the
// source roots.
const (
	keyProject      = "project"
	keyName         = "name"
	keyBuildSystem  = "build-system"
	keyBuildBackend = "build-backend"
	keyTool         = "tool"
	keySetuptools   = "setuptools"
	keyPackageDir   = "package-dir"
	keyPackages     = "packages"
	keyFind         = "find"
	keyWhere        = "where"
	keyHatch        = "hatch"
	keyBuild        = "build"
	keyTargets      = "targets"
	keyWheel        = "wheel"
	keySources      = "sources"
	keyPoetry       = "poetry"
	keyFrom         = "from"
	keyUV           = "uv"
	keyModuleRoot   = "module-root"
	keyPDM          = "pdm"
	keyMaturin      = "maturin"
	keyPythonSource = "python-source"
)

// Project is the part of one project file that places the modules of its
// project. The zero Project places no file.
type Project struct {
	// Name is the name of the distribution of the project: the name of the
	// table project, or the name of the table of poetry for a file without
	// it, and empty for a file without either.
	Name string
	// Dir is the slash path of the directory of the project file, "." at
	// the root of the workspace.
	Dir string
	// Roots are the source roots that the table of the build backend
	// states, as slash paths from the root of the workspace, the longest
	// first, and in path order among roots of one length.
	Roots []string

	// src is the root src below Dir, which places the files below it where
	// the table states no root.
	src string
}

// Read decodes the project file at the slash path p from its bytes b. It
// reads the name of the project, and the source roots from the table of
// the build backend that the table build-system names, and from the table
// of setuptools where the file names no backend. A root is relative to the
// directory of the file, and a root that leaves the workspace is left out.
// A key of another type than the backend documents states no root.
//
// Error modes: Read returns an error that wraps the error of the TOML
// decoder for bytes that are not a TOML document.
//
// # Allocation contract
//
// Read allocates the box of the document, what the decoder allocates for
// it, the list of roots and the joined path of each root, or the joined
// path of src for a table without roots.
func Read(p string, b []byte) (Project, error) {
	var doc map[string]any
	if err := toml.Unmarshal(b, &doc); err != nil {
		return Project{}, fmt.Errorf("project: %s: %w", p, err)
	}
	pr := Project{Dir: path.Dir(p)}
	pr.Name, _ = table(doc, keyProject)[keyName].(string)
	if pr.Name == "" {
		pr.Name, _ = table(doc, keyTool, keyPoetry)[keyName].(string)
	}
	for _, root := range declared(doc) {
		joined := path.Join(pr.Dir, root)
		if joined == parentDir || strings.HasPrefix(joined, parentDir+"/") || slices.Contains(pr.Roots, joined) {
			continue
		}
		pr.Roots = append(pr.Roots, joined)
	}
	if len(pr.Roots) == 0 {
		pr.src = path.Join(pr.Dir, srcDir)
	}
	slices.SortFunc(pr.Roots, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(b), len(a)), strings.Compare(a, b))
	})
	return pr, nil
}

// Root returns the source root that contains the file at the slash path
// file: the longest root of the project that contains it, and src below
// the directory of a project without roots for a file below src. It
// reports false for a file outside every such root. Root does not
// allocate.
func (p Project) Root(file string) (string, bool) {
	roots := p.Roots
	if len(roots) == 0 && p.src != "" {
		roots = []string{p.src}
	}
	for _, root := range roots {
		if root == "." || len(file) > len(root) && file[len(root)] == '/' && strings.HasPrefix(file, root) {
			return root, true
		}
	}
	return "", false
}

// declared returns the source roots that the table of the build backend
// of a document states, relative to the directory of the project file.
func declared(doc map[string]any) []string {
	name, _ := table(doc, keyBuildSystem)[keyBuildBackend].(string)
	tool := table(doc, keyTool)
	switch backend(name) {
	case setuptools, setuptoolsLegacy, "":
		var roots []string
		if dir, is := table(tool, keySetuptools, keyPackageDir)[""].(string); is {
			roots = append(roots, dir)
		}
		return append(roots, texts(table(tool, keySetuptools, keyPackages, keyFind)[keyWhere])...)
	case hatchling:
		wheel := table(tool, keyHatch, keyBuild, keyTargets, keyWheel)
		roots := texts(wheel[keySources])
		if rewrites, is := wheel[keySources].(map[string]any); is {
			for prefix, to := range rewrites {
				if to == "" {
					roots = append(roots, prefix)
				}
			}
		}
		for _, pkg := range texts(wheel[keyPackages]) {
			roots = append(roots, path.Dir(pkg))
		}
		return roots
	case poetry:
		packages, _ := table(tool, keyPoetry)[keyPackages].([]any)
		roots := make([]string, 0, len(packages))
		for _, entry := range packages {
			spec, _ := entry.(map[string]any)
			from, _ := spec[keyFrom].(string)
			roots = append(roots, from)
		}
		return roots
	case uvBuild:
		return single(table(tool, keyUV, keyBuildBackend)[keyModuleRoot])
	case pdmBackend:
		return single(table(tool, keyPDM, keyBuild)[keyPackageDir])
	case maturin:
		return single(table(tool, keyMaturin)[keyPythonSource])
	default:
		return nil
	}
}

// table returns the table at the keys below t, and nil where a key is
// absent or names a value that is not a table.
func table(t map[string]any, keys ...string) map[string]any {
	for _, k := range keys {
		t, _ = t[k].(map[string]any)
	}
	return t
}

// texts returns the strings of a TOML array in their order, and nil for a
// value that is not an array. An entry that is not a string is left out.
func texts(v any) []string {
	list, _ := v.([]any)
	var out []string
	for _, entry := range list {
		if s, is := entry.(string); is {
			out = append(out, s)
		}
	}
	return out
}

// single returns the one root of a value that is a string, and nil for any
// other value. The empty string is the directory of the project file.
func single(v any) []string {
	root, is := v.(string)
	if !is {
		return nil
	}
	return []string{root}
}
