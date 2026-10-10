// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"strings"

	"go.dokimi.dev/eidos/core/symbol"
)

// rootDir is the workspace-relative spelling of the tree's root
// directory.
const rootDir = "."

// Module is one toolchain module the load resolved, read off the
// gen.module and gen.moduleRoot facts of its packages.
type Module struct {
	// Lang is the language of the packages the facts are on.
	Lang symbol.Lang
	// Path is the module's identity: a Go module path, a Maven
	// artifact.
	Path string
	// Root is the workspace-relative directory the module is declared
	// in, "." for the tree's root.
	Root string
}

// Contains reports whether a workspace-relative directory is the
// module's root or below it. The tree's root contains every
// directory. It allocates nothing.
func (m Module) Contains(dir string) bool {
	if m.Root == rootDir {
		return true
	}
	rest, under := strings.CutPrefix(dir, m.Root)
	return under && (rest == "" || rest[0] == '/')
}

// Rel returns a directory's path relative to the module's root, and
// "." for the root itself. The directory is one [Module.Contains]
// reports true for, and any other directory returns unchanged. It
// allocates nothing, because the result shares the directory's bytes.
func (m Module) Rel(dir string) string {
	switch {
	case dir == m.Root:
		return rootDir
	case m.Root == rootDir:
		return dir
	}
	if rest, under := strings.CutPrefix(dir, m.Root); under && rest != "" && rest[0] == '/' {
		return rest[1:]
	}
	return dir
}

// Resident is one source file the load placed in a directory, with
// the package the file declares. The package's Name is the name its
// files declare it under.
type Resident struct {
	File string
	Pkg  symbol.Identity
}

// Placement is what a target reads to name the package of a routed
// file. The run derives it from the frozen graph and the fact store,
// so a target reads no source.
type Placement struct {
	// Path is the routed file, workspace-relative and slash-separated.
	Path string
	// Origin is the package of the file's first unit: the package the
	// file's declarations derive from, zero for a plan file.
	Origin symbol.Identity
	// Residents are the source files the load placed in the routed
	// file's directory, each with the package it declares, sorted by
	// file.
	Residents []Resident
	// Modules are the toolchain modules the load resolved, innermost
	// root first.
	Modules []Module
	// ImportBase is the package path of BaseDir, the plan's output
	// directory, for a directory no loaded module contains. Both are
	// empty where the plan states no import base.
	ImportBase, BaseDir string
}

// ModuleOf returns the innermost module of a language whose root
// contains a directory, and false where no module of the language
// contains it. It allocates nothing.
func (p Placement) ModuleOf(lang symbol.Lang, dir string) (Module, bool) {
	for _, m := range p.Modules {
		if m.Lang == lang && m.Contains(dir) {
			return m, true
		}
	}
	return Module{}, false
}

// Packager is a target's package half: the package a file at a routed
// path declares, which is the package the target language's own
// frontend names when it reads that file.
type Packager interface {
	// PackageAt returns the package a file declares, with Name set to
	// the name its package clause writes. It returns an error naming
	// what is missing where the target derives no package for the
	// path.
	PackageAt(p Placement) (symbol.Identity, error)
}

// PackageRule is a target's package rule in the form a backend kit
// declares it: the function [Packager.PackageAt] runs.
type PackageRule func(p Placement) (symbol.Identity, error)
