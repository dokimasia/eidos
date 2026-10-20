// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest_test

import (
	"errors"
	"path"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The fixture packages the routing cases place units in.
const (
	leftPkg  = "svc/left"
	rightPkg = "svc/right"
)

// The source file of a per-source unit whose package path is not its
// directory, the package of the unit, the path of its routed file, and a
// directory in which the packager derives no package.
const (
	sourceKey  = "app/catalog.src"
	sourcePkg  = "catalog"
	sourcePath = "app/stub.txt"
	refusedDir = "refused"
)

// errNoPackage is the error of a placement in refusedDir.
var errNoPackage = errors.New("backendtest_test: the directory has no package")

// wordSpeller names every unit after its word, and splits a unit
// whose word is split into one part per declaration.
type wordSpeller struct{}

// SplitUnit returns one part per declaration of a split unit, and the
// unit whole otherwise.
func (wordSpeller) SplitUnit(u plugin.Unit) []plugin.Unit {
	if u.Word != "split" {
		return []plugin.Unit{u}
	}
	parts := make([]plugin.Unit, 0, len(u.Decls))
	for _, d := range u.Decls {
		part := u
		part.Decls = []symbol.Symbol{d}
		part.Word = emit.DeclaredName(d)
		parts = append(parts, part)
	}
	return parts
}

// FileName returns the unit's word.
func (wordSpeller) FileName(u plugin.Unit) string { return u.Word + ".txt" }

// packagingSpeller is a [wordSpeller] with the package half of a backend.
// It records the residents of each placement under its path.
type packagingSpeller struct {
	wordSpeller

	residents map[string][]plugin.Resident
}

// PackageAt returns a package named after the path of the placement, in
// the language of its origin, and errNoPackage for a path in refusedDir.
func (s packagingSpeller) PackageAt(p plugin.Placement) (symbol.Identity, error) {
	s.residents[p.Path] = p.Residents
	if path.Dir(p.Path) == refusedDir {
		return symbol.Identity{}, errNoPackage
	}
	return symbol.Identity{Lang: p.Origin.Lang, Package: p.Path, Kind: symbol.KindPackage}, nil
}

// TestFiles checks that a hand-built store routes into files the way a
// plan routes a store whose packages are at their package paths. The
// suite of a backend then renders the files that a plan hands it.
func TestFiles(t *testing.T) {
	t.Parallel()

	t.Run("Files", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give []plugin.Unit
			want []string
		}{
			{
				name: "returns one file per spelled name under each package's path in path order",
				give: []plugin.Unit{
					routable("gen", rightPkg, "stub", "Row"),
					routable("gen", leftPkg, "stub", "Row"),
				},
				want: []string{leftPkg + "/stub.txt: gen", rightPkg + "/stub.txt: gen"},
			},
			{
				name: "returns the units that spell one name under one package as one file in store order",
				give: []plugin.Unit{
					routable("weaver", leftPkg, "stub", "Row"),
					routable("gen", leftPkg, "stub", "Store"),
				},
				want: []string{leftPkg + "/stub.txt: gen weaver"},
			},
			{
				name: "returns a plan unit's file under its name alone",
				give: []plugin.Unit{{
					Plugin: "gen", Per: plugin.PerPlan, Word: "registry",
					Decls: []symbol.Symbol{&emit.Struct{Name: "Registry"}},
				}},
				want: []string{"registry.txt: gen"},
			},
			{
				name: "returns one file per part the speller splits a unit into",
				give: []plugin.Unit{routable("gen", leftPkg, "split", "Row", "Store")},
				want: []string{leftPkg + "/Row.txt: gen", leftPkg + "/Store.txt: gen"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				e := plugin.NewEmit()
				assert.Total(t, e.Add, tt.give, "the fixture unit arrives")
				assert.Equal(t, paths(backendtest.Files(e, wordSpeller{})), tt.want, "the routed files")
			})
		}

		t.Run("returns the files of two languages at one path in package order", func(t *testing.T) {
			t.Parallel()

			native := routable("gen", leftPkg, "stub", "Row")
			foreign := routable("gen", leftPkg, "stub", "Row")
			foreign.Pkg.Lang = "other"
			e := plugin.NewEmit()
			assert.NoError(t, e.Add(foreign), "the second language's unit arrives")
			assert.NoError(t, e.Add(native), "the first language's unit arrives")
			files := backendtest.Files(e, wordSpeller{})
			assert.Length(t, files, 2, "two packages spell one path apart")
			assert.Equal(t, files[0].Pkg, native.Pkg, "the package that sorts first")
			assert.Equal(t, files[1].Pkg, foreign.Pkg, "the package that sorts second")
		})

		t.Run("returns a per-source file in the directory of its source", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, e.Add(sourced(sourceKey)), "the unit arrives")
			assert.Equal(t, paths(backendtest.Files(e, wordSpeller{})), []string{sourcePath + ": gen"},
				"the file is beside the source file of its unit, as a plan routes it")
		})

		t.Run("returns the package that the packager of the speller derives for the path", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, e.Add(sourced(sourceKey)), "the unit arrives")
			files := backendtest.Files(e, packagingSpeller{residents: map[string][]plugin.Resident{}})
			assert.Length(t, files, 1, "the unit routes into one file")
			assert.Equal(t, files[0].Pkg, symbol.Identity{
				Lang: coretest.PackageID(sourcePkg).Lang, Package: sourcePath, Kind: symbol.KindPackage,
			}, "the file declares the package of its path")
		})

		t.Run("passes the source files of the directory to the packager as residents", func(t *testing.T) {
			t.Parallel()

			unit := sourced(sourceKey)
			e := plugin.NewEmit()
			assert.NoError(t, e.Add(unit), "the unit arrives")
			s := packagingSpeller{residents: map[string][]plugin.Resident{}}
			backendtest.Files(e, s)
			assert.Equal(t, s.residents[sourcePath], []plugin.Resident{{File: sourceKey, Pkg: unit.Pkg}},
				"the source file of the unit is a resident of the directory")
		})

		t.Run("returns the package of the first unit where the packager derives none", func(t *testing.T) {
			t.Parallel()

			unit := sourced(refusedDir + "/catalog.src")
			e := plugin.NewEmit()
			assert.NoError(t, e.Add(unit), "the unit arrives")
			files := backendtest.Files(e, packagingSpeller{residents: map[string][]plugin.Resident{}})
			assert.Length(t, files, 1, "the unit routes into one file")
			assert.Equal(t, files[0].Pkg, unit.Pkg, "the file keeps the package of its unit")
		})

		t.Run("returns the header that the emit.File declarations of a file state", func(t *testing.T) {
			t.Parallel()

			header := symbol.Annotations{{Name: "protobuf:syntax", Args: []string{"proto3"}}}
			u := routable("gen", leftPkg, "stub", "Row")
			u.Decls = append(u.Decls, &emit.File{Annotations: header})
			e := plugin.NewEmit()
			assert.NoError(t, e.Add(u), "the unit arrives")
			files := backendtest.Files(e, wordSpeller{})
			assert.Length(t, files, 1, "the unit routes into one file")
			assert.Equal(t, files[0].Header, emit.File{Annotations: header}, "the file has the header of its unit")
		})
	})
}

// routable returns one unit of a plugin under a package, its
// declarations structs of the given names.
func routable(p plugin.ID, pkg, word string, names ...string) plugin.Unit {
	u := plugin.Unit{Plugin: p, Per: plugin.PerPackage, Word: word, Key: pkg, Pkg: coretest.PackageID(pkg)}
	for _, n := range names {
		u.Decls = append(u.Decls, &emit.Struct{Origin: coretest.Struct(pkg, n).ID, Name: n})
	}
	return u
}

// sourced returns a per-source unit of the plugin gen for the source file
// key, in the package sourcePkg, with the struct Row.
func sourced(key string) plugin.Unit {
	return plugin.Unit{
		Plugin: "gen", Per: plugin.PerSource, Word: "stub", Key: key, Pkg: coretest.PackageID(sourcePkg),
		Decls: []symbol.Symbol{&emit.Struct{Origin: coretest.Struct(sourcePkg, "Row").ID, Name: "Row"}},
	}
}

// paths returns each file's path and the plugins of its units.
func paths(files []plugin.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		line := f.Path + ":"
		var lineSb133 strings.Builder
		for _, u := range f.Units {
			lineSb133.WriteString(" " + string(u.Plugin))
		}
		line += lineSb133.String()
		out = append(out, line)
	}
	return out
}
