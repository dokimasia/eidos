// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest_test

import (
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

// routable returns one unit of a plugin under a package, its
// declarations structs of the given names.
func routable(p plugin.ID, pkg, word string, names ...string) plugin.Unit {
	u := plugin.Unit{Plugin: p, Per: plugin.PerPackage, Word: word, Key: pkg, Pkg: coretest.PackageID(pkg)}
	for _, n := range names {
		u.Decls = append(u.Decls, &emit.Struct{Origin: coretest.Struct(pkg, n).ID, Name: n})
	}
	return u
}

// paths returns each file's path and the plugins of its units.
func paths(files []plugin.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		line := f.Path + ":"
		for _, u := range f.Units {
			line += " " + string(u.Plugin)
		}
		out = append(out, line)
	}
	return out
}

// A hand-built store routes into files the way a plan routes a store
// whose packages are at their package paths, so a backend's suite
// renders what a plan would hand it.
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
				for _, u := range tt.give {
					assert.NoError(t, e.Add(u), "the fixture unit arrives")
				}
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
	})
}
