// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest

import (
	"strconv"
	"strings"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// Lang is the source language every fixture is written in. A case
// needing two languages spells the second one itself.
const Lang symbol.Lang = "golang"

// The package paths the fixtures use. Two are enough for every case
// that has to tell one package from another, and using the same two
// everywhere keeps a failure message recognizable.
const (
	StorePath = "svc/store"
	CachePath = "svc/cache"
)

// UnitFile is the one file [Package] puts its declarations in.
const UnitFile = "unit.go"

// PackageID returns the identity a package of that path returns.
func PackageID(path string) symbol.Identity {
	return ID(path, "", symbol.KindPackage)
}

// FileID returns the identity of the file [Package] builds.
func FileID(path string) symbol.Identity {
	return ID(path, UnitFile, symbol.KindFile)
}

// Struct returns a struct declaration in one package, carrying the
// identity the resolution step would have assigned it.
//
// A case wanting a declaration the resolution step has not reached
// builds one itself: this fixture is always named.
func Struct(path, name string) *node.Struct {
	return &node.Struct{
		ID:   ID(path, name, symbol.KindStruct),
		Name: name,
	}
}

// Package returns a package holding decls in one file, as a frontend
// would hand it over.
func Package(path string, decls ...symbol.Symbol) *node.Package {
	return &node.Package{
		ID:   PackageID(path),
		Path: strings.Split(path, "/"),
		Files: []*node.File{{
			ID:    FileID(path),
			Path:  UnitFile,
			Decls: node.Symbols(decls),
		}},
	}
}

// Workspace returns a loaded fixture at scale: packages of files of
// decls, each count set by the caller. A benchmark loads it to
// measure a cost that grows with the graph.
//
// The names are positional rather than meaningful, because nothing
// reading a workspace of this size reads one declaration by name.
// The shape is a real one: a workspace is counted in files, and a
// package holds many.
func Workspace(packages, files, decls int) []*node.Package {
	out := make([]*node.Package, 0, packages)
	for pkg := range packages {
		path := StorePath + "/" + strconv.Itoa(pkg)
		held := make([]*node.File, 0, files)
		for file := range files {
			name := "unit" + strconv.Itoa(file) + ".go"
			decl := make(node.Symbols, 0, decls)
			for i := range decls {
				decl = append(decl, Struct(path, "Decl"+strconv.Itoa(file)+"_"+strconv.Itoa(i)))
			}
			held = append(held, &node.File{
				ID: symbol.Identity{
					Lang: Lang, Package: path, Name: name, Kind: symbol.KindFile,
				},
				Path:  name,
				Decls: decl,
			})
		}
		out = append(out, &node.Package{
			ID:    PackageID(path),
			Path:  strings.Split(path, "/"),
			Name:  strconv.Itoa(pkg),
			Files: held,
		})
	}
	return out
}

// Names returns the declared names of what a traversal yielded,
// which is what a case compares against.
func Names(tb assert.TB, decls []symbol.Symbol) []string {
	tb.Helper()

	out := make([]string, 0, len(decls))
	for _, decl := range decls {
		named, names := decl.(node.Declaration)
		assert.True(tb, names,
			"everything the traversal yielded names a declaration")
		if !names {
			continue
		}
		out = append(out, named.Identity().Name)
	}
	return out
}
