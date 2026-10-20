// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The stub of the module of the cases, a class that it declares, and a
// project file that does not decode.
const (
	appStub    = "shop/app.pyi"
	classSrc   = "class Order:\n    id: int\n"
	badProject = "[project\n"
)

// The canonical scale every layer benches at: 1000 packages of 10 files
// of 20 declarations, 200k declarations in all. A Python unit is one
// module, so the corpus is 10,000 units.
const (
	benchPackages = 1000
	benchFiles    = 10
	benchDecls    = 20
)

// parseAllocs is the budget of one parse of the canonical corpus of
// 200,000 declarations in 10,000 modules. The budget is about 5% above the
// 1,710,047 allocations of one run. tree-sitter builds its trees in C
// memory, which the count does not see.
const parseAllocs = 1_800_000

// TestParse pins how a unit enters the graph: its package, its pair of a
// source and a stub, its module and the places of its dependencies.
func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("declares the declarations of a module into the package that its path names", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, classSrc)
			packages := gb.Packages()
			assert.Length(t, packages, 1, "the unit declares one package")
			assert.Equal(t, packages[0].ID.Package, appPkg, "the package of shop/app.py")
			assert.Equal(t, packages[0].Name, "app", "the package is named after its last segment")
		})

		t.Run("lowers the stub of a pair of a source and a stub", func(t *testing.T) {
			t.Parallel()

			gbs, _ := parsedTree(t, fstest.MapFS{
				projectFile: {Data: []byte(rootProject)},
				appFile:     {Data: []byte("class Source:\n    pass\n")},
				appStub:     {Data: []byte(classSrc)},
			}, plugin.DepthFull)
			assert.Length(t, gbs, 1, "a source and its stub are one unit")
			files := gbs[0].Packages()[0].Files
			assert.Length(t, files, 2, "the package has a File for each member")
			assert.Equal(t, declNames(files[1]), []string{"Order"}, "the stub declares the module")
			assert.Empty(t, files[0].Decls, "the source declares nothing")
		})

		t.Run("stamps python.stubbed on the source of a pair", func(t *testing.T) {
			t.Parallel()

			gbs, _ := parsedTree(t, fstest.MapFS{
				projectFile: {Data: []byte(rootProject)}, appFile: {Data: []byte("x = 1\n")},
				appStub: {Data: []byte("x: int\n")},
			}, plugin.DepthFull)
			source := gbs[0].Packages()[0].Files[0]
			assert.Equal(t, stampsOn(gbs[0], source, python.StubbedKey), []any{true}, "a type checker reads the stub")
		})

		t.Run("stamps the name of the project as the module of the package", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, classSrc)
			pkg := gb.Packages()[0]
			assert.Equal(t, stampsOn(gb, pkg, meta.ModuleKey), []any{"shop"}, "the name of the project")
			assert.Equal(t, stampsOn(gb, pkg, meta.ModuleRootKey), []any{"."}, "the directory of the project file")
		})

		t.Run("stamps no module for a module outside every project", func(t *testing.T) {
			t.Parallel()

			gbs, _ := parsedTree(t, fstest.MapFS{appFile: {Data: []byte(classSrc)}}, plugin.DepthFull)
			assert.Empty(t, stampsOn(gbs[0], gbs[0].Packages()[0], meta.ModuleKey), "a module outside a project")
		})

		t.Run("places a module outside every project below the run of its package directories", func(t *testing.T) {
			t.Parallel()

			gbs, _ := parsedTree(t, fstest.MapFS{
				"lib/shop/__init__.py": {Data: []byte("\n")}, "lib/shop/models.py": {Data: []byte(classSrc)},
			}, plugin.DepthFull)
			paths := make([]string, 0, len(gbs))
			for _, gb := range gbs {
				paths = append(paths, gb.Packages()[0].ID.Package)
			}
			assert.Equal(t, paths, []string{"shop", "shop/models"}, "lib is above the package directory shop")
		})

		t.Run("reports UnreadProject for a project file that does not decode", func(t *testing.T) {
			t.Parallel()

			gbs, sink := parsedTree(t, fstest.MapFS{
				projectFile: {Data: []byte(badProject)}, "shop/__init__.py": {Data: []byte("\n")},
				appFile: {Data: []byte(classSrc)},
			}, plugin.DepthFull)
			assert.Contains(t, codesOf(sink), frontend.UnreadProject, "the fault of the project file reports")
			assert.Equal(t, gbs[1].Packages()[0].ID.Package, appPkg, "the module takes the root of its run")
		})

		tests := []struct {
			name   string
			give   string
			stores map[string]fs.FS
			want   string
		}{
			{
				name: "places a module of the typeshed store below its stdlib directory",
				give: "typeshed://stdlib/os/path.pyi",
				want: "os/path",
				stores: map[string]fs.FS{
					frontend.TypeshedStore: fstest.MapFS{"stdlib/os/path.pyi": {Data: []byte("\n")}},
				},
			},
			{
				name: "places the module of a package directory of the typeshed store in the package",
				give: "typeshed://stdlib/os/__init__.pyi", want: "os",
				stores: map[string]fs.FS{
					frontend.TypeshedStore: fstest.MapFS{"stdlib/os/__init__.pyi": {Data: []byte("\n")}},
				},
			},
			{
				name: "places a module of the venv store below its root", give: "venv://requests/api.py",
				want:   "requests/api",
				stores: map[string]fs.FS{frontend.VenvStore: fstest.MapFS{"requests/api.py": {Data: []byte("\n")}}},
			},
			{
				name: "places a module of a package of stubs alone in the package of its name",
				give: "venv://requests-stubs/api.pyi", want: "requests/api",
				stores: map[string]fs.FS{
					frontend.VenvStore: fstest.MapFS{"requests-stubs/api.pyi": {Data: []byte("\n")}},
				},
			},
			{
				name: "places a module of a site-packages directory of the workspace below the directory",
				give: ".venv/lib/python3.12/site-packages/requests/api.py", want: "requests/api",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				f := frontend.New(nil)
				tree := storeTree{
					MapFS:  fstest.MapFS{".venv/lib/python3.12/site-packages/requests/api.py": {Data: []byte("\n")}},
					stores: tt.stores,
				}
				u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: tt.give}}, tree, plugin.DepthSignatures,
					f.Syntax(), brand, diag.NewSink(), f.Name())
				assert.NoError(t, f.Parse(t.Context(), u), "the dependency parses")
				assert.Equal(t, u.Graph().Packages()[0].ID.Package, tt.want, "the package of the module")
			})
		}

		t.Run("returns the error of the context for a done context", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(nil)
			tree := fstest.MapFS{appFile: {Data: []byte(classSrc)}}
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: appFile}}, tree, plugin.DepthFull, f.Syntax(), brand,
				diag.NewSink(), f.Name())
			assert.HonoursCancellation(t, func(ctx context.Context) error { return f.Parse(ctx, u) },
				"a cancelled load parses nothing")
		})

		t.Run("returns the error of the read for a member outside the tree", func(t *testing.T) {
			t.Parallel()

			f := frontend.New(nil)
			u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: appFile}}, fstest.MapFS{}, plugin.DepthFull, f.Syntax(),
				brand, diag.NewSink(), f.Name())
			assert.HasError(t, f.Parse(t.Context(), u), "a member that does not read fails the unit")
		})

		t.Run("returns the error of the read of a package probe that is a directory", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{
				"lib/shop/__init__.pyi/README": {Data: []byte("\n")}, "lib/shop/models.py": {Data: []byte(classSrc)},
			}
			f := frontend.New(nil)
			units, err := f.Partition(t.Context(), claimedIn(tree), treeReader{tree})
			assert.NoError(t, err, "the tree partitions")
			assert.Length(t, units, 1, "the module is one unit")
			u := plugin.NewSourceUnit(units[0], tree, plugin.DepthFull, f.Syntax(), brand, diag.NewSink(), f.Name())
			assert.ErrorIs(t, f.Parse(t.Context(), u), fs.ErrInvalid, "a probe that does not read fails the unit")
		})
	})
}

// BenchmarkParse drives the parse at the canonical scale: every unit of the
// scaled corpus through Parse into a builder of its own, the partition run
// once before the loop, so the number measures the tree-sitter parse and
// the lowering alone, and fails above parseAllocs. Only -bench checks the
// ceiling, because the 101 calls of an allocation check would take minutes.
func BenchmarkParse(b *testing.B) {
	tree := scaledPython()
	f := frontend.New(nil)
	units, err := f.Partition(b.Context(), claimedIn(tree), treeReader{tree})
	assert.NoError(b, err, "the corpus partitions")
	sink := diag.NewSink()
	c := bench.Start(b).MaxAllocs(parseAllocs)
	defer c.End()
	for c.Loop() {
		for _, unit := range units {
			u := plugin.NewSourceUnit(unit, tree, plugin.DepthFull, f.Syntax(), brand, sink, f.Name())
			err = cmp.Or(err, f.Parse(b.Context(), u))
		}
	}
	assert.NoError(b, err, "the corpus parses")
}

// scaledPython returns the canonical corpus as Python modules below a
// project file whose module root is the root of the workspace: a directory
// for each package of modules of four classes, whose first field is a
// class of the package before it, and sixteen constants.
func scaledPython() fstest.MapFS {
	tree := fstest.MapFS{projectFile: {Data: []byte(rootProject)}}
	for p := range benchPackages {
		prev := (p + benchPackages - 1) % benchPackages
		for f := range benchFiles {
			var b strings.Builder
			fmt.Fprintf(&b, "from typing import Final\n\nfrom p%d.f%d import T%d_0 as Prev\n\n\n", prev, f, f)
			for d := range benchDecls {
				if d < 4 {
					fmt.Fprintf(&b, "class T%d_%d:\n    f0: Prev\n    f1: int\n\n\n", f, d)
				} else {
					fmt.Fprintf(&b, "C%d_%d: Final = %d\n", f, d, d)
				}
			}
			tree[fmt.Sprintf("p%d/f%d.py", p, f)] = &fstest.MapFile{Data: []byte(b.String())}
		}
	}
	return tree
}
